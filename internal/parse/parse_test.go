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

func TestParseEnum(t *testing.T) {
	src := `enum Color { Red, Green, Blue }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	ed, ok := f.Decls[0].(*ast.EnumDecl)
	if !ok {
		t.Fatal("expected enum")
	}
	if ed.Name != "Color" || len(ed.Variants) != 3 {
		t.Fatalf("enum: %+v", ed)
	}
}

func TestParseConst(t *testing.T) {
	src := `
const PI = 3
fn main { say(PI) }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if _, ok := f.Decls[0].(*ast.ConstDecl); !ok {
		t.Fatal("expected const")
	}
}

func TestParsePub(t *testing.T) {
	src := `pub fn helper { 1 }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	fn := f.Decls[0].(*ast.FnDecl)
	if !fn.Pub {
		t.Fatal("should be pub")
	}
}

func TestParsePubType(t *testing.T) {
	src := `pub type Foo { x: int }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	td := f.Decls[0].(*ast.TypeDecl)
	if !td.Pub {
		t.Fatal("should be pub")
	}
}

func TestParsePubEnum(t *testing.T) {
	src := `pub enum Dir { Up, Down }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	ed := f.Decls[0].(*ast.EnumDecl)
	if !ed.Pub {
		t.Fatal("should be pub")
	}
}

func TestParsePubInvalid(t *testing.T) {
	src := `pub let x = 1`
	_, errs := ParseFile("t.weft", src)
	if !errs.HasErrors() {
		t.Fatal("pub let should error")
	}
}

func TestParseUseAlias(t *testing.T) {
	src := `use http as h
fn main { say(1) }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	imp := f.Decls[0].(*ast.ImportDecl)
	if imp.Alias != "h" {
		t.Fatal("alias")
	}
}

func TestParseFor(t *testing.T) {
	src := `fn main {
    for x in [1, 2] {
        say(x)
    }
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	fn := f.Decls[0].(*ast.FnDecl)
	if _, ok := fn.Body.Stmts[0].(*ast.ForStmt); !ok {
		t.Fatal("expected for")
	}
}

func TestParseWhile(t *testing.T) {
	src := `fn main {
    while true { break }
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	fn := f.Decls[0].(*ast.FnDecl)
	if _, ok := fn.Body.Stmts[0].(*ast.WhileStmt); !ok {
		t.Fatal("expected while")
	}
}

func TestParseDefer(t *testing.T) {
	src := `fn main {
    defer say("bye")
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	fn := f.Decls[0].(*ast.FnDecl)
	if _, ok := fn.Body.Stmts[0].(*ast.DeferStmt); !ok {
		t.Fatal("expected defer")
	}
}

func TestParseMatch(t *testing.T) {
	src := `fn main {
    x := match 1 {
        1 { "one" }
        _ { "other" }
    }
    say(x)
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	_ = f
}

func TestParseFuncLit(t *testing.T) {
	src := `fn main {
    f := fn(x) { x * 2 }
    say(f(5))
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	_ = f
}

func TestParsePipeline(t *testing.T) {
	src := `
fn double(x) { x * 2 }
fn main { 5 |> double |> say }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	_ = f
}

func TestParseNullCoalesce(t *testing.T) {
	src := `fn main {
    x := null
    say(x ?? 42)
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	_ = f
}

func TestParseQuestion(t *testing.T) {
	src := `fn f() -> Result {
    x := Ok(1)?
    x
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	_ = f
}

func TestParseResultType(t *testing.T) {
	src := `fn f() -> Result { 42 }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	fn := f.Decls[0].(*ast.FnDecl)
	if _, ok := fn.Ret.(*ast.ResultType); !ok {
		t.Fatal("expected Result type")
	}
}

func TestParseOptionalType(t *testing.T) {
	src := `type Foo {
    name: str?
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	td := f.Decls[0].(*ast.TypeDecl)
	if len(td.Fields) != 1 {
		t.Fatal("fields")
	}
}

func TestParseListType(t *testing.T) {
	src := `type Foo {
    items: [int]
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	_ = f
}

func TestParseMapType(t *testing.T) {
	src := `type Foo {
    data: Map[str, int]
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	_ = f
}

func TestParseSayStmt(t *testing.T) {
	src := `fn main { say "hello" }`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	_ = f
}

func TestParseStructLit(t *testing.T) {
	src := `
type Pt { x: int, y: int }
fn main {
    p := Pt{x: 1, y: 2}
    say(p)
}`
	f, errs := ParseFile("t.weft", src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	_ = f
}

func TestParseIllegalToken(t *testing.T) {
	src := `fn main { @invalid }`
	_, errs := ParseFile("t.weft", src)
	if !errs.HasErrors() {
		t.Fatal("should have errors")
	}
}

func TestParseInvalidDecl(t *testing.T) {
	src := `123`
	_, errs := ParseFile("t.weft", src)
	if !errs.HasErrors() {
		t.Fatal("number at top level should error")
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
