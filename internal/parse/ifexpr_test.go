package parse_test

import (
	"testing"

	"github.com/loreste/weft/internal/parse"
)

func TestParseIfExpr(t *testing.T) {
	src := `fn main {
    x := if true { 1 } else { 2 }
    y := if false { "a" } else if true { "b" } else { "c" }
}`
	f, errs := parse.ParseFile("if.weft", src)
	if errs.HasErrors() {
		t.Fatal(errs)
	}
	if f == nil || len(f.Decls) == 0 {
		t.Fatal("no decls")
	}
}

func TestParseIfExprInCall(t *testing.T) {
	src := `fn main { say(if true { "ok" } else { "no" }) }`
	_, errs := parse.ParseFile("c.weft", src)
	if errs.HasErrors() {
		t.Fatal(errs)
	}
}
