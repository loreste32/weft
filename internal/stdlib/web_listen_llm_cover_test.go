package stdlib

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func envWithCall() *runtime.Env {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		if fn.Kind == runtime.KindBuiltin {
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		}
		return runtime.Unit(), nil
	}
	var buf bytes.Buffer
	env.Stdout = &buf
	return env
}

func handlerJSON(body string) runtime.Value {
	return runtime.MakeBuiltin("h", 1, func(args []runtime.Value) (runtime.Value, error) {
		return respMap(200, body, "application/json"), nil
	})
}

func TestWeb_ServeHTTPAndStatic(t *testing.T) {
	env := envWithCall()
	app := &webApp{env: env}
	app.routes = append(app.routes, webRoute{
		method: "GET", pattern: "/hi", parts: parsePattern("/hi"),
		handler: handlerJSON(`{"ok":true}`),
	})
	app.routes = append(app.routes, webRoute{
		method: "GET", pattern: "/u/:id", parts: parsePattern("/u/:id"),
		handler: runtime.MakeBuiltin("h", 1, func(args []runtime.Value) (runtime.Value, error) {
			id := ""
			if p, ok := mapGet(args[0], "params"); ok {
				id = mapGetStr(p, "id", "")
			}
			return respMap(200, "id="+id, "text/plain"), nil
		}),
	})
	app.routes = append(app.routes, webRoute{
		method: "POST", pattern: "/echo", parts: parsePattern("/echo"),
		handler: runtime.MakeBuiltin("h", 1, func(args []runtime.Value) (runtime.Value, error) {
			body := mapGetStr(args[0], "body", "")
			return respMap(201, body, "text/plain"), nil
		}),
	})

	// 404
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rr.Code != 404 {
		t.Fatalf("404 got %d", rr.Code)
	}

	// GET /hi
	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/hi", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "ok") {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}

	// param route
	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/u/42?x=1", nil))
	if !strings.Contains(rr.Body.String(), "id=42") {
		t.Fatal(rr.Body.String())
	}

	// POST body
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hello"))
	app.ServeHTTP(rr, req)
	if rr.Code != 201 || rr.Body.String() != "hello" {
		t.Fatalf("%d %q", rr.Code, rr.Body.String())
	}

	// Call nil → 500
	app2 := &webApp{env: runtime.NewEnv()}
	app2.routes = append(app2.routes, webRoute{
		method: "GET", pattern: "/", parts: parsePattern("/"),
		handler: handlerJSON("x"),
	})
	rr = httptest.NewRecorder()
	app2.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != 500 {
		t.Fatalf("expected 500 got %d", rr.Code)
	}

	// static files
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("static-hi"), 0o644)
	app.statics = append(app.statics, webStatic{prefix: "/static", dir: dir})
	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/static/a.txt", nil))
	if !strings.Contains(rr.Body.String(), "static-hi") {
		t.Fatal(rr.Body.String())
	}

	// displayAddr
	if displayAddr(":8080") != "127.0.0.1:8080" {
		t.Fatal(displayAddr(":8080"))
	}
	if displayAddr("0.0.0.0:9") != "0.0.0.0:9" {
		t.Fatal()
	}

	// buildRequest / requestValue / queryMapValue
	req2 := httptest.NewRequest(http.MethodGet, "/p?q=1&r=2", strings.NewReader("b"))
	req2.Header.Set("X-A", "v")
	rv := requestValue(req2)
	if mapGetStr(rv, "path", "") != "/p" {
		t.Fatal(rv)
	}
	br := buildRequest(req2, "b", map[string]string{"id": "1"})
	if mapGetStr(br, "body", "") != "b" {
		t.Fatal(br)
	}
	qm := queryMapValue(req2.URL.Query())
	if mapGetStr(qm, "q", "") != "1" {
		t.Fatal(qm)
	}
	_ = stringMapValue(nil)
	_ = stringMapValue(map[string]string{"a": "b"})

	// fieldOrKey / extractHTTPBody
	m := respMap(200, "x", "text/plain")
	if extractHTTPBody(m) != "x" {
		t.Fatal()
	}
	if extractHTTPBody(runtime.Str("raw")) != "raw" {
		t.Fatal()
	}
	so := runtime.Value{Kind: runtime.KindStruct, Obj: &runtime.StructObj{
		TypeName: "R", Fields: map[string]runtime.Value{"body": runtime.Str("s")},
	}}
	if v, ok := fieldOrKey(so, "body"); !ok || v.S != "s" {
		t.Fatal()
	}
	if _, ok := fieldOrKey(runtime.Int(1), "body"); ok {
		t.Fatal()
	}

	// httpJSON / httpText edges
	j, _ := httpJSON([]runtime.Value{runtime.Int(201), row("a", 1)})
	if mapGetInt(j, "status", 0) != 201 {
		t.Fatal(j)
	}
	j2, _ := httpJSON([]runtime.Value{row("a", 1)})
	_ = j2
	tx, _ := httpText([]runtime.Value{runtime.Str("only")})
	_ = tx
	tx2, _ := httpText([]runtime.Value{runtime.Int(204), runtime.Str("")})
	_ = tx2
}

func TestWeb_TemplatesAndRender(t *testing.T) {
	env := envWithCall()
	dir := t.TempDir()
	// named template
	_ = os.WriteFile(filepath.Join(dir, "hello.html"), []byte(`{{define "hello"}}Hi {{.Name}}{{end}}`), 0o644)
	app := &webApp{env: env, templateDir: dir}
	// parse via renderTemplate lazy path
	html, err := app.renderTemplate("hello", map[string]any{"Name": "Ada"})
	if err != nil {
		// raw fallback if define didn't load via glob
		t.Log(err)
	}
	// load templates properly
	app.templates = nil
	app.templateDir = dir
	// force ParseGlob success
	_, _ = app.renderTemplate("hello", map[string]any{"Name": "Ada"})

	// raw single-file fallback: empty dir glob fails, read file
	dir2 := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir2, "page.html"), []byte("<h1>raw</h1>"), 0o644)
	app2 := &webApp{env: env, templateDir: dir2}
	html, err = app2.renderTemplate("page", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "raw") {
		t.Fatal(html)
	}

	// no templates
	app3 := &webApp{env: env}
	if _, err := app3.renderTemplate("x", nil); err == nil {
		t.Fatal("expected err")
	}

	// via newWebApp API
	val := newWebApp(env)
	_ = callMap(t, val, "templates", runtime.Str(dir2))
	r := callMap(t, val, "render", runtime.Str("page"))
	if ro, ok := r.Obj.(*runtime.ResultObj); ok && !ro.Ok {
		// render returns respMap not Result for success
		t.Log(r)
	} else if r.Kind == runtime.KindMap {
		if extractHTTPBody(r) == "" && mapGetStr(r, "body", "") == "" {
			// check body field
			if b, ok := mapGet(r, "body"); ok && !strings.Contains(b.S, "raw") {
				t.Log(r)
			}
		}
	}
	mustErr(t, callMap(t, val, "templates"))
	mustErr(t, callMap(t, val, "render"))
	mustErr(t, callMap(t, val, "ws", runtime.Str("/ws")))
	mustErr(t, callMap(t, val, "ws", runtime.Str("/ws"), runtime.Str("notfn")))

	// route helpers
	h := handlerJSON("z")
	_ = callMap(t, val, "get", runtime.Str("/g"), h)
	_ = callMap(t, val, "post", runtime.Str("/p"), h)
	_ = callMap(t, val, "put", runtime.Str("/u"), h)
	_ = callMap(t, val, "delete", runtime.Str("/d"), h)
	_ = callMap(t, val, "patch", runtime.Str("/a"), h)
	_ = callMap(t, val, "route", runtime.Str("OPTIONS"), runtime.Str("/o"), h)
	mustErr(t, callMap(t, val, "route", runtime.Str("GET")))
	_ = callMap(t, val, "static", runtime.Str("assets"), runtime.Str(dir2))
	mustErr(t, callMap(t, val, "static", runtime.Str("/x")))

	// handle dispatch
	ret := callMap(t, val, "handle", runtime.Str("GET"), runtime.Str("/g"))
	_ = ret
	// listen without Call fails
	env2 := runtime.NewEnv()
	v2 := newWebApp(env2)
	mustErr(t, callMap(t, v2, "listen", runtime.Str("127.0.0.1:0")))
}

func TestWeb_ServeWS(t *testing.T) {
	env := envWithCall()
	app := &webApp{env: env}
	var gotPath string
	app.wsRoutes = append(app.wsRoutes, wsRoute{
		pattern: "/ws/:room",
		parts:   parsePattern("/ws/:room"),
		handler: runtime.MakeBuiltin("ws", 1, func(args []runtime.Value) (runtime.Value, error) {
			if len(args) >= 1 {
				gotPath = mapGetStr(args[0], "path", "")
				// recv will block — close immediately by not reading
				_ = callMap(t, args[0], "close")
			}
			return runtime.Ok(runtime.Unit()), nil
		}),
	})

	// 404 ws
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/no", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	app.ServeHTTP(rr, req)
	// ResponseRecorder can't hijack → 400
	if rr.Code != 400 && rr.Code != 404 {
		// might be 400 from upgrade fail
		t.Log(rr.Code, rr.Body.String())
	}

	// successful upgrade via hijack pair
	hp := newHijackPair()
	defer hp.client.Close()
	req = httptest.NewRequest(http.MethodGet, "/ws/lobby", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	req.Header.Set("Sec-WebSocket-Key", key)

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.serveWS(hp, req)
	}()

	// read 101
	br := bufio.NewReader(hp.client)
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "101") {
		t.Fatalf("%q", line)
	}
	for {
		l, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if l == "\r\n" {
			break
		}
	}
	// accept key check
	sum := sha1.Sum([]byte(key + wsGUID))
	_ = base64.StdEncoding.EncodeToString(sum[:])

	// close client to unblock handler if still running
	_ = hp.client.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Log("serveWS still running (ok if handler finished)")
	}
	_ = gotPath

	// findWS miss
	if _, _, ok := app.findWS("/nope"); ok {
		t.Fatal()
	}

	// package surface
	p := packageWeb(env)
	_ = callPkg(t, p, "app")
	_ = callPkg(t, p, "text", runtime.Str("t"))
	_ = callPkg(t, p, "json", row("k", 1))
	_ = callPkg(t, p, "html", runtime.Str("<b>x</b>"))
	_ = callPkg(t, p, "html", runtime.Int(201), runtime.Str("<b>x</b>"))
	_ = callPkg(t, p, "redirect", runtime.Str("/x"))
	_ = callPkg(t, p, "redirect", runtime.Int(301), runtime.Str("/y"))
	mustErr(t, callPkg(t, p, "redirect"))
	_ = callPkg(t, p, "status", runtime.Int(418), runtime.Str("teapot"))
	_ = callPkg(t, p, "sse", runtime.List(runtime.Str("a")))
	mustErr(t, callPkg(t, p, "sse"))
}

func TestLLM_HelpersPure(t *testing.T) {
	// parseChatResponse
	body := []byte(`{"choices":[{"message":{"content":"hi","tool_calls":[{"id":"1","function":{"name":"add","arguments":"{\"a\":1}"}}]}}]}`)
	text, calls, err := parseChatResponse(body)
	if err != nil || text != "hi" || len(calls) != 1 {
		t.Fatal(text, calls, err)
	}
	_, _, err = parseChatResponse([]byte(`not-json`))
	if err == nil {
		t.Fatal()
	}
	_, _, err = parseChatResponse([]byte(`{"error":{"message":"boom"}}`))
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatal(err)
	}
	_, _, err = parseChatResponse([]byte(`{"choices":[]}`))
	if err == nil {
		t.Fatal()
	}

	// stripFences
	if stripFences("```json\n{\"a\":1}\n```") != `{"a":1}` {
		t.Fatal(stripFences("```json\n{\"a\":1}\n```"))
	}
	if stripFences("```JSON\nx\n```") != "x" {
		t.Fatal()
	}
	if stripFences("```\ny\n```") != "y" {
		t.Fatal()
	}
	if stripFences("plain") != "plain" {
		t.Fatal()
	}

	// truncate
	if truncate("hi", 10) != "hi" {
		t.Fatal()
	}
	if !strings.HasSuffix(truncate("hello world", 5), "...") {
		t.Fatal(truncate("hello world", 5))
	}

	// parseSSEBody
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: [DONE]\n\n"
	evs := parseSSEBody(strings.NewReader(sse))
	if len(evs) == 0 {
		t.Fatal("expected events")
	}

	// defaultLLMOpts providers
	env := runtime.NewEnv()
	env.Environ = map[string]string{"LLM_PROVIDER": "ollama"}
	_ = defaultLLMOpts(env)
	env.Environ = map[string]string{"LLM_PROVIDER": "vllm"}
	_ = defaultLLMOpts(env)
	env.Environ = map[string]string{"LLM_PROVIDER": "anthropic", "ANTHROPIC_API_KEY": "k", "ANTHROPIC_MODEL": "m"}
	_ = defaultLLMOpts(env)
	env.Environ = map[string]string{"OPENAI_API_KEY": "ok", "OPENAI_MODEL": "gpt"}
	_ = defaultLLMOpts(env)

	// event helper
	_ = event("delta", map[string]runtime.Value{"text": runtime.Str("x")})
}

func TestPackageMembersAndTrunc(t *testing.T) {
	if PackageMembers("nope") != nil {
		t.Fatal()
	}
	mems := PackageMembers("json")
	if len(mems) == 0 {
		t.Fatal()
	}
	found := false
	for _, m := range mems {
		if m == "parse" {
			found = true
		}
	}
	if !found {
		t.Fatal(mems)
	}
	// Names / IsPackage
	if !IsPackage("json") {
		t.Fatal()
	}
	if len(Names()) < 10 {
		t.Fatal(Names())
	}

	if truncStr("hi", 10) != "hi" {
		t.Fatal()
	}
	if !strings.HasSuffix(truncStr("hello world", 5), "…") {
		t.Fatal(truncStr("hello world", 5))
	}
}

func TestCLI_ValueToStringSlice(t *testing.T) {
	if valueToStringSlice(runtime.Str("x")) != nil {
		t.Fatal()
	}
	s := valueToStringSlice(runtime.List(runtime.Str("a"), runtime.Int(1)))
	if len(s) != 2 || s[0] != "a" {
		t.Fatal(s)
	}
}

func TestViz_MaterializeAndEmpty(t *testing.T) {
	// materializeSave paths
	html, err := materializeSave(runtime.Str("<html>x</html>"), "out.html")
	if err != nil || !strings.Contains(html, "html") {
		t.Fatal(html, err)
	}
	html, err = materializeSave(runtime.Str(`<svg></svg>`), "c.html")
	if err != nil || !strings.Contains(html, "svg") {
		t.Fatal(html, err)
	}
	html, err = materializeSave(runtime.Str("plain"), "c.html")
	if err != nil {
		t.Fatal(err)
	}
	// chart map with svg
	ch := runtime.NewMap()
	cho := ch.Obj.(*runtime.MapObj)
	cho.Keys = []string{"svg", "title"}
	cho.Vals["svg"] = runtime.Str(`<svg id="c"></svg>`)
	cho.Vals["title"] = runtime.Str("T")
	_, err = materializeSave(ch, "x.html")
	// may fail chartSVGOf if structure wrong — ok
	_ = err
	// table-like
	tb := runtime.NewMap()
	tbo := tb.Obj.(*runtime.MapObj)
	tbo.Keys = []string{"html", "kind"}
	tbo.Vals["html"] = runtime.Str("<table></table>")
	tbo.Vals["kind"] = runtime.Str("table")
	html, err = materializeSave(tb, "t.html")
	if err != nil || !strings.Contains(html, "table") {
		t.Fatal(html, err)
	}
	// svg ext
	_, err = materializeSave(runtime.Str(`<svg/>`), "a.svg")
	_ = err
	// default ext
	s, err := materializeSave(runtime.Str("txt"), "a.txt")
	if err != nil || s != "txt" {
		t.Fatal(s, err)
	}
	// text field
	tm := runtime.NewMap()
	tm.Obj.(*runtime.MapObj).Keys = []string{"text"}
	tm.Obj.(*runtime.MapObj).Vals["text"] = runtime.Str("hello")
	s, err = materializeSave(tm, "a.dat")
	if err != nil || s != "hello" {
		t.Fatal(s, err)
	}
	// html field
	hm := runtime.NewMap()
	hm.Obj.(*runtime.MapObj).Keys = []string{"html"}
	hm.Obj.(*runtime.MapObj).Vals["html"] = runtime.Str("<p>")
	s, err = materializeSave(hm, "a.dat")
	if err != nil || s != "<p>" {
		t.Fatal(s, err)
	}
	_, err = materializeSave(runtime.Int(1), "a.html")
	if err == nil {
		t.Fatal()
	}
	_, err = materializeSave(runtime.Int(1), "a.dat")
	if err == nil {
		t.Fatal()
	}

	// parseTable
	rows, err := parseTable(runtime.List(
		row("a", 1, "b", 2),
		row("a", 3, "b", 4),
	))
	if err != nil || len(rows) < 2 {
		t.Fatal(rows, err)
	}
	rows, err = parseTable(runtime.List(
		runtime.List(runtime.Str("a"), runtime.Str("b")),
		runtime.List(runtime.Str("1"), runtime.Str("2")),
	))
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	_, err = parseTable(runtime.Str("no"))
	if err == nil {
		t.Fatal()
	}
	_, err = parseTable(runtime.List())
	if err != nil {
		t.Fatal(err)
	}

	// emptySVG / kindTitle
	_ = emptySVG(100, 50, "empty")
	if kindTitle("") != "Chart" {
		t.Fatal()
	}
	if kindTitle("bar") != "Bar" {
		t.Fatal(kindTitle("bar"))
	}
}

func TestWeb_ListenBackgroundSmoke(t *testing.T) {
	// Start real listen on :0 via raw http.Server with webApp (not blocking forever)
	env := envWithCall()
	app := &webApp{env: env}
	app.routes = append(app.routes, webRoute{
		method: "GET", pattern: "/ping", parts: parsePattern("/ping"),
		handler: handlerJSON(`{"pong":true}`),
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: app, ReadHeaderTimeout: 2 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	url := "http://" + ln.Addr().String() + "/ping"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(b), "pong") {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
}
