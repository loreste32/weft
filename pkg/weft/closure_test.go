package weft_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestClosureCaptureByValue(t *testing.T) {
	src := `
fn main {
    x := 10
    f := fn() { x + 1 }
    say(f())

    mut n := 1
    g := fn() { n }
    n = 99
    // capture is by value at creation — still 1
    say(g())
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "c.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	got := strings.TrimSpace(out.String())
	if got != "11\n1" {
		t.Fatalf("got %q", got)
	}
}

func TestClosureInMapHandler(t *testing.T) {
	src := `
fn main {
    label := "ok"
    h := fn(x) { label + ":" + x }
    say(h("a"))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "c2.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if strings.TrimSpace(out.String()) != "ok:a" {
		t.Fatal(out.String())
	}
}
