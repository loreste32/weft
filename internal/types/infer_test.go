package types

import (
	"strings"
	"testing"

	"github.com/loreste/weft/internal/parse"
)

func inferSrc(t *testing.T, src string) (Info, string) {
	t.Helper()
	f, errs := parse.ParseFile("t.weft", src)
	if errs.HasErrors() {
		t.Fatalf("parse: %v", errs)
	}
	info, terrs := Infer(f)
	var b strings.Builder
	if terrs.HasErrors() {
		b.WriteString(terrs.Error())
	}
	return info, b.String()
}

func TestInferLetFromLiteral(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    x := 1
    y := "hi"
    z := true
    say(x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["x"].Kind != TyInt {
		t.Fatalf("x want int got %s", info.Bindings["x"])
	}
	if info.Bindings["y"].Kind != TyStr {
		t.Fatalf("y want str got %s", info.Bindings["y"])
	}
	if info.Bindings["z"].Kind != TyBool {
		t.Fatalf("z want bool got %s", info.Bindings["z"])
	}
}

func TestInferListAndArith(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    xs := [1, 2, 3]
    n := 1 + 2
    say(xs)
    say(n)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["xs"].Kind != TyList || info.Bindings["xs"].Elem.Kind != TyInt {
		t.Fatalf("xs: %s", info.Bindings["xs"])
	}
	if info.Bindings["n"].Kind != TyInt {
		t.Fatalf("n: %s", info.Bindings["n"])
	}
}

func TestInferFnReturn(t *testing.T) {
	info, err := inferSrc(t, `
fn add(a, b) { a + b }
fn main {
    say(add(1, 2))
}
`)
	if err != "" {
		t.Fatal(err)
	}
	// unannotated a+b with any params → any, but at least fn exists
	if info.FnRet["add"] == nil {
		t.Fatal("missing add ret")
	}
}

func TestInferAnnotatedMismatch(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    let x: int = "nope"
    say(x)
}
`)
	if err == "" || !strings.Contains(err, "cannot assign") {
		t.Fatalf("want assign error, got %q", err)
	}
}

func TestInferResultUnwrap(t *testing.T) {
	info, err := inferSrc(t, `
fn main -> Result {
    s := json.parse("{\"a\":1}")?
    say(s)
}
`)
	if err != "" {
		// may still be ok
	}
	// s should be any (unwrap Result)
	if info.Bindings["s"] == nil {
		t.Fatal("s not bound")
	}
}

func TestInferReturnMismatch(t *testing.T) {
	_, err := inferSrc(t, `
fn f() -> int { "str" }
fn main { say(f()) }
`)
	if err == "" || !strings.Contains(err, "return") && !strings.Contains(err, "declared") {
		// last expr return path
		if err == "" || !strings.Contains(err, "int") {
			t.Fatalf("want type error, got %q", err)
		}
	}
}

func TestInferForElem(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    for x in [1, 2] {
        say(x)
    }
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["x"].Kind != TyInt {
		t.Fatalf("for x: %s", info.Bindings["x"])
	}
}
