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

// expose reset via same-package retest — use Integration through public API only.
// Circuit state is process-global; isolate by unique host (httptest unique ports).

func TestHTTPCircuitOpensAfterThreshold(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(503)
	}))
	defer srv.Close()

	// threshold 2: two failures open the circuit
	src := fmt.Sprintf(`
fn main {
    // fail twice → circuit opens
    r1 := http.get(%q, {"circuit": true, "circuit_threshold": 2, "circuit_cooldown_ms": 5000, "retries": 0})
    r2 := http.get(%q, {"circuit": true, "circuit_threshold": 2, "circuit_cooldown_ms": 5000, "retries": 0})
    // third should be fail-fast without hitting server
    r3 := http.get(%q, {"circuit": true, "circuit_threshold": 2, "circuit_cooldown_ms": 5000, "retries": 0})
    println(r1.ok)
    println(r2.ok)
    println(r3.ok)
    if !r3.ok {
        println(r3.err.message)
    }
}
`, srv.URL, srv.URL, srv.URL)

	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "circuit.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	// first two hit server (503 still returns Ok Result with response); third is circuit open Err
	// Actually 503 with retries=0 returns Ok(Response) and records failure
	// After 2 failures, third is Err with circuit open
	if hits.Load() != 2 {
		t.Fatalf("hits=%d want 2 (third short-circuited)", hits.Load())
	}
	s := out.String()
	if !strings.Contains(s, "circuit open") {
		t.Fatalf("want circuit open message, got %q", s)
	}
}

func TestHTTPCircuitSuccessResets(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main {
    _ := http.get(%q, {"circuit": true, "circuit_threshold": 5, "retries": 0})
    r := http.get(%q, {"circuit": true, "circuit_threshold": 5, "retries": 0})?
    println(r.body)
}
`, srv.URL, srv.URL)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "creset.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatal(out.String())
	}
}
