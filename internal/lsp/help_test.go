package lsp

import (
	"strings"
	"testing"

	"github.com/loreste/weft/internal/stdlib"
)

func TestMemberCatalogMatchesStdlib(t *testing.T) {
	// Every dotted catalog entry must be a real stdlib package.member.
	for key, h := range memberCatalog {
		if !strings.Contains(key, ".") {
			continue // prelude
		}
		pkg, mem, ok := strings.Cut(key, ".")
		if !ok || pkg == "" || mem == "" {
			t.Errorf("bad catalog key %q", key)
			continue
		}
		if !stdlib.IsPackage(pkg) {
			t.Errorf("%s: unknown package %q", key, pkg)
			continue
		}
		found := false
		for _, m := range stdlib.PackageMembers(pkg) {
			if m == mem {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: not in weft stdlib %s (members: %v)", key, pkg, stdlib.PackageMembers(pkg))
		}
		if h.Sig == "" || h.Detail == "" {
			t.Errorf("%s: empty sig or detail", key)
		}
	}
}

func TestLookupMemberHighTraffic(t *testing.T) {
	cases := []struct{ pkg, mem string }{
		{"llm", "stream_text"},
		{"web", "sse"},
		{"yaml", "parse"},
		{"db", "open"},
		{"table", "pluck"},
	}
	for _, c := range cases {
		h, ok := lookupMember(c.pkg, c.mem)
		if !ok || !strings.Contains(h.Sig, c.pkg+"."+c.mem) {
			t.Fatalf("%s.%s → %+v ok=%v", c.pkg, c.mem, h, ok)
		}
	}
}
