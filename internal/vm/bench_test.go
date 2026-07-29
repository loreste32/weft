package vm_test

import (
	"bytes"
	"testing"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/vm"
)

func benchRun(b *testing.B, src string) {
	b.Helper()
	f, errs := parse.ParseFile("b.weft", src)
	if errs.HasErrors() {
		b.Fatal(errs)
	}
	env := runtime.NewEnv()
	env.Stdout = &bytes.Buffer{}
	env.Stderr = env.Stdout
	stdlib.Register(env, stdlib.Options{})
	prog, cerrs := compile.CompileFile(f, env)
	if cerrs.HasErrors() {
		b.Fatal(cerrs)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := vm.New(env).RunFunc(prog.Main, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFib20(b *testing.B) {
	benchRun(b, `
fn fib(n) {
    if n < 2 { return n }
    fib(n - 1) + fib(n - 2)
}
fn main { fib(20) }
`)
}

func BenchmarkMap1000(b *testing.B) {
	benchRun(b, `
fn main {
    xs := range(1000)
    map(xs, fn(n) { n + 1 })
}
`)
}

func BenchmarkSeqMap1000(b *testing.B) {
	benchRun(b, `
fn main {
    xs := range(1000)
    seq_map(xs, fn(n) { n + 1 })
}
`)
}

func BenchmarkCompileHello(b *testing.B) {
	src := `fn main { say("hello") }`
	for i := 0; i < b.N; i++ {
		f, errs := parse.ParseFile("b.weft", src)
		if errs.HasErrors() {
			b.Fatal(errs)
		}
		env := runtime.NewEnv()
		stdlib.Register(env, stdlib.Options{})
		if _, cerrs := compile.CompileFile(f, env); cerrs.HasErrors() {
			b.Fatal(cerrs)
		}
	}
}

// Glue-oriented: JSON stringify/parse loop (agent/API scripts).
func BenchmarkJSONRoundtrip200(b *testing.B) {
	benchRun(b, `
fn main {
    mut i := 0
    while i < 200 {
        s := json.stringify({"n": i, "tag": "item"})
        _ := json.parse(s)
        i = i + 1
    }
}
`)
}

// Glue-oriented: string split/join/upper.
func BenchmarkStrSplitJoin500(b *testing.B) {
	benchRun(b, `
fn main {
    base := "alpha,beta,gamma,delta,epsilon"
    mut i := 0
    while i < 500 {
        parts := str.split(base, ",")
        _ := str.join(seq_map(parts, fn(p) { str.upper(p) }), "|")
        i = i + 1
    }
}
`)
}
