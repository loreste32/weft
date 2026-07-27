package runtime_test

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

// --- constructors ---

func TestNull(t *testing.T) {
	v := runtime.Null()
	if v.Kind != runtime.KindNull {
		t.Fatal("expected null kind")
	}
}

func TestUnit(t *testing.T) {
	v := runtime.Unit()
	if v.Kind != runtime.KindUnit {
		t.Fatal("expected unit kind")
	}
}

func TestBool(t *testing.T) {
	tr := runtime.Bool(true)
	fa := runtime.Bool(false)
	if !tr.B || fa.B {
		t.Fatal("bool constructors broken")
	}
}

func TestInt(t *testing.T) {
	v := runtime.Int(42)
	if v.I != 42 || v.Kind != runtime.KindInt {
		t.Fatal("int constructor")
	}
}

func TestFloat(t *testing.T) {
	v := runtime.Float(3.14)
	if v.F != 3.14 || v.Kind != runtime.KindFloat {
		t.Fatal("float constructor")
	}
}

func TestStr(t *testing.T) {
	v := runtime.Str("hello")
	if v.S != "hello" || v.Kind != runtime.KindStr {
		t.Fatal("str constructor")
	}
}

func TestList(t *testing.T) {
	v := runtime.List(runtime.Int(1), runtime.Int(2))
	lo := v.Obj.(*runtime.ListObj)
	if len(lo.Items) != 2 || lo.Items[0].I != 1 {
		t.Fatal("list constructor")
	}
}

func TestNewMap(t *testing.T) {
	v := runtime.NewMap()
	mo := v.Obj.(*runtime.MapObj)
	if mo.Vals == nil || len(mo.Vals) != 0 {
		t.Fatal("map constructor")
	}
}

func TestStruct(t *testing.T) {
	v := runtime.Struct("Point", map[string]runtime.Value{
		"x": runtime.Int(1),
		"y": runtime.Int(2),
	}, []string{"x", "y"})
	so := v.Obj.(*runtime.StructObj)
	if so.TypeName != "Point" || so.Fields["x"].I != 1 {
		t.Fatal("struct constructor")
	}
}

func TestStructNilFields(t *testing.T) {
	v := runtime.Struct("Empty", nil, nil)
	so := v.Obj.(*runtime.StructObj)
	if so.Fields == nil {
		t.Fatal("nil fields should be initialized")
	}
}

func TestFunc(t *testing.T) {
	fn := &runtime.FuncObj{Name: "add", Arity: 2}
	v := runtime.Func(fn)
	if v.Kind != runtime.KindFunc || v.Obj.(*runtime.FuncObj).Name != "add" {
		t.Fatal("func constructor")
	}
}

func TestMakeBuiltin(t *testing.T) {
	v := runtime.MakeBuiltin("test", 1, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Int(42), nil
	})
	if v.Kind != runtime.KindBuiltin {
		t.Fatal("builtin constructor")
	}
	bo := v.Obj.(*runtime.BuiltinObj)
	if bo.Name != "test" || bo.Arity != 1 {
		t.Fatal("builtin fields")
	}
	r, err := bo.Fn(nil)
	if err != nil || r.I != 42 {
		t.Fatal("builtin call")
	}
}

// --- Ok / Err / Error ---

func TestOkErr(t *testing.T) {
	ok := runtime.Ok(runtime.Int(7))
	ro := ok.Obj.(*runtime.ResultObj)
	if !ro.Ok || ro.Val.I != 7 {
		t.Fatal("Ok")
	}

	er := runtime.Err(runtime.NewError("boom", "test"))
	ro2 := er.Obj.(*runtime.ResultObj)
	if ro2.Ok {
		t.Fatal("Err should not be ok")
	}
}

func TestNewError(t *testing.T) {
	e := runtime.NewError("oops", "")
	so := e.Obj.(*runtime.StructObj)
	if so.TypeName != "Error" {
		t.Fatal("error type name")
	}
	if so.Fields["message"].S != "oops" {
		t.Fatal("error message")
	}
	if so.Fields["kind"].S != "user" {
		t.Fatal("default kind should be 'user'")
	}
}

func TestNewErrorCode(t *testing.T) {
	e := runtime.NewErrorCode("not found", "http", runtime.Int(404))
	so := e.Obj.(*runtime.StructObj)
	if so.Fields["code"].I != 404 {
		t.Fatal("error code")
	}
}

func TestWrapError(t *testing.T) {
	inner := runtime.NewError("disk full", "io")
	outer := runtime.WrapError(inner, "save failed")
	so := outer.Obj.(*runtime.StructObj)
	if so.Fields["message"].S != "save failed: disk full" {
		t.Fatalf("wrap message = %q", so.Fields["message"].S)
	}
	if so.Fields["kind"].S != "io" {
		t.Fatal("wrap should preserve inner kind")
	}
	if so.Fields["cause"].Kind != runtime.KindStruct {
		t.Fatal("wrap should set cause")
	}
}

func TestWrapErrorEmptyContext(t *testing.T) {
	inner := runtime.NewError("x", "io")
	outer := runtime.WrapError(inner, "")
	so := outer.Obj.(*runtime.StructObj)
	if so.Fields["message"].S != "x" {
		t.Fatalf("empty context message = %q", so.Fields["message"].S)
	}
}

func TestWrapErrorNonError(t *testing.T) {
	// wrapping a non-Error value
	outer := runtime.WrapError(runtime.Str("raw"), "context")
	so := outer.Obj.(*runtime.StructObj)
	if so.Fields["message"].S != "context" {
		t.Fatal("wrap non-error should use context as message")
	}
}

func TestWrapErrorPreservesCode(t *testing.T) {
	inner := runtime.NewErrorCode("bad", "http", runtime.Int(500))
	so := inner.Obj.(*runtime.StructObj)
	so.Fields["at"] = runtime.Str("file.weft:10")
	outer := runtime.WrapError(inner, "oops")
	oo := outer.Obj.(*runtime.StructObj)
	if oo.Fields["code"].I != 500 {
		t.Fatal("should preserve code")
	}
	if oo.Fields["at"].S != "file.weft:10" {
		t.Fatal("should preserve at")
	}
}

func TestErrorWithLocation(t *testing.T) {
	e := runtime.NewError("x", "user")
	e2 := runtime.ErrorWithLocation(e, "test.weft:5")
	so := e2.Obj.(*runtime.StructObj)
	if so.Fields["at"].S != "test.weft:5" {
		t.Fatal("location not set")
	}
	// should not overwrite existing location
	e3 := runtime.ErrorWithLocation(e2, "other:10")
	so3 := e3.Obj.(*runtime.StructObj)
	if so3.Fields["at"].S != "test.weft:5" {
		t.Fatal("should not overwrite existing location")
	}
}

func TestErrorWithLocationNonError(t *testing.T) {
	v := runtime.Str("not an error")
	v2 := runtime.ErrorWithLocation(v, "x:1")
	if v2.S != "not an error" {
		t.Fatal("should return non-error unchanged")
	}
}

func TestErrorWithLocationEmpty(t *testing.T) {
	e := runtime.NewError("x", "user")
	e2 := runtime.ErrorWithLocation(e, "")
	// empty at should not set
	if e2.Obj.(*runtime.StructObj).Fields["at"].Kind != runtime.KindNull {
		t.Fatal("empty location should not set at")
	}
}

func TestIsError(t *testing.T) {
	if !runtime.IsError(runtime.NewError("x", "y")) {
		t.Fatal("should be error")
	}
	if runtime.IsError(runtime.Int(5)) {
		t.Fatal("int should not be error")
	}
	if runtime.IsError(runtime.Struct("Foo", nil, nil)) {
		t.Fatal("non-Error struct should not be error")
	}
}

func TestErrorMessage(t *testing.T) {
	e := runtime.NewError("boom", "user")
	if runtime.ErrorMessage(e) != "boom" {
		t.Fatal("ErrorMessage on Error struct")
	}
	r := runtime.Err(e)
	if runtime.ErrorMessage(r) != "boom" {
		t.Fatal("ErrorMessage on Result Err")
	}
	if runtime.ErrorMessage(runtime.Int(42)) != "42" {
		t.Fatal("ErrorMessage on plain value")
	}
}

// --- IsTruthy ---

func TestIsTruthy(t *testing.T) {
	cases := []struct {
		v    runtime.Value
		want bool
	}{
		{runtime.Null(), false},
		{runtime.Unit(), false},
		{runtime.Bool(true), true},
		{runtime.Bool(false), false},
		{runtime.Int(0), false},
		{runtime.Int(1), true},
		{runtime.Float(0), false},
		{runtime.Float(0.1), true},
		{runtime.Str(""), false},
		{runtime.Str("x"), true},
		{runtime.List(), false},
		{runtime.List(runtime.Int(1)), true},
		{runtime.Ok(runtime.Int(1)), true},
		{runtime.Err(runtime.NewError("x", "y")), false},
		{runtime.Func(&runtime.FuncObj{Name: "f"}), true},
	}
	for i, tc := range cases {
		if tc.v.IsTruthy() != tc.want {
			t.Errorf("case %d: IsTruthy(%v) = %v, want %v", i, tc.v, tc.v.IsTruthy(), tc.want)
		}
	}
	// map truthy
	m := runtime.NewMap()
	if m.IsTruthy() {
		t.Fatal("empty map should be falsy")
	}
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"k"}
	mo.Vals["k"] = runtime.Int(1)
	if !m.IsTruthy() {
		t.Fatal("non-empty map should be truthy")
	}
}

// --- String ---

func TestString(t *testing.T) {
	cases := []struct {
		v    runtime.Value
		want string
	}{
		{runtime.Null(), "null"},
		{runtime.Unit(), "unit"},
		{runtime.Bool(true), "true"},
		{runtime.Bool(false), "false"},
		{runtime.Int(42), "42"},
		{runtime.Float(3.14), "3.14"},
		{runtime.Str("hi"), "hi"},
		{runtime.List(runtime.Int(1), runtime.Int(2)), "[1, 2]"},
		{runtime.Ok(runtime.Int(7)), "Ok(7)"},
		{runtime.Err(runtime.NewError("x", "y")), "Err(Error{message: x, kind: y, code: null, cause: null, at: null})"},
		{runtime.Func(&runtime.FuncObj{Name: "add"}), "<fn add>"},
		{runtime.MakeBuiltin("test", 0, nil), "<builtin test>"},
		{runtime.MakeIter(&runtime.SliceIter{}), "<iter>"},
	}
	for i, tc := range cases {
		if tc.v.String() != tc.want {
			t.Errorf("case %d: String() = %q, want %q", i, tc.v.String(), tc.want)
		}
	}
}

func TestStringMap(t *testing.T) {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"a"}
	mo.Vals["a"] = runtime.Int(1)
	s := m.String()
	if s != `{"a": 1}` {
		t.Fatalf("map string = %q", s)
	}
}

func TestStringStruct(t *testing.T) {
	v := runtime.Struct("Point", map[string]runtime.Value{
		"x": runtime.Int(1),
	}, []string{"x"})
	if v.String() != "Point{x: 1}" {
		t.Fatalf("struct string = %q", v.String())
	}
	// anonymous struct
	v2 := runtime.Struct("", map[string]runtime.Value{"a": runtime.Int(1)}, []string{"a"})
	if v2.String() != "struct{a: 1}" {
		t.Fatalf("anon struct string = %q", v2.String())
	}
}

func TestStringSecret(t *testing.T) {
	v := runtime.Struct("Secret", map[string]runtime.Value{"key": runtime.Str("password")}, []string{"key"})
	if v.String() != "***" {
		t.Fatal("Secret should mask")
	}
}

func TestStringTypeInfo(t *testing.T) {
	ti := &runtime.TypeInfo{Name: "MyType", Kind: "struct"}
	v := runtime.TypeInfoValue(ti)
	if v.String() != "<type MyType>" {
		t.Fatalf("typeinfo string = %q", v.String())
	}
}

// --- Equal ---

func TestEqual(t *testing.T) {
	cases := []struct {
		a, b runtime.Value
		want bool
	}{
		{runtime.Null(), runtime.Null(), true},
		{runtime.Unit(), runtime.Unit(), true},
		{runtime.Bool(true), runtime.Bool(true), true},
		{runtime.Bool(true), runtime.Bool(false), false},
		{runtime.Int(1), runtime.Int(1), true},
		{runtime.Int(1), runtime.Int(2), false},
		{runtime.Float(1.5), runtime.Float(1.5), true},
		{runtime.Float(1.5), runtime.Float(2.5), false},
		{runtime.Str("a"), runtime.Str("a"), true},
		{runtime.Str("a"), runtime.Str("b"), false},
		// cross-type int/float
		{runtime.Int(1), runtime.Float(1.0), true},
		{runtime.Float(1.0), runtime.Int(1), true},
		{runtime.Int(1), runtime.Float(1.5), false},
		// different kinds
		{runtime.Int(1), runtime.Str("1"), false},
		{runtime.Null(), runtime.Bool(false), false},
	}
	for i, tc := range cases {
		if runtime.Equal(tc.a, tc.b) != tc.want {
			t.Errorf("case %d: Equal(%v, %v) = %v, want %v", i, tc.a, tc.b, runtime.Equal(tc.a, tc.b), tc.want)
		}
	}
}

func TestEqualList(t *testing.T) {
	a := runtime.List(runtime.Int(1), runtime.Int(2))
	b := runtime.List(runtime.Int(1), runtime.Int(2))
	c := runtime.List(runtime.Int(1))
	if !runtime.Equal(a, b) {
		t.Fatal("equal lists")
	}
	if runtime.Equal(a, c) {
		t.Fatal("unequal lists")
	}
}

func TestEqualMap(t *testing.T) {
	a := runtime.NewMap()
	ao := a.Obj.(*runtime.MapObj)
	ao.Keys = []string{"x"}
	ao.Vals["x"] = runtime.Int(1)

	b := runtime.NewMap()
	bo := b.Obj.(*runtime.MapObj)
	bo.Keys = []string{"x"}
	bo.Vals["x"] = runtime.Int(1)

	c := runtime.NewMap()
	co := c.Obj.(*runtime.MapObj)
	co.Keys = []string{"x"}
	co.Vals["x"] = runtime.Int(2)

	if !runtime.Equal(a, b) {
		t.Fatal("equal maps")
	}
	if runtime.Equal(a, c) {
		t.Fatal("unequal maps")
	}
	// different sizes
	d := runtime.NewMap()
	if runtime.Equal(a, d) {
		t.Fatal("different size maps")
	}
}

func TestEqualStruct(t *testing.T) {
	a := runtime.Struct("P", map[string]runtime.Value{"x": runtime.Int(1)}, []string{"x"})
	b := runtime.Struct("P", map[string]runtime.Value{"x": runtime.Int(1)}, []string{"x"})
	c := runtime.Struct("Q", map[string]runtime.Value{"x": runtime.Int(1)}, []string{"x"})
	d := runtime.Struct("P", map[string]runtime.Value{"x": runtime.Int(2)}, []string{"x"})
	if !runtime.Equal(a, b) {
		t.Fatal("equal structs")
	}
	if runtime.Equal(a, c) {
		t.Fatal("different type names")
	}
	if runtime.Equal(a, d) {
		t.Fatal("different field values")
	}
	// different field count
	e := runtime.Struct("P", map[string]runtime.Value{"x": runtime.Int(1), "y": runtime.Int(2)}, []string{"x", "y"})
	if runtime.Equal(a, e) {
		t.Fatal("different field count")
	}
}

func TestEqualResult(t *testing.T) {
	a := runtime.Ok(runtime.Int(1))
	b := runtime.Ok(runtime.Int(1))
	c := runtime.Ok(runtime.Int(2))
	d := runtime.Err(runtime.NewError("x", "y"))
	if !runtime.Equal(a, b) {
		t.Fatal("equal Ok")
	}
	if runtime.Equal(a, c) {
		t.Fatal("unequal Ok")
	}
	if runtime.Equal(a, d) {
		t.Fatal("Ok != Err")
	}
	e1 := runtime.Err(runtime.NewError("a", "b"))
	e2 := runtime.Err(runtime.NewError("a", "b"))
	if !runtime.Equal(e1, e2) {
		t.Fatal("equal Err")
	}
}

func TestEqualDefault(t *testing.T) {
	// two builtins: equal only if same object
	a := runtime.MakeBuiltin("x", 0, nil)
	b := runtime.MakeBuiltin("x", 0, nil)
	if runtime.Equal(a, b) {
		t.Fatal("different builtin objects should not be equal")
	}
	if !runtime.Equal(a, a) {
		t.Fatal("same object should be equal")
	}
}

// --- AsInt ---

func TestAsInt(t *testing.T) {
	n, err := runtime.AsInt(runtime.Int(5))
	if err != nil || n != 5 {
		t.Fatal("AsInt int")
	}
	n, err = runtime.AsInt(runtime.Float(3.7))
	if err != nil || n != 3 {
		t.Fatal("AsInt float")
	}
	n, err = runtime.AsInt(runtime.Str("42"))
	if err != nil || n != 42 {
		t.Fatal("AsInt str")
	}
	_, err = runtime.AsInt(runtime.Str("abc"))
	if err == nil {
		t.Fatal("AsInt bad str should fail")
	}
	_, err = runtime.AsInt(runtime.Bool(true))
	if err == nil {
		t.Fatal("AsInt bool should fail")
	}
}

// --- KindName ---

func TestKindName(t *testing.T) {
	cases := []struct {
		v    runtime.Value
		want string
	}{
		{runtime.Null(), "null"},
		{runtime.Bool(true), "bool"},
		{runtime.Int(1), "int"},
		{runtime.Float(1), "float"},
		{runtime.Str("x"), "str"},
		{runtime.List(), "list"},
		{runtime.NewMap(), "map"},
		{runtime.Struct("X", nil, nil), "struct"},
		{runtime.Func(&runtime.FuncObj{}), "fn"},
		{runtime.MakeBuiltin("x", 0, nil), "builtin"},
		{runtime.Ok(runtime.Int(1)), "Result"},
		{runtime.Unit(), "unit"},
		{runtime.MakeIter(&runtime.SliceIter{}), "Iter"},
	}
	for _, tc := range cases {
		if tc.v.KindName() != tc.want {
			t.Errorf("KindName() = %q, want %q", tc.v.KindName(), tc.want)
		}
	}
	ti := runtime.TypeInfoValue(&runtime.TypeInfo{Name: "T"})
	if ti.KindName() != "TypeInfo" {
		t.Fatal("TypeInfo kind name")
	}
}

// --- DeepCopy ---

func TestDeepCopyListMap(t *testing.T) {
	a := runtime.List(runtime.Int(1), runtime.Int(2))
	b := runtime.DeepCopy(a)
	a.Obj.(*runtime.ListObj).Items[0] = runtime.Int(99)
	if b.Obj.(*runtime.ListObj).Items[0].I != 1 {
		t.Fatal("deepcopy failed for list")
	}
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"k"}
	mo.Vals["k"] = runtime.Str("v")
	m2 := runtime.DeepCopy(m)
	mo.Vals["k"] = runtime.Str("x")
	if m2.Obj.(*runtime.MapObj).Vals["k"].S != "v" {
		t.Fatal("deepcopy failed for map")
	}
}

func TestDeepCopyStruct(t *testing.T) {
	a := runtime.Struct("P", map[string]runtime.Value{"x": runtime.Int(1)}, []string{"x"})
	b := runtime.DeepCopy(a)
	a.Obj.(*runtime.StructObj).Fields["x"] = runtime.Int(99)
	if b.Obj.(*runtime.StructObj).Fields["x"].I != 1 {
		t.Fatal("deepcopy failed for struct")
	}
}

func TestDeepCopyScalar(t *testing.T) {
	v := runtime.Int(5)
	c := runtime.DeepCopy(v)
	if c.I != 5 {
		t.Fatal("deepcopy scalar")
	}
}

// --- TypeInfoValue ---

func TestTypeInfoValue(t *testing.T) {
	ti := &runtime.TypeInfo{Name: "Foo", Kind: "struct"}
	v := runtime.TypeInfoValue(ti)
	if v.Kind != runtime.KindTypeInfo {
		t.Fatal("kind should be TypeInfo")
	}
	if v.Obj.(*runtime.TypeInfo).Name != "Foo" {
		t.Fatal("typeinfo name")
	}
}
