package lsp_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/lsp"
)

// runCodeAction drives initialize → didOpen → textDocument/codeAction → shutdown
// and returns the result payload of the codeAction response.
func runCodeAction(t *testing.T, uri, src string, line int) any {
	t.Helper()
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": uri, "text": src}},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/codeAction",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"range": map[string]any{
				"start": map[string]int{"line": line, "character": 0},
				"end":   map[string]int{"line": line, "character": 0},
			},
			"context": map[string]any{"diagnostics": []any{}},
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	for _, m := range readAllMessages(out) {
		if id, _ := m["id"].(float64); id == 2 {
			return m["result"]
		}
	}
	t.Fatal("no codeAction response")
	return nil
}

// autoImportAction finds the "Add `use <pkg>`" action for pkg, or nil.
func autoImportAction(result any, uri, pkg string) map[string]any {
	items, _ := result.([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m["title"] == "Add `use "+pkg+"`" {
			return m
		}
	}
	return nil
}

func TestLSPAutoImportOffersUse(t *testing.T) {
	uri := "file:///tmp/ai.weft"
	src := "fn main { json.parse(\"{}\") }\n"
	action := autoImportAction(runCodeAction(t, uri, src, 0), uri, "json")
	if action == nil {
		t.Fatal("missing auto-import action for json")
	}
	if action["kind"] != "quickfix" {
		t.Fatalf("kind = %v, want quickfix", action["kind"])
	}
	edit, _ := action["edit"].(map[string]any)
	changes, _ := edit["changes"].(map[string]any)
	edits, _ := changes[uri].([]any)
	if len(edits) != 1 {
		t.Fatalf("want 1 edit, got %v", changes)
	}
	ed, _ := edits[0].(map[string]any)
	if ed["newText"] != "use json\n" {
		t.Fatalf("newText = %q", ed["newText"])
	}
	// no existing use statements → insert at line 0
	rng, _ := ed["range"].(map[string]any)
	start, _ := rng["start"].(map[string]any)
	if int(start["line"].(float64)) != 0 || int(start["character"].(float64)) != 0 {
		t.Fatalf("insert range = %v", rng)
	}
}

func TestLSPAutoImportSkipsImported(t *testing.T) {
	uri := "file:///tmp/ai2.weft"
	src := "use json\nfn main { json.parse(\"{}\") }\n"
	if action := autoImportAction(runCodeAction(t, uri, src, 1), uri, "json"); action != nil {
		t.Fatal("should not offer import for already-imported package")
	}
}

func TestLSPAutoImportInsertsAfterUse(t *testing.T) {
	uri := "file:///tmp/ai3.weft"
	src := "use http\n\nfn main { json.parse(\"{}\") }\n"
	action := autoImportAction(runCodeAction(t, uri, src, 2), uri, "json")
	if action == nil {
		t.Fatal("missing auto-import action for json")
	}
	edit, _ := action["edit"].(map[string]any)
	changes, _ := edit["changes"].(map[string]any)
	edits, _ := changes[uri].([]any)
	ed, _ := edits[0].(map[string]any)
	rng, _ := ed["range"].(map[string]any)
	start, _ := rng["start"].(map[string]any)
	// inserted right after the existing `use http` on line 0
	if int(start["line"].(float64)) != 1 {
		t.Fatalf("insert line = %v, want 1", start["line"])
	}
}

func TestLSPAutoImportIgnoresUnknown(t *testing.T) {
	uri := "file:///tmp/ai4.weft"
	src := "fn main { foo.bar() }\n"
	result := runCodeAction(t, uri, src, 0)
	items, _ := result.([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if title, _ := m["title"].(string); strings.HasPrefix(title, "Add `use ") {
			t.Fatalf("unexpected auto-import action for unknown package: %q", title)
		}
	}
}
