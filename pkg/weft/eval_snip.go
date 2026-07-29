package weft

import (
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/vm"
)

// Eval runs a snippet in the context. Used by the REPL.
func (c *Context) Eval(src string) (runtime.Value, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return runtime.Unit(), nil
	}

	if looksLikeProgram(src) {
		file, errs := parse.ParseFile("<repl>", src)
		if errs.HasErrors() {
			return runtime.Null(), errs
		}
		// CompileFileREPL: no main required; top-level fn/type/const/enum
		// bind into the shared env so later lines can call them.
		prog, cerrs := compile.CompileFileREPL(file, c.env)
		if cerrs.HasErrors() {
			return runtime.Null(), cerrs
		}
		if prog.Main != nil {
			return vm.New(c.env).RunFunc(prog.Main, nil)
		}
		return runtime.Unit(), nil
	}

	if isLikelyExpr(src) {
		return c.evalExpr(src)
	}
	return c.evalREPLBlock(src)
}

func looksLikeProgram(src string) bool {
	s := strings.TrimSpace(src)
	for _, p := range []string{"fn ", "type ", "import ", "use ", "const ", "pub "} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return strings.Contains(s, "\nfn ")
}

func isLikelyExpr(src string) bool {
	s := strings.TrimSpace(src)
	for _, p := range []string{"let ", "let\t", "mut ", "if ", "for ", "while ", "return ", "fn ", "const ", "type ", "say "} {
		if strings.HasPrefix(s, p) {
			return false
		}
	}
	if strings.Contains(s, "\n") && (strings.Contains(s, "let ") || strings.Contains(s, ":=") || strings.Contains(s, "if ")) {
		return false
	}
	// x := ... is a statement
	if strings.Contains(s, ":=") {
		return false
	}
	return true
}

func (c *Context) evalExpr(src string) (runtime.Value, error) {
	wrapped := "fn __repl() {\nreturn (" + src + ")\n}"
	file, errs := parse.ParseFile("<repl>", wrapped)
	if errs.HasErrors() {
		return runtime.Null(), errs
	}
	prog, cerrs := compile.CompileFileREPL(file, c.env)
	if cerrs.HasErrors() {
		return runtime.Null(), cerrs
	}
	fn, ok := prog.Funcs["__repl"]
	if !ok {
		return runtime.Null(), fmt.Errorf("internal: missing __repl")
	}
	return vm.New(c.env).RunFunc(fn, nil)
}

func (c *Context) evalREPLBlock(src string) (runtime.Value, error) {
	wrapped := "fn __repl() {\n" + src + "\n}"
	file, errs := parse.ParseFile("<repl>", wrapped)
	if errs.HasErrors() {
		return runtime.Null(), errs
	}
	prog, cerrs := compile.CompileFileREPL(file, c.env)
	if cerrs.HasErrors() {
		return runtime.Null(), cerrs
	}
	fn := prog.Funcs["__repl"]
	if fn == nil {
		return runtime.Null(), fmt.Errorf("internal: missing __repl")
	}
	return vm.New(c.env).RunFunc(fn, nil)
}

// EvalString is a convenience for tests.
func EvalString(src string) (string, error) {
	var b strings.Builder
	ctx := New(Options{Stdout: &b, Stderr: &b})
	v, err := ctx.Eval(src)
	if err != nil {
		return b.String(), err
	}
	if v.Kind == runtime.KindUnit || v.Kind == runtime.KindNull {
		return strings.TrimSpace(b.String()), nil
	}
	out := strings.TrimSpace(b.String())
	if out != "" {
		return out + "\n" + v.String(), nil
	}
	return v.String(), nil
}
