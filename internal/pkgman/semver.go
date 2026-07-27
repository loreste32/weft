package pkgman

import (
	"fmt"
	"strconv"
	"strings"
)

// looksLikeVersionConstraint is true for *, ranges, and plain semver (not branch names).
func looksLikeVersionConstraint(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return true
	}
	if strings.HasPrefix(s, "^") || strings.HasPrefix(s, "~") ||
		strings.HasPrefix(s, ">=") || strings.HasPrefix(s, "==") {
		return true
	}
	_, _, _, err := ParseSemver(s)
	return err == nil
}

// ParseSemver parses "1.2.3" or "v1.2.3" into major.minor.patch.
func ParseSemver(s string) (maj, min, pat int, err error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	// strip pre-release / build
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return 0, 0, 0, fmt.Errorf("bad semver %q", s)
	}
	nums := []int{0, 0, 0}
	for i, p := range parts {
		n, e := strconv.Atoi(p)
		if e != nil || n < 0 {
			return 0, 0, 0, fmt.Errorf("bad semver %q", s)
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], nil
}

// MatchConstraint checks version against a simple constraint:
//
//	"" or "*"     — always true
//	"1.2.3"       — exact
//	"^1.2.3"      — same major, >= 1.2.3
//	"~1.2.3"      — same major.minor, >= 1.2.3
//	">=1.2.3"     — greater or equal
func MatchConstraint(version, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return true
	}
	vMaj, vMin, vPat, err := ParseSemver(version)
	if err != nil {
		return version == constraint || version == "v"+constraint
	}
	op := ""
	spec := constraint
	switch {
	case strings.HasPrefix(constraint, "^"):
		op = "^"
		spec = strings.TrimPrefix(constraint, "^")
	case strings.HasPrefix(constraint, "~"):
		op = "~"
		spec = strings.TrimPrefix(constraint, "~")
	case strings.HasPrefix(constraint, ">="):
		op = ">="
		spec = strings.TrimPrefix(constraint, ">=")
	case strings.HasPrefix(constraint, "=="):
		op = "=="
		spec = strings.TrimPrefix(constraint, "==")
	}
	cMaj, cMin, cPat, err := ParseSemver(spec)
	if err != nil {
		return version == constraint
	}
	cmp := cmp3(vMaj, vMin, vPat, cMaj, cMin, cPat)
	switch op {
	case "":
		return cmp == 0
	case "==":
		return cmp == 0
	case ">=":
		return cmp >= 0
	case "^":
		if vMaj != cMaj {
			return false
		}
		return cmp >= 0
	case "~":
		if vMaj != cMaj || vMin != cMin {
			return false
		}
		return cmp >= 0
	default:
		return cmp == 0
	}
}

func cmp3(a, b, c, x, y, z int) int {
	if a != x {
		if a < x {
			return -1
		}
		return 1
	}
	if b != y {
		if b < y {
			return -1
		}
		return 1
	}
	if c != z {
		if c < z {
			return -1
		}
		return 1
	}
	return 0
}
