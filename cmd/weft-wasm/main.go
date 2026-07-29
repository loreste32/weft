//go:build js && wasm

// Command weft-wasm is the browser Wasm entry for Weft (GOOS=js GOARCH=wasm).
// Exposes runWeft(code) → {output, error} for the playground / weft.js loader.
package main

import (
	"bytes"
	"context"
	"fmt"
	"syscall/js"
	"time"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/vm"
)

func main() {
	js.Global().Set("runWeft", js.FuncOf(runWeft))
	js.Global().Set("weftVersion", js.ValueOf("0.4.1-wasm"))
	// Keep Go runtime alive for JS callbacks
	select {}
}

func runWeft(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return jsResult("", "missing code argument")
	}
	code := args[0].String()
	if len(code) > 100_000 {
		return jsResult("", "code too large (max 100KB)")
	}
	timeoutMs := 5000
	if len(args) >= 2 && args[1].Type() == js.TypeNumber {
		timeoutMs = args[1].Int()
		if timeoutMs <= 0 || timeoutMs > 30_000 {
			timeoutMs = 5000
		}
	}

	var out bytes.Buffer
	env := runtime.NewEnv()
	env.Stdout = &out
	env.Stderr = &out
	stdlib.Register(env, stdlib.Options{})
	env.Call = func(fn runtime.Value, callArgs []runtime.Value) (runtime.Value, error) {
		switch fn.Kind {
		case runtime.KindBuiltin:
			return fn.Obj.(*runtime.BuiltinObj).Fn(callArgs)
		case runtime.KindFunc:
			return vm.New(env).RunFunc(fn.Obj.(*runtime.FuncObj), callArgs)
		default:
			return runtime.Null(), fmt.Errorf("not callable")
		}
	}
	env.SetShared("args", runtime.List())

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	env.Ctx = ctx

	file, perrs := parse.ParseFile("playground.weft", code)
	if perrs.HasErrors() {
		return jsResult(out.String(), perrs.Error())
	}
	prog, cerrs := compile.CompileFile(file, env)
	if cerrs.HasErrors() {
		return jsResult(out.String(), cerrs.Error())
	}
	if prog.Main == nil {
		return jsResult(out.String(), "no main function")
	}
	_, err := vm.New(env).RunFunc(prog.Main, nil)
	if err != nil {
		if ctx.Err() != nil {
			return jsResult(out.String(), "execution timed out")
		}
		return jsResult(out.String(), err.Error())
	}
	return jsResult(out.String(), "")
}

func jsResult(output, errMsg string) js.Value {
	obj := js.Global().Get("Object").New()
	obj.Set("output", output)
	if errMsg != "" {
		obj.Set("error", errMsg)
	} else {
		obj.Set("error", js.Null())
	}
	return obj
}
