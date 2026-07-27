package weft

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// RunREPL starts an interactive session on in/out.
func (c *Context) RunREPL(in io.Reader, out io.Writer) error {
	fmt.Fprintf(out, "weft %s — type :help, :quit\n", Version)
	sc := bufio.NewScanner(in)
	// allow large pastes
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	var multi strings.Builder
	depth := 0

	prompt := func() {
		if depth > 0 || multi.Len() > 0 {
			fmt.Fprint(out, "... ")
		} else {
			fmt.Fprint(out, "> ")
		}
	}

	for {
		prompt()
		if !sc.Scan() {
			fmt.Fprintln(out)
			return sc.Err()
		}
		line := sc.Text()

		if multi.Len() == 0 {
			trim := strings.TrimSpace(line)
			switch trim {
			case ":q", ":quit", ":exit":
				return nil
			case ":help", ":h":
				fmt.Fprint(out, replHelp)
				continue
			case "":
				continue
			}
		}

		multi.WriteString(line)
		multi.WriteByte('\n')
		depth = braceDepth(multi.String())
		if depth > 0 {
			continue
		}
		// incomplete string? simple check
		if unbalancedQuotes(multi.String()) {
			continue
		}

		src := strings.TrimSpace(multi.String())
		multi.Reset()
		depth = 0
		if src == "" {
			continue
		}

		v, err := c.Eval(src)
		if err != nil {
			fmt.Fprintln(out, err.Error())
			continue
		}
		if shouldPrint(v) {
			fmt.Fprintln(out, v.String())
		}
	}
}

func shouldPrint(v runtime.Value) bool {
	switch v.Kind {
	case runtime.KindUnit, runtime.KindNull:
		return false
	default:
		return true
	}
}

func braceDepth(s string) int {
	d := 0
	inStr := false
	esc := false
	for _, r := range s {
		if inStr {
			if esc {
				esc = false
				continue
			}
			if r == '\\' {
				esc = true
				continue
			}
			if r == '"' {
				inStr = false
			}
			continue
		}
		if r == '"' {
			inStr = true
			continue
		}
		if r == '{' || r == '(' || r == '[' {
			d++
		} else if r == '}' || r == ')' || r == ']' {
			d--
		}
	}
	if d < 0 {
		return 0
	}
	return d
}

func unbalancedQuotes(s string) bool {
	n := 0
	esc := false
	for _, r := range s {
		if esc {
			esc = false
			continue
		}
		if r == '\\' {
			esc = true
			continue
		}
		if r == '"' {
			n++
		}
	}
	return n%2 == 1
}

const replHelp = `commands:
  :help   this text
  :quit   exit

examples:
  1 + 2
  let mut n = 0
  n = n + 1
  fn double(x) { x * 2 }
  double(21)
  llm.ask("hi")          // needs API key
`

// StartREPL is the CLI entry using stdin/stdout.
func StartREPL() error {
	ctx := New(Options{Stdout: os.Stdout, Stderr: os.Stderr})
	return ctx.RunREPL(os.Stdin, os.Stdout)
}
