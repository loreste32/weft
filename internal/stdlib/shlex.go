package stdlib

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/loreste/weft/internal/runtime"
)

// packageShlex — shell-safe split/quote/join.
func packageShlex() runtime.Value {
	p := pkg()

	// shlex.split(s) -> Result[[str]]
	set(p, "split", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("shlex.split(s)", "shlex"), nil
		}
		parts, err := shlexSplit(args[0].String())
		if err != nil {
			return errRes(err.Error(), "shlex"), nil
		}
		items := make([]runtime.Value, len(parts))
		for i, s := range parts {
			items[i] = runtime.Str(s)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// shlex.quote(s) -> str  (POSIX single-quote style)
	set(p, "quote", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str("''"), nil
		}
		return runtime.Str(shlexQuote(args[0].String())), nil
	}, 1)

	// shlex.join(list) -> str
	set(p, "join", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.Str(""), nil
		}
		var b strings.Builder
		for i, it := range args[0].Obj.(*runtime.ListObj).Items {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(shlexQuote(it.String()))
		}
		return runtime.Str(b.String()), nil
	}, 1)

	return p
}

func shlexQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if r == '\'' || unicode.IsSpace(r) || strings.ContainsRune(`|&;<>()$`+"`"+`\"`, r) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shlexSplit(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inSingle, inDouble := false, false
	escape := false
	hadToken := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			cur.WriteByte(c)
			escape = false
			hadToken = true
			continue
		}
		if inSingle {
			if c == '\'' {
				inSingle = false
			} else {
				cur.WriteByte(c)
			}
			hadToken = true
			continue
		}
		if inDouble {
			if c == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\' || s[i+1] == '$') {
				escape = true
				continue
			}
			if c == '"' {
				inDouble = false
			} else {
				cur.WriteByte(c)
			}
			hadToken = true
			continue
		}
		switch c {
		case '\\':
			escape = true
			hadToken = true
		case '\'':
			inSingle = true
			hadToken = true
		case '"':
			inDouble = true
			hadToken = true
		case ' ', '\t', '\n', '\r':
			if hadToken {
				out = append(out, cur.String())
				cur.Reset()
				hadToken = false
			}
		default:
			cur.WriteByte(c)
			hadToken = true
		}
	}
	if inSingle || inDouble || escape {
		return nil, strconv.ErrSyntax
	}
	if hadToken {
		out = append(out, cur.String())
	}
	return out, nil
}
