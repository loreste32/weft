package stdlib

import (
	"regexp"

	"github.com/loreste/weft/internal/runtime"
)

// packageRe — regular expressions for validation and parsing.
func packageRe() runtime.Value {
	p := pkg()

	set(p, "match", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		ok, err := regexp.MatchString(args[0].String(), args[1].String())
		if err != nil {
			return errRes(err.Error(), "re"), nil
		}
		return runtime.Bool(ok), nil
	}, 2)

	set(p, "find", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Null(), nil
		}
		re, err := regexp.Compile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "re"), nil
		}
		m := re.FindString(args[1].String())
		if m == "" {
			return runtime.Null(), nil
		}
		return runtime.Str(m), nil
	}, 2)

	set(p, "find_all", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.List(), nil
		}
		re, err := regexp.Compile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "re"), nil
		}
		ms := re.FindAllString(args[1].String(), -1)
		items := make([]runtime.Value, len(ms))
		for i, m := range ms {
			items[i] = runtime.Str(m)
		}
		return runtime.List(items...), nil
	}, 2)

	set(p, "replace", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return runtime.Str(""), nil
		}
		re, err := regexp.Compile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "re"), nil
		}
		return runtime.Str(re.ReplaceAllString(args[1].String(), args[2].String())), nil
	}, 3)

	set(p, "split", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.List(), nil
		}
		re, err := regexp.Compile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "re"), nil
		}
		parts := re.Split(args[1].String(), -1)
		items := make([]runtime.Value, len(parts))
		for i, p := range parts {
			items[i] = runtime.Str(p)
		}
		return runtime.List(items...), nil
	}, 2)

	// re.groups(pattern, text) -> Result[[str]]  full match + capture groups (or Err if no match)
	set(p, "groups", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("re.groups(pattern, text)", "re"), nil
		}
		re, err := regexp.Compile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "re"), nil
		}
		m := re.FindStringSubmatch(args[1].String())
		if m == nil {
			return errRes("no match", "re"), nil
		}
		items := make([]runtime.Value, len(m))
		for i, g := range m {
			items[i] = runtime.Str(g)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 2)

	// re.find_all_groups(pattern, text) -> [[groups...], ...]
	set(p, "find_all_groups", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.List(), nil
		}
		re, err := regexp.Compile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "re"), nil
		}
		ms := re.FindAllStringSubmatch(args[1].String(), -1)
		out := make([]runtime.Value, len(ms))
		for i, m := range ms {
			items := make([]runtime.Value, len(m))
			for j, g := range m {
				items[j] = runtime.Str(g)
			}
			out[i] = runtime.List(items...)
		}
		return runtime.List(out...), nil
	}, 2)

	// re.is_match — alias of match (clearer name)
	set(p, "is_match", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		ok, err := regexp.MatchString(args[0].String(), args[1].String())
		if err != nil {
			return errRes(err.Error(), "re"), nil
		}
		return runtime.Bool(ok), nil
	}, 2)

	return p
}
