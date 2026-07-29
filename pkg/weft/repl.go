package weft

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
			switch {
			case trim == ":q" || trim == ":quit" || trim == ":exit":
				return nil
			case trim == ":help" || trim == ":h":
				fmt.Fprint(out, replHelp)
				continue
			case trim == ":clear" || trim == ":c":
				fmt.Fprint(out, "\033[H\033[2J")
				continue
			case trim == ":history":
				showHistory(out)
				continue
			case trim == ":version" || trim == ":v":
				fmt.Fprintf(out, "weft %s\n", Version)
				continue
			case strings.HasPrefix(trim, ":stdlib"):
				pkg := strings.TrimSpace(strings.TrimPrefix(trim, ":stdlib"))
				showStdlibHelp(out, pkg)
				continue
			case trim == "":
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

		appendHistory(src)
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
  :help      this text
  :quit      exit
  :clear     clear screen
  :stdlib    list stdlib packages
  :stdlib fs show members of a package
  :history   show recent history
  :version   show version

multi-line: open braces continue on next line
  fn add(a, b) {
  ...   a + b
  ... }

examples:
  1 + 2
  x := "hello"
  say("$x, weft")
  fn double(x) { x * 2 }
  double(21)
  map([1,2,3], fn(x) { x * x })

history saved to ~/.weft/history
tip: use rlwrap weft for arrow-key editing
`

// StartREPL is the CLI entry using stdin/stdout.
func StartREPL() error {
	ctx := New(Options{Stdout: os.Stdout, Stderr: os.Stderr})
	err := ctx.RunREPL(os.Stdin, os.Stdout)
	return err
}

func showHistory(out io.Writer) {
	p := historyPath()
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		fmt.Fprintln(out, "(no history)")
		return
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	start := 0
	if len(lines) > 20 {
		start = len(lines) - 20
	}
	for i := start; i < len(lines); i++ {
		fmt.Fprintf(out, "  %d  %s\n", i+1, lines[i])
	}
}

func showStdlibHelp(out io.Writer, pkg string) {
	if pkg == "" {
		names := StdlibNames()
		fmt.Fprintf(out, "%d packages: %s\n", len(names), strings.Join(names, ", "))
		fmt.Fprintln(out, "  :stdlib <name> for members")
	} else {
		members := StdlibMembers(pkg)
		if len(members) == 0 {
			fmt.Fprintf(out, "unknown package: %s\n", pkg)
			return
		}
		fmt.Fprintf(out, "%s (%d members):\n", pkg, len(members))
		for _, m := range members {
			fmt.Fprintf(out, "  %s.%s\n", pkg, m)
		}
	}
}

// historyPath returns ~/.weft/history (created on first write).
func historyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".weft", "history")
}

// appendHistory saves one line to the history file.
func appendHistory(line string) {
	p := historyPath()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	fmt.Fprintln(f, line)
	f.Close()
}
