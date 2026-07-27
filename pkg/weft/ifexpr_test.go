package weft_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestIfExprValue(t *testing.T) {
	src := `
fn main {
    x := if true { 1 } else { 2 }
    y := if false { "a" } else if true { "b" } else { "c" }
    z := if x == 1 { "ok" } else { "no" }
    say(x)
    say(y)
    say(z)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "if.weft", src); err != nil {
		t.Fatal(err)
	}
	s := strings.TrimSpace(out.String())
	if !strings.Contains(s, "1") || !strings.Contains(s, "b") || !strings.Contains(s, "ok") {
		t.Fatalf("got %q", s)
	}
}

func TestIfExprBindEnvDefault(t *testing.T) {
	// non-verbose env default pattern
	src := `
fn main {
    v := env.get("WEFT_IFEXPR_TEST_MISSING")
    base := if v == null || v == "" { "http://127.0.0.1" } else { v }
    say(base)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "if2.weft", src); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "http://127.0.0.1") {
		t.Fatal(out.String())
	}
}
