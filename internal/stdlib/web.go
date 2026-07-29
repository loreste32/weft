//go:build !js

package stdlib

import (
	"fmt"
	"html/template"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageWeb is multi-route HTTP apps for Weft (stdlib, pure Go), including HTMX helpers.
func packageWeb(env *runtime.Env) runtime.Value {
	p := pkg()
	// web.app() -> App
	set(p, "app", func(args []runtime.Value) (runtime.Value, error) {
		return newWebApp(env), nil
	}, 0)
	// response helpers (also available as web.json / web.html / …)
	set(p, "text", func(args []runtime.Value) (runtime.Value, error) {
		return httpText(args)
	}, 2)
	set(p, "json", func(args []runtime.Value) (runtime.Value, error) {
		return httpJSON(args)
	}, 2)
	set(p, "html", func(args []runtime.Value) (runtime.Value, error) {
		status := int64(200)
		body := ""
		if len(args) == 1 {
			body = args[0].String()
		} else if len(args) >= 2 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				status = n
				body = args[1].String()
			} else {
				body = args[0].String()
			}
		}
		return respMap(status, body, "text/html; charset=utf-8"), nil
	}, 2)
	set(p, "redirect", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("web.redirect(url)", "web"), nil
		}
		code := int64(302)
		loc := args[0].String()
		if len(args) >= 2 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				code = n
				loc = args[1].String()
			}
		}
		m := respMap(code, "", "text/plain")
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = append(mo.Keys, "headers")
		h := runtime.NewMap()
		ho := h.Obj.(*runtime.MapObj)
		ho.Keys = append(ho.Keys, "Location")
		ho.Vals["Location"] = runtime.Str(loc)
		mo.Vals["headers"] = h
		return m, nil
	}, 2)
	set(p, "status", func(args []runtime.Value) (runtime.Value, error) {
		code := int64(200)
		if len(args) >= 1 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				code = n
			}
		}
		body := ""
		if len(args) >= 2 {
			body = args[1].String()
		}
		return respMap(code, body, "text/plain; charset=utf-8"), nil
	}, 2)
	// web.sse(source) -> streaming text/event-stream response.
	// source: list or iter of str | map{data,event?} | llm stream event {kind,text}.
	// writeWeftResponse flushes each event as it is pulled — no full-body buffer.
	set(p, "sse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("web.sse(list|iter)", "web"), nil
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"status", "type", "stream", "sse"}
		mo.Vals["status"] = runtime.Int(200)
		mo.Vals["type"] = runtime.Str("text/event-stream; charset=utf-8")
		mo.Vals["stream"] = args[0]
		mo.Vals["sse"] = runtime.Bool(true)
		return m, nil
	}, 1)

	// web.sse_channel() -> {send(data), close(), response}
	// Push-based SSE: handler returns response, then calls send() to push events.
	set(p, "sse_channel", func(args []runtime.Value) (runtime.Value, error) {
		ch := make(chan string, 64)
		done := make(chan struct{})

		// Iterator that pulls from channel
		iter := &chanSSEIter{ch: ch, done: done}

		// Build response with the iterator as stream source
		resp := runtime.NewMap()
		rmo := resp.Obj.(*runtime.MapObj)
		rmo.Keys = []string{"status", "type", "stream", "sse"}
		rmo.Vals["status"] = runtime.Int(200)
		rmo.Vals["type"] = runtime.Str("text/event-stream; charset=utf-8")
		rmo.Vals["stream"] = runtime.MakeIter(iter)
		rmo.Vals["sse"] = runtime.Bool(true)

		// Build control handle
		handle := runtime.NewMap()
		hmo := handle.Obj.(*runtime.MapObj)
		hmo.Keys = []string{"send", "close", "response"}
		hmo.Vals["send"] = runtime.MakeBuiltin("sse.send", 1, func(args []runtime.Value) (runtime.Value, error) {
			if len(args) < 1 {
				return runtime.Unit(), nil
			}
			msg := args[0].String()
			select {
			case ch <- msg:
			case <-done:
			}
			return runtime.Unit(), nil
		})
		hmo.Vals["close"] = runtime.MakeBuiltin("sse.close", 0, func(args []runtime.Value) (runtime.Value, error) {
			select {
			case <-done:
			default:
				close(done)
			}
			return runtime.Unit(), nil
		})
		hmo.Vals["response"] = resp

		return handle, nil
	}, 0)

	// --- HTMX ---------------------------------------------------------------
	// web.is_htmx(req) -> bool  (HX-Request: true)
	set(p, "is_htmx", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(reqIsHTMX(args[0])), nil
	}, 1)
	// web.htmx(html|opts, opts?) -> HTML fragment response with HX-* headers.
	// opts: trigger, trigger_after_settle, trigger_after_swap, redirect, refresh,
	//       location, push_url, replace_url, retarget, reswap, reselect, status
	set(p, "htmx", func(args []runtime.Value) (runtime.Value, error) {
		return htmxResponse(args)
	}, 2)
	// web.htmx_redirect(url) -> empty body + HX-Redirect (client-side navigation)
	set(p, "htmx_redirect", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("web.htmx_redirect(url)", "web"), nil
		}
		return htmxResponse([]runtime.Value{
			runtime.Str(""),
			mapStrPairs("redirect", args[0].String()),
		})
	}, 1)
	// web.htmx_refresh() -> HX-Refresh: true
	set(p, "htmx_refresh", func(args []runtime.Value) (runtime.Value, error) {
		return htmxResponse([]runtime.Value{
			runtime.Str(""),
			mapStrPairs("refresh", "true"),
		})
	}, 0)
	// web.htmx_trigger(event|map) -> HX-Trigger header (body empty unless second arg HTML)
	set(p, "htmx_trigger", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("web.htmx_trigger(event|map, html?)", "web"), nil
		}
		body := ""
		if len(args) >= 2 {
			body = args[1].String()
		}
		opts := runtime.NewMap()
		omo := opts.Obj.(*runtime.MapObj)
		omo.Keys = []string{"trigger"}
		omo.Vals["trigger"] = args[0]
		return htmxResponse([]runtime.Value{runtime.Str(body), opts})
	}, 2)
	// web.htmx_location(url|opts) -> HX-Location (client-side soft nav)
	set(p, "htmx_location", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("web.htmx_location(url|opts)", "web"), nil
		}
		opts := runtime.NewMap()
		omo := opts.Obj.(*runtime.MapObj)
		omo.Keys = []string{"location"}
		omo.Vals["location"] = args[0]
		return htmxResponse([]runtime.Value{runtime.Str(""), opts})
	}, 1)
	// web.htmx_cdn() -> script tag for unpkg htmx (convenience)
	set(p, "htmx_cdn", func(args []runtime.Value) (runtime.Value, error) {
		ver := "2.0.4"
		if len(args) >= 1 && args[0].String() != "" {
			ver = args[0].String()
		}
		return runtime.Str(`<script src="https://unpkg.com/htmx.org@` + ver + `"></script>`), nil
	}, 1)

	// web.form(req) -> map  (same as req.form; empty map if missing)
	set(p, "form", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.NewMap(), nil
		}
		if f, ok := mapGet(args[0], "form"); ok && f.Kind == runtime.KindMap {
			return f, nil
		}
		// rebuild from body + headers if present
		body := mapGetStr(args[0], "body", "")
		ctype := ""
		if h, ok := mapGet(args[0], "headers"); ok {
			ctype = headerGetCI(h, "Content-Type")
		}
		form, _, _ := parseFormParts(body, ctype, nil)
		return form, nil
	}, 1)
	// web.form_get(req, key, default?) -> str
	set(p, "form_get", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Str(""), nil
		}
		def := ""
		if len(args) >= 3 {
			def = args[2].String()
		}
		form, _ := mapGet(args[0], "form")
		if form.Kind != runtime.KindMap {
			body := mapGetStr(args[0], "body", "")
			ctype := ""
			if h, ok := mapGet(args[0], "headers"); ok {
				ctype = headerGetCI(h, "Content-Type")
			}
			form, _, _ = parseFormParts(body, ctype, nil)
		}
		return runtime.Str(mapGetStr(form, args[1].String(), def)), nil
	}, 3)
	// web.form_list(req, key) -> [str]  all values for multi-select / checkboxes
	set(p, "form_list", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.List(), nil
		}
		all, ok := mapGet(args[0], "form_all")
		if !ok || all.Kind != runtime.KindMap {
			body := mapGetStr(args[0], "body", "")
			ctype := ""
			if h, ok := mapGet(args[0], "headers"); ok {
				ctype = headerGetCI(h, "Content-Type")
			}
			_, all, _ = parseFormParts(body, ctype, nil)
		}
		if v, ok := mapGet(all, args[1].String()); ok {
			return v, nil
		}
		return runtime.List(), nil
	}, 2)
	// web.file(req, field) -> map|null  {filename, content_type, size, body}
	set(p, "file", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Null(), nil
		}
		files, ok := mapGet(args[0], "files")
		if !ok || files.Kind != runtime.KindMap {
			return runtime.Null(), nil
		}
		if v, ok := mapGet(files, args[1].String()); ok {
			return v, nil
		}
		return runtime.Null(), nil
	}, 2)

	// web.htmx_oob(id, html) -> HTML fragment with hx-swap-oob="true"
	set(p, "htmx_oob", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("web.htmx_oob(id, html) or web.htmx_oob(html)", "web"), nil
		}
		if len(args) == 1 {
			// raw HTML already containing oob, or wrap whole blob
			s := args[0].String()
			if strings.Contains(s, "hx-swap-oob") {
				return runtime.Str(s), nil
			}
			return runtime.Str(htmxOOBWrap("oob", s)), nil
		}
		return runtime.Str(htmxOOBWrap(args[0].String(), args[1].String())), nil
	}, 2)

	// web.cookie(name, value, opts?) -> Set-Cookie string (for response cookies list)
	set(p, "cookie", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("web.cookie(name, value, opts?)", "web"), nil
		}
		opts := runtime.Null()
		if len(args) >= 3 {
			opts = args[2]
		}
		return runtime.Str(buildSetCookie(args[0].String(), args[1].String(), opts, false)), nil
	}, 3)
	// web.clear_cookie(name, opts?) -> Set-Cookie that expires
	set(p, "clear_cookie", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("web.clear_cookie(name, opts?)", "web"), nil
		}
		opts := runtime.Null()
		if len(args) >= 2 {
			opts = args[1]
		}
		return runtime.Str(buildSetCookie(args[0].String(), "", opts, true)), nil
	}, 2)
	// web.cookie_get(req, name, default?) -> str
	set(p, "cookie_get", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Str(""), nil
		}
		def := ""
		if len(args) >= 3 {
			def = args[2].String()
		}
		if c, ok := mapGet(args[0], "cookies"); ok && c.Kind == runtime.KindMap {
			return runtime.Str(mapGetStr(c, args[1].String(), def)), nil
		}
		return runtime.Str(def), nil
	}, 3)

	return p
}

func httpText(args []runtime.Value) (runtime.Value, error) {
	status := int64(200)
	body := ""
	if len(args) >= 1 {
		if n, err := runtime.AsInt(args[0]); err == nil {
			status = n
		} else {
			body = args[0].String()
		}
	}
	if len(args) >= 2 {
		body = args[1].String()
	}
	return respMap(status, body, "text/plain; charset=utf-8"), nil
}

func httpJSON(args []runtime.Value) (runtime.Value, error) {
	status := int64(200)
	var val runtime.Value
	if len(args) == 1 {
		val = args[0]
	} else if len(args) >= 2 {
		if n, err := runtime.AsInt(args[0]); err == nil {
			status = n
			val = args[1]
		} else {
			val = args[0]
		}
	}
	b, err := jsonMarshal(val)
	if err != nil {
		return errRes(err.Error(), "web"), nil
	}
	return respMap(status, b, "application/json"), nil
}

type webApp struct {
	env         *runtime.Env
	mu          sync.RWMutex
	routes      []webRoute
	statics     []webStatic
	wsRoutes    []wsRoute
	befores     []runtime.Value // app.before(fn) middleware
	templateDir string
	templates   *template.Template
	notFound    runtime.Value
	onError     runtime.Value
}

type webRoute struct {
	method  string
	pattern string
	parts   []routePart
	handler runtime.Value
}

type routePart struct {
	lit   string
	param string // if non-empty, this segment is a :param
}

type webStatic struct {
	prefix string
	dir    string
}

type wsRoute struct {
	pattern string
	parts   []routePart
	handler runtime.Value
}

func newWebApp(env *runtime.Env) runtime.Value {
	app := &webApp{env: env}
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)

	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("web.app."+name, arity, fn)
	}

	// app.get(path, handler)
	put("get", 2, func(args []runtime.Value) (runtime.Value, error) {
		return app.addRoute("GET", args)
	})
	put("post", 2, func(args []runtime.Value) (runtime.Value, error) {
		return app.addRoute("POST", args)
	})
	put("put", 2, func(args []runtime.Value) (runtime.Value, error) {
		return app.addRoute("PUT", args)
	})
	put("delete", 2, func(args []runtime.Value) (runtime.Value, error) {
		return app.addRoute("DELETE", args)
	})
	put("patch", 2, func(args []runtime.Value) (runtime.Value, error) {
		return app.addRoute("PATCH", args)
	})
	// app.route(method, path, handler)
	put("route", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("app.route(method, path, handler)", "web"), nil
		}
		return app.addRoute(strings.ToUpper(args[0].String()), args[1:])
	})
	// app.static(url_prefix, dir)
	put("static", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("app.static(prefix, dir)", "web"), nil
		}
		prefix := args[0].String()
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		prefix = strings.TrimRight(prefix, "/")
		dir := args[1].String()
		app.mu.Lock()
		app.statics = append(app.statics, webStatic{prefix: prefix, dir: dir})
		app.mu.Unlock()
		return m, nil
	})
	// app.templates(dir)
	put("templates", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("app.templates(dir)", "web"), nil
		}
		dir := args[0].String()
		t, err := template.ParseGlob(filepath.Join(dir, "*.html"))
		if err != nil {
			// allow empty / missing until first render — try again later
			app.mu.Lock()
			app.templateDir = dir
			app.templates = nil
			app.mu.Unlock()
			return m, nil
		}
		app.mu.Lock()
		app.templateDir = dir
		app.templates = t
		app.mu.Unlock()
		return m, nil
	})
	// app.render(name, data?) -> html response
	put("render", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("app.render(name, data?)", "web"), nil
		}
		name := args[0].String()
		var data any
		if len(args) >= 2 {
			data = valueToGo(args[1])
		}
		html, err := app.renderTemplate(name, data)
		if err != nil {
			return errRes(err.Error(), "web"), nil
		}
		return respMap(200, html, "text/html; charset=utf-8"), nil
	})
	// app.ws(path, handler) — handler(conn)
	put("ws", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("app.ws(path, handler)", "web"), nil
		}
		pat := args[0].String()
		h := args[1]
		if h.Kind != runtime.KindFunc && h.Kind != runtime.KindBuiltin {
			return errRes("app.ws: handler must be a function", "web"), nil
		}
		app.mu.Lock()
		app.wsRoutes = append(app.wsRoutes, wsRoute{pattern: pat, parts: parsePattern(pat), handler: h})
		app.mu.Unlock()
		return m, nil
	})
	// app.before(fn) — run before every route handler; return a response map to short-circuit
	put("before", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("app.before(fn)", "web"), nil
		}
		h := args[0]
		if h.Kind != runtime.KindFunc && h.Kind != runtime.KindBuiltin {
			return errRes("app.before: need function", "web"), nil
		}
		app.mu.Lock()
		app.befores = append(app.befores, h)
		app.mu.Unlock()
		return m, nil
	})
	// app.listen(addr) / app.run(addr) — blocks
	listen := func(args []runtime.Value) (runtime.Value, error) {
		addr := ":8080"
		if len(args) >= 1 && args[0].String() != "" {
			addr = args[0].String()
		}
		if env.Call == nil {
			return errRes("web: runtime Call not configured", "web"), nil
		}
		srv := &http.Server{
			Addr:              addr,
			Handler:           app,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			// WriteTimeout 0: allow long SSE / llm.stream proxy responses
			WriteTimeout: 0,
			IdleTimeout:  120 * time.Second,
		}
		fmt.Fprintf(env.Stdout, "weft web listening on http://%s\n", displayAddr(addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return errRes(err.Error(), "web"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}
	put("listen", 1, listen)
	put("run", 1, listen)
	// app.handle(method, path, body?) — in-process dispatch for tests
	put("handle", 3, func(args []runtime.Value) (runtime.Value, error) {
		method := "GET"
		pathStr := "/"
		body := ""
		if len(args) >= 1 {
			method = strings.ToUpper(args[0].String())
		}
		if len(args) >= 2 {
			pathStr = args[1].String()
		}
		if len(args) >= 3 {
			body = args[2].String()
		}
		return app.dispatch(method, pathStr, body, nil), nil
	})
	return m
}

func (a *webApp) addRoute(method string, args []runtime.Value) (runtime.Value, error) {
	if len(args) < 2 {
		return errRes("app."+strings.ToLower(method)+"(path, handler)", "web"), nil
	}
	pat := args[0].String()
	h := args[1]
	if h.Kind != runtime.KindFunc && h.Kind != runtime.KindBuiltin {
		return errRes("handler must be a function", "web"), nil
	}
	a.mu.Lock()
	a.routes = append(a.routes, webRoute{
		method:  method,
		pattern: pat,
		parts:   parsePattern(pat),
		handler: h,
	})
	a.mu.Unlock()
	// return app for chaining — reconstruct handle from receiver is hard; return unit ok map
	return runtime.Unit(), nil
}

func parsePattern(pat string) []routePart {
	pat = path.Clean("/" + strings.TrimPrefix(pat, "/"))
	if pat != "/" {
		pat = strings.TrimRight(pat, "/")
	}
	segs := strings.Split(strings.Trim(pat, "/"), "/")
	if pat == "/" || pat == "" {
		return nil // root
	}
	out := make([]routePart, 0, len(segs))
	for _, s := range segs {
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, ":") {
			out = append(out, routePart{param: strings.TrimPrefix(s, ":")})
		} else if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			out = append(out, routePart{param: strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")})
		} else {
			out = append(out, routePart{lit: s})
		}
	}
	return out
}

func matchPattern(parts []routePart, urlPath string) (map[string]string, bool) {
	urlPath = path.Clean("/" + strings.TrimPrefix(urlPath, "/"))
	if urlPath != "/" {
		urlPath = strings.TrimRight(urlPath, "/")
	}
	if len(parts) == 0 {
		return map[string]string{}, urlPath == "/" || urlPath == ""
	}
	segs := strings.Split(strings.Trim(urlPath, "/"), "/")
	if urlPath == "/" || urlPath == "" {
		segs = nil
	}
	if len(segs) != len(parts) {
		return nil, false
	}
	params := make(map[string]string)
	for i, p := range parts {
		if p.param != "" {
			params[p.param] = segs[i]
			continue
		}
		if p.lit != segs[i] {
			return nil, false
		}
	}
	return params, true
}

func (a *webApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// WebSocket upgrade (still runs before hooks)
	if isWebSocketRequest(r) {
		a.serveWS(w, r)
		return
	}

	body, _ := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	_ = r.Body.Close()
	req := buildRequest(r, string(body), nil)
	if a.env.Call == nil {
		http.Error(w, "runtime Call not configured", 500)
		return
	}
	// before hooks apply to routes AND static (auth cannot be bypassed via static/)
	if short, stop := a.runBefores(req); stop {
		writeWeftResponse(w, short)
		return
	}

	// static
	a.mu.RLock()
	statics := append([]webStatic(nil), a.statics...)
	a.mu.RUnlock()
	for _, st := range statics {
		if r.URL.Path == st.prefix || strings.HasPrefix(r.URL.Path, st.prefix+"/") {
			rel := strings.TrimPrefix(r.URL.Path, st.prefix)
			if rel == "" {
				rel = "/"
			}
			http.StripPrefix(st.prefix, http.FileServer(http.Dir(st.dir))).ServeHTTP(w, r)
			return
		}
	}

	params, handler, ok := a.findRoute(r.Method, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	// rebuild request with path params
	req = buildRequest(r, string(body), params)
	ret, err := a.env.Call(handler, []runtime.Value{req})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeWeftResponse(w, ret)
}

func (a *webApp) runBefores(req runtime.Value) (short runtime.Value, stop bool) {
	if a.env == nil || a.env.Call == nil {
		return runtime.Null(), false
	}
	a.mu.RLock()
	befores := append([]runtime.Value(nil), a.befores...)
	a.mu.RUnlock()
	for _, bfn := range befores {
		br, err := a.env.Call(bfn, []runtime.Value{req})
		if err != nil {
			return errRes(err.Error(), "web"), true
		}
		if s, ok := middlewareResponse(br); ok {
			return s, true
		}
	}
	return runtime.Null(), false
}

// middlewareResponse reports whether ret is a short-circuit HTTP response.
// null / unit / false / empty → continue; map with status|body|type|headers|cookies → stop.
func middlewareResponse(ret runtime.Value) (runtime.Value, bool) {
	if ret.Kind == runtime.KindResult {
		ro := ret.Obj.(*runtime.ResultObj)
		if !ro.Ok {
			return ret, true
		}
		ret = ro.Val
	}
	switch ret.Kind {
	case runtime.KindNull, runtime.KindUnit:
		return runtime.Null(), false
	case runtime.KindBool:
		if !ret.B {
			return runtime.Null(), false
		}
		// true alone is not a response
		return runtime.Null(), false
	case runtime.KindMap:
		if _, ok := mapGet(ret, "status"); ok {
			return ret, true
		}
		if _, ok := mapGet(ret, "body"); ok {
			return ret, true
		}
		if _, ok := mapGet(ret, "type"); ok {
			return ret, true
		}
		if _, ok := mapGet(ret, "headers"); ok {
			return ret, true
		}
		if _, ok := mapGet(ret, "cookies"); ok {
			return ret, true
		}
		return runtime.Null(), false
	case runtime.KindStr:
		// bare string treated as HTML/text body response
		return respMap(200, ret.S, "text/html; charset=utf-8"), true
	default:
		return runtime.Null(), false
	}
}

func (a *webApp) findRoute(method, urlPath string) (map[string]string, runtime.Value, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	method = strings.ToUpper(method)
	for _, rt := range a.routes {
		if rt.method != method && rt.method != "*" {
			continue
		}
		if params, ok := matchPattern(rt.parts, urlPath); ok {
			return params, rt.handler, true
		}
	}
	return nil, runtime.Null(), false
}

func (a *webApp) dispatch(method, pathStr, body string, headers map[string]string) runtime.Value {
	params, handler, ok := a.findRoute(method, pathStr)
	if !ok {
		return respMap(404, "not found", "text/plain; charset=utf-8")
	}
	// synthetic request
	u, _ := url.Parse(pathStr)
	req := runtime.NewMap()
	mo := req.Obj.(*runtime.MapObj)
	put := func(k string, v runtime.Value) {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	put("method", runtime.Str(method))
	put("path", runtime.Str(u.Path))
	put("query", runtime.Str(u.RawQuery))
	put("body", runtime.Str(body))
	put("params", stringMapValue(params))
	put("headers", stringMapValue(headers))
	put("query_map", queryMapValue(u.Query()))
	ctype := ""
	if headers != nil {
		for k, v := range headers {
			if strings.EqualFold(k, "Content-Type") {
				ctype = v
				break
			}
		}
	}
	form, formAll, files := parseFormParts(body, ctype, u.Query())
	put("form", form)
	put("form_all", formAll)
	put("files", files)
	cookieHdr := ""
	if headers != nil {
		for k, v := range headers {
			if strings.EqualFold(k, "Cookie") {
				cookieHdr = v
				break
			}
		}
	}
	put("cookies", cookieMapFromHeader(cookieHdr))
	put("htmx", htmxFromHeaders(func(name string) string {
		if headers == nil {
			return ""
		}
		for k, v := range headers {
			if strings.EqualFold(k, name) {
				return v
			}
		}
		return ""
	}))
	if a.env.Call == nil {
		return errRes("runtime Call not configured", "web")
	}
	// before hooks
	a.mu.RLock()
	befores := append([]runtime.Value(nil), a.befores...)
	a.mu.RUnlock()
	for _, bfn := range befores {
		br, err := a.env.Call(bfn, []runtime.Value{req})
		if err != nil {
			return errRes(err.Error(), "web")
		}
		if short, ok := middlewareResponse(br); ok {
			return short
		}
	}
	ret, err := a.env.Call(handler, []runtime.Value{req})
	if err != nil {
		return errRes(err.Error(), "web")
	}
	return ret
}

func (a *webApp) renderTemplate(name string, data any) (string, error) {
	a.mu.RLock()
	t := a.templates
	dir := a.templateDir
	a.mu.RUnlock()
	if t == nil && dir != "" {
		parsed, err := template.ParseGlob(filepath.Join(dir, "*.html"))
		if err != nil {
			// single file
			p := filepath.Join(dir, name)
			if !strings.HasSuffix(p, ".html") {
				p = p + ".html"
			}
			b, err2 := os.ReadFile(p)
			if err2 != nil {
				return "", err
			}
			return string(b), nil // raw fallback without data
		}
		a.mu.Lock()
		a.templates = parsed
		t = parsed
		a.mu.Unlock()
	}
	if t == nil {
		return "", fmt.Errorf("no templates loaded — app.templates(dir)")
	}
	var buf strings.Builder
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		// try name.html
		if err2 := t.ExecuteTemplate(&buf, name+".html", data); err2 != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func (a *webApp) serveWS(w http.ResponseWriter, r *http.Request) {
	params, handler, ok := a.findWS(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	// before hooks before upgrade (auth cannot be skipped via WS)
	req := buildRequest(r, "", params)
	if short, stop := a.runBefores(req); stop {
		writeWeftResponse(w, short)
		return
	}
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer conn.Close()
	wsVal := newWSConnValue(a.env, conn, params, r)
	if a.env.Call == nil {
		return
	}
	_, _ = a.env.Call(handler, []runtime.Value{wsVal})
}

func (a *webApp) findWS(urlPath string) (map[string]string, runtime.Value, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, rt := range a.wsRoutes {
		if params, ok := matchPattern(rt.parts, urlPath); ok {
			return params, rt.handler, true
		}
	}
	return nil, runtime.Null(), false
}

func buildRequest(r *http.Request, body string, params map[string]string) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(k string, v runtime.Value) {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	put("method", runtime.Str(r.Method))
	put("path", runtime.Str(r.URL.Path))
	put("query", runtime.Str(r.URL.RawQuery))
	put("body", runtime.Str(body))
	put("host", runtime.Str(r.Host))
	put("remote", runtime.Str(r.RemoteAddr))
	put("params", stringMapValue(params))
	put("query_map", queryMapValue(r.URL.Query()))
	hdrs := runtime.NewMap()
	hmo := hdrs.Obj.(*runtime.MapObj)
	for k, vs := range r.Header {
		if len(vs) > 0 {
			hmo.Keys = append(hmo.Keys, k)
			hmo.Vals[k] = runtime.Str(vs[0])
		}
	}
	put("headers", hdrs)
	// form + multi-value + files
	form, formAll, files := parseFormParts(body, r.Header.Get("Content-Type"), r.URL.Query())
	put("form", form)
	put("form_all", formAll)
	put("files", files)
	// cookies
	put("cookies", cookieMapFromHeader(r.Header.Get("Cookie")))
	// HTMX request surface (always present; request=false when not HTMX)
	hx := htmxFromHeaders(func(name string) string { return r.Header.Get(name) })
	put("htmx", hx)
	return m
}

// parseFormMap keeps the first value per key (compat).
func parseFormMap(body, contentType string, query url.Values) runtime.Value {
	form, _, _ := parseFormParts(body, contentType, query)
	return form
}

// parseFormParts returns form (first value), form_all (list per key), files map.
func parseFormParts(body, contentType string, query url.Values) (form, formAll, files runtime.Value) {
	form = runtime.NewMap()
	formAll = runtime.NewMap()
	files = runtime.NewMap()
	fmo := form.Obj.(*runtime.MapObj)
	amo := formAll.Obj.(*runtime.MapObj)
	xmo := files.Obj.(*runtime.MapObj)

	addVal := func(k, v string) {
		// form_all: accumulate every value
		if cur, ok := amo.Vals[k]; ok && cur.Kind == runtime.KindList {
			lo := cur.Obj.(*runtime.ListObj)
			lo.Items = append(lo.Items, runtime.Str(v))
		} else {
			if _, ok := amo.Vals[k]; !ok {
				amo.Keys = append(amo.Keys, k)
			}
			amo.Vals[k] = runtime.List(runtime.Str(v))
		}
		// form: last wins (POST body overrides query)
		if _, ok := fmo.Vals[k]; !ok {
			fmo.Keys = append(fmo.Keys, k)
		}
		fmo.Vals[k] = runtime.Str(v)
	}
	addFile := func(field, filename, ctype string, data []byte) {
		if _, ok := xmo.Vals[field]; !ok {
			xmo.Keys = append(xmo.Keys, field)
		}
		fm := runtime.NewMap()
		fmo2 := fm.Obj.(*runtime.MapObj)
		fmo2.Keys = []string{"filename", "content_type", "size", "body"}
		fmo2.Vals["filename"] = runtime.Str(filename)
		fmo2.Vals["content_type"] = runtime.Str(ctype)
		fmo2.Vals["size"] = runtime.Int(int64(len(data)))
		fmo2.Vals["body"] = runtime.Str(string(data))
		xmo.Vals[field] = fm
		// also expose filename in form for convenience
		if filename != "" {
			addVal(field, filename)
		}
	}

	for k, vs := range query {
		for _, v := range vs {
			addVal(k, v)
		}
	}
	if med, params, err := mime.ParseMediaType(contentType); err == nil && med == "multipart/form-data" {
		boundary := params["boundary"]
		if boundary != "" && body != "" {
			mr := multipart.NewReader(strings.NewReader(body), boundary)
			const maxMultipartParts = 1024
			parts := 0
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}
				parts++
				if parts > maxMultipartParts {
					_ = p.Close()
					break // stop parsing; refuse to burn CPU on part storms
				}
				name := p.FormName()
				if name == "" {
					_ = p.Close()
					continue
				}
				b, err := io.ReadAll(io.LimitReader(p, 8<<20))
				fn := p.FileName()
				ct := p.Header.Get("Content-Type")
				_ = p.Close()
				if err != nil {
					continue
				}
				if fn != "" {
					addFile(name, fn, ct, b)
				} else {
					addVal(name, string(b))
				}
			}
		}
		return form, formAll, files
	}
	if strings.Contains(strings.ToLower(contentType), "application/x-www-form-urlencoded") ||
		(body != "" && (contentType == "" || !strings.Contains(contentType, "/"))) {
		vals, err := url.ParseQuery(body)
		if err == nil {
			for k, vs := range vals {
				for _, v := range vs {
					addVal(k, v)
				}
			}
		}
	}
	return form, formAll, files
}

func cookieMapFromHeader(raw string) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	if raw == "" {
		return m
	}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if _, exists := mo.Vals[k]; !exists {
			mo.Keys = append(mo.Keys, k)
		}
		mo.Vals[k] = runtime.Str(v)
	}
	return m
}

func buildSetCookie(name, value string, opts runtime.Value, clear bool) string {
	if clear {
		value = ""
	}
	// reject injection in name
	name = strings.Map(func(r rune) rune {
		if r <= 32 || r == ';' || r == ',' || r == '=' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, name)
	if name == "" {
		name = "cookie"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s=%s", name, url.QueryEscape(value))
	path := "/"
	maxAge := int64(-1)
	httpOnly, secure := true, false
	sameSite := "Lax"
	if opts.Kind == runtime.KindMap {
		if s := mapGetStr(opts, "path", ""); s != "" && !strings.ContainsAny(s, "\r\n;") {
			path = s
		}
		if n := mapGetInt(opts, "max_age", -999); n != -999 {
			maxAge = n
		}
		if v, ok := mapGet(opts, "http_only"); ok && v.Kind == runtime.KindBool {
			httpOnly = v.B
		}
		if v, ok := mapGet(opts, "secure"); ok && v.Kind == runtime.KindBool {
			secure = v.B
		}
		if s := mapGetStr(opts, "same_site", mapGetStr(opts, "samesite", "")); s != "" {
			sameSite = s
		}
	}
	if clear && maxAge < 0 {
		maxAge = 0
	}
	fmt.Fprintf(&b, "; Path=%s", path)
	if maxAge >= 0 {
		fmt.Fprintf(&b, "; Max-Age=%d", maxAge)
	}
	if httpOnly {
		b.WriteString("; HttpOnly")
	}
	if secure {
		b.WriteString("; Secure")
	}
	if sameSite != "" && !strings.ContainsAny(sameSite, "\r\n;") {
		fmt.Fprintf(&b, "; SameSite=%s", sameSite)
	}
	return b.String()
}

func htmxOOBWrap(id, html string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "#")
	// allow only safe id chars (block attribute breakout if id is request-derived)
	id = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, id)
	if id == "" {
		id = "oob"
	}
	html = strings.TrimSpace(html)
	// if caller already marked oob, leave alone
	if strings.Contains(html, "hx-swap-oob") {
		return html
	}
	// inject into opening tag if present
	if strings.HasPrefix(html, "<") {
		// find first space or >
		end := strings.IndexAny(html, " >")
		if end > 1 {
			tag := html[1:end]
			// ensure id attribute
			rest := html[end:]
			head := html
			if len(head) > 200 {
				head = head[:200]
			}
			if !strings.Contains(strings.ToLower(head), "id=") {
				return fmt.Sprintf(`<%s id="%s" hx-swap-oob="true"%s`, tag, id, rest)
			}
			return fmt.Sprintf(`<%s hx-swap-oob="true"%s`, tag, rest)
		}
	}
	return fmt.Sprintf(`<div id="%s" hx-swap-oob="true">%s</div>`, id, html)
}

func headerGetCI(headers runtime.Value, name string) string {
	if headers.Kind != runtime.KindMap {
		return ""
	}
	mo := headers.Obj.(*runtime.MapObj)
	for k, v := range mo.Vals {
		if strings.EqualFold(k, name) {
			return v.String()
		}
	}
	return ""
}

// htmxFromHeaders builds the req.htmx map from a header getter.
func htmxFromHeaders(get func(string) string) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(k string, v runtime.Value) {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	req := strings.EqualFold(get("HX-Request"), "true")
	put("request", runtime.Bool(req))
	put("boosted", runtime.Bool(strings.EqualFold(get("HX-Boosted"), "true")))
	put("history_restore", runtime.Bool(strings.EqualFold(get("HX-History-Restore-Request"), "true")))
	put("target", runtime.Str(get("HX-Target")))
	put("trigger", runtime.Str(get("HX-Trigger")))
	put("trigger_name", runtime.Str(get("HX-Trigger-Name")))
	put("current_url", runtime.Str(get("HX-Current-URL")))
	put("prompt", runtime.Str(get("HX-Prompt")))
	return m
}

func reqIsHTMX(req runtime.Value) bool {
	if hx, ok := mapGet(req, "htmx"); ok {
		if b, ok := mapGet(hx, "request"); ok && b.Kind == runtime.KindBool {
			return b.B
		}
	}
	// fallback: raw headers
	if h, ok := mapGet(req, "headers"); ok {
		// case-insensitive scan
		if h.Kind == runtime.KindMap {
			mo := h.Obj.(*runtime.MapObj)
			for k, v := range mo.Vals {
				if strings.EqualFold(k, "HX-Request") && strings.EqualFold(v.String(), "true") {
					return true
				}
			}
		}
	}
	return false
}

func mapStrPairs(kvs ...string) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	for i := 0; i+1 < len(kvs); i += 2 {
		mo.Keys = append(mo.Keys, kvs[i])
		mo.Vals[kvs[i]] = runtime.Str(kvs[i+1])
	}
	return m
}

// htmxResponse builds an HTML response with optional HTMX response headers.
func htmxResponse(args []runtime.Value) (runtime.Value, error) {
	body := ""
	var opts runtime.Value
	status := int64(200)
	if len(args) >= 1 {
		if args[0].Kind == runtime.KindMap {
			opts = args[0]
			body = mapGetStr(opts, "body", mapGetStr(opts, "html", ""))
		} else {
			body = args[0].String()
		}
	}
	if len(args) >= 2 && args[1].Kind == runtime.KindMap {
		if opts.Kind == runtime.KindMap {
			opts = mergeOpts(opts, args[1])
		} else {
			opts = args[1]
		}
	}
	if opts.Kind == runtime.KindMap {
		if n := mapGetInt(opts, "status", 0); n > 0 {
			status = n
		}
		if s := mapGetStr(opts, "body", ""); s != "" && body == "" {
			body = s
		}
		if s := mapGetStr(opts, "html", ""); s != "" && body == "" {
			body = s
		}
	}
	m := respMap(status, body, "text/html; charset=utf-8")
	if opts.Kind != runtime.KindMap {
		return m, nil
	}
	hdrs := runtime.NewMap()
	hmo := hdrs.Obj.(*runtime.MapObj)
	setH := func(name, val string) {
		if val == "" {
			return
		}
		if _, ok := hmo.Vals[name]; !ok {
			hmo.Keys = append(hmo.Keys, name)
		}
		hmo.Vals[name] = runtime.Str(val)
	}
	// string opts → headers
	setH("HX-Redirect", mapGetStr(opts, "redirect", ""))
	setH("HX-Push-Url", mapGetStr(opts, "push_url", mapGetStr(opts, "pushUrl", "")))
	setH("HX-Replace-Url", mapGetStr(opts, "replace_url", mapGetStr(opts, "replaceUrl", "")))
	setH("HX-Retarget", mapGetStr(opts, "retarget", ""))
	setH("HX-Reswap", mapGetStr(opts, "reswap", ""))
	setH("HX-Reselect", mapGetStr(opts, "reselect", ""))
	// refresh: bool or string
	if v, ok := mapGet(opts, "refresh"); ok {
		if v.Kind == runtime.KindBool && v.B {
			setH("HX-Refresh", "true")
		} else if s := v.String(); s != "" && s != "false" && s != "0" {
			setH("HX-Refresh", "true")
		}
	}
	// trigger variants (str or map → JSON)
	setTrigger := func(optKey, hdr string) {
		v, ok := mapGet(opts, optKey)
		if !ok {
			return
		}
		if v.Kind == runtime.KindMap || v.Kind == runtime.KindList {
			if s, err := jsonMarshal(v); err == nil {
				setH(hdr, s)
			}
			return
		}
		setH(hdr, v.String())
	}
	setTrigger("trigger", "HX-Trigger")
	setTrigger("trigger_after_settle", "HX-Trigger-After-Settle")
	setTrigger("trigger_after_swap", "HX-Trigger-After-Swap")
	// location: str or map (HX-Location JSON)
	if v, ok := mapGet(opts, "location"); ok {
		if v.Kind == runtime.KindMap {
			if s, err := jsonMarshal(v); err == nil {
				setH("HX-Location", s)
			}
		} else {
			setH("HX-Location", v.String())
		}
	}
	// merge extra headers map if provided
	if extra, ok := mapGet(opts, "headers"); ok && extra.Kind == runtime.KindMap {
		emo := extra.Obj.(*runtime.MapObj)
		for _, k := range emo.Keys {
			setH(k, emo.Vals[k].String())
		}
	}
	// oob: str | [str|map{id,html}] appended after main body
	if oob, ok := mapGet(opts, "oob"); ok {
		body = appendOOB(body, oob)
		// update body on response map
		mo := m.Obj.(*runtime.MapObj)
		mo.Vals["body"] = runtime.Str(body)
	}
	// cookies: list of Set-Cookie strings or single string
	if ck, ok := mapGet(opts, "cookies"); ok {
		mo := m.Obj.(*runtime.MapObj)
		if _, exists := mo.Vals["cookies"]; !exists {
			mo.Keys = append(mo.Keys, "cookies")
		}
		mo.Vals["cookies"] = ck
	}
	if cookie, ok := mapGet(opts, "cookie"); ok {
		// single cookie string
		mo := m.Obj.(*runtime.MapObj)
		var list []runtime.Value
		if cur, ok := mo.Vals["cookies"]; ok && cur.Kind == runtime.KindList {
			list = append([]runtime.Value{}, cur.Obj.(*runtime.ListObj).Items...)
		}
		list = append(list, runtime.Str(cookie.String()))
		if _, exists := mo.Vals["cookies"]; !exists {
			mo.Keys = append(mo.Keys, "cookies")
		}
		mo.Vals["cookies"] = runtime.List(list...)
	}
	if len(hmo.Keys) > 0 {
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = append(mo.Keys, "headers")
		mo.Vals["headers"] = hdrs
	}
	return m, nil
}

func appendOOB(body string, oob runtime.Value) string {
	var parts []string
	if body != "" {
		parts = append(parts, body)
	}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" {
			parts = append(parts, s)
		}
	}
	switch oob.Kind {
	case runtime.KindStr:
		add(oob.S)
	case runtime.KindList:
		for _, it := range oob.Obj.(*runtime.ListObj).Items {
			if it.Kind == runtime.KindMap {
				id := mapGetStr(it, "id", mapGetStr(it, "target", "oob"))
				html := mapGetStr(it, "html", mapGetStr(it, "body", ""))
				add(htmxOOBWrap(id, html))
			} else {
				add(it.String())
			}
		}
	case runtime.KindMap:
		id := mapGetStr(oob, "id", mapGetStr(oob, "target", "oob"))
		html := mapGetStr(oob, "html", mapGetStr(oob, "body", ""))
		add(htmxOOBWrap(id, html))
	}
	return strings.Join(parts, "\n")
}

func stringMapValue(m map[string]string) runtime.Value {
	out := runtime.NewMap()
	if m == nil {
		return out
	}
	mo := out.Obj.(*runtime.MapObj)
	for k, v := range m {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = runtime.Str(v)
	}
	return out
}

func queryMapValue(q url.Values) runtime.Value {
	out := runtime.NewMap()
	mo := out.Obj.(*runtime.MapObj)
	for k, vs := range q {
		if len(vs) > 0 {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = runtime.Str(vs[0])
		}
	}
	return out
}

func displayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

// chanSSEIter is a pull iterator backed by a channel for push-based SSE.
type chanSSEIter struct {
	ch   <-chan string
	done <-chan struct{}
}

func (it *chanSSEIter) Next() (runtime.Value, bool) {
	select {
	case msg, ok := <-it.ch:
		if !ok {
			return runtime.Null(), false
		}
		return runtime.Str(msg), true
	case <-it.done:
		return runtime.Null(), false
	}
}
