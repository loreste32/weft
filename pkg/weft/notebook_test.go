package weft

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitCells(t *testing.T) {
	src := `use math

fn main {
    say("cell 1")
}

fn cell_2 {
    say("cell 2")
}
`
	cells := splitCells(src)
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}
	if !strings.Contains(cells[0], "cell 1") {
		t.Fatal("cell 1 content")
	}
	if !strings.Contains(cells[1], "cell 2") {
		t.Fatal("cell 2 content")
	}
	// shared code should be in both
	if !strings.Contains(cells[0], "use math") {
		t.Fatal("shared code in cell 1")
	}
}

func TestSplitCellsSingleFile(t *testing.T) {
	src := `say("hello")`
	cells := splitCells(src)
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(cells))
	}
}

func TestRunNotebookSource(t *testing.T) {
	src := `fn main { say("hello") }
fn cell_2 { say("world") }
`
	cells, err := RunNotebookSource("test.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) < 1 {
		t.Fatal("no cells")
	}
	found := false
	for _, c := range cells {
		if strings.Contains(c.Output, "hello") {
			found = true
		}
	}
	if !found {
		t.Fatal("hello not in output")
	}
}

func TestNotebookHTML(t *testing.T) {
	cells := []NotebookCell{
		{Source: `say("hi")`, Output: "hi\n"},
		{Source: `1 / 0`, Error: "division by zero"},
	}
	html := NotebookHTML("test", cells)
	if !strings.Contains(html, "Cell 1") {
		t.Fatal("cell 1")
	}
	if !strings.Contains(html, "hi") {
		t.Fatal("output")
	}
	if !strings.Contains(html, "division") {
		t.Fatal("error")
	}
	if !strings.Contains(html, "<html>") {
		t.Fatal("html")
	}
}

func TestRunNotebookToHTML(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "nb.weft")
	os.WriteFile(src, []byte(`fn main { say(1 + 2) }`), 0644)

	out := filepath.Join(dir, "nb.html")
	if err := RunNotebookToHTML(src, out); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), "3") {
		t.Fatal("output not in html")
	}
}

func TestRunNotebookToHTMLDefault(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "demo.weft")
	os.WriteFile(src, []byte(`fn main { say("ok") }`), 0644)

	// empty outPath → auto
	if err := RunNotebookToHTML(src, ""); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dir, "demo.html")
	if _, err := os.Stat(expected); err != nil {
		t.Fatal("default output not created")
	}
}
