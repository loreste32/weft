package weft

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"
)

// tryTermReader returns a line reader with editing + tab completion when
// stdin is a TTY. Otherwise returns nil (caller uses bufio.Scanner).
func tryTermReader(in io.Reader, out io.Writer, hist *[]string) *termLineReader {
	f, ok := in.(*os.File)
	if !ok {
		return nil
	}
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}
	return &termLineReader{f: f, fd: fd, out: out, hist: hist}
}

type termLineReader struct {
	f    *os.File
	fd   int
	out  io.Writer
	hist *[]string
}

// ReadLine reads one line with basic editing (backspace, left/right not full,
// up/down history, tab completion). Restores terminal on return.
func (r *termLineReader) ReadLine(prompt string) (string, error) {
	old, err := term.MakeRaw(r.fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(r.fd, old)

	fmt.Fprint(r.out, prompt)

	var buf []rune
	pos := 0 // cursor index in buf
	histIdx := -1
	var stash []rune // buffer before history navigation

	redraw := func() {
		// clear line and rewrite
		fmt.Fprint(r.out, "\r\033[K")
		fmt.Fprint(r.out, prompt)
		fmt.Fprint(r.out, string(buf))
		// move cursor to pos
		back := len(buf) - pos
		if back > 0 {
			fmt.Fprintf(r.out, "\033[%dD", back)
		}
	}

	for {
		var b [8]byte
		n, err := r.f.Read(b[:])
		if err != nil {
			fmt.Fprintln(r.out)
			return string(buf), err
		}
		if n == 0 {
			continue
		}

		// Escape sequences: arrows
		if b[0] == 0x1b && n >= 3 && b[1] == '[' {
			switch b[2] {
			case 'A': // up
				if r.hist == nil || len(*r.hist) == 0 {
					continue
				}
				if histIdx < 0 {
					stash = append([]rune(nil), buf...)
					histIdx = len(*r.hist)
				}
				if histIdx > 0 {
					histIdx--
					buf = []rune((*r.hist)[histIdx])
					pos = len(buf)
					redraw()
				}
				continue
			case 'B': // down
				if histIdx < 0 {
					continue
				}
				histIdx++
				if histIdx >= len(*r.hist) {
					histIdx = -1
					buf = append([]rune(nil), stash...)
				} else {
					buf = []rune((*r.hist)[histIdx])
				}
				pos = len(buf)
				redraw()
				continue
			case 'C': // right
				if pos < len(buf) {
					pos++
					redraw()
				}
				continue
			case 'D': // left
				if pos > 0 {
					pos--
					redraw()
				}
				continue
			}
		}

		c := b[0]
		switch c {
		case 0x03: // Ctrl-C
			fmt.Fprint(r.out, "^C\r\n")
			return "", fmt.Errorf("interrupt")
		case 0x04: // Ctrl-D
			if len(buf) == 0 {
				fmt.Fprintln(r.out)
				return "", io.EOF
			}
		case 0x7f, 0x08: // backspace
			if pos > 0 {
				buf = append(buf[:pos-1], buf[pos:]...)
				pos--
				redraw()
			}
		case '\t':
			prefix, start := completablePrefix(buf, pos)
			cands := replCompletions(prefix)
			if len(cands) == 1 {
				// replace prefix with candidate
				ins := []rune(cands[0])
				buf = append(buf[:start], append(ins, buf[pos:]...)...)
				pos = start + len(ins)
				redraw()
			} else if len(cands) > 1 {
				// common prefix
				cp := commonPrefix(cands)
				if len(cp) > len(prefix) {
					ins := []rune(cp)
					buf = append(buf[:start], append(ins, buf[pos:]...)...)
					pos = start + len(ins)
					redraw()
				} else {
					fmt.Fprint(r.out, "\r\n")
					fmt.Fprintln(r.out, strings.Join(cands, "  "))
					redraw()
				}
			}
		case '\r', '\n':
			fmt.Fprint(r.out, "\r\n")
			return string(buf), nil
		default:
			if c >= 32 && c < 127 {
				// insert printable
				buf = append(buf[:pos], append([]rune{rune(c)}, buf[pos:]...)...)
				pos++
				histIdx = -1
				redraw()
			}
		}
	}
}

// completablePrefix returns the identifier prefix before cursor and its start index.
func completablePrefix(buf []rune, pos int) (string, int) {
	if pos > len(buf) {
		pos = len(buf)
	}
	i := pos
	for i > 0 {
		r := buf[i-1]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' {
			i--
			continue
		}
		break
	}
	return string(buf[i:pos]), i
}

func commonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	p := ss[0]
	for _, s := range ss[1:] {
		for len(p) > 0 && !strings.HasPrefix(s, p) {
			p = p[:len(p)-1]
		}
	}
	return p
}

// replCompletions returns candidates matching prefix (keywords, prelude, stdlib, :cmds).
func replCompletions(prefix string) []string {
	if prefix == "" {
		return nil
	}
	var pool []string
	if strings.HasPrefix(prefix, ":") {
		pool = []string{
			":help", ":quit", ":q", ":exit", ":clear", ":c", ":cancel",
			":history", ":version", ":v", ":stdlib",
		}
	} else if i := strings.LastIndex(prefix, "."); i >= 0 {
		pkg := prefix[:i]
		memPrefix := prefix[i+1:]
		for _, m := range StdlibMembers(pkg) {
			if strings.HasPrefix(m, memPrefix) {
				pool = append(pool, pkg+"."+m)
			}
		}
		return pool
	} else {
		pool = append(pool,
			"fn", "mut", "use", "import", "pub", "type", "const", "enum",
			"match", "if", "else", "while", "for", "in", "return", "defer", "as",
			"map", "seq_map", "filter", "seq_filter", "reduce", "each",
			"spawn", "parallel", "gather", "race", "timeout",
			"channel", "send", "recv", "close",
			"say", "println", "Ok", "Err", "len", "range", "push",
			"ensure", "bail", "true", "false", "null",
		)
		pool = append(pool, StdlibNames()...)
	}
	var out []string
	for _, c := range pool {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}
