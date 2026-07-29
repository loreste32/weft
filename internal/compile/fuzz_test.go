package compile

import (
	"testing"

	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
)

func FuzzCompileValidate(f *testing.F) {
	for _, s := range []string{
		`fn main { say(1) }`,
		`fn main { x := 1 + 2 * 3; say(x) }`,
		`fn f(a) { a }; fn main { say(f(1)) }`,
		`fn main { if true { 1 } else { 2 } }`,
		`fn main { xs := [1,2]; for x in xs { say(x) } }`,
		`fn main -> Result { Ok(1)? }`,
		`not even code`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		file, perrs := parse.ParseFile("fuzz.weft", src)
		if perrs.HasErrors() {
			return
		}
		env := runtime.NewEnv()
		stdlib.Register(env, stdlib.Options{})
		prog, cerrs := CompileFileLib(file, env) // no main required
		if cerrs.HasErrors() || prog == nil {
			return
		}
		// Valid compiler output must pass structural validation
		if err := ValidateProgram(prog); err != nil {
			t.Fatalf("compiler produced invalid bytecode: %v\nsrc=%q", err, src)
		}
	})
}
