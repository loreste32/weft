package parse

import (
	"testing"

	"github.com/loreste/weft/internal/ast"
)

func TestParseHello(t *testing.T) {
	src := `fn main() {
    println("hello, weft")
}`
	f, errs := ParseFile("hello.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(f.Decls) != 1 {
		t.Fatalf("decls: %d", len(f.Decls))
	}
	fn, ok := f.Decls[0].(*ast.FnDecl)
	if !ok || fn.Name != "main" {
		t.Fatalf("want main fn, got %#v", f.Decls[0])
	}
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("stmts: %d", len(fn.Body.Stmts))
	}
}

func TestParseLetIf(t *testing.T) {
	src := `
fn add(a: int, b: int) -> int {
    let mut x = a + b
    if x > 0 {
        return x
    } else {
        return 0
    }
}
`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	fn := f.Decls[0].(*ast.FnDecl)
	if len(fn.Params) != 2 || fn.Ret == nil {
		t.Fatalf("params/ret: %#v", fn)
	}
}

func TestParseImportAndType(t *testing.T) {
	src := `
import http
import "./util.weft" as util
type Point {
    x: int
    y: int
}
type UserId = str
fn main() {}
`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(f.Decls) != 5 {
		t.Fatalf("decls %d", len(f.Decls))
	}
}

func TestParseFString(t *testing.T) {
	src := `fn main() { println(f"hi {name}") }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	fn := f.Decls[0].(*ast.FnDecl)
	es := fn.Body.Stmts[0].(*ast.ExprStmt)
	call := es.X.(*ast.CallExpr)
	if _, ok := call.Args[0].(*ast.FStringExpr); !ok {
		t.Fatalf("want FStringExpr, got %#v", call.Args[0])
	}
}
