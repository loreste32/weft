//go:build js && wasm

// Command weft-wasm is the browser Wasm entry for Weft (GOOS=js GOARCH=wasm).
// Exposes runWeft(code) → {output, error} for the playground / weft.js loader.
package main

import (
	"fmt"
	"math"
	"syscall/js"
	"time"
)

// weftVersion is set at build time via -ldflags "-X main.weftVersion=..."
// (see the wasm target in the Makefile, which derives it from pkg/weft.Version).
var weftVersion = "dev-wasm"

var (
	runWeftFunc      js.Func
	runWeftAsyncFunc js.Func
)

func main() {
	runWeftFunc = js.FuncOf(runWeft)
	runWeftAsyncFunc = js.FuncOf(runWeftAsync)
	js.Global().Set("runWeft", runWeftFunc)
	js.Global().Set("runWeftAsync", runWeftAsyncFunc)
	js.Global().Set("weftVersion", js.ValueOf(weftVersion))
	// Keep Go runtime alive for JS callbacks
	select {}
}

type runRequest struct {
	code    string
	timeout time.Duration
}

func parseRunRequest(args []js.Value) (runRequest, error) {
	if len(args) < 1 {
		return runRequest{}, fmt.Errorf("missing code argument")
	}
	if args[0].Type() != js.TypeString {
		return runRequest{}, fmt.Errorf("code argument must be a string")
	}
	code := args[0].String()
	if len(code) > maxSourceBytes {
		return runRequest{}, fmt.Errorf("code too large (max %d bytes)", maxSourceBytes)
	}
	timeout := defaultTimeout
	if len(args) >= 2 && args[1].Type() != js.TypeUndefined {
		if args[1].Type() != js.TypeNumber {
			return runRequest{}, fmt.Errorf("timeout must be a finite number of milliseconds")
		}
		milliseconds := args[1].Float()
		if math.IsNaN(milliseconds) || math.IsInf(milliseconds, 0) ||
			milliseconds <= 0 || milliseconds > float64(maxTimeout/time.Millisecond) ||
			math.Trunc(milliseconds) != milliseconds {
			return runRequest{}, fmt.Errorf("timeout must be an integer from 1 to %d milliseconds", maxTimeout/time.Millisecond)
		}
		timeout = time.Duration(milliseconds) * time.Millisecond
	}
	return runRequest{code: code, timeout: timeout}, nil
}

func runWeft(this js.Value, args []js.Value) any {
	request, err := parseRunRequest(args)
	if err != nil {
		return jsResult("", err.Error())
	}
	output, err := runSource(request.code, request.timeout)
	if err != nil {
		return jsResult(output, err.Error())
	}
	return jsResult(output, "")
}

// runWeftAsync keeps CPU-bound programs and browser-backed operations off the
// JavaScript call stack. It resolves with the same result shape as runWeft.
func runWeftAsync(this js.Value, args []js.Value) any {
	request, err := parseRunRequest(args)
	if err != nil {
		return jsResult("", err.Error())
	}

	executor := js.FuncOf(func(this js.Value, promiseArgs []js.Value) any {
		resolve := promiseArgs[0]
		go func() {
			output, runErr := runSourceMode(request.code, request.timeout, true)
			message := ""
			if runErr != nil {
				message = runErr.Error()
			}
			resolve.Invoke(jsResult(output, message))
		}()
		return nil
	})
	defer executor.Release()
	return js.Global().Get("Promise").New(executor)
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
