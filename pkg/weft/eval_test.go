package weft

import (
	"strings"
	"testing"
)

func TestEval(t *testing.T) {
	ctx := New(Options{})
	v, err := ctx.Eval("1 + 2")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "3" {
		t.Fatalf("eval: %s", v.String())
	}
}

func TestEvalLetAndReuse(t *testing.T) {
	ctx := New(Options{})
	_, err := ctx.Eval("x := 21")
	if err != nil {
		t.Fatal(err)
	}
	v, err := ctx.Eval("x * 2")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "42" {
		t.Fatalf("eval reuse: %s", v.String())
	}
}

func TestEvalMultiLine(t *testing.T) {
	ctx := New(Options{})
	v, err := ctx.Eval("let mut x = 1\nx = x + 1\nx")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "2" {
		t.Fatalf("multiline: %s", v.String())
	}
}

func TestEvalError(t *testing.T) {
	ctx := New(Options{})
	_, err := ctx.Eval("1 / 0")
	if err == nil {
		t.Fatal("should error")
	}
}

func TestEvalString(t *testing.T) {
	s, err := EvalString("1 + 2")
	if err != nil {
		t.Fatal(err)
	}
	if s != "3" {
		t.Fatalf("EvalString: %q", s)
	}
}

func TestEvalStringExpr(t *testing.T) {
	s, err := EvalString(`"hello"`)
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello" {
		t.Fatalf("EvalString str: %q", s)
	}
}

func TestIsLikelyExpr(t *testing.T) {
	exprs := []string{"1 + 2", `"hello"`, "true", "null", "foo(1)", "[1,2]", `{"a": 1}`}
	for _, s := range exprs {
		if !isLikelyExpr(s) {
			t.Errorf("should be expr: %q", s)
		}
	}
	stmts := []string{"fn main {}", "if true { 1 }", "for x in [1] { say(x) }"}
	for _, s := range stmts {
		if isLikelyExpr(s) {
			t.Errorf("should not be expr: %q", s)
		}
	}
}

func TestLooksLikeProgram(t *testing.T) {
	if !looksLikeProgram("fn main { 1 }") {
		t.Fatal("fn main is a program")
	}
	if looksLikeProgram("1 + 2") {
		t.Fatal("expr is not a program")
	}
	if !looksLikeProgram("use json\nfn main { 1 }") {
		t.Fatal("import + fn is a program")
	}
}

func TestEvalParseError(t *testing.T) {
	ctx := New(Options{})
	_, err := ctx.Eval("@@@")
	if err == nil {
		t.Fatal("should error on bad syntax")
	}
}

func TestContextRunSource(t *testing.T) {
	ctx := New(Options{})
	err := ctx.RunSource(nil, "test.weft", `fn main { say("ok") }`)
	// nil context should work (defaults to background)
	_ = err
}

func TestContextRunSourceError(t *testing.T) {
	ctx := New(Options{})
	err := ctx.RunSource(nil, "test.weft", `fn main { 1 / 0 }`)
	if err == nil {
		t.Fatal("should error")
	}
	if !strings.Contains(err.Error(), "division") {
		t.Fatal(err)
	}
}
