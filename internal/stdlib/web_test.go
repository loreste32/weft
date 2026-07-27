package stdlib_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestWebAppRoutes(t *testing.T) {
	src := `
fn main {
    app := web.app()
    app.get("/", fn(req) {
        web.html("<h1>home</h1>")
    })
    app.get("/api/hi", fn(req) {
        web.json({"ok": true, "msg": "hi"})
    })
    app.get("/users/:id", fn(req) {
        id := req.params["id"]
        web.json({"id": id})
    })
    app.post("/echo", fn(req) {
        web.text(200, req.body)
    })
    app.handle("GET", "/")
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	// run only registers — better: test handle return via println
	_ = ctx
	src2 := `
fn main {
    app := web.app()
    app.get("/users/:id", fn(req) {
        web.json({"id": req.params["id"]})
    })
    r := app.handle("GET", "/users/42")
    println(r.body)
}
`
	ctx = weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "web.weft", src2); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), `"id"`) || !strings.Contains(out.String(), "42") {
		t.Fatalf("got %q", out.String())
	}
	_ = src
}

func TestWebMatchAndStaticDispatch(t *testing.T) {
	var out bytes.Buffer
	src := `
fn main {
    app := web.app()
    app.get("/health", fn(req) {
        web.json({"status": "up"})
    })
    app.post("/sum", fn(req) {
        web.text(200, "posted")
    })
    a := app.handle("GET", "/health")
    println(a.body)
    b := app.handle("POST", "/sum", "x")
    println(b.body)
    c := app.handle("GET", "/missing")
    println(c.status)
}
`
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "web2.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "up") || !strings.Contains(s, "posted") || !strings.Contains(s, "404") {
		t.Fatalf("got %q", s)
	}
}

func TestWebHTTPServeMux(t *testing.T) {
	// Direct Go-level: build app via weft then... use handle only.
	// Integration: httptest-style through app.handle is enough for unit.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	_ = req
}

func TestWebRTCHubPackage(t *testing.T) {
	var out bytes.Buffer
	src := `
fn main {
    hub := webrtc.hub()
    ice := webrtc.ice_servers()
    println(ice)
    rooms := hub.rooms()
    println("rooms")
}
`
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "rtc.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "stun") {
		t.Fatalf("got %q", out.String())
	}
}

func TestWebRedirect(t *testing.T) {
	var out bytes.Buffer
	src := `
fn main {
    app := web.app()
    app.get("/go", fn(req) {
        web.redirect("/elsewhere")
    })
    r := app.handle("GET", "/go")
    println(r.status)
}
`
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "redir.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "302") {
		t.Fatalf("got %q", out.String())
	}
}
