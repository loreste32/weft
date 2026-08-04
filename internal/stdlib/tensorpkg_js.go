//go:build js

package stdlib

import "github.com/loreste/weft/internal/runtime"

func packageTensor() runtime.Value {
	p := pkg()
	set(p, "supported", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(false), nil
	}, 0)
	deny := func(args []runtime.Value) (runtime.Value, error) {
		return errRes("host tensor package is unavailable in browser WASM", "tensor"), nil
	}
	for _, name := range []string{
		"from_list", "to_list", "shape", "dtype", "numel",
		"add", "sub", "mul", "div", "matmul", "sum", "contiguous", "free", "info",
	} {
		set(p, name, deny, -1)
	}
	return p
}
