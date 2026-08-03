//go:build js

package stdlib

import "github.com/loreste/weft/internal/runtime"

// Native shared libraries cannot be loaded in browser WASM. The package is
// still registered so code can feature-detect and choose CPU or remote paths.
func packageAccelerator() runtime.Value {
	p := pkg()
	set(p, "supported", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(false), nil
	}, 0)
	set(p, "load", func(args []runtime.Value) (runtime.Value, error) {
		return errRes("native accelerator plugins are unavailable in browser WASM", "accelerator"), nil
	}, 1)
	set(p, "run", func(args []runtime.Value) (runtime.Value, error) {
		return errRes("native accelerator plugins are unavailable in browser WASM", "accelerator"), nil
	}, 3)
	set(p, "close", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Ok(runtime.Bool(false)), nil
	}, 1)
	set(p, "backends", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.List(), nil
	}, 0)
	return p
}
