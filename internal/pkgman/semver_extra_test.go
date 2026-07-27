package pkgman

import "testing"

func TestLooksLikeVersionConstraint(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"", true},
		{"*", true},
		{"^1.0.0", true},
		{"~2.0.0", true},
		{">=1.0.0", true},
		{"==1.0.0", true},
		{"1.0.0", true},
		{"v1.0.0", true},
		{"main", false},
		{"feature-branch", false},
	}
	for _, tc := range cases {
		if got := looksLikeVersionConstraint(tc.s); got != tc.want {
			t.Errorf("looksLikeVersionConstraint(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}
