package weft

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/loreste/weft/internal/runtime"
)

// RunREPL starts an interactive session on in/out.
func (c *Context) RunREPL(in io.Reader, out io.Writer) error {
	fmt.Fprintf(out, "weft %s — type :help, :quit\n", Version)

	// Session history (also mirrored to ~/.weft/history).
	hist := loadHistory()

	// Interactive TTY: line editing + tab completion. Pipes keep Scanner path.
	termR := tryTermReader(in, out, &hist)
	var sc *bufio.Scanner
	if termR == nil {
		sc = bufio.NewScanner(in)
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 1024*1024)
	}

	var multi strings.Builder
	depth := 0

	promptStr := func() string {
		if depth > 0 || multi.Len() > 0 {
			return "... "
		}
		return "> "
	}

	readLine := func() (string, error) {
		if termR != nil {
			return termR.ReadLine(promptStr())
		}
		fmt.Fprint(out, promptStr())
		if !sc.Scan() {
			fmt.Fprintln(out)
			return "", sc.Err()
		}
		return sc.Text(), nil
	}

	for {
		line, err := readLine()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			// Ctrl-C → new prompt
			if strings.Contains(err.Error(), "interrupt") {
				multi.Reset()
				depth = 0
				continue
			}
			return err
		}

		if multi.Len() == 0 {
			trim := strings.TrimSpace(line)
			switch {
			case trim == ":q" || trim == ":quit" || trim == ":exit":
				return nil
			case trim == ":help" || trim == ":h" || trim == "help":
				fmt.Fprint(out, replHelp)
				continue
			case trim == "exit" || trim == "quit":
				return nil
			case trim == ":clear" || trim == ":c":
				fmt.Fprint(out, "\033[H\033[2J")
				continue
			case trim == ":history" || strings.HasPrefix(trim, ":history "):
				showHistoryFiltered(out, hist, strings.TrimSpace(strings.TrimPrefix(trim, ":history")))
				continue
			case strings.HasPrefix(trim, ":!") || (strings.HasPrefix(trim, "!") && len(trim) > 1 && unicode.IsDigit(rune(trim[1]))):
				// :!N or !N — re-run history entry N (1-based)
				numStr := strings.TrimPrefix(trim, ":")
				numStr = strings.TrimPrefix(numStr, "!")
				n, err := strconv.Atoi(numStr)
				if err != nil || n < 1 || n > len(hist) {
					fmt.Fprintf(out, "no history entry %s (have %d)\n", numStr, len(hist))
					continue
				}
				src := hist[n-1]
				fmt.Fprintf(out, "// re-run %d: %s\n", n, firstLine(src))
				appendHistoryLine(&hist, src)
				v, err := c.Eval(src)
				if err != nil {
					fmt.Fprintln(out, err.Error())
					continue
				}
				if shouldPrint(v) {
					fmt.Fprintln(out, v.String())
				}
				continue
			case trim == ":version" || trim == ":v":
				fmt.Fprintf(out, "weft %s\n", Version)
				continue
			case strings.HasPrefix(trim, ":stdlib"):
				pkg := strings.TrimSpace(strings.TrimPrefix(trim, ":stdlib"))
				showStdlibHelp(out, pkg)
				continue
			case trim == ":cancel":
				// no multi-line buffer at primary prompt
				continue
			case trim == "":
				continue
			}
		} else {
			// Mid multi-line buffer
			trim := strings.TrimSpace(line)
			if trim == ":cancel" || trim == ":c" {
				multi.Reset()
				depth = 0
				fmt.Fprintln(out, "(cancelled)")
				continue
			}
		}

		multi.WriteString(line)
		multi.WriteByte('\n')
		srcSoFar := multi.String()
		depth = braceDepth(srcSoFar)
		if depth > 0 || unbalancedQuotes(srcSoFar) || incompleteLine(srcSoFar) {
			continue
		}

		src := strings.TrimSpace(srcSoFar)
		multi.Reset()
		depth = 0
		if src == "" {
			continue
		}

		appendHistoryLine(&hist, src)
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

// incompleteLine is true when the buffer looks like a partial statement that
// should keep reading (trailing operator / keyword), even if braces are balanced.
func incompleteLine(s string) bool {
	t := strings.TrimRight(s, " \t\r\n")
	if t == "" {
		return false
	}
	// Ignore a final full-line // comment for the trailing check.
	if i := strings.LastIndex(t, "\n"); i >= 0 {
		last := strings.TrimSpace(t[i+1:])
		if strings.HasPrefix(last, "//") {
			t = strings.TrimRight(t[:i], " \t\r\n")
		}
	}
	if t == "" {
		return false
	}
	// Trailing binary operators / separators (not complete statements).
	switch t[len(t)-1] {
	case '+', '-', '*', '/', '%', ',', '.', '=', '<', '>', '!', '&', '|', '?':
		return true
	case ':':
		// `x :=` incomplete; bare labels unlikely in Weft
		return true
	}
	// Last token is an opening keyword (word boundary).
	last := lastWord(t)
	switch last {
	case "fn", "if", "else", "for", "while", "match", "return", "type", "enum",
		"use", "import", "const", "mut", "let", "defer", "pub", "in", "as":
		return true
	}
	return false
}

func lastWord(s string) string {
	i := len(s)
	for i > 0 {
		r := rune(s[i-1])
		if unicode.IsLetter(r) || r == '_' {
			i--
			continue
		}
		break
	}
	return s[i:]
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}

const replHelp = `commands:
  :help              this text
  :quit / :q         exit
  :clear             clear screen
  :cancel / :c       abort multi-line input
  :stdlib            list stdlib packages
  :stdlib fs         show members of a package
  :history           show recent history
  :history <text>    filter history by substring
  :!N  or  !N        re-run history entry N
  :version           show version

multi-line:
  open braces/parens/brackets continue on the next line
  trailing operators (+, ,, :=, …) also continue
  :cancel (or :c while multi-line) aborts the buffer

examples:
  1 + 2
  x := "hello"
  say("$x, weft")
  fn double(x) { x * 2 }
  double(21)
  map([1,2,3], fn(x) { x * x })

history saved to ~/.weft/history
interactive TTY: Tab completes keywords/stdlib/:commands; ↑/↓ history
(non-TTY / pipes still work; rlwrap optional)
`

// StartREPL is the CLI entry using stdin/stdout.
func StartREPL() error {
	ctx := New(Options{Stdout: os.Stdout, Stderr: os.Stderr})
	err := ctx.RunREPL(os.Stdin, os.Stdout)
	return err
}

func showHistoryFiltered(out io.Writer, hist []string, filter string) {
	if len(hist) == 0 {
		fmt.Fprintln(out, "(no history)")
		return
	}
	start := 0
	if len(hist) > 40 {
		start = len(hist) - 40
	}
	n := 0
	for i := start; i < len(hist); i++ {
		line := hist[i]
		if filter != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(filter)) {
			continue
		}
		// One visual line for multi-line entries
		fmt.Fprintf(out, "  %d  %s\n", i+1, firstLine(line))
		n++
	}
	if n == 0 && filter != "" {
		fmt.Fprintf(out, "(no history matching %q)\n", filter)
	}
}

// showHistory is kept for tests / external callers (last 20 from file).
func showHistory(out io.Writer) {
	showHistoryFiltered(out, loadHistory(), "")
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

func loadHistory() []string {
	p := historyPath()
	if p == "" {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		// File stores one physical line per entry; multi-line was flattened
		// with spaces historically — keep as-is.
		out = append(out, line)
	}
	// Cap in-memory to last 500
	if len(out) > 500 {
		out = out[len(out)-500:]
	}
	return out
}

// appendHistory saves one entry to the history file (legacy helper).
func appendHistory(line string) {
	var hist []string
	appendHistoryLine(&hist, line)
}

func appendHistoryLine(hist *[]string, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	// Flatten newlines for the file (one entry per line).
	flat := strings.ReplaceAll(line, "\n", " ")
	flat = strings.Join(strings.Fields(flat), " ")
	*hist = append(*hist, line)
	p := historyPath()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	fmt.Fprintln(f, flat)
	f.Close()
}
