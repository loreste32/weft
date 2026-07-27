package runtime_test

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

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

func TestResultOkErr(t *testing.T) {
	ok := runtime.Ok(runtime.Int(7))
	if !ok.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("ok")
	}
	er := runtime.Err(runtime.NewError("boom", "test"))
	if er.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("err")
	}
}
