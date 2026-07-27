package stdlib

import (
	"fmt"
	"html/template"
	"io"
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

// packageWeb is Flask/Django-style web apps for Weft (stdlib, pure Go).
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
	// WebSocket upgrade
	if isWebSocketRequest(r) {
		a.serveWS(w, r)
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

	body, _ := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	_ = r.Body.Close()
	params, handler, ok := a.findRoute(r.Method, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	req := buildRequest(r, string(body), params)
	if a.env.Call == nil {
		http.Error(w, "runtime Call not configured", 500)
		return
	}
	ret, err := a.env.Call(handler, []runtime.Value{req})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeWeftResponse(w, ret)
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
	if a.env.Call == nil {
		return errRes("runtime Call not configured", "web")
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
	return m
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
