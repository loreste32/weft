package pkgman

import "testing"

func TestMatchConstraint(t *testing.T) {
	cases := []struct {
		ver, cons string
		ok        bool
	}{
		{"1.2.3", "1.2.3", true},
		{"v1.2.3", "1.2.3", true},
		{"1.2.4", "^1.2.3", true},
		{"2.0.0", "^1.2.3", false},
		{"1.2.9", "~1.2.3", true},
		{"1.3.0", "~1.2.3", false},
		{"1.2.3", ">=1.2.0", true},
		{"1.1.0", ">=1.2.0", false},
		{"9.9.9", "*", true},
		{"1.0.0", "", true},
	}
	for _, c := range cases {
		if MatchConstraint(c.ver, c.cons) != c.ok {
			t.Errorf("%s vs %s want %v", c.ver, c.cons, c.ok)
		}
	}
}
