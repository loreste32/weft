package stdlib

import (
	"sync"

	"github.com/loreste/weft/internal/runtime"
)

// packageFunctools — partial/once helpers (Python functools lite).
func packageFunctools(env *runtime.Env) runtime.Value {
	p := pkg()

	// functools.partial(fn, ...fixed) -> builtin that prepends fixed args
	set(p, "partial", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("functools.partial(fn, ...)", "functools"), nil
		}
		fn := args[0]
		if fn.Kind != runtime.KindFunc && fn.Kind != runtime.KindBuiltin && fn.Kind != runtime.KindClosure {
			return errRes("functools.partial: need function", "functools"), nil
		}
		fixed := append([]runtime.Value(nil), args[1:]...)
		return runtime.MakeBuiltin("partial", -1, func(callArgs []runtime.Value) (runtime.Value, error) {
			if env.Call == nil {
				return errRes("functools.partial: Call not configured", "functools"), nil
			}
			all := make([]runtime.Value, 0, len(fixed)+len(callArgs))
			all = append(all, fixed...)
			all = append(all, callArgs...)
			return env.Call(fn, all)
		}), nil
	}, -1)

	// functools.once(fn) -> callable that runs at most once, caches Result/value
	set(p, "once", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("functools.once(fn)", "functools"), nil
		}
		fn := args[0]
		var mu sync.Mutex
		var done bool
		var cached runtime.Value
		var cachedErr error
		return runtime.MakeBuiltin("once", -1, func(callArgs []runtime.Value) (runtime.Value, error) {
			mu.Lock()
			defer mu.Unlock()
			if done {
				return cached, cachedErr
			}
			if env.Call == nil {
				return errRes("functools.once: Call not configured", "functools"), nil
			}
			cached, cachedErr = env.Call(fn, callArgs)
			done = true
			return cached, cachedErr
		}), nil
	}, 1)

	// functools.identity(x) -> x
	set(p, "identity", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		return args[0], nil
	}, 1)

	return p
}
