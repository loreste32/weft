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

func TestMapFilterReduce(t *testing.T) {
	src := `
fn double(x) { x * 2 }
fn even(x) { x % 2 == 0 }
fn add(a, b) { a + b }
fn main {
    xs := [1, 2, 3, 4]
    println(map(xs, double))
    println(filter(xs, even))
    println(reduce(xs, 0, add))
    println(any(xs, even))
    println(all(xs, even))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "p.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "[2, 4, 6, 8]") || !strings.Contains(s, "[2, 4]") || !strings.Contains(s, "10") {
		t.Fatal(s)
	}
}

func TestTableAndJSONL(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.jsonl")
	_ = os.WriteFile(in, []byte(`{"name":"Ada","ok":true,"role":"admin"}
{"name":"Bob","ok":false,"role":"user"}
{"name":"Cy","ok":true,"role":"ops"}
`), 0o644)
	outp := filepath.Join(dir, "out.jsonl")
	src := `
fn main -> Result {
    rows := jsonl.read("` + in + `")?
    active := table.where_truthy(rows, "ok")
    slim := table.project(active, ["name", "role"])
    ranked := table.sort(slim, "name", false)
    jsonl.write("` + outp + `", ranked)?
    println(len(ranked))
    println(table.pluck(ranked, "name"))
    g := table.group_by(ranked, "role")
    println(len(keys(g)))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "t.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "2") || !strings.Contains(out.String(), "Ada") {
		t.Fatal(out.String())
	}
	b, _ := os.ReadFile(outp)
	if !strings.Contains(string(b), "Ada") || strings.Contains(string(b), "Bob") {
		t.Fatal(string(b))
	}
}

func TestPipeBatch(t *testing.T) {
	src := `
fn main {
    b := pipe.batch(range(10), 3)
    println(len(b))
    println(len(b[0]))
    println(pipe.flatten([[1,2],[3]]))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "b.weft", src); err != nil {
		t.Fatal(err)
	}
	// range(10) = 0..9 → 4 batches (3+3+3+1)
	if !strings.Contains(out.String(), "4") || !strings.Contains(out.String(), "[1, 2, 3]") {
		t.Fatal(out.String())
	}
}

func TestParMap(t *testing.T) {
	src := `
fn sq(x) { x * x }
fn main {
    println(par_map([1,2,3,4], sq, 2))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "pm.weft", src); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[1, 4, 9, 16]") {
		t.Fatal(out.String())
	}
}
