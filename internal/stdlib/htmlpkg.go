package stdlib

import (
	"html"
	"regexp"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageHTML — light HTML helpers (Python html module cousin).
func packageHTML() runtime.Value {
	p := pkg()

	// html.escape(s, quote?) -> str
	set(p, "escape", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		s := args[0].String()
		// html.EscapeString escapes <>&'"; quote always true in Go
		return runtime.Str(html.EscapeString(s)), nil
	}, 2)

	// html.unescape(s) -> str
	set(p, "unescape", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(html.UnescapeString(args[0].String())), nil
	}, 1)

	// html.strip_tags(s) -> str  crude tag strip for agents
	tagRe := regexp.MustCompile(`(?s)<[^>]*>`)
	set(p, "strip_tags", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		s := tagRe.ReplaceAllString(args[0].String(), "")
		s = html.UnescapeString(s)
		// collapse whitespace a bit
		s = strings.Join(strings.Fields(s), " ")
		return runtime.Str(s), nil
	}, 1)

	// html.text(s) -> str  alias of strip_tags for agent readability
	set(p, "text", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		s := tagRe.ReplaceAllString(args[0].String(), "")
		s = html.UnescapeString(s)
		return runtime.Str(strings.Join(strings.Fields(s), " ")), nil
	}, 1)

	// html.links(s) -> list[str]  href= values (crude)
	hrefRe := regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)
	set(p, "links", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.List(), nil
		}
		ms := hrefRe.FindAllStringSubmatch(args[0].String(), -1)
		var out []runtime.Value
		seen := map[string]bool{}
		for _, m := range ms {
			if len(m) < 2 || seen[m[1]] {
				continue
			}
			seen[m[1]] = true
			out = append(out, runtime.Str(m[1]))
		}
		return runtime.List(out...), nil
	}, 1)

	return p
}
