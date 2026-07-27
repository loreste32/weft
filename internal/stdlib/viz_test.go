package stdlib_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func runViz(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "viz.weft", src); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	return out.String()
}

func TestVizBarAndSpark(t *testing.T) {
	out := runViz(t, `
fn main {
    c := viz.bar({"a": 10, "b": 20, "c": 5}, {"title": "Sales"})
    println(c.kind)
    println(c.svg)
    println(viz.spark([1, 3, 2, 5, 4]))
}
`)
	if !strings.Contains(out, "bar") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "<svg") || !strings.Contains(out, "Sales") {
		t.Fatal("missing svg", out[:min(200, len(out))])
	}
	if !strings.Contains(out, "▁") && !strings.Contains(out, "█") {
		// spark uses block chars
		if !strings.ContainsAny(out, "▂▃▄▅▆▇") {
			t.Fatal("spark", out)
		}
	}
}

func TestVizLinePieScatter(t *testing.T) {
	out := runViz(t, `
fn main {
    l := viz.line([1, 3, 2, 5], {"title": "Trend"})
    p := viz.pie({"x": 30, "y": 70}, {"title": "Share"})
    s := viz.scatter([[1, 2], [2, 1], [3, 4]], {"title": "Pts"})
    a := viz.area([1, 2, 3, 2])
    h := viz.hist([1, 1, 2, 2, 2, 5, 5], {"bins": 4})
    println(l.kind, p.kind, s.kind, a.kind, h.kind)
    println(l.svg)
}
`)
	for _, k := range []string{"line", "pie", "scatter", "area", "hist"} {
		if !strings.Contains(out, k) {
			t.Fatalf("missing %s in %q", k, out)
		}
	}
	if !strings.Contains(out, "<svg") {
		t.Fatal(out)
	}
}

func TestVizSaveAndPage(t *testing.T) {
	dir := t.TempDir()
	svgPath := filepath.Join(dir, "c.svg")
	htmlPath := filepath.Join(dir, "d.html")
	src := `
fn main {
    c := viz.bar([3, 1, 4], {"title": "Pi"})
    viz.save("` + svgPath + `", c)?
    page := viz.page("Dash", [c])
    viz.save("` + htmlPath + `", page)?
    println("ok")
}
`
	// escape for weft string - paths may have backslashes on windows; we're on mac
	src = strings.ReplaceAll(src, `\`, `/`)
	runViz(t, src)
	b, err := os.ReadFile(svgPath)
	if err != nil || !strings.Contains(string(b), "<svg") {
		t.Fatal(err, string(b))
	}
	h, err := os.ReadFile(htmlPath)
	if err != nil || !strings.Contains(string(h), "Dash") {
		t.Fatal(err, string(h))
	}
}

func TestVizTable(t *testing.T) {
	out := runViz(t, `
fn main {
    t := viz.table([["name", "n"], ["a", 1], ["b", 2]])
    println(t.text)
    println(t.html)
}
`)
	if !strings.Contains(out, "name") || !strings.Contains(out, "<table") {
		t.Fatal(out)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
