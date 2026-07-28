package weft

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// NotebookCell is one code block + its output.
type NotebookCell struct {
	Source string
	Output string
	Error  string
}

// RunNotebook executes a .weft file, treating each top-level fn as a cell.
// Returns cells with captured output.
func RunNotebook(path string) ([]NotebookCell, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return RunNotebookSource(path, string(src))
}

// RunNotebookSource runs notebook from source string.
func RunNotebookSource(name, src string) ([]NotebookCell, error) {
	// Split into cells: each fn is a cell, top-level code before first fn is cell 0
	cells := splitCells(src)
	var results []NotebookCell

	for _, cell := range cells {
		var out bytes.Buffer
		ctx := New(Options{Stdout: &out, Stderr: &out})
		err := ctx.RunSource(context.Background(), name, cell)
		c := NotebookCell{Source: cell, Output: out.String()}
		if err != nil {
			c.Error = err.Error()
		}
		results = append(results, c)
	}
	return results, nil
}

// splitCells splits source into executable cells.
// Each fn with "main" or "cell" in the name is a separate cell.
// Shared declarations (types, enums, other fns) are prepended to each cell.
func splitCells(src string) []string {
	lines := strings.Split(src, "\n")
	var shared strings.Builder
	var cells []string
	var current strings.Builder
	inCell := false

	depth := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track brace depth
		for _, r := range line {
			if r == '{' {
				depth++
			}
			if r == '}' {
				depth--
			}
		}

		isCellStart := (strings.HasPrefix(trimmed, "fn main") ||
			strings.HasPrefix(trimmed, "fn cell")) && !inCell

		if isCellStart {
			inCell = true
			current.Reset()
			current.WriteString(line)
			current.WriteByte('\n')
			if depth <= 0 {
				// single-line fn
				cells = append(cells, shared.String()+current.String())
				inCell = false
				depth = 0
			}
			continue
		}

		if inCell {
			current.WriteString(line)
			current.WriteByte('\n')
			if depth <= 0 {
				cells = append(cells, shared.String()+current.String())
				inCell = false
				depth = 0
			}
		} else {
			if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
				shared.WriteString(line)
				shared.WriteByte('\n')
			}
		}
	}

	if len(cells) == 0 && shared.Len() > 0 {
		// No cells found — treat entire file as one cell
		cells = append(cells, src)
	}

	return cells
}

// NotebookHTML renders cells as a self-contained HTML page.
func NotebookHTML(title string, cells []NotebookCell) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>` + html.EscapeString(title) + `</title>
<style>
body { font-family: 'SF Mono', 'Fira Code', monospace; max-width: 800px; margin: 2em auto; background: #1e1e1e; color: #d4d4d4; }
h1 { color: #569cd6; }
.cell { margin: 1em 0; border: 1px solid #333; border-radius: 6px; overflow: hidden; }
.source { background: #252526; padding: 12px; white-space: pre-wrap; font-size: 14px; }
.output { background: #1e1e1e; padding: 12px; border-top: 1px solid #333; color: #6a9955; white-space: pre-wrap; }
.error { background: #3e1e1e; padding: 12px; border-top: 1px solid #533; color: #f44747; white-space: pre-wrap; }
.cell-num { color: #666; font-size: 12px; padding: 4px 12px; background: #2d2d2d; }
</style>
</head>
<body>
<h1>` + html.EscapeString(title) + `</h1>
`)
	for i, cell := range cells {
		b.WriteString(fmt.Sprintf(`<div class="cell">
<div class="cell-num">Cell %d</div>
<div class="source">%s</div>
`, i+1, html.EscapeString(strings.TrimSpace(cell.Source))))
		if cell.Output != "" {
			b.WriteString(fmt.Sprintf(`<div class="output">%s</div>
`, html.EscapeString(cell.Output)))
		}
		if cell.Error != "" {
			b.WriteString(fmt.Sprintf(`<div class="error">%s</div>
`, html.EscapeString(cell.Error)))
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// RunNotebookToHTML runs a .weft file and writes an HTML notebook.
func RunNotebookToHTML(path, outPath string) error {
	cells, err := RunNotebook(path)
	if err != nil {
		return err
	}
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	h := NotebookHTML(title, cells)
	if outPath == "" {
		outPath = strings.TrimSuffix(path, filepath.Ext(path)) + ".html"
	}
	return os.WriteFile(outPath, []byte(h), 0o644)
}
