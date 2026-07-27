package stdlib

import (
	"github.com/loreste/weft/internal/runtime"
)

// packagePipe — pipeline helpers and aliases for ETL composition.
func packagePipe(env *runtime.Env) runtime.Value {
	p := pkg()

	// pipe.batch(list, size) -> [[...], ...]
	set(p, "batch", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("pipe.batch(list, size)", "pipe"), nil
		}
		n, err := runtime.AsInt(args[1])
		if err != nil || n <= 0 {
			return errRes("pipe.batch: size > 0", "pipe"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		var batches []runtime.Value
		for i := 0; i < len(items); i += int(n) {
			end := i + int(n)
			if end > len(items) {
				end = len(items)
			}
			batches = append(batches, runtime.List(items[i:end]...))
		}
		return runtime.List(batches...), nil
	}, 2)

	// pipe.flatten(list_of_lists) -> list
	set(p, "flatten", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.List(), nil
		}
		var out []runtime.Value
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			if it.Kind == runtime.KindList {
				out = append(out, it.Obj.(*runtime.ListObj).Items...)
			} else {
				out = append(out, it)
			}
		}
		return runtime.List(out...), nil
	}, 1)

	// pipe.compact(list) -> drop null
	set(p, "compact", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.List(), nil
		}
		var out []runtime.Value
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			if it.Kind != runtime.KindNull {
				out = append(out, it)
			}
		}
		return runtime.List(out...), nil
	}, 1)

	// pipe.zip(a, b) -> [[a0,b0], ...]
	set(p, "zip", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList || args[1].Kind != runtime.KindList {
			return errRes("pipe.zip(list, list)", "pipe"), nil
		}
		a := args[0].Obj.(*runtime.ListObj).Items
		b := args[1].Obj.(*runtime.ListObj).Items
		n := len(a)
		if len(b) < n {
			n = len(b)
		}
		out := make([]runtime.Value, n)
		for i := 0; i < n; i++ {
			out[i] = runtime.List(a[i], b[i])
		}
		return runtime.List(out...), nil
	}, 2)

	// pipe.enumerate(list) -> [[i, x], ...]
	set(p, "enumerate", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.List(), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		out := make([]runtime.Value, len(items))
		for i, it := range items {
			out[i] = runtime.List(runtime.Int(int64(i)), it)
		}
		return runtime.List(out...), nil
	}, 1)

	// aliases
	set(p, "map", func(args []runtime.Value) (runtime.Value, error) {
		return env.Globals["map"].Obj.(*runtime.BuiltinObj).Fn(args)
	}, 2)
	set(p, "filter", func(args []runtime.Value) (runtime.Value, error) {
		return env.Globals["filter"].Obj.(*runtime.BuiltinObj).Fn(args)
	}, 2)
	set(p, "reduce", func(args []runtime.Value) (runtime.Value, error) {
		return env.Globals["reduce"].Obj.(*runtime.BuiltinObj).Fn(args)
	}, 3)
	set(p, "par_map", func(args []runtime.Value) (runtime.Value, error) {
		return env.Globals["par_map"].Obj.(*runtime.BuiltinObj).Fn(args)
	}, 3)

	return p
}
