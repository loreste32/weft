package stdlib

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageIO covers stdin/stdout helpers for pipe-friendly CLIs.
func packageIO(env *runtime.Env) runtime.Value {
	p := pkg()

	// io.stdin() -> Result[str]  read all stdin
	set(p, "stdin", func(args []runtime.Value) (runtime.Value, error) {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return errRes(err.Error(), "io"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 0)

	// io.lines() -> Result[[str]]  stdin lines (trim right newline)
	set(p, "lines", func(args []runtime.Value) (runtime.Value, error) {
		sc := bufio.NewScanner(os.Stdin)
		// allow long lines
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 10*1024*1024)
		var items []runtime.Value
		for sc.Scan() {
			items = append(items, runtime.Str(sc.Text()))
		}
		if err := sc.Err(); err != nil {
			return errRes(err.Error(), "io"), nil
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 0)

	// io.eprint / io.eprintln — stderr
	set(p, "eprint", func(args []runtime.Value) (runtime.Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		fmt.Fprint(env.Stderr, strings.Join(parts, " "))
		return runtime.Unit(), nil
	}, -1)
	set(p, "eprintln", func(args []runtime.Value) (runtime.Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		fmt.Fprintln(env.Stderr, strings.Join(parts, " "))
		return runtime.Unit(), nil
	}, -1)

	// io.is_tty() -> bool  (stdout)
	set(p, "is_tty", func(args []runtime.Value) (runtime.Value, error) {
		fi, err := os.Stdout.Stat()
		if err != nil {
			return runtime.Bool(false), nil
		}
		return runtime.Bool((fi.Mode() & os.ModeCharDevice) != 0), nil
	}, 0)

	return p
}
