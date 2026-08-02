package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/vm"
)

const (
	maxSourceBytes = 100_000
	defaultTimeout = 5 * time.Second
	maxTimeout     = 30 * time.Second
)

var errExecutionTimeout = errors.New("execution timed out")

// runSource is the host-independent part of the browser runtime. Keeping the
// compiler/VM path separate from syscall/js makes it testable with ordinary Go
// tests and guarantees that sync and async browser calls execute identically.
func runSource(code string, timeout time.Duration) (output string, err error) {
	return runSourceMode(code, timeout, false)
}

func runSourceMode(code string, timeout time.Duration, browserAsync bool) (output string, err error) {
	var out bytes.Buffer
	defer func() {
		output = out.String()
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("Wasm runtime panic: %v", recovered)
		}
	}()

	if len(code) > maxSourceBytes {
		return "", fmt.Errorf("code too large (max %d bytes)", maxSourceBytes)
	}
	if timeout <= 0 || timeout > maxTimeout {
		return "", fmt.Errorf("timeout must be greater than 0 and no more than %s", maxTimeout)
	}

	env := runtime.NewEnv()
	env.BrowserAsync = browserAsync
	env.BrowserWasm = true
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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	env.Ctx = ctx

	file, parseErrors := parse.ParseFile("playground.weft", code)
	if parseErrors.HasErrors() {
		return out.String(), errors.New(parseErrors.Error())
	}
	program, compileErrors := compile.CompileFile(file, env)
	if compileErrors.HasErrors() {
		return out.String(), errors.New(compileErrors.Error())
	}
	if program.Main == nil {
		return out.String(), errors.New("no main function")
	}

	_, err = vm.New(env).RunFunc(program.Main, nil)
	if err != nil {
		if errors.Is(env.ContextErr(), context.DeadlineExceeded) {
			return out.String(), errExecutionTimeout
		}
		if errors.Is(env.ContextErr(), context.Canceled) {
			return out.String(), context.Canceled
		}
	}
	return out.String(), err
}
