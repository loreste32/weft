package stdlib_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

// Native HTTP limits must be explicit: over-limit bodies are errors, never
// silent truncation, and the boundary itself must work.

func TestHTTPResponseBodyTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		chunk := bytes.Repeat([]byte("x"), 1<<20)
		for i := 0; i < 33; i++ { // 33 MiB > 32 MiB limit
			_, _ = w.Write(chunk)
		}
	}))
	defer srv.Close()

	// Inspect the Result explicitly: over-limit must be Err, never truncated Ok.
	src := fmt.Sprintf(`
fn main {
    r := http.get(%q)
    if r.is_err { println("err: ${r.err.message}") } else { println("ok len=${len(r.value.body)}") }
}
`, srv.URL)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "limits.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "err:") || !strings.Contains(s, "too large") {
		t.Fatalf("want explicit too-large error, got: %q", s)
	}
	if strings.Contains(s, "ok len=") {
		t.Fatalf("over-limit body must not succeed: %q", s)
	}
}

func TestHTTPResponseBodyAtLimitSucceeds(t *testing.T) {
	const size = 32 << 20 // exactly the limit
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := bytes.Repeat([]byte("y"), 1<<20)
		for i := 0; i < size/(1<<20); i++ {
			_, _ = w.Write(chunk)
		}
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main {
    r := http.get(%q)
    if r.is_err { println("err: ${r.err.message}") } else { println("ok len=${len(r.value.body)}") }
}
`, srv.URL)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "limits.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, fmt.Sprintf("ok len=%d", size)) {
		t.Fatalf("at-limit body must succeed with exact length, got: %q", s)
	}
}

func TestHTTPRequestBodyTooLarge(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main {
    body := str.repeat("x", 33554433)
    r := http.post(%q, body)
    if r.is_err { println("err: ${r.err.message}") } else { println("unexpected ok") }
}
`, srv.URL)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "limits.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "err:") || !strings.Contains(s, "exceeds") {
		t.Fatalf("want explicit request-body-limit error, got: %q", s)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("over-limit request must be rejected before send; server saw %d request(s)", n)
	}
}
