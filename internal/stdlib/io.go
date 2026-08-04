package stdlib

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/loreste/weft/internal/runtime"
	"golang.org/x/term"
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

	// io.is_tty()          -> bool  (stdin)
	// io.is_tty("stdout")  -> bool
	// io.is_tty("stderr")  -> bool
	set(p, "is_tty", func(args []runtime.Value) (runtime.Value, error) {
		stream := "stdin"
		if len(args) >= 1 && args[0].Kind == runtime.KindStr {
			stream = args[0].S
		}
		var fd uintptr
		switch stream {
		case "stdin":
			fd = os.Stdin.Fd()
		case "stdout":
			fd = os.Stdout.Fd()
		case "stderr":
			fd = os.Stderr.Fd()
		default:
			return runtime.Bool(false), fmt.Errorf("is_tty: unknown stream %q (use \"stdin\", \"stdout\", or \"stderr\")", stream)
		}
		return runtime.Bool(term.IsTerminal(int(fd))), nil
	}, -1)

	return p
}
