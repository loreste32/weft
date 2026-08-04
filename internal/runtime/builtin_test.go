package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestNewEnv(t *testing.T) {
	env := runtime.NewEnv()
	if env.Globals == nil {
		t.Fatal("globals nil")
	}
	if _, ok := env.Globals["println"]; !ok {
		t.Fatal("println missing from prelude")
	}
	if _, ok := env.Globals["say"]; !ok {
		t.Fatal("say missing from prelude")
	}
	if _, ok := env.Types["str"]; !ok {
		t.Fatal("str type missing")
	}
}

func TestEnvContext(t *testing.T) {
	env := runtime.NewEnv()
	ctx := env.Context()
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
	// nil env
	var nilEnv *runtime.Env
	ctx = nilEnv.Context()
	if ctx == nil {
		t.Fatal("nil env context should return background")
	}
}

func TestGetSet(t *testing.T) {
	env := runtime.NewEnv()
	env.Set("x", runtime.Int(5))
	v, ok := env.Get("x")
	if !ok || v.I != 5 {
		t.Fatal("get/set")
	}
	_, ok = env.Get("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent")
	}
}

func TestSetShared(t *testing.T) {
	env := runtime.NewEnv()
	env.SetShared("mylib", runtime.Str("lib"))
	if !env.IsHostShared("mylib") {
		t.Fatal("should be shared")
	}
	v, ok := env.Get("mylib")
	if !ok || v.S != "lib" {
		t.Fatal("shared value")
	}
}

func TestMarkShared(t *testing.T) {
	env := runtime.NewEnv()
	env.Set("a", runtime.Int(1))
	env.MarkShared("a", "b", "")
	if !env.IsHostShared("a") {
		t.Fatal("a should be shared")
	}
	if !env.IsHostShared("b") {
		t.Fatal("b should be shared")
	}
	// empty string should be ignored
	if env.IsHostShared("") {
		t.Fatal("empty should not be shared")
	}
}

func TestMarkAllGlobalsShared(t *testing.T) {
	env := runtime.NewEnv()
	env.Set("custom", runtime.Int(1))
	env.MarkAllGlobalsShared()
	if !env.IsHostShared("custom") {
		t.Fatal("should be shared after MarkAll")
	}
}

func TestIsHostSharedNil(t *testing.T) {
	var env *runtime.Env
	if env.IsHostShared("x") {
		t.Fatal("nil env should return false")
	}
}

func TestCopyHostSharedInto(t *testing.T) {
	parent := runtime.NewEnv()
	parent.SetShared("mylib", runtime.Str("lib"))

	child := runtime.NewEnv()
	parent.CopyHostSharedInto(child)
	v, ok := child.Get("mylib")
	if !ok || v.S != "lib" {
		t.Fatal("should copy shared")
	}
	if !child.IsHostShared("mylib") {
		t.Fatal("should mark as shared in child")
	}
}

func TestCopyHostSharedIntoNil(t *testing.T) {
	// should not panic
	var nilEnv *runtime.Env
	nilEnv.CopyHostSharedInto(runtime.NewEnv())

	parent := runtime.NewEnv()
	parent.CopyHostSharedInto(nil)
}

func TestRevokeHost(t *testing.T) {
	env := runtime.NewEnv()
	env.SetShared("danger", runtime.Str("rm"))
	env.RevokeHost("danger")
	_, ok := env.Get("danger")
	if ok {
		t.Fatal("revoked should be gone")
	}
	if env.IsHostShared("danger") {
		t.Fatal("revoked should not be shared")
	}
}

func TestRevokeHostNil(t *testing.T) {
	var env *runtime.Env
	env.RevokeHost("x") // should not panic
}

// --- prelude builtins ---

func callBuiltin(env *runtime.Env, name string, args []runtime.Value) (runtime.Value, error) {
	v := env.Globals[name]
	return v.Obj.(*runtime.BuiltinObj).Fn(args)
}

func callMapBuiltin(env *runtime.Env, pkg, name string, args []runtime.Value) (runtime.Value, error) {
	p := env.Globals[pkg]
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals[name]
	return fn.Obj.(*runtime.BuiltinObj).Fn(args)
}

func TestPrintln(t *testing.T) {
	env := runtime.NewEnv()
	var buf bytes.Buffer
	env.Stdout = &buf
	// re-register prelude to capture Stdout
	env.RegisterPrelude()

	callBuiltin(env, "println", []runtime.Value{runtime.Str("hello"), runtime.Int(42)})
	if buf.String() != "hello 42\n" {
		t.Fatalf("println output = %q", buf.String())
	}
}

func TestSayIsPrintln(t *testing.T) {
	env := runtime.NewEnv()
	// say should be the same builtin as println
	say := env.Globals["say"]
	println := env.Globals["println"]
	if say.Obj != println.Obj {
		t.Fatal("say should alias println")
	}
}

func TestPrint(t *testing.T) {
	env := runtime.NewEnv()
	var buf bytes.Buffer
	env.Stdout = &buf
	env.RegisterPrelude()

	callBuiltin(env, "print", []runtime.Value{runtime.Str("no"), runtime.Str("newline")})
	if buf.String() != "no newline" {
		t.Fatalf("print output = %q", buf.String())
	}
}

func TestOkErrBuiltins(t *testing.T) {
	env := runtime.NewEnv()
	v, _ := callBuiltin(env, "Ok", []runtime.Value{runtime.Int(5)})
	if !v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("Ok builtin")
	}

	// Err with string
	v, _ = callBuiltin(env, "Err", []runtime.Value{runtime.Str("boom")})
	ro := v.Obj.(*runtime.ResultObj)
	if ro.Ok {
		t.Fatal("Err builtin")
	}

	// Err with string + kind
	v, _ = callBuiltin(env, "Err", []runtime.Value{runtime.Str("x"), runtime.Str("io")})
	ro = v.Obj.(*runtime.ResultObj)
	so := ro.Err.Obj.(*runtime.StructObj)
	if so.Fields["kind"].S != "io" {
		t.Fatal("Err with kind")
	}

	// Err with Error struct
	errVal := runtime.NewError("existing", "net")
	v, _ = callBuiltin(env, "Err", []runtime.Value{errVal})
	ro = v.Obj.(*runtime.ResultObj)
	if ro.Err.Obj.(*runtime.StructObj).Fields["message"].S != "existing" {
		t.Fatal("Err with Error struct")
	}

	// Err with non-string non-error (int)
	v, _ = callBuiltin(env, "Err", []runtime.Value{runtime.Int(42)})
	ro = v.Obj.(*runtime.ResultObj)
	if ro.Err.Obj.(*runtime.StructObj).Fields["message"].S != "42" {
		t.Fatal("Err wraps any as message")
	}
}

func TestOkWrongArity(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callBuiltin(env, "Ok", nil)
	if err == nil {
		t.Fatal("Ok with no args should error")
	}
}

func TestErrNoArgs(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callBuiltin(env, "Err", nil)
	if err == nil {
		t.Fatal("Err with no args should error")
	}
}

func TestErrorNew(t *testing.T) {
	env := runtime.NewEnv()
	v, _ := callMapBuiltin(env, "Error", "new", []runtime.Value{runtime.Str("msg")})
	if !runtime.IsError(v) {
		t.Fatal("Error.new should return Error")
	}
	// with kind
	v, _ = callMapBuiltin(env, "Error", "new", []runtime.Value{runtime.Str("msg"), runtime.Str("io")})
	if v.Obj.(*runtime.StructObj).Fields["kind"].S != "io" {
		t.Fatal("Error.new kind")
	}
}

func TestErrorWrap(t *testing.T) {
	env := runtime.NewEnv()
	cause := runtime.NewError("inner", "io")
	v, _ := callMapBuiltin(env, "Error", "wrap", []runtime.Value{cause, runtime.Str("outer")})
	if !runtime.IsError(v) {
		t.Fatal("Error.wrap should return Error")
	}
}

func TestErrorWith(t *testing.T) {
	env := runtime.NewEnv()
	opts := runtime.NewMap()
	mo := opts.Obj.(*runtime.MapObj)
	mo.Keys = []string{"kind", "code"}
	mo.Vals["kind"] = runtime.Str("http")
	mo.Vals["code"] = runtime.Int(404)

	v, _ := callMapBuiltin(env, "Error", "with", []runtime.Value{runtime.Str("not found"), opts})
	so := v.Obj.(*runtime.StructObj)
	if so.Fields["kind"].S != "http" || so.Fields["code"].I != 404 {
		t.Fatal("Error.with opts")
	}
}

func TestErrorWithCause(t *testing.T) {
	env := runtime.NewEnv()
	cause := runtime.NewError("low", "io")
	opts := runtime.NewMap()
	mo := opts.Obj.(*runtime.MapObj)
	mo.Keys = []string{"cause"}
	mo.Vals["cause"] = cause
	v, _ := callMapBuiltin(env, "Error", "with", []runtime.Value{runtime.Str("high"), opts})
	so := v.Obj.(*runtime.StructObj)
	if so.Fields["cause"].Kind != runtime.KindStruct {
		t.Fatal("Error.with cause")
	}
}

func TestErrorWithMessageFromMap(t *testing.T) {
	env := runtime.NewEnv()
	opts := runtime.NewMap()
	mo := opts.Obj.(*runtime.MapObj)
	mo.Keys = []string{"message"}
	mo.Vals["message"] = runtime.Str("from map")
	v, _ := callMapBuiltin(env, "Error", "with", []runtime.Value{runtime.Str(""), opts})
	so := v.Obj.(*runtime.StructObj)
	if so.Fields["message"].S != "from map" {
		t.Fatal("Error.with message from map")
	}
}

func TestEnsure(t *testing.T) {
	env := runtime.NewEnv()
	// true
	v, _ := callBuiltin(env, "ensure", []runtime.Value{runtime.Bool(true)})
	if !v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("ensure true")
	}
	// false
	v, _ = callBuiltin(env, "ensure", []runtime.Value{runtime.Bool(false)})
	if v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("ensure false should be Err")
	}
	// with message
	v, _ = callBuiltin(env, "ensure", []runtime.Value{runtime.Bool(false), runtime.Str("bad")})
	msg := runtime.ErrorMessage(v.Obj.(*runtime.ResultObj).Err)
	if msg != "bad" {
		t.Fatalf("ensure message = %q", msg)
	}
}

func TestBail(t *testing.T) {
	env := runtime.NewEnv()
	v, _ := callBuiltin(env, "bail", []runtime.Value{runtime.Str("oops")})
	if v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("bail should be Err")
	}
	// with kind
	v, _ = callBuiltin(env, "bail", []runtime.Value{runtime.Str("x"), runtime.Str("io")})
	so := v.Obj.(*runtime.ResultObj).Err.Obj.(*runtime.StructObj)
	if so.Fields["kind"].S != "io" {
		t.Fatal("bail kind")
	}
	// no args
	v, _ = callBuiltin(env, "bail", nil)
	if v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("bail no args should still be Err")
	}
}

func TestIsOkIsErr(t *testing.T) {
	env := runtime.NewEnv()
	ok := runtime.Ok(runtime.Int(1))
	er := runtime.Err(runtime.NewError("x", "y"))

	v, _ := callBuiltin(env, "is_ok", []runtime.Value{ok})
	if !v.B {
		t.Fatal("is_ok on Ok")
	}
	v, _ = callBuiltin(env, "is_ok", []runtime.Value{er})
	if v.B {
		t.Fatal("is_ok on Err")
	}
	v, _ = callBuiltin(env, "is_ok", nil)
	if v.B {
		t.Fatal("is_ok no args")
	}

	v, _ = callBuiltin(env, "is_err", []runtime.Value{er})
	if !v.B {
		t.Fatal("is_err on Err")
	}
	v, _ = callBuiltin(env, "is_err", []runtime.Value{ok})
	if v.B {
		t.Fatal("is_err on Ok")
	}
	// is_err on Error struct (not Result)
	v, _ = callBuiltin(env, "is_err", []runtime.Value{runtime.NewError("x", "y")})
	if !v.B {
		t.Fatal("is_err on Error struct")
	}
	v, _ = callBuiltin(env, "is_err", nil)
	if v.B {
		t.Fatal("is_err no args")
	}
}

func TestTypeOfPreservesNumericKind(t *testing.T) {
	env := runtime.NewEnv()
	cases := []struct {
		name  string
		value runtime.Value
		want  string
	}{
		{"null", runtime.Null(), "null"},
		{"unit", runtime.Unit(), "unit"},
		{"bool", runtime.Bool(true), "bool"},
		{"int", runtime.Int(1), "int"},
		{"float", runtime.Float(1), "float"},
		{"str", runtime.Str("x"), "str"},
		{"list", runtime.List(runtime.Int(1)), "list"},
		{"map", runtime.NewMap(), "map"},
		{"struct", runtime.Struct("Point", nil, nil), "struct"},
		{"result", runtime.Ok(runtime.Int(1)), "result"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := callBuiltin(env, "type_of", []runtime.Value{tc.value})
			if err != nil {
				t.Fatalf("type_of(%s): %v", tc.name, err)
			}
			if got.Kind != runtime.KindStr || got.S != tc.want {
				t.Fatalf("type_of(%s) = %#v, want %q", tc.name, got, tc.want)
			}
		})
	}
	if _, err := callBuiltin(env, "type_of", nil); err == nil {
		t.Fatal("type_of should reject invalid arity")
	}
}

func TestIntParse(t *testing.T) {
	env := runtime.NewEnv()
	v, _ := callMapBuiltin(env, "int", "parse", []runtime.Value{runtime.Str("42")})
	if !v.Obj.(*runtime.ResultObj).Ok || v.Obj.(*runtime.ResultObj).Val.I != 42 {
		t.Fatal("int.parse")
	}
	v, _ = callMapBuiltin(env, "int", "parse", []runtime.Value{runtime.Str("abc")})
	if v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("int.parse invalid should Err")
	}
}

func TestLen(t *testing.T) {
	env := runtime.NewEnv()
	// list
	v, _ := callBuiltin(env, "len", []runtime.Value{runtime.List(runtime.Int(1), runtime.Int(2))})
	if v.I != 2 {
		t.Fatal("len list")
	}
	// map
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"a"}
	mo.Vals["a"] = runtime.Int(1)
	v, _ = callBuiltin(env, "len", []runtime.Value{m})
	if v.I != 1 {
		t.Fatal("len map")
	}
	// string (rune count)
	v, _ = callBuiltin(env, "len", []runtime.Value{runtime.Str("héllo")})
	if v.I != 5 {
		t.Fatalf("len str = %d, want 5", v.I)
	}
	// null
	v, _ = callBuiltin(env, "len", []runtime.Value{runtime.Null()})
	if v.I != 0 {
		t.Fatal("len null")
	}
	// unsupported
	_, err := callBuiltin(env, "len", []runtime.Value{runtime.Bool(true)})
	if err == nil {
		t.Fatal("len bool should error")
	}
}

func TestPush(t *testing.T) {
	env := runtime.NewEnv()
	list := runtime.List(runtime.Int(1))
	v, _ := callBuiltin(env, "push", []runtime.Value{list, runtime.Int(2)})
	items := v.Obj.(*runtime.ListObj).Items
	if len(items) != 2 || items[1].I != 2 {
		t.Fatal("push")
	}
}

func TestConcat(t *testing.T) {
	env := runtime.NewEnv()
	a := runtime.List(runtime.Int(1))
	b := runtime.List(runtime.Int(2))
	v, _ := callBuiltin(env, "concat", []runtime.Value{a, b})
	items := v.Obj.(*runtime.ListObj).Items
	if len(items) != 2 {
		t.Fatal("concat")
	}
}

func TestSlice(t *testing.T) {
	env := runtime.NewEnv()
	list := runtime.List(runtime.Int(10), runtime.Int(20), runtime.Int(30))
	// slice(list, 1)
	v, _ := callBuiltin(env, "slice", []runtime.Value{list, runtime.Int(1)})
	items := v.Obj.(*runtime.ListObj).Items
	if len(items) != 2 || items[0].I != 20 {
		t.Fatal("slice start")
	}
	// slice(list, 0, 2)
	v, _ = callBuiltin(env, "slice", []runtime.Value{list, runtime.Int(0), runtime.Int(2)})
	if len(v.Obj.(*runtime.ListObj).Items) != 2 {
		t.Fatal("slice start end")
	}
	// negative
	v, _ = callBuiltin(env, "slice", []runtime.Value{list, runtime.Int(-2)})
	if len(v.Obj.(*runtime.ListObj).Items) != 2 {
		t.Fatal("slice negative start")
	}
}

func TestRange(t *testing.T) {
	env := runtime.NewEnv()
	// range(3)
	v, _ := callBuiltin(env, "range", []runtime.Value{runtime.Int(3)})
	items := v.Obj.(*runtime.ListObj).Items
	if len(items) != 3 || items[0].I != 0 || items[2].I != 2 {
		t.Fatal("range(3)")
	}
	// range(2, 5)
	v, _ = callBuiltin(env, "range", []runtime.Value{runtime.Int(2), runtime.Int(5)})
	items = v.Obj.(*runtime.ListObj).Items
	if len(items) != 3 || items[0].I != 2 {
		t.Fatal("range(2,5)")
	}
	// range(5, 3) -> empty
	v, _ = callBuiltin(env, "range", []runtime.Value{runtime.Int(5), runtime.Int(3)})
	if len(v.Obj.(*runtime.ListObj).Items) != 0 {
		t.Fatal("reversed range should be empty")
	}
}

func TestRangeTooLarge(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callBuiltin(env, "range", []runtime.Value{runtime.Int(20_000_000)})
	if err == nil {
		t.Fatal("huge range should error")
	}
}

func TestPop(t *testing.T) {
	env := runtime.NewEnv()
	list := runtime.List(runtime.Int(1), runtime.Int(2))
	v, _ := callBuiltin(env, "pop", []runtime.Value{list})
	if v.I != 2 {
		t.Fatal("pop value")
	}
	if len(list.Obj.(*runtime.ListObj).Items) != 1 {
		t.Fatal("pop mutates")
	}
	// pop empty
	empty := runtime.List()
	v, _ = callBuiltin(env, "pop", []runtime.Value{empty})
	if v.Kind != runtime.KindNull {
		t.Fatal("pop empty should return null")
	}
}

func TestContains(t *testing.T) {
	env := runtime.NewEnv()
	// string
	v, _ := callBuiltin(env, "contains", []runtime.Value{runtime.Str("hello"), runtime.Str("ell")})
	if !v.B {
		t.Fatal("contains str")
	}
	// list
	list := runtime.List(runtime.Int(1), runtime.Int(2), runtime.Int(3))
	v, _ = callBuiltin(env, "contains", []runtime.Value{list, runtime.Int(2)})
	if !v.B {
		t.Fatal("contains list")
	}
	v, _ = callBuiltin(env, "contains", []runtime.Value{list, runtime.Int(99)})
	if v.B {
		t.Fatal("contains list missing")
	}
	// unsupported type
	v, _ = callBuiltin(env, "contains", []runtime.Value{runtime.Int(1), runtime.Int(1)})
	if v.B {
		t.Fatal("contains int should be false")
	}
}

func TestKeys(t *testing.T) {
	env := runtime.NewEnv()
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"a", "b"}
	mo.Vals["a"] = runtime.Int(1)
	mo.Vals["b"] = runtime.Int(2)
	v, _ := callBuiltin(env, "keys", []runtime.Value{m})
	items := v.Obj.(*runtime.ListObj).Items
	if len(items) != 2 || items[0].S != "a" {
		t.Fatal("keys")
	}
	// non-map
	v, _ = callBuiltin(env, "keys", []runtime.Value{runtime.Int(1)})
	if len(v.Obj.(*runtime.ListObj).Items) != 0 {
		t.Fatal("keys non-map should return empty list")
	}
}

func TestValues(t *testing.T) {
	env := runtime.NewEnv()
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"a"}
	mo.Vals["a"] = runtime.Int(7)
	v, _ := callBuiltin(env, "values", []runtime.Value{m})
	items := v.Obj.(*runtime.ListObj).Items
	if len(items) != 1 || items[0].I != 7 {
		t.Fatal("values")
	}
}

func TestDelete(t *testing.T) {
	env := runtime.NewEnv()
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"a", "b"}
	mo.Vals["a"] = runtime.Int(1)
	mo.Vals["b"] = runtime.Int(2)
	callBuiltin(env, "delete", []runtime.Value{m, runtime.Str("a")})
	if _, ok := mo.Vals["a"]; ok {
		t.Fatal("delete should remove key")
	}
	if len(mo.Keys) != 1 {
		t.Fatal("delete should remove from keys list")
	}
}

func TestDeepCopyBuiltin(t *testing.T) {
	env := runtime.NewEnv()
	list := runtime.List(runtime.Int(1))
	v, _ := callBuiltin(env, "deepcopy", []runtime.Value{list})
	list.Obj.(*runtime.ListObj).Items[0] = runtime.Int(99)
	if v.Obj.(*runtime.ListObj).Items[0].I != 1 {
		t.Fatal("deepcopy builtin")
	}
}

func TestChannelBuiltins(t *testing.T) {
	env := envWithCall()
	// channel(1)
	ch, _ := callBuiltin(env, "channel", []runtime.Value{runtime.Int(1)})
	// send
	v, _ := callBuiltin(env, "send", []runtime.Value{ch, runtime.Int(42)})
	if !v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("send")
	}
	// recv
	v, _ = callBuiltin(env, "recv", []runtime.Value{ch})
	ro := v.Obj.(*runtime.ResultObj)
	if !ro.Ok || ro.Val.I != 42 {
		t.Fatal("recv")
	}
	// try_recv empty
	v, _ = callBuiltin(env, "try_recv", []runtime.Value{ch})
	ro = v.Obj.(*runtime.ResultObj)
	if !ro.Ok {
		t.Fatal("try_recv should be ok")
	}
	// close
	v, _ = callBuiltin(env, "close", []runtime.Value{ch})
	if !v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("close")
	}
}

func TestSelectRecvBuiltin(t *testing.T) {
	env := envWithCall()
	ch1, _ := callBuiltin(env, "channel", []runtime.Value{runtime.Int(1)})
	callBuiltin(env, "send", []runtime.Value{ch1, runtime.Str("hi")})
	v, _ := callBuiltin(env, "select_recv", []runtime.Value{runtime.List(ch1)})
	ro := v.Obj.(*runtime.ResultObj)
	if !ro.Ok {
		t.Fatal("select_recv")
	}
}

func TestChannelZeroBuffer(t *testing.T) {
	env := envWithCall()
	ch, _ := callBuiltin(env, "channel", nil)
	if ch.Kind != runtime.KindStruct {
		t.Fatal("channel() should work with no args")
	}
}

// --- edge cases for remaining coverage ---

func TestSpawnWithArgs(t *testing.T) {
	env := envWithCall()
	fn := runtime.MakeBuiltin("add1", 1, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Int(args[0].I + 1), nil
	})
	spawnFn := env.Globals["spawn"].Obj.(*runtime.BuiltinObj)
	handle, err := spawnFn.Fn([]runtime.Value{fn, runtime.Int(10)})
	if err != nil {
		t.Fatal(err)
	}
	joinFn := handle.Obj.(*runtime.MapObj).Vals["join"].Obj.(*runtime.BuiltinObj)
	v, _ := joinFn.Fn(nil)
	if v.I != 11 {
		t.Fatal("spawn with args")
	}
}

func TestSpawnError(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		return runtime.Null(), fmt.Errorf("task failed")
	}
	fn := runtime.MakeBuiltin("fail", 0, func(_ []runtime.Value) (runtime.Value, error) {
		return runtime.Null(), fmt.Errorf("fail")
	})
	spawnFn := env.Globals["spawn"].Obj.(*runtime.BuiltinObj)
	handle, _ := spawnFn.Fn([]runtime.Value{fn})
	// join should return Err Result
	joinFn := handle.Obj.(*runtime.MapObj).Vals["join"].Obj.(*runtime.BuiltinObj)
	v, _ := joinFn.Fn(nil)
	if v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("join on failed task should be Err")
	}
	// await on error
	awaitFn := handle.Obj.(*runtime.MapObj).Vals["await"].Obj.(*runtime.BuiltinObj)
	v, _ = awaitFn.Fn(nil)
	if v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("await on failed task should be Err")
	}
}

func TestSpawnReturnsResult(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		return fn.Obj.(*runtime.BuiltinObj).Fn(args)
	}
	fn := runtime.MakeBuiltin("res", 0, func(_ []runtime.Value) (runtime.Value, error) {
		return runtime.Ok(runtime.Int(99)), nil
	})
	spawnFn := env.Globals["spawn"].Obj.(*runtime.BuiltinObj)
	handle, _ := spawnFn.Fn([]runtime.Value{fn})
	awaitFn := handle.Obj.(*runtime.MapObj).Vals["await"].Obj.(*runtime.BuiltinObj)
	v, _ := awaitFn.Fn(nil)
	// Should pass through Result as-is
	ro := v.Obj.(*runtime.ResultObj)
	if !ro.Ok || ro.Val.I != 99 {
		t.Fatal("await should pass through Result")
	}
}

func TestParallelError(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		return runtime.Null(), fmt.Errorf("boom")
	}
	fn := runtime.MakeBuiltin("f", 0, nil)
	parallelFn := env.Globals["parallel"].Obj.(*runtime.BuiltinObj)
	r, _ := parallelFn.Fn([]runtime.Value{runtime.List(fn)})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("parallel with error should be Err")
	}
}

func TestRaceError(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		return runtime.Null(), fmt.Errorf("boom")
	}
	fn := runtime.MakeBuiltin("f", 0, nil)
	raceFn := env.Globals["race"].Obj.(*runtime.BuiltinObj)
	r, _ := raceFn.Fn([]runtime.Value{runtime.List(fn)})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("race with error should be Err")
	}
}

func TestRaceReturnsResult(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		return runtime.Ok(runtime.Str("done")), nil
	}
	fn := runtime.MakeBuiltin("f", 0, nil)
	raceFn := env.Globals["race"].Obj.(*runtime.BuiltinObj)
	r, _ := raceFn.Fn([]runtime.Value{runtime.List(fn)})
	ro := r.Obj.(*runtime.ResultObj)
	if !ro.Ok || ro.Val.S != "done" {
		t.Fatal("race should pass through Result")
	}
}

func TestTimeoutError(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		return runtime.Null(), fmt.Errorf("fail")
	}
	fn := runtime.MakeBuiltin("f", 0, nil)
	timeoutFn := env.Globals["timeout"].Obj.(*runtime.BuiltinObj)
	r, _ := timeoutFn.Fn([]runtime.Value{runtime.Int(5), fn})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("timeout with error should be Err")
	}
}

func TestTimeoutReturnsResult(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		return runtime.Ok(runtime.Int(1)), nil
	}
	fn := runtime.MakeBuiltin("f", 0, nil)
	timeoutFn := env.Globals["timeout"].Obj.(*runtime.BuiltinObj)
	r, _ := timeoutFn.Fn([]runtime.Value{runtime.Int(5), fn})
	ro := r.Obj.(*runtime.ResultObj)
	if !ro.Ok || ro.Val.I != 1 {
		t.Fatal("timeout should pass through Result")
	}
}

func TestTimeoutTooFewArgs(t *testing.T) {
	env := envWithCall()
	timeoutFn := env.Globals["timeout"].Obj.(*runtime.BuiltinObj)
	r, _ := timeoutFn.Fn([]runtime.Value{runtime.Int(1)})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("too few args should Err")
	}
}

func TestTimeoutStringDuration(t *testing.T) {
	env := envWithCall()
	fn := runtime.MakeBuiltin("f", 0, func(_ []runtime.Value) (runtime.Value, error) {
		return runtime.Int(1), nil
	})
	timeoutFn := env.Globals["timeout"].Obj.(*runtime.BuiltinObj)
	// string "5" coerced through AsInt default case
	r, _ := timeoutFn.Fn([]runtime.Value{runtime.Str("5"), fn})
	// d would be 5 seconds, fn returns immediately
	ro := r.Obj.(*runtime.ResultObj)
	if !ro.Ok {
		t.Fatal("string duration")
	}
}

func TestSendRecvCloseErrors(t *testing.T) {
	env := envWithCall()
	// too few args
	r, _ := callBuiltin(env, "send", []runtime.Value{runtime.Int(1)})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("send too few args")
	}
	r, _ = callBuiltin(env, "recv", nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("recv no args")
	}
	r, _ = callBuiltin(env, "close", nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("close no args")
	}
	r, _ = callBuiltin(env, "try_recv", nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("try_recv no args")
	}
	r, _ = callBuiltin(env, "select_recv", nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("select_recv no args")
	}
}

func TestGroupGoNoArgs(t *testing.T) {
	env := envWithCall()
	groupFn := env.Globals["group"].Obj.(*runtime.BuiltinObj)
	g, _ := groupFn.Fn(nil)
	mo := g.Obj.(*runtime.MapObj)
	goFn := mo.Vals["go"].Obj.(*runtime.BuiltinObj)
	_, err := goFn.Fn(nil)
	if err == nil {
		t.Fatal("group.go no args should error")
	}
}

func TestParallelBadArg(t *testing.T) {
	env := envWithCall()
	parallelFn := env.Globals["parallel"].Obj.(*runtime.BuiltinObj)
	// not a list
	r, _ := parallelFn.Fn([]runtime.Value{runtime.Int(1)})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("parallel non-list should Err")
	}
	// no args
	r, _ = parallelFn.Fn(nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("parallel no args should Err")
	}
}

func TestRaceBadArgs(t *testing.T) {
	env := envWithCall()
	raceFn := env.Globals["race"].Obj.(*runtime.BuiltinObj)
	r, _ := raceFn.Fn([]runtime.Value{runtime.Int(1)})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("race non-list")
	}
	r, _ = raceFn.Fn(nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("race no args")
	}
}

func TestKindNameUnknown(t *testing.T) {
	v := runtime.Value{Kind: 255}
	if v.KindName() != "?" {
		t.Fatal("unknown kind")
	}
}

func TestStringUnknown(t *testing.T) {
	v := runtime.Value{Kind: 255}
	if v.String() != "<unknown>" {
		t.Fatal("unknown string")
	}
}

func TestMarkSharedNilHostShared(t *testing.T) {
	env := runtime.NewEnv()
	env.HostShared = nil
	env.MarkShared("x")
	if !env.IsHostShared("x") {
		t.Fatal("MarkShared with nil map should init")
	}
}

func TestMarkAllGlobalsSharedNilMap(t *testing.T) {
	env := runtime.NewEnv()
	env.HostShared = nil
	env.MarkAllGlobalsShared()
	if env.HostShared == nil {
		t.Fatal("should init map")
	}
}

func TestCopyHostSharedIntoNilDstMap(t *testing.T) {
	parent := runtime.NewEnv()
	parent.SetShared("x", runtime.Int(1))
	child := runtime.NewEnv()
	child.HostShared = nil
	parent.CopyHostSharedInto(child)
	if !child.IsHostShared("x") {
		t.Fatal("should init child map")
	}
}

func TestContextWithCtx(t *testing.T) {
	env := runtime.NewEnv()
	// env.Ctx is nil by default, Context() returns Background
	ctx := env.Context()
	if ctx == nil {
		t.Fatal("should not be nil")
	}
	// with explicit context
	ctx2, cancel := context.WithCancel(context.Background())
	defer cancel()
	env.Ctx = ctx2
	if env.Context() != ctx2 {
		t.Fatal("should return set context")
	}
}

func TestEqualMapMissingKey(t *testing.T) {
	a := runtime.NewMap()
	ao := a.Obj.(*runtime.MapObj)
	ao.Keys = []string{"x"}
	ao.Vals["x"] = runtime.Int(1)

	b := runtime.NewMap()
	bo := b.Obj.(*runtime.MapObj)
	bo.Keys = []string{"y"}
	bo.Vals["y"] = runtime.Int(1)

	if runtime.Equal(a, b) {
		t.Fatal("maps with different keys should not be equal")
	}
}

func TestErrorWithLocationAddsToOrder(t *testing.T) {
	// Error without "at" in order
	fields := map[string]runtime.Value{
		"message": runtime.Str("x"),
		"kind":    runtime.Str("y"),
	}
	e := runtime.Struct("Error", fields, []string{"message", "kind"})
	e2 := runtime.ErrorWithLocation(e, "file:1")
	so := e2.Obj.(*runtime.StructObj)
	if so.Fields["at"].S != "file:1" {
		t.Fatal("at should be set")
	}
	// "at" should be in order
	found := false
	for _, k := range so.Order {
		if k == "at" {
			found = true
		}
	}
	if !found {
		t.Fatal("at should be in order")
	}
}

func TestSelectRecvClosedChannel(t *testing.T) {
	ch := runtime.MakeChannel(0)
	runtime.ChannelClose(ch)
	// select_recv on a closed, empty channel — the goroutine reads ok=false and returns
	// without sending to out, so this should timeout if we use a timeout
	_, _, err := runtime.SelectRecv([]runtime.Value{ch}, 50)
	if err == nil {
		t.Fatal("select_recv on closed should timeout")
	}
}

func TestRegisterPreludeSliceBadArgs(t *testing.T) {
	env := runtime.NewEnv()
	// slice with non-list
	_, err := callBuiltin(env, "slice", []runtime.Value{runtime.Int(1), runtime.Int(0)})
	if err == nil {
		t.Fatal("slice non-list should error")
	}
}

func TestPushBadArgs(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callBuiltin(env, "push", []runtime.Value{runtime.Int(1), runtime.Int(2)})
	if err == nil {
		t.Fatal("push non-list should error")
	}
}

func TestConcatBadArgs(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callBuiltin(env, "concat", []runtime.Value{runtime.Int(1), runtime.Int(2)})
	if err == nil {
		t.Fatal("concat non-list should error")
	}
}

func TestDeleteBadArgs(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callBuiltin(env, "delete", []runtime.Value{runtime.Int(1), runtime.Str("a")})
	if err == nil {
		t.Fatal("delete non-map should error")
	}
}

func TestDeepCopyNil(t *testing.T) {
	env := runtime.NewEnv()
	v, _ := callBuiltin(env, "deepcopy", nil)
	if v.Kind != runtime.KindNull {
		t.Fatal("deepcopy nil should return null")
	}
}

func TestPopBadArgs(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callBuiltin(env, "pop", []runtime.Value{runtime.Int(1)})
	if err == nil {
		t.Fatal("pop non-list should error")
	}
}

func TestLenWrongArity(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callBuiltin(env, "len", nil)
	if err == nil {
		t.Fatal("len no args should error")
	}
}

func TestRangeNoArgs(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callBuiltin(env, "range", nil)
	if err == nil {
		t.Fatal("range no args should error")
	}
}

func TestContainsNoArgs(t *testing.T) {
	env := runtime.NewEnv()
	v, _ := callBuiltin(env, "contains", nil)
	if v.B {
		t.Fatal("contains no args should be false")
	}
}

func TestIntParseWrongArity(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callMapBuiltin(env, "int", "parse", nil)
	if err == nil {
		t.Fatal("int.parse no args should error")
	}
}

func TestErrorNewNoArgs(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callMapBuiltin(env, "Error", "new", nil)
	if err == nil {
		t.Fatal("Error.new no args should error")
	}
}

func TestErrorWrapNoArgs(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callMapBuiltin(env, "Error", "wrap", nil)
	if err == nil {
		t.Fatal("Error.wrap no args should error")
	}
}

func TestErrorWithNoArgs(t *testing.T) {
	env := runtime.NewEnv()
	_, err := callMapBuiltin(env, "Error", "with", nil)
	if err == nil {
		t.Fatal("Error.with no args should error")
	}
}

func TestEnsureNoArgs(t *testing.T) {
	env := runtime.NewEnv()
	v, _ := callBuiltin(env, "ensure", nil)
	if v.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("ensure no args should Err")
	}
}
