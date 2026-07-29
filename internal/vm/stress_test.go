package vm_test

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/vm"
)

func runSrc(t *testing.T, src string) (string, error) {
	t.Helper()
	f, errs := parse.ParseFile("t.weft", src)
	if errs.HasErrors() {
		t.Fatal(errs)
	}
	env := runtime.NewEnv()
	var buf bytes.Buffer
	env.Stdout = &buf
	env.Stderr = &buf
	stdlib.Register(env, stdlib.Options{})
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		switch fn.Kind {
		case runtime.KindBuiltin:
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		case runtime.KindFunc:
			return vm.New(env).RunFunc(fn.Obj.(*runtime.FuncObj), args)
		default:
			return runtime.Null(), fmt.Errorf("not callable")
		}
	}
	prog, cerrs := compile.CompileFile(f, env)
	if cerrs.HasErrors() {
		t.Fatal(cerrs)
	}
	if err := compile.ValidateProgram(prog); err != nil {
		t.Fatal(err)
	}
	_, err := vm.New(env).RunFunc(prog.Main, nil)
	return buf.String(), err
}

// Concurrent-by-default map/filter must not corrupt results for pure lambdas.
func TestConcurrentMapDeterministic(t *testing.T) {
	src := `
fn main -> Result {
    xs := range(100)
    a := map(xs, fn(n) { n * 2 })
    b := seq_map(xs, fn(n) { n * 2 })
    ensure(len(a) == len(b), "len")?
    mut i := 0
    while i < len(a) {
        ensure(a[i] == b[i], "order")?
        i = i + 1
    }
    say("ok")
    Ok(unit)
}
`
	out, err := runSrc(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty output")
	}
}

// Many VMs in parallel must not race on shared env registration (each has own env).
func TestParallelVMsNoRace(t *testing.T) {
	const n = 32
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			src := fmt.Sprintf(`fn main { say(%d) }`, id)
			f, perrs := parse.ParseFile("p.weft", src)
			if perrs.HasErrors() {
				errs <- perrs
				return
			}
			env := runtime.NewEnv()
			var buf bytes.Buffer
			env.Stdout = &buf
			stdlib.Register(env, stdlib.Options{})
			prog, cerrs := compile.CompileFile(f, env)
			if cerrs.HasErrors() {
				errs <- cerrs
				return
			}
			if _, err := vm.New(env).RunFunc(prog.Main, nil); err != nil {
				errs <- err
				return
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestStackOverflowBounded(t *testing.T) {
	// Deep recursion should error, not hang forever or segfault.
	src := `
fn rec(n) {
    if n <= 0 { return 0 }
    rec(n - 1)
}
fn main { rec(100000) }
`
	_, err := runSrc(t, src)
	if err == nil {
		t.Fatal("expected stack overflow or depth error")
	}
}

func FuzzCompileAndRun(f *testing.F) {
	for _, s := range []string{
		`fn main { }`,
		`fn main { say(1) }`,
		`fn main { x := [1,2,3]; say(len(x)) }`,
		`fn main { say(1+2) }`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		file, perrs := parse.ParseFile("fuzz.weft", src)
		if perrs.HasErrors() {
			return
		}
		env := runtime.NewEnv()
		var discard bytes.Buffer
		env.Stdout = &discard
		env.Stderr = &discard
		stdlib.Register(env, stdlib.Options{})
		prog, cerrs := compile.CompileFile(file, env)
		if cerrs.HasErrors() || prog == nil || prog.Main == nil {
			return
		}
		if err := compile.ValidateProgram(prog); err != nil {
			t.Fatalf("invalid bytecode: %v", err)
		}
		_, _ = vm.New(env).RunFunc(prog.Main, nil)
	})
}
