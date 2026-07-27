package stdlib

import (
	"strings"
	"testing"
)

func TestDifflibUnifiedAndNdiff(t *testing.T) {
	u := unifiedDiff([]string{"a", "b"}, []string{"a", "c"}, "old", "new")
	if !strings.Contains(u, "--- old") || !strings.Contains(u, "+++ new") {
		t.Fatal(u)
	}
	if !strings.Contains(u, "-b") || !strings.Contains(u, "+c") {
		t.Fatal(u)
	}
	n := ndiff([]string{"x"}, []string{"y"})
	if len(n) < 2 {
		t.Fatal(n)
	}
}
