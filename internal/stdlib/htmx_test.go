package stdlib

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func mustMapGet(t *testing.T, m runtime.Value, key string) runtime.Value {
	t.Helper()
	v, ok := mapGet(m, key)
	if !ok {
		t.Fatalf("missing %s", key)
	}
	return v
}

func TestHTMX_RequestEnrichment(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/click?x=1", strings.NewReader("name=Ada"))
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "result")
	req.Header.Set("HX-Trigger", "btn")
	req.Header.Set("HX-Trigger-Name", "save")
	req.Header.Set("HX-Current-URL", "http://localhost/")
	req.Header.Set("HX-Prompt", "sure?")
	req.Header.Set("HX-Boosted", "true")

	v := buildRequest(req, "name=Ada", map[string]string{"id": "1"})
	if !reqIsHTMX(v) {
		t.Fatal("expected htmx")
	}
	hx := mustMapGet(t, v, "htmx")
	if !mustMapGet(t, hx, "request").B {
		t.Fatal()
	}
	if mapGetStr(hx, "target", "") != "result" {
		t.Fatal(hx)
	}
	if mapGetStr(hx, "trigger", "") != "btn" {
		t.Fatal()
	}
	if mapGetStr(hx, "trigger_name", "") != "save" {
		t.Fatal()
	}
	if mapGetStr(hx, "prompt", "") != "sure?" {
		t.Fatal()
	}
	if !mustMapGet(t, hx, "boosted").B {
		t.Fatal()
	}

	// plain request
	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	pv := buildRequest(plain, "", nil)
	if reqIsHTMX(pv) {
		t.Fatal("plain")
	}
	hx2 := mustMapGet(t, pv, "htmx")
	if mustMapGet(t, hx2, "request").B {
		t.Fatal()
	}
}

func TestHTMX_PackageHelpers(t *testing.T) {
	env := envWithCall()
	p := packageWeb(env)

	// is_htmx
	if callPkg(t, p, "is_htmx").B {
		t.Fatal()
	}
	req := runtime.NewMap()
	// via headers fallback
	h := runtime.NewMap()
	ho := h.Obj.(*runtime.MapObj)
	ho.Keys = []string{"HX-Request"}
	ho.Vals["HX-Request"] = runtime.Str("true")
	ro := req.Obj.(*runtime.MapObj)
	ro.Keys = []string{"headers"}
	ro.Vals["headers"] = h
	if !callPkg(t, p, "is_htmx", req).B {
		t.Fatal("headers fallback")
	}

	// htmx html + opts
	r := callPkg(t, p, "htmx", runtime.Str("<div>ok</div>"), row(
		"trigger", "done",
		"retarget", "#box",
		"reswap", "outerHTML",
		"push_url", "/done",
		"status", 201,
	))
	if mapGetInt(r, "status", 0) != 201 {
		t.Fatal(r)
	}
	if extractHTTPBody(r) != "<div>ok</div>" {
		t.Fatal(r)
	}
	hdrs := mustMapGet(t, r, "headers")
	if mapGetStr(hdrs, "HX-Trigger", "") != "done" {
		t.Fatal(hdrs)
	}
	if mapGetStr(hdrs, "HX-Retarget", "") != "#box" {
		t.Fatal()
	}
	if mapGetStr(hdrs, "HX-Reswap", "") != "outerHTML" {
		t.Fatal()
	}
	if mapGetStr(hdrs, "HX-Push-Url", "") != "/done" {
		t.Fatal()
	}

	// trigger map → JSON
	r2 := callPkg(t, p, "htmx", runtime.Str(""), row("trigger", row("saved", true)))
	h2 := mustMapGet(t, r2, "headers")
	trig := mapGetStr(h2, "HX-Trigger", "")
	if !strings.Contains(trig, "saved") {
		t.Fatal(trig)
	}

	// opts-only first arg
	r3 := callPkg(t, p, "htmx", row("html", "<p>x</p>", "refresh", true, "redirect", "/go"))
	h3 := mustMapGet(t, r3, "headers")
	if mapGetStr(h3, "HX-Refresh", "") != "true" {
		t.Fatal(h3)
	}
	if mapGetStr(h3, "HX-Redirect", "") != "/go" {
		t.Fatal()
	}
	if extractHTTPBody(r3) != "<p>x</p>" {
		t.Fatal(r3)
	}

	// dedicated helpers
	rd := callPkg(t, p, "htmx_redirect", runtime.Str("/home"))
	if mapGetStr(mustMapGet(t, rd, "headers"), "HX-Redirect", "") != "/home" {
		t.Fatal(rd)
	}
	rf := callPkg(t, p, "htmx_refresh")
	if mapGetStr(mustMapGet(t, rf, "headers"), "HX-Refresh", "") != "true" {
		t.Fatal(rf)
	}
	tr := callPkg(t, p, "htmx_trigger", runtime.Str("ping"), runtime.Str("<i>hi</i>"))
	if mapGetStr(mustMapGet(t, tr, "headers"), "HX-Trigger", "") != "ping" {
		t.Fatal(tr)
	}
	if extractHTTPBody(tr) != "<i>hi</i>" {
		t.Fatal(tr)
	}
	loc := callPkg(t, p, "htmx_location", runtime.Str("/page"))
	if mapGetStr(mustMapGet(t, loc, "headers"), "HX-Location", "") != "/page" {
		t.Fatal(loc)
	}
	loc2 := callPkg(t, p, "htmx_location", row("path", "/p", "target", "#main"))
	lhdr := mapGetStr(mustMapGet(t, loc2, "headers"), "HX-Location", "")
	if !strings.Contains(lhdr, "path") {
		t.Fatal(lhdr)
	}
	cdn := callPkg(t, p, "htmx_cdn")
	if !strings.Contains(cdn.S, "htmx.org") {
		t.Fatal(cdn)
	}
	cdn2 := callPkg(t, p, "htmx_cdn", runtime.Str("1.9.12"))
	if !strings.Contains(cdn2.S, "1.9.12") {
		t.Fatal(cdn2)
	}

	mustErr(t, callPkg(t, p, "htmx_redirect"))
	mustErr(t, callPkg(t, p, "htmx_trigger"))
	mustErr(t, callPkg(t, p, "htmx_location"))
}

func TestWeb_FormParse(t *testing.T) {
	// urlencoded
	req := httptest.NewRequest(http.MethodPost, "/g?src=q", strings.NewReader("name=Ada+Lovelace&city=London"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	v := buildRequest(req, "name=Ada+Lovelace&city=London", nil)
	form := mustMapGet(t, v, "form")
	if mapGetStr(form, "name", "") != "Ada Lovelace" {
		t.Fatal(form)
	}
	if mapGetStr(form, "city", "") != "London" {
		t.Fatal()
	}
	if mapGetStr(form, "src", "") != "q" {
		t.Fatal("query should merge into form")
	}

	// web.form / form_get
	env := envWithCall()
	p := packageWeb(env)
	f := callPkg(t, p, "form", v)
	if mapGetStr(f, "name", "") != "Ada Lovelace" {
		t.Fatal(f)
	}
	if callPkg(t, p, "form_get", v, runtime.Str("name")).S != "Ada Lovelace" {
		t.Fatal()
	}
	if callPkg(t, p, "form_get", v, runtime.Str("missing"), runtime.Str("x")).S != "x" {
		t.Fatal()
	}
	if callPkg(t, p, "form").Kind != runtime.KindMap {
		t.Fatal()
	}

	// multipart
	var body strings.Builder
	w := multipart.NewWriter(&body)
	_ = w.WriteField("title", "hello")
	_ = w.WriteField("n", "3")
	_ = w.Close()
	raw := body.String()
	mreq := httptest.NewRequest(http.MethodPost, "/up", strings.NewReader(raw))
	mreq.Header.Set("Content-Type", w.FormDataContentType())
	mv := buildRequest(mreq, raw, nil)
	mf := mustMapGet(t, mv, "form")
	if mapGetStr(mf, "title", "") != "hello" || mapGetStr(mf, "n", "") != "3" {
		t.Fatal(mf)
	}
}

func TestHTMX_ServeHTTPEndToEnd(t *testing.T) {
	env := envWithCall()
	app := &webApp{env: env}
	app.routes = append(app.routes, webRoute{
		method: "POST", pattern: "/inc", parts: parsePattern("/inc"),
		handler: runtime.MakeBuiltin("inc", 1, func(args []runtime.Value) (runtime.Value, error) {
			req := args[0]
			if !reqIsHTMX(req) {
				return respMap(400, "need htmx", "text/plain"), nil
			}
			hx := mustMapGet(t, req, "htmx")
			tgt := mapGetStr(hx, "target", "")
			return htmxResponse([]runtime.Value{
				runtime.Str(`<span id="n">42</span>`),
				mapStrPairs("trigger", "updated", "retarget", "#"+tgt),
			})
		}),
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/inc", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "n")
	app.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatal(rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "42") {
		t.Fatal(rr.Body.String())
	}
	if rr.Header().Get("HX-Trigger") != "updated" {
		t.Fatal(rr.Header())
	}
	if rr.Header().Get("HX-Retarget") != "#n" {
		t.Fatal(rr.Header())
	}

	// full page without HX → 400
	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/inc", nil))
	if rr.Code != 400 {
		t.Fatal(rr.Code)
	}
}
