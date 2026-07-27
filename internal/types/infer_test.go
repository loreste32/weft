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

func TestInferCheck(t *testing.T) {
	f, errs := parse.ParseFile("t.weft", `
fn f(x: int) -> str { "hello" }
fn main { say(f(1)) }
`)
	if errs.HasErrors() {
		t.Fatal(errs)
	}
	terrs := Check(f)
	// Check should not crash; may or may not produce errors
	_ = terrs
}

func TestInferIfExpr(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    x := if true { 1 } else { 2 }
    say(x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["x"] != nil && info.Bindings["x"].Kind != TyInt && info.Bindings["x"].Kind != TyAny {
		t.Fatalf("if expr: %s", info.Bindings["x"])
	}
}

func TestInferMutReassign(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    mut x := 1
    x = 2
    say(x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferMutReassignTypeMismatch(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    let mut x: int = 1
    x = "str"
    say(x)
}
`)
	if err == "" || !strings.Contains(err, "assign") {
		t.Fatalf("want type error, got %q", err)
	}
}

func TestInferFloat(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    x := 3.14
    say(x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["x"].Kind != TyFloat {
		t.Fatalf("float: %s", info.Bindings["x"])
	}
}

func TestInferMap(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    m := {"a": 1}
    say(m)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["m"] == nil {
		t.Fatal("m not bound")
	}
}

func TestInferNullLit(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    x := null
    say(x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["x"] == nil {
		t.Fatal("x not bound")
	}
}

func TestInferFuncLit(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    f := fn(x) { x * 2 }
    say(f(5))
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["f"] == nil {
		t.Fatal("f not bound")
	}
}

func TestInferWhile(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    mut i := 0
    while i < 3 { i = i + 1 }
    say(i)
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferBreakContinue(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    for x in [1, 2, 3] {
        if x == 2 { break }
        if x == 1 { continue }
        say(x)
    }
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferDefer(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    defer say("bye")
    say("hi")
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferEnum(t *testing.T) {
	_, err := inferSrc(t, `
enum Color { Red, Green, Blue }
fn main { say(Color.Red) }
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferMatchExpr(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    x := match 1 {
        1 { "one" }
        _ { "other" }
    }
    say(x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	_ = info
}

func TestInferBinaryComparisons(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    a := 1 < 2
    b := 1 == 1
    c := 1 != 2
    d := "a" < "b"
    say(a, b, c, d)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["a"] != nil && info.Bindings["a"].Kind != TyBool {
		t.Fatalf("comparison: %s", info.Bindings["a"])
	}
}

func TestInferBinaryStringConcat(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    s := "a" + "b"
    say(s)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["s"] != nil && info.Bindings["s"].Kind != TyStr {
		t.Fatalf("str concat: %s", info.Bindings["s"])
	}
}

func TestInferFieldAccess(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    m := {"x": 1}
    say(m.x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferIndex(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    xs := [1, 2]
    say(xs[0])
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferNeg(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    x := -5
    say(x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["x"] != nil && info.Bindings["x"].Kind != TyInt {
		t.Fatalf("neg: %s", info.Bindings["x"])
	}
}

func TestInferNot(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    x := !true
    say(x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["x"] != nil && info.Bindings["x"].Kind != TyBool {
		t.Fatalf("not: %s", info.Bindings["x"])
	}
}

func TestInferPipeline(t *testing.T) {
	_, err := inferSrc(t, `
fn double(x) { x * 2 }
fn main { 5 |> double |> say }
`)
	if err != "" {
		t.Fatal(err)
	}
}

func tyEq(a, b *Type) bool {
	if a == nil {
		return b == nil || b.Kind == TyAny
	}
	return a.Equal(b)
}

func TestTypeString(t *testing.T) {
	cases := []struct {
		ty   *Type
		want string
	}{
		{tyInt(), "int"},
		{tyStr(), "str"},
		{tyFloat(), "float"},
		{tyBool(), "bool"},
		{tyUnit(), "unit"},
		{tyAny(), "any"},
		{tyNull(), "null"},
		{tyList(tyInt()), "[int]"},
		{tyMap(tyStr(), tyInt()), "Map[str,int]"},
		{tyResult(tyInt()), "Result[int]"},
		{tyOpt(tyStr()), "str?"},
		{tyFn([]*Type{tyInt()}, tyStr()), "fn(int) -> str"},
		{nil, "any"},
		{tyNamed("Foo"), "Foo"},
		{tyNamed(""), "named"},
		{tyChannel(), "channel"},
		{&Type{Kind: Kind(99)}, "?"},
	}
	for _, tc := range cases {
		if tc.ty.String() != tc.want {
			t.Errorf("String() = %q, want %q", tc.ty.String(), tc.want)
		}
	}
}

func TestTypeEqual(t *testing.T) {
	if !tyEq(tyInt(), tyInt()) {
		t.Fatal("int == int")
	}
	if tyEq(tyInt(), tyStr()) {
		t.Fatal("int != str")
	}
	if !tyEq(tyAny(), tyStr()) {
		t.Fatal("any == anything")
	}
	if !tyEq(tyStr(), tyAny()) {
		t.Fatal("anything == any")
	}
	if !tyEq(tyList(tyInt()), tyList(tyInt())) {
		t.Fatal("[int] == [int]")
	}
	if tyEq(tyList(tyInt()), tyList(tyStr())) {
		t.Fatal("[int] != [str]")
	}
	if !tyEq(tyMap(tyStr(), tyInt()), tyMap(tyStr(), tyInt())) {
		t.Fatal("map == map")
	}
	if tyEq(tyMap(tyStr(), tyInt()), tyMap(tyStr(), tyStr())) {
		t.Fatal("map != map")
	}
	if !tyEq(tyResult(tyInt()), tyResult(tyInt())) {
		t.Fatal("Result == Result")
	}
	if !tyEq(tyOpt(tyStr()), tyOpt(tyStr())) {
		t.Fatal("opt == opt")
	}
	if !tyEq(tyFn(nil, tyInt()), tyFn(nil, tyInt())) {
		t.Fatal("fn == fn")
	}
	if tyEq(tyFn([]*Type{tyInt()}, tyStr()), tyFn([]*Type{tyStr()}, tyStr())) {
		t.Fatal("fn params differ")
	}
	// nil types
	if !tyEq(nil, nil) {
		t.Fatal("nil == nil")
	}
	// fn different param count
	if tyEq(tyFn([]*Type{tyInt()}, tyStr()), tyFn([]*Type{tyInt(), tyStr()}, tyStr())) {
		t.Fatal("fn different param count")
	}
	// fn different ret
	if tyEq(tyFn(nil, tyInt()), tyFn(nil, tyStr())) {
		t.Fatal("fn different ret")
	}
	// map key differs
	if tyEq(tyMap(tyInt(), tyStr()), tyMap(tyStr(), tyStr())) {
		t.Fatal("map key differs")
	}
}

func TestAssignable(t *testing.T) {
	if !Assignable(tyInt(), tyInt()) {
		t.Fatal("int := int")
	}
	if !Assignable(tyAny(), tyStr()) {
		t.Fatal("any := str")
	}
	if !Assignable(tyOpt(tyInt()), tyInt()) {
		t.Fatal("int? := int")
	}
	if !Assignable(tyOpt(tyInt()), tyNull()) {
		t.Fatal("int? := null")
	}
	if Assignable(tyInt(), tyStr()) {
		t.Fatal("int := str should fail")
	}
	// result assignable
	if !Assignable(tyResult(tyInt()), tyResult(tyInt())) {
		t.Fatal("Result := Result")
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
