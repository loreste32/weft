package weft_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestStdlibNames(t *testing.T) {
	names := weft.StdlibNames()
	if len(names) < 20 {
		t.Fatalf("too few packages: %d", len(names))
	}
	found := false
	for _, n := range names {
		if n == "json" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("json missing")
	}
	mem := weft.StdlibMembers("json")
	if len(mem) < 3 {
		t.Fatalf("json members %v", mem)
	}
}

func TestFmtFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.weft")
	// messy spacing
	if err := os.WriteFile(path, []byte("fn main{mut x:=1+2\nsay(x)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := weft.FmtFiles([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("changed=%d", n)
	}
	b, _ := os.ReadFile(path)
	s := string(b)
	if !strings.Contains(s, "fn main {") {
		t.Fatalf("pretty: %q", s)
	}
	if !strings.Contains(s, "mut x := 1 + 2") {
		t.Fatalf("bind: %q", s)
	}
	// second pass no-op (stable)
	n2, err := weft.FmtFiles([]string{path})
	if err != nil || n2 != 0 {
		t.Fatalf("second n=%d err=%v body=%q", n2, err, s)
	}
}

func TestBenchSmoke(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x_bench.weft")
	if err := os.WriteFile(path, []byte(`
fn bench_add {
    1 + 1
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := weft.RunBench(weft.BenchOptions{Paths: []string{path}, N: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Results) != 1 || !rep.Results[0].OK {
		t.Fatalf("%+v", rep)
	}
}
