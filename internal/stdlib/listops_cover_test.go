package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestListOps_InstallAndCall(t *testing.T) {
	env := runtime.NewEnv()
	installListOps(env)
	// call common list ops via globals
	names := []string{"map", "filter", "reduce", "each", "find", "any", "all", "sort", "reverse", "unique", "zip", "flatten", "enumerate", "count", "seq_map", "seq_filter", "par_map"}
	lst := runtime.List(runtime.Int(3), runtime.Int(1), runtime.Int(2), runtime.Int(1))
	id := runtime.MakeBuiltin("id", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		return args[0], nil
	})
	pred := runtime.MakeBuiltin("pred", 1, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(true), nil
	})
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		if fn.Kind == runtime.KindBuiltin {
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		}
		return runtime.Null(), nil
	}
	for _, name := range names {
		v, ok := env.Globals[name]
		if !ok {
			// may be on shared
			v, ok = env.Get(name)
		}
		if !ok || v.Kind != runtime.KindBuiltin {
			continue
		}
		fn := v.Obj.(*runtime.BuiltinObj).Fn
		_, _ = fn([]runtime.Value{lst})
		_, _ = fn([]runtime.Value{lst, id})
		_, _ = fn([]runtime.Value{lst, pred})
		_, _ = fn([]runtime.Value{lst, id, runtime.Int(2)})
		_, _ = fn([]runtime.Value{lst, lst})
		_, _ = fn(nil)
	}
}
