package runtime_test

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestSliceIter(t *testing.T) {
	it := &runtime.SliceIter{Items: []runtime.Value{runtime.Int(1), runtime.Int(2)}}
	v, ok := it.Next()
	if !ok || v.I != 1 {
		t.Fatal("first")
	}
	v, ok = it.Next()
	if !ok || v.I != 2 {
		t.Fatal("second")
	}
	_, ok = it.Next()
	if ok {
		t.Fatal("should be exhausted")
	}
}

func TestChanIter(t *testing.T) {
	ch := make(chan runtime.Value, 2)
	ch <- runtime.Str("a")
	ch <- runtime.Str("b")
	close(ch)

	it := &runtime.ChanIter{Ch: ch}
	v, ok := it.Next()
	if !ok || v.S != "a" {
		t.Fatal("first")
	}
	v, ok = it.Next()
	if !ok || v.S != "b" {
		t.Fatal("second")
	}
	_, ok = it.Next()
	if ok {
		t.Fatal("should be exhausted")
	}
}

func TestMakeIterAndAsIter(t *testing.T) {
	si := &runtime.SliceIter{Items: []runtime.Value{runtime.Int(1)}}
	v := runtime.MakeIter(si)
	if v.Kind != runtime.KindIter {
		t.Fatal("kind")
	}
	it, err := runtime.AsIter(v)
	if err != nil {
		t.Fatal(err)
	}
	val, ok := it.Next()
	if !ok || val.I != 1 {
		t.Fatal("iter round-trip")
	}
}

func TestAsIterList(t *testing.T) {
	list := runtime.List(runtime.Int(10), runtime.Int(20))
	it, err := runtime.AsIter(list)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := it.Next()
	if !ok || v.I != 10 {
		t.Fatal("list iter first")
	}
	v, ok = it.Next()
	if !ok || v.I != 20 {
		t.Fatal("list iter second")
	}
	// modifying original shouldn't affect iterator (snapshot)
	list.Obj.(*runtime.ListObj).Items[0] = runtime.Int(99)
	// already consumed, but the snapshot was taken at creation
}

func TestAsIterMap(t *testing.T) {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"a", "b"}
	mo.Vals["a"] = runtime.Int(1)
	mo.Vals["b"] = runtime.Int(2)
	it, err := runtime.AsIter(m)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := it.Next()
	if !ok || v.S != "a" {
		t.Fatal("map iter yields keys")
	}
}

func TestAsIterStr(t *testing.T) {
	it, err := runtime.AsIter(runtime.Str("hi"))
	if err != nil {
		t.Fatal(err)
	}
	v, ok := it.Next()
	if !ok || v.S != "h" {
		t.Fatal("str iter first")
	}
	v, ok = it.Next()
	if !ok || v.S != "i" {
		t.Fatal("str iter second")
	}
	_, ok = it.Next()
	if ok {
		t.Fatal("exhausted")
	}
}

func TestAsIterUnsupported(t *testing.T) {
	_, err := runtime.AsIter(runtime.Int(1))
	if err == nil {
		t.Fatal("int should not be iterable")
	}
}

func TestAsIterInvalidObj(t *testing.T) {
	// KindIter but obj is not Iter
	v := runtime.Value{Kind: runtime.KindIter, Obj: "not an iter"}
	_, err := runtime.AsIter(v)
	if err == nil {
		t.Fatal("invalid iter obj should error")
	}
}
