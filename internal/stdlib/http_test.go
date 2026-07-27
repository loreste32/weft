package stdlib_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loreste/weft/pkg/weft"
)

func TestHTTPRetriesOn503(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(503)
			_, _ = w.Write([]byte("busy"))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main {
    r := http.get(%q, {"retries": 5, "retry_ms": 10})?
    println(r.status)
    println(r.body)
}
`, srv.URL)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "retry.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if hits.Load() < 3 {
		t.Fatalf("want retries, hits=%d", hits.Load())
	}
	s := out.String()
	if !strings.Contains(s, "200") || !strings.Contains(s, "ok") {
		t.Fatal(s)
	}
}

func TestHTTPRetriesExhausted(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(503)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main {
    r := http.get(%q, {"retries": 2, "retry_ms": 5})
    // transport succeeded: Result is Ok, but response.ok is false
    println(r.ok)
    println(r.value.status)
    println(r.value.ok)
}
`, srv.URL)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "retry_fail.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	// 1 initial + 2 retries = 3
	if hits.Load() != 3 {
		t.Fatalf("hits=%d want 3", hits.Load())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 || lines[0] != "true" || lines[1] != "503" || lines[2] != "false" {
		t.Fatalf("want true/503/false, got %q", out.String())
	}
}

func TestHTTPPostFormMultipart(t *testing.T) {
	var gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("done"))
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main {
    r := http.post_form(%q, {"name": "weft", "n": "1"})?
    println(r.body)
}
`, srv.URL)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "form.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(gotCT, "multipart/form-data") {
		t.Fatalf("content-type %q", gotCT)
	}
	if !strings.Contains(gotBody, "name") || !strings.Contains(gotBody, "weft") {
		t.Fatalf("body %q", gotBody)
	}
	if !strings.Contains(out.String(), "done") {
		t.Fatal(out.String())
	}
}

func TestHTTPFetchWithFiles(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(fp, []byte("file-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	// escape backslashes for Weft string on Windows-ish paths
	esc := strings.ReplaceAll(fp, `\`, `\\`)

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(201)
		_, _ = w.Write([]byte("stored"))
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main {
    r := http.fetch({
        "url": %q,
        "method": "POST",
        "form": {"meta": "yes"},
        "files": {"upload": %q}
    })?
    println(r.status)
    println(r.body)
}
`, srv.URL, esc)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "files.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(gotBody, "file-bytes") || !strings.Contains(gotBody, "meta") {
		t.Fatalf("multipart body missing fields: %q", gotBody)
	}
	if !strings.Contains(out.String(), "201") {
		t.Fatal(out.String())
	}
}

func TestHTTPRetryDoesNotHang(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main {
    r := http.get(%q, {"retries": 2, "retry_ms": 20})
    println(r.ok)
}
`, srv.URL)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	start := time.Now()
	_ = ctx.RunSource(context.Background(), "hang.weft", src)
	if time.Since(start) > 3*time.Second {
		t.Fatal("retry hung too long")
	}
}
