package weft_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestDeferLIFOAndReturn(t *testing.T) {
	src := `
fn main {
    defer println("a")
    defer println("b")
    println("body")
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "defer.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	got := strings.TrimSpace(out.String())
	// body first, then LIFO defers
	if got != "body\nb\na" {
		t.Fatalf("got %q", got)
	}
}

func TestDeferRunsOnEarlyReturn(t *testing.T) {
	src := `
fn cleanup(msg) {
    println(msg)
}

fn work() {
    defer cleanup("done")
    return 1
}

fn main {
    say(work())
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "defer2.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "done") || !strings.Contains(got, "1") {
		t.Fatalf("got %q", got)
	}
}

func TestDeferRequiresCall(t *testing.T) {
	src := `
fn main {
    defer 1
}
`
	ctx := weft.New(weft.Options{})
	err := ctx.RunSource(context.Background(), "defer3.weft", src)
	if err == nil || !strings.Contains(err.Error(), "call expression") {
		t.Fatalf("want call expression error, got %v", err)
	}
}

func TestCountValueAndPred(t *testing.T) {
	src := `
fn main {
    say(count([1, 1, 2, 1], 1))
    say(count([1, 2, 3, 4], fn(x) { x > 2 }))
    say(count([9, 8]))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "count.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || lines[0] != "3" || lines[1] != "2" || lines[2] != "2" {
		t.Fatalf("got %q", out.String())
	}
}

func TestTryRecv(t *testing.T) {
	src := `
fn main {
    ch := channel(1)
    empty := try_recv(ch)?
    say(empty.ok)
    send(ch, 7)?
    got := try_recv(ch)?
    say(got.ok)
    say(got.value)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "tryrecv.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	got := strings.TrimSpace(out.String())
	if got != "false\ntrue\n7" {
		t.Fatalf("got %q", got)
	}
}

func TestEvalStringAndCheckFile(t *testing.T) {
	s, err := weft.EvalString("1 + 2")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(s) != "3" {
		t.Fatalf("EvalString: %q", s)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "ok.weft")
	if err := os.WriteFile(path, []byte("fn main { println(1) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := weft.CheckFile(path); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.weft")
	if err := os.WriteFile(bad, []byte("fn main { println(nope) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := weft.CheckFile(bad); err == nil {
		t.Fatal("expected CheckFile error")
	}
}

func TestContextEnv(t *testing.T) {
	ctx := weft.New(weft.Options{})
	if ctx.Env() == nil {
		t.Fatal("Env nil")
	}
	if ctx.EnvContext() == nil {
		t.Fatal("EnvContext nil")
	}
}
