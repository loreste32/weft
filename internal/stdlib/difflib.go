package stdlib

import (
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageDifflib — line diffs for tools/tests.
func packageDifflib() runtime.Value {
	p := pkg()
	set(p, "unified_diff", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("difflib.unified_diff(a, b, fromfile?, tofile?)", "difflib"), nil
		}
		a, b := diffLines(args[0]), diffLines(args[1])
		from, to := "a", "b"
		if len(args) >= 3 && args[2].String() != "" {
			from = args[2].String()
		}
		if len(args) >= 4 && args[3].String() != "" {
			to = args[3].String()
		}
		return runtime.Str(unifiedDiff(a, b, from, to)), nil
	}, 4)
	set(p, "ndiff", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("difflib.ndiff(a, b)", "difflib"), nil
		}
		var out []runtime.Value
		for _, line := range ndiff(diffLines(args[0]), diffLines(args[1])) {
			out = append(out, runtime.Str(line))
		}
		return runtime.List(out...), nil
	}, 2)
	return p
}

func diffLines(v runtime.Value) []string {
	if v.Kind == runtime.KindList {
		var out []string
		for _, it := range v.Obj.(*runtime.ListObj).Items {
			out = append(out, strings.TrimRight(it.String(), "\n"))
		}
		return out
	}
	s := strings.ReplaceAll(v.String(), "\r\n", "\n")
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func unifiedDiff(a, b []string, from, to string) string {
	var bld strings.Builder
	fmt.Fprintf(&bld, "--- %s\n+++ %s\n", from, to)
	fmt.Fprintf(&bld, "@@ -1,%d +1,%d @@\n", max(1, len(a)), max(1, len(b)))
	i, j := 0, 0
	for i < len(a) && j < len(b) && a[i] == b[j] {
		bld.WriteString(" " + a[i] + "\n")
		i++
		j++
	}
	for _, line := range ndiff(a[i:], b[j:]) {
		switch {
		case strings.HasPrefix(line, "- "):
			bld.WriteString("-" + strings.TrimPrefix(line, "- ") + "\n")
		case strings.HasPrefix(line, "+ "):
			bld.WriteString("+" + strings.TrimPrefix(line, "+ ") + "\n")
		default:
			bld.WriteString(" " + strings.TrimPrefix(line, "  ") + "\n")
		}
	}
	return bld.String()
}

func ndiff(a, b []string) []string {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			out = append(out, "  "+a[i])
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			out = append(out, "- "+a[i])
			i++
		} else {
			out = append(out, "+ "+b[j])
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, "- "+a[i])
	}
	for ; j < m; j++ {
		out = append(out, "+ "+b[j])
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
