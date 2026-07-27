package runtime_test

import (
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// helper: env with Call configured
func envWithCall() *runtime.Env {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		if fn.Kind == runtime.KindBuiltin {
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		}
		return runtime.Unit(), nil
	}
	return env
}

func TestSpawnJoinAwait(t *testing.T) {
	env := envWithCall()
	fn := runtime.MakeBuiltin("test", 0, func(_ []runtime.Value) (runtime.Value, error) {
		return runtime.Int(42), nil
	})
	env.Set("spawn", env.Globals["spawn"])

	// call spawn directly
	spawnBuiltin := env.Globals["spawn"].Obj.(*runtime.BuiltinObj)
	handle, err := spawnBuiltin.Fn([]runtime.Value{fn})
	if err != nil {
		t.Fatal(err)
	}
	// join
	joinFn := handle.Obj.(*runtime.MapObj).Vals["join"].Obj.(*runtime.BuiltinObj)
	v, err := joinFn.Fn(nil)
	if err != nil || v.I != 42 {
		t.Fatalf("join: v=%v, err=%v", v, err)
	}
	// await (returns Result)
	awaitFn := handle.Obj.(*runtime.MapObj).Vals["await"].Obj.(*runtime.BuiltinObj)
	v, err = awaitFn.Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	ro := v.Obj.(*runtime.ResultObj)
	if !ro.Ok || ro.Val.I != 42 {
		t.Fatal("await should return Ok(42)")
	}
}

func TestSpawnNotFunction(t *testing.T) {
	env := envWithCall()
	spawnFn := env.Globals["spawn"].Obj.(*runtime.BuiltinObj)
	_, err := spawnFn.Fn([]runtime.Value{runtime.Int(1)})
	if err == nil {
		t.Fatal("spawn non-function should error")
	}
}

func TestSpawnNoCall(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = nil
	spawnFn := env.Globals["spawn"].Obj.(*runtime.BuiltinObj)
	_, err := spawnFn.Fn([]runtime.Value{runtime.MakeBuiltin("f", 0, nil)})
	if err == nil {
		t.Fatal("spawn without Call should error")
	}
}

func TestSpawnTooFewArgs(t *testing.T) {
	env := envWithCall()
	spawnFn := env.Globals["spawn"].Obj.(*runtime.BuiltinObj)
	_, err := spawnFn.Fn(nil)
	if err != nil {
		// returns Null + error
	}
}

func TestParallel(t *testing.T) {
	env := envWithCall()
	fn1 := runtime.MakeBuiltin("a", 0, func(_ []runtime.Value) (runtime.Value, error) {
		return runtime.Int(1), nil
	})
	fn2 := runtime.MakeBuiltin("b", 0, func(_ []runtime.Value) (runtime.Value, error) {
		return runtime.Int(2), nil
	})
	parallelFn := env.Globals["parallel"].Obj.(*runtime.BuiltinObj)
	r, err := parallelFn.Fn([]runtime.Value{runtime.List(fn1, fn2)})
	if err != nil {
		t.Fatal(err)
	}
	ro := r.Obj.(*runtime.ResultObj)
	if !ro.Ok {
		t.Fatal("parallel should return Ok")
	}
	items := ro.Val.Obj.(*runtime.ListObj).Items
	if len(items) != 2 || items[0].I != 1 || items[1].I != 2 {
		t.Fatal("parallel results")
	}
}

func TestParallelNotFunction(t *testing.T) {
	env := envWithCall()
	parallelFn := env.Globals["parallel"].Obj.(*runtime.BuiltinObj)
	r, _ := parallelFn.Fn([]runtime.Value{runtime.List(runtime.Int(1))})
	ro := r.Obj.(*runtime.ResultObj)
	if ro.Ok {
		t.Fatal("non-function should fail")
	}
}

func TestParallelNoCall(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = nil
	fn := runtime.MakeBuiltin("a", 0, func(_ []runtime.Value) (runtime.Value, error) { return runtime.Int(1), nil })
	parallelFn := env.Globals["parallel"].Obj.(*runtime.BuiltinObj)
	r, _ := parallelFn.Fn([]runtime.Value{runtime.List(fn)})
	ro := r.Obj.(*runtime.ResultObj)
	if ro.Ok {
		t.Fatal("no Call should fail")
	}
}

func TestRace(t *testing.T) {
	env := envWithCall()
	fn := runtime.MakeBuiltin("fast", 0, func(_ []runtime.Value) (runtime.Value, error) {
		return runtime.Str("won"), nil
	})
	raceFn := env.Globals["race"].Obj.(*runtime.BuiltinObj)
	r, err := raceFn.Fn([]runtime.Value{runtime.List(fn)})
	if err != nil {
		t.Fatal(err)
	}
	ro := r.Obj.(*runtime.ResultObj)
	if !ro.Ok || ro.Val.S != "won" {
		t.Fatal("race result")
	}
}

func TestRaceEmpty(t *testing.T) {
	env := envWithCall()
	raceFn := env.Globals["race"].Obj.(*runtime.BuiltinObj)
	r, _ := raceFn.Fn([]runtime.Value{runtime.List()})
	ro := r.Obj.(*runtime.ResultObj)
	if ro.Ok {
		t.Fatal("empty race should fail")
	}
}

func TestRaceNoCall(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = nil
	raceFn := env.Globals["race"].Obj.(*runtime.BuiltinObj)
	r, _ := raceFn.Fn([]runtime.Value{runtime.List(runtime.MakeBuiltin("f", 0, nil))})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("no Call should fail")
	}
}

func TestRaceNotFunction(t *testing.T) {
	env := envWithCall()
	raceFn := env.Globals["race"].Obj.(*runtime.BuiltinObj)
	r, _ := raceFn.Fn([]runtime.Value{runtime.List(runtime.Int(1))})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("non-function should fail")
	}
}

func TestTimeout(t *testing.T) {
	env := envWithCall()
	fn := runtime.MakeBuiltin("fast", 0, func(_ []runtime.Value) (runtime.Value, error) {
		return runtime.Int(42), nil
	})
	timeoutFn := env.Globals["timeout"].Obj.(*runtime.BuiltinObj)
	r, err := timeoutFn.Fn([]runtime.Value{runtime.Int(5), fn})
	if err != nil {
		t.Fatal(err)
	}
	ro := r.Obj.(*runtime.ResultObj)
	if !ro.Ok || ro.Val.I != 42 {
		t.Fatal("timeout result")
	}
}

func TestTimeoutExpires(t *testing.T) {
	env := envWithCall()
	slow := runtime.MakeBuiltin("slow", 0, func(_ []runtime.Value) (runtime.Value, error) {
		time.Sleep(200 * time.Millisecond)
		return runtime.Int(1), nil
	})
	timeoutFn := env.Globals["timeout"].Obj.(*runtime.BuiltinObj)
	r, _ := timeoutFn.Fn([]runtime.Value{runtime.Float(0.01), slow}) // 10ms
	ro := r.Obj.(*runtime.ResultObj)
	if ro.Ok {
		t.Fatal("should have timed out")
	}
}

func TestTimeoutNotFunction(t *testing.T) {
	env := envWithCall()
	timeoutFn := env.Globals["timeout"].Obj.(*runtime.BuiltinObj)
	r, _ := timeoutFn.Fn([]runtime.Value{runtime.Int(1), runtime.Int(1)})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("non-function should fail")
	}
}

func TestTimeoutNegativeDuration(t *testing.T) {
	env := envWithCall()
	fn := runtime.MakeBuiltin("f", 0, func(_ []runtime.Value) (runtime.Value, error) { return runtime.Int(1), nil })
	timeoutFn := env.Globals["timeout"].Obj.(*runtime.BuiltinObj)
	r, _ := timeoutFn.Fn([]runtime.Value{runtime.Int(-1), fn})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("negative duration should fail")
	}
}

func TestTimeoutNoCall(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = nil
	fn := runtime.MakeBuiltin("f", 0, nil)
	timeoutFn := env.Globals["timeout"].Obj.(*runtime.BuiltinObj)
	r, _ := timeoutFn.Fn([]runtime.Value{runtime.Int(1), fn})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("no Call should fail")
	}
}

func TestGroup(t *testing.T) {
	env := envWithCall()
	groupFn := env.Globals["group"].Obj.(*runtime.BuiltinObj)
	g, err := groupFn.Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	mo := g.Obj.(*runtime.MapObj)
	goFn := mo.Vals["go"].Obj.(*runtime.BuiltinObj)
	fn := runtime.MakeBuiltin("work", 0, func(_ []runtime.Value) (runtime.Value, error) {
		return runtime.Int(10), nil
	})
	goFn.Fn([]runtime.Value{fn})
	goFn.Fn([]runtime.Value{fn})

	waitFn := mo.Vals["wait"].Obj.(*runtime.BuiltinObj)
	r, err := waitFn.Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	ro := r.Obj.(*runtime.ResultObj)
	if !ro.Ok {
		t.Fatal("group wait should be ok")
	}
	items := ro.Val.Obj.(*runtime.ListObj).Items
	if len(items) != 2 {
		t.Fatalf("expected 2 results, got %d", len(items))
	}
}
