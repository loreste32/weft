package stdlib

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func jsonBytes(v runtime.Value) ([]byte, error) {
	return json.Marshal(valueToGo(v))
}

const (
	defaultHTTPTimeout = 30 * time.Second
	maxBodyBytes       = 32 << 20 // 32 MiB
)

func packageHTTP(env *runtime.Env) runtime.Value {
	p := pkg()
	// http.get(url, opts?)
	set(p, "get", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.get needs url", "http"), nil
		}
		return doRequest(env, "GET", args[0].String(), "", nil, optArg(args, 1))
	}, 2)
	// http.get_json(url, opts?) -> Result[any]
	// GET + parse JSON body (agent glue: one call instead of get/parse).
	set(p, "get_json", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.get_json(url, opts?)", "http"), nil
		}
		res, err := doRequest(env, "GET", args[0].String(), "", nil, optArg(args, 1))
		if err != nil {
			return runtime.Null(), err
		}
		if res.Kind != runtime.KindResult {
			return errRes("http.get_json: unexpected response", "http"), nil
		}
		ro := res.Obj.(*runtime.ResultObj)
		if !ro.Ok {
			return res, nil
		}
		// Prefer non-2xx as Err so ? works for API scripts.
		if st, ok := fieldOrKey(ro.Val, "status"); ok {
			if n, e := runtime.AsInt(st); e == nil && (n < 200 || n >= 300) {
				return errRes(fmt.Sprintf("http.get_json: HTTP %d", n), "http"), nil
			}
		}
		body := extractHTTPBody(ro.Val)
		var raw any
		dec := json.NewDecoder(strings.NewReader(body))
		dec.UseNumber()
		if e := dec.Decode(&raw); e != nil {
			return errRes("http.get_json: "+e.Error(), "http"), nil
		}
		return runtime.Ok(goToValue(raw)), nil
	}, 2)
	// http.post(url, body, opts?)
	set(p, "post", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.post needs url", "http"), nil
		}
		body := ""
		if len(args) > 1 {
			body = args[1].String()
		}
		return doRequest(env, "POST", args[0].String(), body, map[string]string{
			"Content-Type": "application/json",
		}, optArg(args, 2))
	}, 3)
	set(p, "put", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.put(url, body?)", "http"), nil
		}
		body := ""
		if len(args) > 1 {
			body = args[1].String()
		}
		return doRequest(env, "PUT", args[0].String(), body, map[string]string{
			"Content-Type": "application/json",
		}, optArg(args, 2))
	}, 3)
	set(p, "patch", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.patch(url, body?)", "http"), nil
		}
		body := ""
		if len(args) > 1 {
			body = args[1].String()
		}
		return doRequest(env, "PATCH", args[0].String(), body, map[string]string{
			"Content-Type": "application/json",
		}, optArg(args, 2))
	}, 3)
	set(p, "delete", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.delete(url)", "http"), nil
		}
		return doRequest(env, "DELETE", args[0].String(), "", nil, optArg(args, 1))
	}, 2)
	// http.fetch(opts) -> Result
	// opts: {url, method?, body?, headers?, timeout?, retries?, form?, files?}
	set(p, "fetch", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.fetch needs opts map", "http"), nil
		}
		url := mapGetStr(args[0], "url", "")
		if url == "" {
			return errRes("http.fetch: url required", "http"), nil
		}
		method := mapGetStr(args[0], "method", "GET")
		body := mapGetStr(args[0], "body", "")
		var headers map[string]string
		if h, ok := mapGet(args[0], "headers"); ok {
			if m, err := asMap(h); err == nil {
				headers = make(map[string]string, len(m))
				for k, v := range m {
					headers[k] = fmtSprint(v)
				}
			}
		}
		return doRequest(env, method, url, body, headers, args[0])
	}, 1)
	// http.post_form(url, form_map, opts?) — multipart/form-data
	set(p, "post_form", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[1].Kind != runtime.KindMap {
			return errRes("http.post_form(url, form, opts?)", "http"), nil
		}
		opts := runtime.NewMap()
		if len(args) >= 3 && args[2].Kind == runtime.KindMap {
			opts = args[2]
		}
		// inject form into opts
		omo := opts.Obj.(*runtime.MapObj)
		if _, ok := omo.Vals["form"]; !ok {
			omo.Keys = append(omo.Keys, "form")
		}
		omo.Vals["form"] = args[1]
		return doRequest(env, "POST", args[0].String(), "", nil, opts)
	}, 3)
	// http.text(status, body) -> response map for serve handlers
	set(p, "text", func(args []runtime.Value) (runtime.Value, error) {
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
	}, 2)
	// http.json(status, value)
	set(p, "json", func(args []runtime.Value) (runtime.Value, error) {
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
			return errRes(err.Error(), "http"), nil
		}
		return respMap(status, b, "application/json"), nil
	}, 2)
	// http.serve(addr, handler) — prefer web.app for production
	set(p, "serve", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("http.serve(addr, handler)", "http"), nil
		}
		addr := args[0].String()
		handler := args[1]
		if handler.Kind != runtime.KindFunc && handler.Kind != runtime.KindBuiltin {
			return errRes("http.serve: handler must be a function", "http"), nil
		}
		if env.Call == nil {
			return errRes("http.serve: runtime Call not configured", "http"), nil
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			req := requestValue(r)
			ret, err := env.Call(handler, []runtime.Value{req})
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			writeWeftResponse(w, ret)
		})
		srv := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil {
			return errRes(err.Error(), "http"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)
	return p
}

func respMap(status int64, body, ctype string) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	for k, v := range map[string]runtime.Value{
		"status": runtime.Int(status),
		"body":   runtime.Str(body),
		"type":   runtime.Str(ctype),
	} {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	return m
}

func extractHTTPBody(v runtime.Value) string {
	if s, ok := fieldOrKey(v, "body"); ok {
		return s.String()
	}
	return v.String()
}

func fieldOrKey(v runtime.Value, name string) (runtime.Value, bool) {
	switch v.Kind {
	case runtime.KindMap:
		mo := v.Obj.(*runtime.MapObj)
		x, ok := mo.Vals[name]
		return x, ok
	case runtime.KindStruct:
		so := v.Obj.(*runtime.StructObj)
		x, ok := so.Fields[name]
		return x, ok
	default:
		return runtime.Null(), false
	}
}

func jsonMarshal(v runtime.Value) (string, error) {
	b, err := jsonBytes(v)
	return string(b), err
}

func requestValue(r *http.Request) runtime.Value {
	body, _ := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	_ = r.Body.Close()
	return buildRequest(r, string(body), nil)
}

func writeWeftResponse(w http.ResponseWriter, ret runtime.Value) {
	if ret.Kind == runtime.KindResult {
		ro := ret.Obj.(*runtime.ResultObj)
		if !ro.Ok {
			http.Error(w, ro.Err.String(), 500)
			return
		}
		ret = ro.Val
	}
	status := 200
	body := ret.String()
	ctype := "text/plain; charset=utf-8"
	var streamSrc runtime.Value
	hasStream := false
	sse := false
	if ret.Kind == runtime.KindMap {
		if v, ok := mapGet(ret, "status"); ok {
			if n, err := runtime.AsInt(v); err == nil {
				status = int(n)
			}
		}
		if v, ok := mapGet(ret, "body"); ok {
			body = v.String()
		}
		if v, ok := mapGet(ret, "type"); ok {
			ctype = v.String()
		}
		if v, ok := mapGet(ret, "headers"); ok && v.Kind == runtime.KindMap {
			mo := v.Obj.(*runtime.MapObj)
			for _, k := range mo.Keys {
				val := mo.Vals[k].String()
				// Set-Cookie must use Add (multiple cookies)
				if strings.EqualFold(k, "Set-Cookie") {
					w.Header().Add("Set-Cookie", val)
				} else {
					w.Header().Set(k, val)
				}
			}
		}
		// cookies: list of Set-Cookie values or single string
		if v, ok := mapGet(ret, "cookies"); ok {
			switch v.Kind {
			case runtime.KindList:
				for _, it := range v.Obj.(*runtime.ListObj).Items {
					if s := it.String(); s != "" {
						w.Header().Add("Set-Cookie", s)
					}
				}
			case runtime.KindStr:
				if v.S != "" {
					w.Header().Add("Set-Cookie", v.S)
				}
			}
		}
		if v, ok := mapGet(ret, "stream"); ok {
			streamSrc = v
			hasStream = true
		}
		if v, ok := mapGet(ret, "sse"); ok {
			sse = v.IsTruthy()
		}
		if !sse && hasStream && strings.Contains(ctype, "text/event-stream") {
			sse = true
		}
	} else if ret.Kind == runtime.KindStr {
		body = ret.S
	}

	// Streaming path: flush each chunk (agent token fan-out to HTTP callers).
	if hasStream {
		if ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		if sse {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")
		}
		w.WriteHeader(status)
		flusher, _ := w.(http.Flusher)
		it, err := runtime.AsIter(streamSrc)
		if err != nil {
			_, _ = w.Write([]byte(fmt.Sprintf("data: {\"error\":%q}\n\n", err.Error())))
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		for {
			v, ok := it.Next()
			if !ok {
				break
			}
			line := formatSSEChunk(v, sse)
			if line == "" {
				continue
			}
			if _, err := io.WriteString(w, line); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		return
	}

	if ctype != "" && w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", ctype)
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// formatSSEChunk turns a stream item into bytes to write.
// sse=true → Server-Sent Events framing; false → raw string body chunks.
func formatSSEChunk(v runtime.Value, sse bool) string {
	if !sse {
		return v.String()
	}
	// Already framed SSE block
	if v.Kind == runtime.KindStr {
		s := v.S
		if strings.HasPrefix(s, "data:") || strings.HasPrefix(s, "event:") {
			if !strings.HasSuffix(s, "\n") {
				s += "\n"
			}
			if !strings.HasSuffix(s, "\n\n") {
				s += "\n"
			}
			return s
		}
		return "data: " + s + "\n\n"
	}
	if v.Kind == runtime.KindMap {
		mo := v.Obj.(*runtime.MapObj)
		// llm.stream event: {kind, text?}
		if kind, ok := mo.Vals["kind"]; ok {
			switch kind.S {
			case "text":
				text := ""
				if t, ok := mo.Vals["text"]; ok {
					text = t.String()
				}
				return "data: " + text + "\n\n"
			case "done":
				return "data: [DONE]\n\n"
			case "error":
				msg := "error"
				if e, ok := mo.Vals["error"]; ok {
					msg = e.String()
				}
				return "event: error\ndata: " + msg + "\n\n"
			}
		}
		// {event?, data}
		var b strings.Builder
		if ev, ok := mo.Vals["event"]; ok && ev.String() != "" {
			b.WriteString("event: ")
			b.WriteString(ev.String())
			b.WriteString("\n")
		}
		data := ""
		if d, ok := mo.Vals["data"]; ok {
			data = d.String()
		} else {
			// whole map as JSON-ish string
			data = v.String()
		}
		b.WriteString("data: ")
		b.WriteString(data)
		b.WriteString("\n\n")
		return b.String()
	}
	return "data: " + v.String() + "\n\n"
}

func fmtSprint(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func doRequest(env *runtime.Env, method, urlStr, body string, headers map[string]string, opts runtime.Value) (runtime.Value, error) {
	client := env.HTTPClient
	if client == nil {
		client = DefaultHTTPClient()
	}
	// per-request options
	timeout := time.Duration(0)
	retries := 0
	retryWait := 200 * time.Millisecond
	useCircuit := false
	circuitThreshold := 5
	circuitCooldown := 30 * time.Second
	insecure := false
	if opts.Kind == runtime.KindMap || opts.Kind == runtime.KindStruct {
		if n := mapGetInt(opts, "timeout", 0); n > 0 {
			timeout = time.Duration(n) * time.Second
		}
		if s := mapGetStr(opts, "timeout", ""); s != "" && timeout == 0 {
			if d, err := time.ParseDuration(s); err == nil {
				timeout = d
			}
		}
		if n := mapGetInt(opts, "timeout_ms", 0); n > 0 {
			timeout = time.Duration(n) * time.Millisecond
		}
		if v, ok := mapGet(opts, "insecure"); ok && v.IsTruthy() {
			insecure = true
		}
		if n := mapGetInt(opts, "retries", 0); n > 0 {
			retries = int(n)
			if retries > 10 {
				retries = 10
			}
		}
		if n := mapGetInt(opts, "retry_ms", 0); n > 0 {
			retryWait = time.Duration(n) * time.Millisecond
		}
		if v, ok := mapGet(opts, "circuit"); ok {
			useCircuit = v.IsTruthy()
		}
		if n := mapGetInt(opts, "circuit_threshold", 0); n > 0 {
			circuitThreshold = int(n)
			useCircuit = true
			if circuitThreshold > 100 {
				circuitThreshold = 100
			}
		}
		if n := mapGetInt(opts, "circuit_cooldown", 0); n > 0 {
			circuitCooldown = time.Duration(n) * time.Second
			useCircuit = true
		}
		if n := mapGetInt(opts, "circuit_cooldown_ms", 0); n > 0 {
			circuitCooldown = time.Duration(n) * time.Millisecond
			useCircuit = true
		}
		if h, ok := mapGet(opts, "headers"); ok && headers == nil {
			if m, err := asMap(h); err == nil {
				headers = make(map[string]string, len(m))
				for k, v := range m {
					headers[k] = fmtSprint(v)
				}
			}
		}
		// multipart form: opts.form = {field: value, ...} and/or opts.files = {field: path}
		if form, ok := mapGet(opts, "form"); ok && form.Kind == runtime.KindMap {
			mb, ctype, err := buildMultipart(form, opts)
			if err != nil {
				return errRes(err.Error(), "http"), nil
			}
			body = string(mb)
			if headers == nil {
				headers = map[string]string{}
			}
			headers["Content-Type"] = ctype
		}
	}
	// TLS skip-verify for local/dev only (opts.insecure=true)
	if insecure {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		} else {
			tr.TLSClientConfig = tr.TLSClientConfig.Clone()
		}
		tr.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // explicit opt-in for local TLS
		client = &http.Client{
			Timeout:       client.Timeout,
			Transport:     tr,
			CheckRedirect: client.CheckRedirect,
			Jar:           client.Jar,
		}
	}
	base := env.Context()
	if base.Err() != nil {
		return errRes(base.Err().Error(), "cancel"), nil
	}

	host := hostKey(urlStr)
	if useCircuit && !globalCircuits.allow(host, circuitCooldown) {
		return errRes(circuitOpenMsg+": "+host, "http"), nil
	}

	var lastErr error
	var lastStatus int
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-base.Done():
				return errRes(base.Err().Error(), "cancel"), nil
			case <-time.After(retryWait * time.Duration(attempt)):
			}
		}
		ctx := base
		var cancel context.CancelFunc
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(base, timeout)
		}
		var rdr io.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, urlStr, rdr)
		if err != nil {
			if cancel != nil {
				cancel()
			}
			return errRes(err.Error(), "http"), nil
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			if cancel != nil {
				cancel()
			}
			if ctx.Err() != nil {
				lastErr = ctx.Err()
			} else {
				lastErr = err
			}
			// retry network errors
			continue
		}
		limited := io.LimitReader(resp.Body, maxBodyBytes+1)
		b, err := io.ReadAll(limited)
		resp.Body.Close()
		if cancel != nil {
			cancel()
		}
		if err != nil {
			lastErr = err
			continue
		}
		if len(b) > maxBodyBytes {
			if useCircuit {
				globalCircuits.failure(host, circuitThreshold, circuitCooldown)
			}
			return errRes("response body too large", "http"), nil
		}
		lastStatus = resp.StatusCode
		// retry selected HTTP statuses
		if attempt < retries && (resp.StatusCode == 429 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504) {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		// success path for breaker: 2xx/3xx/4xx (not retriable server fail) close circuit
		if useCircuit {
			if resp.StatusCode >= 500 {
				globalCircuits.failure(host, circuitThreshold, circuitCooldown)
			} else {
				globalCircuits.success(host)
			}
		}
		hdrs := runtime.NewMap()
		hmo := hdrs.Obj.(*runtime.MapObj)
		for k, vs := range resp.Header {
			if len(vs) > 0 {
				hmo.Keys = append(hmo.Keys, k)
				hmo.Vals[k] = runtime.Str(vs[0])
			}
		}
		out := runtime.Struct("Response", map[string]runtime.Value{
			"status":  runtime.Int(int64(resp.StatusCode)),
			"body":    runtime.Str(string(b)),
			"headers": hdrs,
			"ok":      runtime.Bool(resp.StatusCode >= 200 && resp.StatusCode < 300),
		}, []string{"status", "body", "headers", "ok"})
		return runtime.Ok(out), nil
	}
	if useCircuit {
		// exhausted retries / network failure
		_ = lastStatus
		globalCircuits.failure(host, circuitThreshold, circuitCooldown)
	}
	if lastErr != nil {
		if base.Err() != nil {
			return errRes(base.Err().Error(), "cancel"), nil
		}
		return errRes(lastErr.Error(), "http"), nil
	}
	return errRes("http request failed", "http"), nil
}

// buildMultipart builds multipart body from form map; optional opts.files = {field: path}.
func buildMultipart(form runtime.Value, opts runtime.Value) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if form.Kind == runtime.KindMap {
		mo := form.Obj.(*runtime.MapObj)
		for _, k := range mo.Keys {
			if err := w.WriteField(k, mo.Vals[k].String()); err != nil {
				return nil, "", err
			}
		}
		for k, v := range mo.Vals {
			found := false
			for _, kk := range mo.Keys {
				if kk == k {
					found = true
					break
				}
			}
			if !found {
				_ = w.WriteField(k, v.String())
			}
		}
	}
	if files, ok := mapGet(opts, "files"); ok && files.Kind == runtime.KindMap {
		fmo := files.Obj.(*runtime.MapObj)
		for _, field := range fmo.Keys {
			path := fmo.Vals[field].String()
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, "", err
			}
			part, err := w.CreateFormFile(field, filepath.Base(path))
			if err != nil {
				return nil, "", err
			}
			if _, err := part.Write(data); err != nil {
				return nil, "", err
			}
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}
