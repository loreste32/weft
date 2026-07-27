package compile_test

import (
	"testing"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
)

func TestCompileMainAndChunkFile(t *testing.T) {
	src := `fn main { say(1) }`
	file, errs := parse.ParseFile("hello.weft", src)
	if errs.HasErrors() {
		t.Fatal(errs)
	}
	env := runtime.NewEnv()
	stdlib.Register(env, stdlib.Options{})
	prog, cerrs := compile.CompileFile(file, env)
	if cerrs.HasErrors() {
		t.Fatal(cerrs)
	}
	if prog.Main == nil {
		t.Fatal("no main")
	}
	ch, ok := prog.Main.Chunk.(*compile.Chunk)
	if !ok || ch == nil {
		t.Fatal("no chunk")
	}
	if ch.File != "hello.weft" {
		t.Fatalf("chunk file %q", ch.File)
	}
	if ch.Name != "main" {
		t.Fatalf("name %q", ch.Name)
	}
	if len(ch.Code) == 0 {
		t.Fatal("empty code")
	}
}

func TestCompileMissingMain(t *testing.T) {
	file, errs := parse.ParseFile("x.weft", `fn helper { 1 }`)
	if errs.HasErrors() {
		t.Fatal(errs)
	}
	env := runtime.NewEnv()
	stdlib.Register(env, stdlib.Options{})
	_, cerrs := compile.CompileFile(file, env)
	if !cerrs.HasErrors() {
		t.Fatal("want no main error")
	}
}
