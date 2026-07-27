package stdlib

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestWeb_FormAllAndFiles(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/x?tag=q", strings.NewReader("tag=a&tag=b&name=Ada"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	v := buildRequest(req, "tag=a&tag=b&name=Ada", nil)
	form := mustMapGet(t, v, "form")
	// last value wins (body overrides query; last body tag is b)
	if mapGetStr(form, "tag", "") != "b" {
		t.Fatal("last wins", form)
	}
	all := mustMapGet(t, v, "form_all")
	tags := mustMapGet(t, all, "tag")
	// query tag=q + body tag=a,b
	if tags.Kind != runtime.KindList || len(tags.Obj.(*runtime.ListObj).Items) < 2 {
		t.Fatal(tags)
	}

	env := envWithCall()
	p := packageWeb(env)
	lst := callPkg(t, p, "form_list", v, runtime.Str("tag"))
	if lst.Kind != runtime.KindList || len(lst.Obj.(*runtime.ListObj).Items) < 2 {
		t.Fatal(lst)
	}

	// multipart file
	var body strings.Builder
	w := multipart.NewWriter(&body)
	_ = w.WriteField("title", "doc")
	part, _ := w.CreateFormFile("upload", "hi.txt")
	_, _ = part.Write([]byte("hello-file"))
	_ = w.Close()
	raw := body.String()
	mreq := httptest.NewRequest(http.MethodPost, "/up", strings.NewReader(raw))
	mreq.Header.Set("Content-Type", w.FormDataContentType())
	mv := buildRequest(mreq, raw, nil)
	files := mustMapGet(t, mv, "files")
	f := mustMapGet(t, files, "upload")
	if mapGetStr(f, "filename", "") != "hi.txt" {
		t.Fatal(f)
	}
	if mapGetStr(f, "body", "") != "hello-file" {
		t.Fatal(f)
	}
	got := callPkg(t, p, "file", mv, runtime.Str("upload"))
	if mapGetStr(got, "filename", "") != "hi.txt" {
		t.Fatal(got)
	}
	if callPkg(t, p, "file", mv, runtime.Str("nope")).Kind != runtime.KindNull {
		t.Fatal()
	}
}

func TestWeb_Cookies(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", "sid=abc; theme=dark")
	v := buildRequest(req, "", nil)
	ck := mustMapGet(t, v, "cookies")
	if mapGetStr(ck, "sid", "") != "abc" || mapGetStr(ck, "theme", "") != "dark" {
		t.Fatal(ck)
	}
	env := envWithCall()
	p := packageWeb(env)
	if callPkg(t, p, "cookie_get", v, runtime.Str("sid")).S != "abc" {
		t.Fatal()
	}
	if callPkg(t, p, "cookie_get", v, runtime.Str("x"), runtime.Str("d")).S != "d" {
		t.Fatal()
	}
	sc := callPkg(t, p, "cookie", runtime.Str("sid"), runtime.Str("xyz"), row("path", "/", "max_age", 3600, "http_only", true))
	if !strings.Contains(sc.S, "sid=") || !strings.Contains(sc.S, "HttpOnly") {
		t.Fatal(sc)
	}
	cl := callPkg(t, p, "clear_cookie", runtime.Str("sid"))
	if !strings.Contains(cl.S, "Max-Age=0") {
		t.Fatal(cl)
	}

	// writeWeftResponse emits Set-Cookie
	rr := httptest.NewRecorder()
	resp := respMap(200, "ok", "text/plain")
	mo := resp.Obj.(*runtime.MapObj)
	mo.Keys = append(mo.Keys, "cookies")
	mo.Vals["cookies"] = runtime.List(runtime.Str(sc.S), runtime.Str(cl.S))
	writeWeftResponse(rr, resp)
	if len(rr.Result().Cookies()) < 1 && rr.Header().Get("Set-Cookie") == "" {
		// Header may have multiple Set-Cookie
		vals := rr.Header().Values("Set-Cookie")
		if len(vals) < 2 {
			t.Fatal(rr.Header())
		}
	}
}

func TestWeb_HTMX_OOB(t *testing.T) {
	env := envWithCall()
	p := packageWeb(env)
	oob := callPkg(t, p, "htmx_oob", runtime.Str("#flash"), runtime.Str("saved!"))
	if !strings.Contains(oob.S, "hx-swap-oob") || !strings.Contains(oob.S, "flash") {
		t.Fatal(oob)
	}
	// inject into existing tag
	oob2 := callPkg(t, p, "htmx_oob", runtime.Str("nav"), runtime.Str(`<nav class="x">hi</nav>`))
	if !strings.Contains(oob2.S, "hx-swap-oob") {
		t.Fatal(oob2)
	}
	// htmx with oob list
	r := callPkg(t, p, "htmx", runtime.Str("<li>item</li>"), row(
		"oob", runtime.List(
			runtime.Str(oob.S),
			row("id", "count", "html", "3"),
		),
		"cookie", callPkg(t, p, "cookie", runtime.Str("flash"), runtime.Str("1")),
	))
	body := extractHTTPBody(r)
	if !strings.Contains(body, "item") || !strings.Contains(body, "hx-swap-oob") {
		t.Fatal(body)
	}
	if _, ok := mapGet(r, "cookies"); !ok {
		t.Fatal("cookies on response")
	}
}

func TestWeb_BeforeMiddleware(t *testing.T) {
	env := envWithCall()
	app := &webApp{env: env}
	// auth before
	app.befores = append(app.befores, runtime.MakeBuiltin("auth", 1, func(args []runtime.Value) (runtime.Value, error) {
		req := args[0]
		ck := mustMapGet(t, req, "cookies")
		if mapGetStr(ck, "sid", "") == "" {
			return respMap(401, "login", "text/plain"), nil
		}
		return runtime.Null(), nil
	}))
	app.routes = append(app.routes, webRoute{
		method: "GET", pattern: "/secret", parts: parsePattern("/secret"),
		handler: runtime.MakeBuiltin("h", 1, func(args []runtime.Value) (runtime.Value, error) {
			return respMap(200, "secret", "text/plain"), nil
		}),
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/secret", nil))
	if rr.Code != 401 {
		t.Fatalf("want 401 got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/secret", nil)
	req.Header.Set("Cookie", "sid=ok")
	app.ServeHTTP(rr, req)
	if rr.Code != 200 || rr.Body.String() != "secret" {
		t.Fatal(rr.Code, rr.Body.String())
	}

	// package surface
	p := packageWeb(env)
	val := newWebApp(env)
	_ = callMap(t, val, "before", runtime.MakeBuiltin("b", 1, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Null(), nil
	}))
	_ = p
}
