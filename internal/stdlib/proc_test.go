package stdlib

import (
	"os"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestProcPackage(t *testing.T) {
	p := packageProc()
	mo := p.Obj.(*runtime.MapObj)

	expected := []string{"self", "kill", "exists", "list", "find"}
	for _, name := range expected {
		if _, ok := mo.Vals[name]; !ok {
			t.Errorf("proc.%s not registered", name)
		}
	}
}

func TestProcSelf(t *testing.T) {
	p := packageProc()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["self"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindMap {
		t.Fatalf("self should return map, got %v", r.Kind)
	}
	rmo := r.Obj.(*runtime.MapObj)
	pid := rmo.Vals["pid"]
	if pid.I != int64(os.Getpid()) {
		t.Errorf("pid = %d, want %d", pid.I, os.Getpid())
	}
}

func TestProcExists(t *testing.T) {
	p := packageProc()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["exists"]

	// Our own PID should exist
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Int(int64(os.Getpid()))})
	if err != nil {
		t.Fatal(err)
	}
	if !r.B {
		t.Error("proc.exists(self) should be true")
	}

	// Very high PID should not exist
	r, err = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Int(999999999)})
	if err != nil {
		t.Fatal(err)
	}
	if r.B {
		t.Error("proc.exists(999999999) should be false")
	}
}

func TestProcList(t *testing.T) {
	p := packageProc()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["list"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Result[list] — should be Ok
	if r.Kind != runtime.KindResult {
		t.Fatalf("list should return Result, got %v", r.Kind)
	}
}

func TestProcRejectsDangerousOrEmptyInputs(t *testing.T) {
	p := packageProc()
	mo := p.Obj.(*runtime.MapObj)

	kill := mo.Vals["kill"].Obj.(*runtime.BuiltinObj)
	result, err := kill.Fn([]runtime.Value{runtime.Int(0)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != runtime.KindResult {
		t.Fatalf("proc.kill should return Result, got %v", result.Kind)
	}
	result, err = kill.Fn([]runtime.Value{runtime.Int(1), runtime.Str("-1")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != runtime.KindResult {
		t.Fatalf("proc.kill should reject negative signals with Result, got %v", result.Kind)
	}

	find := mo.Vals["find"].Obj.(*runtime.BuiltinObj)
	result, err = find.Fn([]runtime.Value{runtime.Str("   ")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != runtime.KindResult {
		t.Fatalf("proc.find should return Result, got %v", result.Kind)
	}
}
