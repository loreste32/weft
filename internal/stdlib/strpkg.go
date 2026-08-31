package stdlib

import (
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageStr — string helpers for data processing CLIs.
func packageStr() runtime.Value {
	p := pkg()

	set(p, "split", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.List(), nil
		}
		s := args[0].String()
		sep := " "
		if len(args) >= 2 {
			sep = args[1].String()
		}
		if sep == "" {
			// split chars
			items := make([]runtime.Value, 0, len(s))
			for _, r := range s {
				items = append(items, runtime.Str(string(r)))
			}
			return runtime.List(items...), nil
		}
		parts := strings.Split(s, sep)
		items := make([]runtime.Value, len(parts))
		for i, p := range parts {
			items[i] = runtime.Str(p)
		}
		return runtime.List(items...), nil
	}, 2)

	set(p, "join", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		sep := ""
		var parts []string
		if args[0].Kind == runtime.KindList {
			sep = ""
			if len(args) >= 2 {
				sep = args[1].String()
			}
			for _, it := range args[0].Obj.(*runtime.ListObj).Items {
				parts = append(parts, it.String())
			}
		} else {
			// str.join(sep, list)
			sep = args[0].String()
			if len(args) >= 2 && args[1].Kind == runtime.KindList {
				for _, it := range args[1].Obj.(*runtime.ListObj).Items {
					parts = append(parts, it.String())
				}
			}
		}
		return runtime.Str(strings.Join(parts, sep)), nil
	}, 2)

	set(p, "trim", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(strings.TrimSpace(args[0].String())), nil
	}, 1)

	set(p, "lower", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(strings.ToLower(args[0].String())), nil
	}, 1)

	set(p, "upper", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(strings.ToUpper(args[0].String())), nil
	}, 1)

	set(p, "contains", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(strings.Contains(args[0].String(), args[1].String())), nil
	}, 2)

	hasPrefix := func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(strings.HasPrefix(args[0].String(), args[1].String())), nil
	}
	hasSuffix := func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(strings.HasSuffix(args[0].String(), args[1].String())), nil
	}
	set(p, "has_prefix", hasPrefix, 2)
	set(p, "has_suffix", hasSuffix, 2)
	// Common aliases (agents often guess these names).
	set(p, "starts_with", hasPrefix, 2)
	set(p, "ends_with", hasSuffix, 2)

	set(p, "replace", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return runtime.Str(""), nil
		}
		return runtime.Str(strings.ReplaceAll(args[0].String(), args[1].String(), args[2].String())), nil
	}, 3)

	set(p, "fields", func(args []runtime.Value) (runtime.Value, error) {
		// whitespace split (like awk)
		if len(args) < 1 {
			return runtime.List(), nil
		}
		parts := strings.Fields(args[0].String())
		items := make([]runtime.Value, len(parts))
		for i, p := range parts {
			items[i] = runtime.Str(p)
		}
		return runtime.List(items...), nil
	}, 1)

	set(p, "lines", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.List(), nil
		}
		s := strings.ReplaceAll(args[0].String(), "\r\n", "\n")
		s = strings.TrimSuffix(s, "\n")
		if s == "" {
			return runtime.List(), nil
		}
		parts := strings.Split(s, "\n")
		items := make([]runtime.Value, len(parts))
		for i, p := range parts {
			items[i] = runtime.Str(p)
		}
		return runtime.List(items...), nil
	}, 1)

	set(p, "repeat", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Str(""), nil
		}
		n64, _ := runtime.AsInt(args[1])
		if n64 < 0 {
			n64 = 0
		}
		n, err := safeInt(n64)
		if err != nil {
			return errRes("str.repeat: count too large", "str"), nil
		}
		return runtime.Str(strings.Repeat(args[0].String(), n)), nil
	}, 2)

	// str.slice(s, start, end?) — rune-safe-ish via bytes for v1 scripts
	set(p, "slice", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Str(""), nil
		}
		s := args[0].String()
		start, _ := runtime.AsInt(args[1])
		end := int64(len(s))
		if len(args) >= 3 {
			if e, err := runtime.AsInt(args[2]); err == nil {
				end = e
			}
		}
		if start < 0 {
			start = int64(len(s)) + start
		}
		if end < 0 {
			end = int64(len(s)) + end
		}
		if start < 0 {
			start = 0
		}
		if end > int64(len(s)) {
			end = int64(len(s))
		}
		if start > end {
			return runtime.Str(""), nil
		}
		return runtime.Str(s[start:end]), nil
	}, 3)

	// str.pad_left / pad_right (width, fill?)
	set(p, "pad_left", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Str(""), nil
		}
		s := args[0].String()
		w64, _ := runtime.AsInt(args[1])
		wi, err := safeInt(w64)
		if err != nil {
			return errRes("str.pad_left: width too large", "str"), nil
		}
		fill := " "
		if len(args) >= 3 && args[2].String() != "" {
			fill = args[2].String()
		}
		for len(s) < wi {
			s = fill + s
		}
		if len(s) > wi {
			s = s[len(s)-wi:]
		}
		return runtime.Str(s), nil
	}, 3)

	set(p, "pad_right", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Str(""), nil
		}
		s := args[0].String()
		w64, _ := runtime.AsInt(args[1])
		wi, err := safeInt(w64)
		if err != nil {
			return errRes("str.pad_right: width too large", "str"), nil
		}
		fill := " "
		if len(args) >= 3 && args[2].String() != "" {
			fill = args[2].String()
		}
		for len(s) < wi {
			s = s + fill
		}
		if len(s) > wi {
			s = s[:wi]
		}
		return runtime.Str(s), nil
	}, 3)

	// str.wrap(s, width) -> str  (textwrap.wrap joined by newlines)
	set(p, "wrap", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		width := 70
		if len(args) >= 2 {
			if n, err := runtime.AsInt(args[1]); err == nil && n > 0 {
				if v, e := safeInt(n); e == nil {
					width = v
				}
			}
		}
		return runtime.Str(strings.Join(textWrap(args[0].String(), width), "\n")), nil
	}, 2)

	// str.fill(s, width) -> str  reflow paragraphs
	set(p, "fill", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		width := 70
		if len(args) >= 2 {
			if n, err := runtime.AsInt(args[1]); err == nil && n > 0 {
				if v, e := safeInt(n); e == nil {
					width = v
				}
			}
		}
		s := strings.ReplaceAll(args[0].String(), "\r\n", "\n")
		paras := strings.Split(s, "\n\n")
		var out []string
		for _, para := range paras {
			one := strings.Join(strings.Fields(para), " ")
			out = append(out, strings.Join(textWrap(one, width), "\n"))
		}
		return runtime.Str(strings.Join(out, "\n\n")), nil
	}, 2)

	// str.indent(s, prefix) -> str
	set(p, "indent", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		prefix := "    "
		if len(args) >= 2 {
			prefix = args[1].String()
		}
		s := strings.ReplaceAll(args[0].String(), "\r\n", "\n")
		lines := strings.Split(s, "\n")
		for i, ln := range lines {
			if ln != "" {
				lines[i] = prefix + ln
			}
		}
		return runtime.Str(strings.Join(lines, "\n")), nil
	}, 2)

	// str.dedent(s) -> str  strip common leading indent
	set(p, "dedent", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(textDedent(args[0].String())), nil
	}, 1)

	// str.find / rfind -> int index or -1
	set(p, "find", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Int(-1), nil
		}
		return runtime.Int(int64(strings.Index(args[0].String(), args[1].String()))), nil
	}, 2)
	set(p, "rfind", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Int(-1), nil
		}
		return runtime.Int(int64(strings.LastIndex(args[0].String(), args[1].String()))), nil
	}, 2)

	// str.count(s, sub) -> int
	set(p, "count", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Int(0), nil
		}
		s, sub := args[0].String(), args[1].String()
		if sub == "" {
			return runtime.Int(int64(len(s) + 1)), nil
		}
		return runtime.Int(int64(strings.Count(s, sub))), nil
	}, 2)

	set(p, "title", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(strings.Title(strings.ToLower(args[0].String()))), nil //nolint:staticcheck
	}, 1)

	set(p, "capitalize", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		s := args[0].String()
		if s == "" {
			return runtime.Str(""), nil
		}
		// first rune upper, rest lower
		r := []rune(strings.ToLower(s))
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		return runtime.Str(string(r)), nil
	}, 1)

	set(p, "reverse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		r := []rune(args[0].String())
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		return runtime.Str(string(r)), nil
	}, 1)

	set(p, "center", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Str(""), nil
		}
		s := args[0].String()
		width64, _ := runtime.AsInt(args[1])
		wi, err := safeInt(width64)
		if err != nil {
			return errRes("str.center: width too large", "str"), nil
		}
		fill := " "
		if len(args) >= 3 && args[2].String() != "" {
			fill = string([]rune(args[2].String())[0])
		}
		if wi <= len(s) {
			return runtime.Str(s), nil
		}
		pad := wi - len(s)
		left := pad / 2
		right := pad - left
		return runtime.Str(strings.Repeat(fill, left) + s + strings.Repeat(fill, right)), nil
	}, 3)

	// str.partition(s, sep) -> [before, sep, after]
	set(p, "partition", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.List(runtime.Str(""), runtime.Str(""), runtime.Str("")), nil
		}
		s, sep := args[0].String(), args[1].String()
		i := strings.Index(s, sep)
		if i < 0 {
			return runtime.List(runtime.Str(s), runtime.Str(""), runtime.Str("")), nil
		}
		return runtime.List(runtime.Str(s[:i]), runtime.Str(sep), runtime.Str(s[i+len(sep):])), nil
	}, 2)

	set(p, "rpartition", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.List(runtime.Str(""), runtime.Str(""), runtime.Str("")), nil
		}
		s, sep := args[0].String(), args[1].String()
		i := strings.LastIndex(s, sep)
		if i < 0 {
			return runtime.List(runtime.Str(""), runtime.Str(""), runtime.Str(s)), nil
		}
		return runtime.List(runtime.Str(s[:i]), runtime.Str(sep), runtime.Str(s[i+len(sep):])), nil
	}, 2)

	set(p, "is_digit", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		s := args[0].String()
		if s == "" {
			return runtime.Bool(false), nil
		}
		for _, r := range s {
			if r < '0' || r > '9' {
				return runtime.Bool(false), nil
			}
		}
		return runtime.Bool(true), nil
	}, 1)

	set(p, "is_alpha", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		s := args[0].String()
		if s == "" {
			return runtime.Bool(false), nil
		}
		for _, r := range s {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return runtime.Bool(false), nil
			}
		}
		return runtime.Bool(true), nil
	}, 1)

	set(p, "is_space", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		s := args[0].String()
		if s == "" {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(strings.TrimSpace(s) == ""), nil
	}, 1)

	set(p, "trim_left", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(strings.TrimLeft(args[0].String(), " \t\n\r")), nil
	}, 1)

	set(p, "trim_right", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(strings.TrimRight(args[0].String(), " \t\n\r")), nil
	}, 1)

	// str.builder() -> builder object with .write(s) and .string() methods
	// O(1) amortised append — use instead of s = s + x in loops
	set(p, "builder", func(args []runtime.Value) (runtime.Value, error) {
		var sb strings.Builder
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = append(mo.Keys, "write", "string", "len", "reset")
		mo.Vals["write"] = runtime.MakeBuiltin("builder.write", 1, func(args []runtime.Value) (runtime.Value, error) {
			if len(args) >= 1 {
				sb.WriteString(args[0].String())
			}
			return runtime.Unit(), nil
		})
		mo.Vals["string"] = runtime.MakeBuiltin("builder.string", 0, func(args []runtime.Value) (runtime.Value, error) {
			return runtime.Str(sb.String()), nil
		})
		mo.Vals["len"] = runtime.MakeBuiltin("builder.len", 0, func(args []runtime.Value) (runtime.Value, error) {
			return runtime.Int(int64(sb.Len())), nil
		})
		mo.Vals["reset"] = runtime.MakeBuiltin("builder.reset", 0, func(args []runtime.Value) (runtime.Value, error) {
			sb.Reset()
			return runtime.Unit(), nil
		})
		return m, nil
	}, 0)

	return p
}

func textWrap(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		if cur.Len() == 0 {
			cur.WriteString(w)
			continue
		}
		if cur.Len()+1+len(w) <= width {
			cur.WriteByte(' ')
			cur.WriteString(w)
			continue
		}
		lines = append(lines, cur.String())
		cur.Reset()
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

func textDedent(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	// find min indent of non-empty lines
	minInd := -1
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		ind := 0
		for _, r := range ln {
			if r == ' ' {
				ind++
			} else if r == '\t' {
				ind += 4
			} else {
				break
			}
		}
		if minInd < 0 || ind < minInd {
			minInd = ind
		}
	}
	if minInd <= 0 {
		return s
	}
	var out []string
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			out = append(out, "")
			continue
		}
		// strip minInd spaces (tabs count as 4)
		i := 0
		left := minInd
		for i < len(ln) && left > 0 {
			if ln[i] == ' ' {
				left--
				i++
			} else if ln[i] == '\t' {
				left -= 4
				i++
			} else {
				break
			}
		}
		if left < 0 {
			// over-stripped tab — keep rest
		}
		out = append(out, ln[i:])
	}
	return strings.Join(out, "\n")
}
