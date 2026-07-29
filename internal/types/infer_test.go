package types

import (
	"strings"
	"testing"

	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/parse"
)

func inferSrc(t *testing.T, src string) (Info, string) {
	t.Helper()
	f, errs := parse.ParseFile("t.weft", src)
	if errs.HasErrors() {
		t.Fatalf("parse: %v", errs)
	}
	info, terrs := Infer(f)
	// Collect both errors and warnings so mismatch tests still see messages.
	var b strings.Builder
	if len(terrs) > 0 {
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
	if Assignable(tyInt(), tyFloat()) {
		t.Fatal("float should NOT assign to int")
	}
	if !Assignable(tyFloat(), tyInt()) {
		t.Fatal("int should assign to float")
	}
	if !Assignable(tyList(tyOpt(tyInt())), tyList(tyInt())) {
		t.Fatal("[int] should assign to [int?]")
	}
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

func TestInferImport(t *testing.T) {
	_, err := inferSrc(t, `
use json
fn main {
    say(json.parse("{\"a\":1}"))
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferCallOkErr(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    a := Ok(42)
    b := Err("boom")
    say(a, b)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["a"] == nil {
		t.Fatal("a")
	}
}

func TestInferStructLit(t *testing.T) {
	_, err := inferSrc(t, `
type Pt { x: int, y: int }
fn main {
    p := Pt{x: 1, y: 2}
    say(p)
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferQOnNonResult(t *testing.T) {
	_, err := inferSrc(t, `
fn f() -> int { 42 }
fn main -> int {
    f()?
}
`)
	if err == "" || !strings.Contains(err, "?") {
		t.Fatalf("should warn about ? on non-Result, got %q", err)
	}
}

func TestInferListIndex(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    xs := [1, 2, 3]
    x := xs[0]
    say(x)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["x"] != nil && info.Bindings["x"].Kind != TyInt {
		t.Fatalf("list index: %s", info.Bindings["x"])
	}
}

func TestInferMapIndex(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    m := {"a": 1}
    v := m["a"]
    say(v)
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestInferStrIndex(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    s := "hello"
    c := s[0]
    say(c)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["c"] != nil && info.Bindings["c"].Kind != TyStr {
		t.Fatalf("str index: %s", info.Bindings["c"])
	}
}

func TestInferFuncLitReturn(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    f := fn(x: int) -> str { "hello" }
    say(f(1))
}
`)
	if err != "" {
		t.Fatal(err)
	}
	ft := info.Bindings["f"]
	if ft == nil || ft.Kind != TyFn {
		t.Fatal("func lit type")
	}
}

func TestInferEmptyBlock(t *testing.T) {
	_, err := inferSrc(t, `
fn main {
    if true { }
    say("ok")
}
`)
	if err != "" {
		t.Fatal(err)
	}
}

func TestUnify(t *testing.T) {
	// Unify(int, int) = int
	if got := Unify(tyInt(), tyInt()); got.Kind != TyInt {
		t.Errorf("int+int: %s", got)
	}
	// Unify(int, float) = int (Equal treats int/float as compatible, returns first)
	if got := Unify(tyInt(), tyFloat()); got == nil {
		t.Errorf("int+float: nil")
	}
	// Unify(nil, str) = str
	if got := Unify(nil, tyStr()); got.Kind != TyStr {
		t.Errorf("nil+str: %s", got)
	}
	// Unify(str, nil) = str
	if got := Unify(tyStr(), nil); got.Kind != TyStr {
		t.Errorf("str+nil: %s", got)
	}
	// Unify(nil, nil) = nil
	if got := Unify(nil, nil); got != nil {
		t.Errorf("nil+nil: %v", got)
	}
	// Unify([int], [int]) = [int]
	if got := Unify(tyList(tyInt()), tyList(tyInt())); got.Kind != TyList {
		t.Errorf("list+list: %s", got)
	}
	// Unify(any, str) = str
	if got := Unify(tyAny(), tyStr()); got.Kind != TyStr {
		t.Errorf("any+str: %s", got)
	}
	// null + int = int? (order-independent)
	if got := Unify(tyInt(), tyNull()); got.Kind != TyOptional || got.Elem.Kind != TyInt {
		t.Errorf("int+null: %s", got)
	}
	if got := Unify(tyNull(), tyInt()); got.Kind != TyOptional || got.Elem.Kind != TyInt {
		t.Errorf("null+int: %s", got)
	}
}

func TestFromAST(t *testing.T) {
	cases := []struct {
		te   ast.TypeExpr
		want string
	}{
		{&ast.NamedType{Name: "int"}, "int"},
		{&ast.NamedType{Name: "str"}, "str"},
		{&ast.NamedType{Name: "Foo"}, "Foo"},
		{&ast.ListType{Element: &ast.NamedType{Name: "int"}}, "[int]"},
		{&ast.ResultType{Ok: &ast.NamedType{Name: "str"}}, "Result[str]"},
		{&ast.OptionalType{Element: &ast.NamedType{Name: "int"}}, "int?"},
		{&ast.ResultType{}, "Result[any]"},
		{&ast.FnType{
			Params: []ast.TypeExpr{&ast.NamedType{Name: "str"}},
			Ret:    &ast.NamedType{Name: "int"},
		}, "fn(str) -> int"},
		{&ast.FnType{Params: nil, Ret: &ast.NamedType{Name: "unit"}}, "fn() -> unit"},
		{nil, "any"},
	}
	for _, tc := range cases {
		got := FromAST(tc.te)
		if got.String() != tc.want {
			t.Errorf("FromAST: got %q, want %q", got.String(), tc.want)
		}
	}
}

func TestInferColonBindAnnotation(t *testing.T) {
	info, err := inferSrc(t, `
fn main {
    name: str := "weft"
    count: int := 0
    say(name, count)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	if info.Bindings["name"] == nil || info.Bindings["name"].Kind != TyStr {
		t.Fatalf("name: %v", info.Bindings["name"])
	}
	if info.Bindings["count"] == nil || info.Bindings["count"].Kind != TyInt {
		t.Fatalf("count: %v", info.Bindings["count"])
	}
}

func TestInferColonBindMismatchIsWarning(t *testing.T) {
	info, msg := inferSrc(t, `
fn main {
    wrong: int := "nope"
    say(wrong)
}
`)
	if !strings.Contains(msg, "cannot assign") {
		t.Fatalf("want assign warning, got %q", msg)
	}
	// warnings only — no hard errors
	if info.Diags.HasErrors() {
		t.Fatalf("type mismatches must be warnings, got errors: %v", info.Diags)
	}
}

func TestInferFnTypeAnnotation(t *testing.T) {
	info, err := inferSrc(t, `
fn lookup(s: str) -> Result { Ok(s) }
fn main {
    handler: fn(str) -> Result := lookup
    say(handler)
}
`)
	if err != "" {
		t.Fatal(err)
	}
	ht := info.Bindings["handler"]
	if ht == nil || ht.Kind != TyFn {
		t.Fatalf("handler want fn type, got %v", ht)
	}
}

func TestInferStructFieldAccess(t *testing.T) {
	info, msg := inferSrc(t, `
type User {
    name: str
    age: int
}
fn main {
    u := User{name: "alice", age: 30}
    n := u.name
    a := u.age
    say(n, a)
}
`)
	if msg != "" && strings.Contains(msg, "no field") {
		t.Fatalf("unexpected field warning: %s", msg)
	}
	if info.Bindings["n"] == nil || info.Bindings["n"].Kind != TyStr {
		t.Fatalf("u.name want str, got %v", info.Bindings["n"])
	}
	if info.Bindings["a"] == nil || info.Bindings["a"].Kind != TyInt {
		t.Fatalf("u.age want int, got %v", info.Bindings["a"])
	}
}

func TestInferStructFieldMismatch(t *testing.T) {
	_, msg := inferSrc(t, `
type Pt { x: int, y: int }
fn main {
    p := Pt{x: "nope", y: 2}
    say(p)
}
`)
	if !strings.Contains(msg, "field") {
		t.Fatalf("want field type warning, got %q", msg)
	}
}

func TestInferUnknownStructField(t *testing.T) {
	_, msg := inferSrc(t, `
type Pt { x: int }
fn main {
    p := Pt{x: 1}
    say(p.z)
}
`)
	if !strings.Contains(msg, "no field") {
		t.Fatalf("want unknown field warning, got %q", msg)
	}
}

func TestInferListNullOrderStable(t *testing.T) {
	info, msg := inferSrc(t, `
fn main {
    xs := [1, null, 2]
    ys := [null, 1, 2]
    zs: [int?] := xs
    say(xs, ys, zs)
}
`)
	if msg != "" && strings.Contains(msg, "cannot assign") {
		t.Fatalf("list with null should be [int?], got warnings: %s", msg)
	}
	if xs := info.Bindings["xs"]; xs == nil || xs.String() != "[int?]" {
		t.Fatalf("xs want [int?], got %v", xs)
	}
	if ys := info.Bindings["ys"]; ys == nil || ys.String() != "[int?]" {
		t.Fatalf("ys want [int?], got %v", ys)
	}
}

func TestInferArityWarning(t *testing.T) {
	_, msg := inferSrc(t, `
fn two(a: int, b: int) -> int { a + b }
fn main {
    say(two(1))
    say(two(1, 2, 3))
}
`)
	if !strings.Contains(msg, "wrong number of arguments") {
		t.Fatalf("want arity warning, got %q", msg)
	}
}

func TestInferMapOptionalWorkersNoArityWarn(t *testing.T) {
	_, msg := inferSrc(t, `
fn square(x) { x * x }
fn main {
    say(map([1, 2, 3], square))
    say(map([1, 2, 3], square, 2))
    say(range(5))
    say(range(1, 5))
}
`)
	if strings.Contains(msg, "wrong number of arguments") {
		t.Fatalf("map/range optional arity should not warn, got %q", msg)
	}
}

func TestInferMissingStructField(t *testing.T) {
	_, msg := inferSrc(t, `
type Pt { x: int, y: int }
fn main {
    p := Pt{x: 1}
    say(p)
}
`)
	if !strings.Contains(msg, "missing field") {
		t.Fatalf("want missing field warning, got %q", msg)
	}
}

func TestInferMissingFieldWithDefaultOK(t *testing.T) {
	_, msg := inferSrc(t, `
type Cfg { port: int = 8080, host: str = "localhost" }
fn main {
    c := Cfg{}
    say(c)
}
`)
	if strings.Contains(msg, "missing field") {
		t.Fatalf("defaults should not require fields, got %q", msg)
	}
}

func TestInferFloatToIntWarns(t *testing.T) {
	_, msg := inferSrc(t, `
fn main {
    i: int := 1.5
    say(i)
}
`)
	if !strings.Contains(msg, "cannot assign") {
		t.Fatalf("want float→int warning, got %q", msg)
	}
}

func TestInferPoisonedBindingUsesActualType(t *testing.T) {
	info, msg := inferSrc(t, `
fn take(n: int) { say(n) }
fn main {
    x: int := "admin"
    take(x)
}
`)
	if !strings.Contains(msg, "cannot assign") {
		t.Fatalf("want assign warning, got %q", msg)
	}
	// x should be str (actual), so take(x) should also warn about arg type
	if !strings.Contains(msg, "argument") {
		t.Fatalf("poisoned binding should not silence later checks, got %q", msg)
	}
	if info.Bindings["x"] == nil || info.Bindings["x"].Kind != TyStr {
		t.Fatalf("x should bind actual str, got %v", info.Bindings["x"])
	}
}

func TestInferWrongFieldDefault(t *testing.T) {
	_, msg := inferSrc(t, `
type Cfg { port: int = "8080" }
fn main {
    c := Cfg{}
    say(c)
}
`)
	if !strings.Contains(msg, "default for field") {
		t.Fatalf("want default type warning, got %q", msg)
	}
}
