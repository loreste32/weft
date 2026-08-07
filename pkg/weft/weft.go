// Package weft is the public embedding API for the Weft language.
package weft

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/pkgman"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/types"
	"github.com/loreste/weft/internal/vm"
)

// Version of the Weft toolchain.
// Release line: 0.4.x (docs/VERSIONING.md). 0.3.x is complete.
const Version = "0.6.0"

// Options configure a Weft context.
type Options struct {
	Stdout     io.Writer
	Stderr     io.Writer
	Args       []string
	HTTPClient *http.Client
	// Environ overrides process environment when non-nil.
	Environ map[string]string
	// LLMBaseURL overrides OpenAI-compat base (tests).
	LLMBaseURL string
	// LLMDo mocks chat completions for tests.
	LLMDo func(reqBody []byte) (content string, calls []runtime.ToolCall, err error)
}

// Context is an isolated Weft execution environment.
type Context struct {
	env  *runtime.Env
	opts Options
}

// New creates a Context with prelude + stdlib.
func New(opts Options) *Context {
	env := runtime.NewEnv()
	if opts.Stdout != nil {
		env.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		env.Stderr = opts.Stderr
	}
	stdlib.ToolchainVersion = Version
	stdlib.Register(env, stdlib.Options{
		HTTPClient: opts.HTTPClient,
		Environ:    opts.Environ,
		LLMBaseURL: opts.LLMBaseURL,
	})
	if opts.LLMDo != nil {
		env.LLMDo = opts.LLMDo
	}

	// Host can call Weft functions (agents/tools). Module funcs carry their Env.
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		switch fn.Kind {
		case runtime.KindBuiltin:
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		case runtime.KindFunc:
			fo := fn.Obj.(*runtime.FuncObj)
			runEnv := env
			if fo.Env != nil {
				runEnv = fo.Env
				if runEnv.Call == nil {
					runEnv.Call = env.Call
				}
			}
			// Module-internal calls resolve globals on the function's home env.
			return vm.New(runEnv).RunFunc(fo, args)
		default:
			return runtime.Null(), fmt.Errorf("not callable")
		}
	}

	args := opts.Args
	if args == nil {
		args = []string{}
	}
	items := make([]runtime.Value, len(args))
	for i, a := range args {
		items[i] = runtime.Str(a)
	}
	// Host-shared so path modules see argv (not a hardcoded clone list).
	env.SetShared("os", runtime.Struct("os", map[string]runtime.Value{
		"args": runtime.List(items...),
	}, []string{"args"}))
	env.SetShared("args", runtime.List(items...))

	// Project root for package imports (vendor/)
	if wd, err := os.Getwd(); err == nil {
		env.ProjectDir = DetectProjectDir(wd)
	}
	return &Context{env: env, opts: opts}
}

// Env exposes the runtime environment (advanced / tests).
func (c *Context) Env() *runtime.Env { return c.env }

// RunFile loads, compiles, and runs path.
func (c *Context) RunFile(ctx context.Context, path string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// Set project dir from script location for vendor resolution
	if abs, err := filepath.Abs(path); err == nil {
		c.env.ProjectDir = DetectProjectDir(filepath.Dir(abs))
	}
	// Fail closed if weft.lock exists but vendor was tampered with.
	if c.env.ProjectDir != "" {
		if err := verifyVendorIfLocked(c.env.ProjectDir); err != nil {
			return err
		}
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return c.RunSource(ctx, path, string(src))
}

func verifyVendorIfLocked(projectDir string) error {
	if _, err := os.Stat(filepath.Join(projectDir, "weft.lock")); err != nil {
		return nil
	}
	return pkgman.VerifyLock(projectDir)
}

// RunSource compiles and runs source with the given filename for diagnostics.
func (c *Context) RunSource(ctx context.Context, filename, src string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// Propagate cancel to I/O (HTTP, LLM, DB timeouts that use env.Context()).
	c.env.Ctx = ctx
	file, errs := parse.ParseFile(filename, src)
	if errs.HasErrors() {
		return errs
	}
	prog, cerrs := compile.CompileFile(file, c.env)
	if cerrs.HasErrors() {
		return cerrs
	}
	if prog.Main == nil {
		return fmt.Errorf("no main function")
	}
	machine := vm.New(c.env)
	ret, err := machine.RunFunc(prog.Main, nil)
	if err != nil {
		if code, ok := runtime.IsExit(err); ok {
			es := err.(*runtime.ExitSignal)
			if es.Message != "" && code != 0 {
				return ExitError{Code: code, Err: fmt.Errorf("%s", es.Message)}
			}
			return ExitError{Code: code}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	if ret.Kind == runtime.KindResult {
		ro := ret.Obj.(*runtime.ResultObj)
		if !ro.Ok {
			return ExitError{Code: 1, Err: fmt.Errorf("%s", ro.Err.String())}
		}
	}
	return nil
}

// Interrupt cancels in-flight work for this context (if RunSource was given a cancelable ctx).
// Prefer passing a cancelable context to RunFile/RunSource.
func (c *Context) EnvContext() context.Context {
	return c.env.Context()
}

// CheckFile runs type inference + checking on a source file (no execution).
func CheckFile(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return CheckSource(path, string(src))
}

// CheckSource type-infers and checks source without running it.
func CheckSource(filename, src string) error {
	_, err := InferSource(filename, src)
	return err
}

// InferSource returns inference info (bindings, fn returns) and any diagnostics.
// Type mismatches are warnings (info.Diags); only hard errors fail the returned error.
func InferSource(filename, src string) (types.Info, error) {
	file, errs := parse.ParseFile(filename, src)
	if errs.HasErrors() {
		return types.Info{}, errs
	}
	info, cerrs := types.Infer(file)
	if cerrs.HasErrors() {
		return info, cerrs
	}
	return info, nil
}

// InferFile infers types for a path.
func InferFile(path string) (types.Info, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return types.Info{}, err
	}
	return InferSource(path, string(src))
}

// ExitError carries a process exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Code == 0 {
		return ""
	}
	return fmt.Sprintf("exit %d", e.Code)
}

// Silent reports whether the exit should print nothing (success exit).
func (e ExitError) Silent() bool {
	return e.Code == 0 && e.Err == nil
}
