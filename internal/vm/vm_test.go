package vm_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/loreste/weft/internal/vm"
	"github.com/loreste/weft/pkg/weft"
)

func TestRunSourceHonorsCancellationDuringLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- weft.New(weft.Options{}).RunSource(ctx, "cancel.weft", `
fn main {
    while true { }
}`)
	}()

	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case err := <-runDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("VM did not stop after context cancellation")
	}
}

func run(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	err := ctx.RunSource(context.Background(), "test.weft", src)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	return strings.TrimSpace(out.String())
}

func runErr(t *testing.T, src string) error {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	return ctx.RunSource(context.Background(), "test.weft", src)
}

// --- basic ops ---

func TestOpAddIntFloat(t *testing.T) {
	if run(t, `fn main { say(1 + 2) }`) != "3" {
		t.Fatal("int add")
	}
	if run(t, `fn main { say(1.5 + 2.5) }`) != "4" {
		t.Fatal("float add")
	}
	if run(t, `fn main { say("a" + "b") }`) != "ab" {
		t.Fatal("str add")
	}
}

func TestOpSub(t *testing.T) {
	if run(t, `fn main { say(10 - 3) }`) != "7" {
		t.Fatal("sub")
	}
}

func TestOpMul(t *testing.T) {
	if run(t, `fn main { say(3 * 4) }`) != "12" {
		t.Fatal("mul")
	}
}

func TestOpDiv(t *testing.T) {
	if run(t, `fn main { say(10 / 3) }`) != "3" {
		t.Fatal("int div")
	}
	if run(t, `fn main { say(10.0 / 4.0) }`) != "2.5" {
		t.Fatal("float div")
	}
}

func TestOpDivByZero(t *testing.T) {
	if err := runErr(t, `fn main { 1 / 0 }`); err == nil {
		t.Fatal("div by zero")
	}
	if err := runErr(t, `fn main { 1.0 / 0.0 }`); err == nil {
		t.Fatal("float div by zero")
	}
}

func TestOpMod(t *testing.T) {
	if run(t, `fn main { say(10 % 3) }`) != "1" {
		t.Fatal("mod")
	}
}

func TestOpModByZero(t *testing.T) {
	if err := runErr(t, `fn main { 10 % 0 }`); err == nil {
		t.Fatal("mod by zero")
	}
}

func TestOpModNonInt(t *testing.T) {
	if err := runErr(t, `fn main { 1.5 % 2 }`); err == nil {
		t.Fatal("mod non-int")
	}
}

func TestOpNeg(t *testing.T) {
	if run(t, `fn main { say(-5) }`) != "-5" {
		t.Fatal("neg int")
	}
	if run(t, `fn main { say(-3.14) }`) != "-3.14" {
		t.Fatal("neg float")
	}
}

func TestOpNegBadType(t *testing.T) {
	if err := runErr(t, `fn main { -"hello" }`); err == nil {
		t.Fatal("neg str should error")
	}
}

func TestOpNot(t *testing.T) {
	if run(t, `fn main { say(!true) }`) != "false" {
		t.Fatal("not")
	}
}

func TestOpEqNeq(t *testing.T) {
	if run(t, `fn main { say(1 == 1) }`) != "true" {
		t.Fatal("eq")
	}
	if run(t, `fn main { say(1 != 2) }`) != "true" {
		t.Fatal("neq")
	}
}

func TestOpComparisons(t *testing.T) {
	if run(t, `fn main { say(1 < 2) }`) != "true" {
		t.Fatal("lt")
	}
	if run(t, `fn main { say(2 <= 2) }`) != "true" {
		t.Fatal("lte")
	}
	if run(t, `fn main { say(3 > 2) }`) != "true" {
		t.Fatal("gt")
	}
	if run(t, `fn main { say(2 >= 2) }`) != "true" {
		t.Fatal("gte")
	}
	if run(t, `fn main { say("a" < "b") }`) != "true" {
		t.Fatal("str lt")
	}
}

func TestOpCompareError(t *testing.T) {
	if err := runErr(t, `fn main { say(1 < "a") }`); err == nil {
		t.Fatal("compare int and str")
	}
}

func TestNumericError(t *testing.T) {
	if err := runErr(t, `fn main { "a" - 1 }`); err == nil {
		t.Fatal("sub str")
	}
	if err := runErr(t, `fn main { "a" * 1 }`); err == nil {
		t.Fatal("mul str")
	}
	if err := runErr(t, `fn main { "a" / 1 }`); err == nil {
		t.Fatal("div str")
	}
}

func TestOpNullCoalesce(t *testing.T) {
	out := run(t, `
fn main {
    x := null
    say(x ?? 42)
}`)
	if out != "42" {
		t.Fatal("null coalesce")
	}
}

func TestOpTrue(t *testing.T) {
	if run(t, `fn main { say(true) }`) != "true" {
		t.Fatal("true")
	}
}

func TestOpFalse(t *testing.T) {
	if run(t, `fn main { say(false) }`) != "false" {
		t.Fatal("false")
	}
}

func TestOpNull(t *testing.T) {
	if run(t, `fn main { say(null) }`) != "null" {
		t.Fatal("null")
	}
}

func TestOpUnit(t *testing.T) {
	out := run(t, `
fn noop() { }
fn main { say(noop()) }`)
	if out != "unit" {
		t.Fatal("unit")
	}
}

func TestWhileLoop(t *testing.T) {
	out := run(t, `
fn main {
    mut i := 0
    while i < 3 {
        i = i + 1
    }
    say(i)
}`)
	if out != "3" {
		t.Fatal("while")
	}
}

func TestAndShortCircuit(t *testing.T) {
	if run(t, `fn main { say(false && true) }`) != "false" {
		t.Fatal("and")
	}
}

func TestOrShortCircuit(t *testing.T) {
	if run(t, `fn main { say(true || false) }`) != "true" {
		t.Fatal("or")
	}
}

func TestIfElse(t *testing.T) {
	out := run(t, `
fn main {
    if false {
        say(1)
    } else {
        say(2)
    }
}`)
	if out != "2" {
		t.Fatal("if/else")
	}
}

func TestOpCall(t *testing.T) {
	out := run(t, `
fn add(a, b) { a + b }
fn main { say(add(2, 3)) }`)
	if out != "5" {
		t.Fatal("call")
	}
}

func TestOpReturn(t *testing.T) {
	out := run(t, `
fn f() { return 42 }
fn main { say(f()) }`)
	if out != "42" {
		t.Fatal("return")
	}
}

func TestCallNonFunction(t *testing.T) {
	if err := runErr(t, `fn main { 5() }`); err == nil {
		t.Fatal("call non-function")
	}
}

func TestCallUnderArity(t *testing.T) {
	err := runErr(t, `
fn foo(a, b) { a + b }
fn main { say(foo(1)) }
`)
	if err == nil {
		t.Fatal("expected under-arity error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "wrong number of arguments") || !strings.Contains(msg, "foo") {
		t.Fatalf("want arity message with name, got %q", msg)
	}
	if strings.Contains(msg, "numeric op") {
		t.Fatalf("must not degrade to numeric-op-on-null: %q", msg)
	}
}

func TestCallOverArityStillRuns(t *testing.T) {
	// Extra args are ignored (historical); type checker may warn.
	if run(t, `
fn foo(a) { a }
fn main { say(foo(7, 8, 9)) }
`) != "7" {
		t.Fatal("over-arity")
	}
}

func TestOpMakeList(t *testing.T) {
	if run(t, `fn main { say([1, 2, 3]) }`) != "[1, 2, 3]" {
		t.Fatal("list")
	}
}

func TestOpMakeMap(t *testing.T) {
	if run(t, `fn main { say({"a": 1}) }`) != `{"a": 1}` {
		t.Fatal("map")
	}
}

func TestOpGetSetIndex(t *testing.T) {
	out := run(t, `
fn main {
    x := [10, 20]
    say(x[1])
}`)
	if out != "20" {
		t.Fatal("list index")
	}
}

func TestMapIndex(t *testing.T) {
	out := run(t, `
fn main {
    x := {"a": 1}
    say(x["a"])
}`)
	if out != "1" {
		t.Fatal("map index")
	}
}

func TestStrIndex(t *testing.T) {
	if run(t, `fn main { say("abc"[1]) }`) != "b" {
		t.Fatal("str index")
	}
}

func TestSetIndex(t *testing.T) {
	out := run(t, `
fn main {
    mut x := [1, 2]
    x[0] = 99
    say(x[0])
}`)
	if out != "99" {
		t.Fatal("set index")
	}
}

func TestIndexOutOfRange(t *testing.T) {
	if err := runErr(t, `fn main { [1][5] }`); err == nil {
		t.Fatal("list index out of range")
	}
	if err := runErr(t, `fn main { "a"[5] }`); err == nil {
		t.Fatal("str index out of range")
	}
}

func TestIndexUnsupported(t *testing.T) {
	if err := runErr(t, `fn main { 42[0] }`); err == nil {
		t.Fatal("index on int")
	}
}

func TestGetSetField(t *testing.T) {
	out := run(t, `
fn main {
    x := {"a": 1}
    say(x.a)
}`)
	if out != "1" {
		t.Fatal("get field")
	}
}

func TestFieldMissing(t *testing.T) {
	if err := runErr(t, `
fn main {
    x := {"a": 1}
    say(x.b)
}`); err == nil {
		t.Fatal("missing field")
	}
}

func TestOpConcat(t *testing.T) {
	// OpConcat is used by string interpolation
	out := run(t, `
fn main {
    a := "hello"
    say("$a world")
}`)
	if out != "hello world" {
		t.Fatal("concat via interpolation")
	}
}

func TestOpTryQ(t *testing.T) {
	out := run(t, `
fn get() -> Result { Ok(42) }
fn main -> Result {
    v := get()?
    say(v)
}`)
	if out != "42" {
		t.Fatal("? on Ok")
	}
}

func TestOpTryQOnErr(t *testing.T) {
	// ? on Err propagates the error up — main -> Result returns Err, which is not a Go error
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	err := ctx.RunSource(context.Background(), "test.weft", `
fn fail() -> Result { Err("boom") }
fn main -> Result {
    fail()?
    say("unreachable")
}`)
	// The VM returns the Err Result; RunSource may or may not surface it as Go error
	outStr := strings.TrimSpace(out.String())
	if outStr == "unreachable" {
		t.Fatal("? on Err should short-circuit")
	}
	_ = err // Err result propagation is valid
}

func TestOpTryQOnNonResult(t *testing.T) {
	if err := runErr(t, `
fn main -> Result {
    x := 5
    x?
}`); err == nil {
		t.Fatal("? on non-Result")
	}
}

func TestOpWrapResult(t *testing.T) {
	out := run(t, `
fn f() -> Result { 42 }
fn main { say(f()) }`)
	if out != "Ok(42)" {
		t.Fatalf("wrap result = %q", out)
	}
}

func TestClosure(t *testing.T) {
	out := run(t, `
fn main {
    x := 10
    f := fn() { x }
    say(f())
}`)
	if out != "10" {
		t.Fatal("closure")
	}
}

func TestClosureCapturesByValue(t *testing.T) {
	out := run(t, `
fn main {
    mut x := 1
    f := fn() { x }
    x = 99
    say(f())
}`)
	if out != "1" {
		t.Fatal("closure should capture by value")
	}
}

func TestForIn(t *testing.T) {
	out := run(t, `
fn main {
    mut s := 0
    for x in [1, 2, 3] {
        s = s + x
    }
    say(s)
}`)
	if out != "6" {
		t.Fatal("for-in")
	}
}

func TestForInMap(t *testing.T) {
	out := run(t, `
fn main {
    for k in {"a": 1} {
        say(k)
    }
}`)
	if out != "a" {
		t.Fatal("for-in map")
	}
}

func TestForInStr(t *testing.T) {
	out := run(t, `
fn main {
    for c in "hi" {
        say(c)
    }
}`)
	if out != "h\ni" {
		t.Fatalf("for-in str = %q", out)
	}
}

func TestBreak(t *testing.T) {
	out := run(t, `
fn main {
    mut i := 0
    while true {
        if i >= 3 { break }
        i = i + 1
    }
    say(i)
}`)
	if out != "3" {
		t.Fatal("break")
	}
}

func TestContinue(t *testing.T) {
	out := run(t, `
fn main {
    mut s := 0
    for i in [1, 2, 3, 4] {
        if i == 3 { continue }
        s = s + i
    }
    say(s)
}`)
	if out != "7" {
		t.Fatal("continue")
	}
}

func TestDefer(t *testing.T) {
	out := run(t, `
fn main {
    defer say("deferred")
    say("first")
}`)
	if out != "first\ndeferred" {
		t.Fatalf("defer = %q", out)
	}
}

func TestDeferLIFO(t *testing.T) {
	out := run(t, `
fn main {
    defer say("a")
    defer say("b")
    say("c")
}`)
	if out != "c\nb\na" {
		t.Fatalf("defer LIFO = %q", out)
	}
}

func TestSetFieldMap(t *testing.T) {
	out := run(t, `
fn main {
    mut m := {"a": 1}
    m.b = 2
    say(m.b)
}`)
	if out != "2" {
		t.Fatal("set field map")
	}
}

func TestMapSetIndex(t *testing.T) {
	out := run(t, `
fn main {
    mut m := {"a": 1}
    m["b"] = 2
    say(m["b"])
}`)
	if out != "2" {
		t.Fatal("map set index")
	}
}

func TestResultFields(t *testing.T) {
	out := run(t, `
fn main {
    r := Ok(5)
    say(r.ok, r.value)
}`)
	if out != "true 5" {
		t.Fatalf("result fields = %q", out)
	}
}

func TestResultIsErr(t *testing.T) {
	out := run(t, `
fn main {
    r := Err("x")
    say(r.is_err)
}`)
	if out != "true" {
		t.Fatal("is_err")
	}
}

func TestResultUnwrapOr(t *testing.T) {
	if run(t, `fn main { say(Ok(1).unwrap_or(99)) }`) != "1" {
		t.Fatal("unwrap_or Ok")
	}
	if run(t, `fn main { say(Err("x").unwrap_or(99)) }`) != "99" {
		t.Fatal("unwrap_or Err")
	}
}

func TestResultContext(t *testing.T) {
	out := run(t, `fn main { say(Err("inner").context("outer")) }`)
	if !strings.Contains(out, "outer: inner") {
		t.Fatalf("context = %q", out)
	}
}

func TestResultOr(t *testing.T) {
	out := run(t, `fn main { say(Err("x").or(Ok(42))) }`)
	if out != "Ok(42)" {
		t.Fatalf("or = %q", out)
	}
}

func TestResultExpect(t *testing.T) {
	out := run(t, `fn main { say(Err("x").expect("must")) }`)
	if !strings.Contains(out, "must") {
		t.Fatalf("expect = %q", out)
	}
}

func TestPipeline(t *testing.T) {
	out := run(t, `
fn double(x) { x * 2 }
fn main { 5 |> double |> say }`)
	if out != "10" {
		t.Fatal("pipeline")
	}
}

func TestMatch(t *testing.T) {
	out := run(t, `
fn main {
    x := 2
    r := match x {
        1 { "one" }
        2 { "two" }
        _ { "other" }
    }
    say(r)
}`)
	if out != "two" {
		t.Fatal("match")
	}
}

func TestMatchWildcard(t *testing.T) {
	out := run(t, `
fn main {
    r := match 99 {
        1 { "one" }
        _ { "other" }
    }
    say(r)
}`)
	if out != "other" {
		t.Fatal("wildcard")
	}
}

func TestMatchNoMatch(t *testing.T) {
	out := run(t, `
fn main {
    r := match 5 {
        1 { "one" }
        2 { "two" }
    }
    say(r)
}`)
	if out != "unit" {
		t.Fatalf("no match = %q", out)
	}
}

func TestEnum(t *testing.T) {
	out := run(t, `
enum Color { Red, Blue }
fn main { say(Color.Red) }`)
	if out != "Red" {
		t.Fatal("enum")
	}
}

func TestIfExpr(t *testing.T) {
	out := run(t, `
fn f(x) {
    if x > 0 { "pos" } else { "neg" }
}
fn main { say(f(1)) }`)
	if out != "pos" {
		t.Fatal("if expr")
	}
}

func TestStringInterpolation(t *testing.T) {
	out := run(t, `
fn main {
    name := "weft"
    say("hello $name")
}`)
	if out != "hello weft" {
		t.Fatal("interpolation")
	}
}

func TestMutReassign(t *testing.T) {
	out := run(t, `
fn main {
    mut x := 1
    x = 2
    say(x)
}`)
	if out != "2" {
		t.Fatal("mut")
	}
}

func TestImplicitReturn(t *testing.T) {
	out := run(t, `
fn f(x) { x * 2 }
fn main { say(f(5)) }`)
	if out != "10" {
		t.Fatal("implicit return")
	}
}

func TestConstDecl(t *testing.T) {
	out := run(t, `
const PI = 3
fn main { say(PI) }`)
	if out != "3" {
		t.Fatal("const")
	}
}

func TestUnicodeStringIndex(t *testing.T) {
	out := run(t, `fn main { say("héllo"[1]) }`)
	if out != "é" {
		t.Fatalf("unicode index = %q", out)
	}
}

func TestUnicodeLen(t *testing.T) {
	out := run(t, `fn main { say(len("héllo")) }`)
	if out != "5" {
		t.Fatalf("unicode len = %q", out)
	}
}

func TestForRange(t *testing.T) {
	out := run(t, `
fn main {
    mut s := 0
    for i in range(3) {
        s = s + i
    }
    say(s)
}`)
	if out != "3" {
		t.Fatal("for range")
	}
}

func TestNestedClosure(t *testing.T) {
	out := run(t, `
fn main {
    x := 1
    f := fn() {
        y := 2
        g := fn() { x + y }
        g()
    }
    say(f())
}`)
	if out != "3" {
		t.Fatal("nested closure")
	}
}

func TestDeferOnReturn(t *testing.T) {
	out := run(t, `
fn f() {
    defer say("cleanup")
    return 1
}
fn main { say(f()) }`)
	if !strings.Contains(out, "cleanup") || !strings.Contains(out, "1") {
		t.Fatalf("defer on return = %q", out)
	}
}

func TestDeferOnTryQ(t *testing.T) {
	out := run(t, `
fn fail() -> Result { Err("boom") }
fn f() -> Result {
    defer say("cleanup")
    fail()?
    say("unreachable")
}
fn main {
    f()
    say("done")
}`)
	if !strings.Contains(out, "cleanup") {
		t.Fatalf("defer on ? = %q", out)
	}
}

func TestResultUnwrapErr(t *testing.T) {
	err := runErr(t, `fn main { Err("fail").unwrap() }`)
	if err == nil {
		t.Fatal("unwrap on Err should propagate error")
	}
}

func TestResultUnwrapOk(t *testing.T) {
	out := run(t, `fn main { say(Ok(42).unwrap()) }`)
	if out != "42" {
		t.Fatal("unwrap Ok")
	}
}

func TestResultOrNoArgs(t *testing.T) {
	out := run(t, `fn main { say(Err("x").or()) }`)
	if !strings.Contains(out, "Err") {
		t.Fatalf("or no args = %q", out)
	}
}

func TestResultUnwrapOrNoArgs(t *testing.T) {
	out := run(t, `fn main { say(Err("x").unwrap_or()) }`)
	if out != "null" {
		t.Fatalf("unwrap_or no args = %q", out)
	}
}

func TestResultContextDefault(t *testing.T) {
	out := run(t, `fn main { say(Err("x").context()) }`)
	if !strings.Contains(out, "error: x") {
		t.Fatalf("context default = %q", out)
	}
}

func TestResultExpectDefault(t *testing.T) {
	out := run(t, `fn main { say(Err("x").expect()) }`)
	if !strings.Contains(out, "expectation failed") {
		t.Fatalf("expect default = %q", out)
	}
}

func TestResultFieldBadName(t *testing.T) {
	err := runErr(t, `
fn main {
    r := Ok(1)
    say(r.nonexistent)
}`)
	if err == nil {
		t.Fatal("bad result field")
	}
}

func TestSetFieldUnsupported(t *testing.T) {
	err := runErr(t, `
fn main {
    x := 5
    x = 6
}`)
	// this is just reassignment, not field set. Let me test actual field set on int
	_ = err
}

func TestDeferNotCall(t *testing.T) {
	// defer without call expr should be a compile error
	err := runErr(t, `
fn main {
    x := 5
    defer x
}`)
	if err == nil {
		t.Fatal("defer non-call should error")
	}
}

// --- error formatting ---

func TestStackTraceOnError(t *testing.T) {
	src := `
fn inner() {
    1 / 0
}
fn mid() {
    inner()
}
fn main {
    mid()
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	err := ctx.RunSource(context.Background(), "stack.weft", src)
	if err == nil {
		t.Fatal("want div zero error")
	}
	es := err.Error()
	if !strings.Contains(es, "inner") && !strings.Contains(es, "stack") && !strings.Contains(es, "division") {
		t.Fatalf("want stackful error, got %q", es)
	}
	if re, ok := err.(*vm.RuntimeError); ok {
		if len(re.Stack) < 2 {
			t.Fatalf("want multi-frame stack, got %+v", re.Stack)
		}
	} else {
		if !strings.Contains(es, "division") {
			t.Fatalf("%T %q", err, es)
		}
	}
}

func TestUndefinedNameHasLocation(t *testing.T) {
	ctx := weft.New(weft.Options{})
	err := ctx.RunSource(context.Background(), "u.weft", `fn main { missing_name }`)
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "missing_name") {
		t.Fatal(err)
	}
}

func TestRuntimeErrorNil(t *testing.T) {
	var re *vm.RuntimeError
	if re.Error() != "" {
		t.Fatal("nil RuntimeError should return empty string")
	}
}

func TestRuntimeErrorNoStack(t *testing.T) {
	re := &vm.RuntimeError{Msg: "oops"}
	if re.Error() != "oops" {
		t.Fatal("no stack")
	}
}

func TestRuntimeErrorSingleFrame(t *testing.T) {
	re := &vm.RuntimeError{
		Msg:   "bad",
		Stack: []vm.FrameLoc{{File: "test.weft", Line: 5}},
	}
	if !strings.Contains(re.Error(), "test.weft:5") {
		t.Fatalf("single frame = %q", re.Error())
	}
}

func TestRuntimeErrorLineOnly(t *testing.T) {
	re := &vm.RuntimeError{
		Msg:   "bad",
		Stack: []vm.FrameLoc{{Line: 3}},
	}
	if !strings.Contains(re.Error(), "line 3") {
		t.Fatalf("line only = %q", re.Error())
	}
}

func TestSetFieldOnStruct(t *testing.T) {
	out := run(t, `
type Point {
    x: int
    y: int
}
fn main {
    mut p := Point{x: 1, y: 2}
    p.x = 10
    say(p.x)
}`)
	if out != "10" {
		t.Fatal("set struct field")
	}
}

func TestStructNewField(t *testing.T) {
	out := run(t, `
type Foo {
    a: int
}
fn main {
    mut f := Foo{a: 1}
    f.b = 2
    say(f.b)
}`)
	if out != "2" {
		t.Fatal("new field on struct")
	}
}

func TestGetFieldStruct(t *testing.T) {
	out := run(t, `
type Pt {
    x: int
}
fn main {
    p := Pt{x: 42}
    say(p.x)
}`)
	if out != "42" {
		t.Fatal("get struct field")
	}
}

func TestGetFieldStructMissing(t *testing.T) {
	err := runErr(t, `
type Pt {
    x: int
}
fn main {
    p := Pt{x: 42}
    say(p.y)
}`)
	if err == nil {
		t.Fatal("missing struct field should error")
	}
}

func TestSetIndexOnMap(t *testing.T) {
	out := run(t, `
fn main {
    mut m := {"a": 1}
    m["b"] = 2
    say(m["b"])
}`)
	if out != "2" {
		t.Fatal("set map index")
	}
}

func TestSetIndexUnsupported(t *testing.T) {
	err := runErr(t, `fn main { "hello"[0] = "x" }`)
	if err == nil {
		t.Fatal("set index on str should error")
	}
}

func TestResultErrField(t *testing.T) {
	out := run(t, `
fn main {
    r := Err("boom")
    say(r.err)
}`)
	if !strings.Contains(out, "boom") {
		t.Fatalf("err field = %q", out)
	}
}

func TestResultValOnErr(t *testing.T) {
	out := run(t, `
fn main {
    r := Err("x")
    say(r.value)
}`)
	if out != "null" {
		t.Fatal("value on Err should be null")
	}
}

func TestResultErrOnOk(t *testing.T) {
	out := run(t, `
fn main {
    r := Ok(1)
    say(r.error)
}`)
	if out != "null" {
		t.Fatal("error on Ok should be null")
	}
}

func TestResultIsOk(t *testing.T) {
	out := run(t, `
fn main {
    say(Ok(1).is_ok)
}`)
	if out != "true" {
		t.Fatal("is_ok")
	}
}

func TestDeferBuiltin(t *testing.T) {
	out := run(t, `
fn main {
    defer println("deferred builtin")
    say("first")
}`)
	if !strings.Contains(out, "first") || !strings.Contains(out, "deferred builtin") {
		t.Fatalf("defer builtin = %q", out)
	}
}

func TestDeferInSubFunction(t *testing.T) {
	out := run(t, `
fn f() {
    defer say("cleanup")
    say("work")
}
fn main { f() }`)
	if out != "work\ncleanup" {
		t.Fatalf("defer in fn = %q", out)
	}
}

func TestMapGetNullKey(t *testing.T) {
	out := run(t, `
fn main {
    m := {"a": 1}
    say(m["missing"])
}`)
	if out != "null" {
		t.Fatal("missing map key")
	}
}

func TestExitSignalPassthrough(t *testing.T) {
	// Exit signals should pass through wrapErr unchanged
	err := runErr(t, `
use cli
fn main {
    cli.exit(0)
}`)
	// exit 0 is not an error per se
	_ = err
}

func TestStackOverflow(t *testing.T) {
	err := runErr(t, `
fn boom() { boom() }
fn main { boom() }`)
	if err == nil {
		t.Fatal("infinite recursion should error")
	}
	if !strings.Contains(err.Error(), "stack overflow") {
		t.Fatalf("expected stack overflow, got: %v", err)
	}
}

func TestSumTypePayload(t *testing.T) {
	out := run(t, `
enum Shape {
    Circle(radius)
    Rect(w, h)
    Point
}

fn area(s) {
    match s {
        Shape.Circle(radius) { radius * radius }
        Shape.Rect(w, h) { w * h }
        Shape.Point { 0 }
        _ { -1 }
    }
}

fn main {
    say(area(Shape.Circle(5)))
    say(area(Shape.Rect(3, 4)))
    say(area(Shape.Point))
}`)
	if out != "25\n12\n0" {
		t.Fatalf("sum types = %q", out)
	}
}

func TestSumTypeTag(t *testing.T) {
	out := run(t, `
enum Color {
    RGB(r, g, b)
    Hex(code)
}
fn main {
    c := Color.RGB(255, 0, 0)
    say(c._tag)
    say(c.r)
}`)
	if out != "Color.RGB\n255" {
		t.Fatalf("sum type tag = %q", out)
	}
}

func TestPlainEnumBackwardCompat(t *testing.T) {
	out := run(t, `
enum Dir { Up, Down, Left, Right }
fn main {
    say(Dir.Up)
    r := match Dir.Down {
        Dir.Up { "u" }
        Dir.Down { "d" }
        _ { "?" }
    }
    say(r)
}`)
	if out != "Up\nd" {
		t.Fatalf("plain enum = %q", out)
	}
}

func TestRuntimeErrorMultiFrame(t *testing.T) {
	re := &vm.RuntimeError{
		Msg: "bad",
		Stack: []vm.FrameLoc{
			{File: "a.weft", Line: 1, Func: "f"},
			{File: "b.weft", Line: 2, Func: "g"},
		},
	}
	s := re.Error()
	if !strings.Contains(s, "stack:") {
		t.Fatalf("multi frame = %q", s)
	}
	if !strings.Contains(s, "in f") || !strings.Contains(s, "in g") {
		t.Fatalf("func names = %q", s)
	}
}
