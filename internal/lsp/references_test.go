package lsp_test

import (
	"bytes"
	"testing"

	"github.com/loreste/weft/internal/lsp"
)

// runReferences drives initialize → didOpen → textDocument/references → shutdown
// and returns the result payload of the references response.
func runReferences(t *testing.T, uri, src string, line, character int) any {
	t.Helper()
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": uri, "text": src}},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/references",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": line, "character": character},
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
	t.Fatal("no references response")
	return nil
}

func TestLSPReferences(t *testing.T) {
	uri := "file:///tmp/refs.weft"
	src := "fn add(a, b) { a + b }\nfn main { say(add(1, 2)) }\n"
	// cursor on "add" at its definition (line 0, char 3)
	result := runReferences(t, uri, src, 0, 3)
	locs, ok := result.([]any)
	if !ok || len(locs) != 2 {
		t.Fatalf("want 2 reference locations, got %v", result)
	}
	for i, want := range [][2]int{{3, 6}, {14, 17}} {
		loc, _ := locs[i].(map[string]any)
		if loc["uri"] != uri {
			t.Fatalf("loc %d uri = %v", i, loc["uri"])
		}
		rng, _ := loc["range"].(map[string]any)
		start, _ := rng["start"].(map[string]any)
		end, _ := rng["end"].(map[string]any)
		if int(start["character"].(float64)) != want[0] || int(end["character"].(float64)) != want[1] {
			t.Errorf("loc %d range = %v..%v, want %d..%d", i, start["character"], end["character"], want[0], want[1])
		}
	}
}

func TestLSPReferencesKeyword(t *testing.T) {
	// cursor on "fn" (a keyword) → null result
	result := runReferences(t, "file:///tmp/kw.weft", "fn main { say(1) }", 0, 0)
	if result != nil {
		t.Fatalf("keyword references should be null, got %v", result)
	}
}

func TestLSPReferencesBoundariesAndStrings(t *testing.T) {
	// "adder" must not match "add" (word boundary), and "add" inside a
	// string literal must not match either → only the definition counts.
	src := "fn add { 1 }\nfn main { adder() }\nsay(\"add\")\n"
	result := runReferences(t, "file:///tmp/bnd.weft", src, 0, 3)
	locs, ok := result.([]any)
	if !ok || len(locs) != 1 {
		t.Fatalf("want exactly 1 location, got %v", result)
	}
}

func TestLSPReferencesNoMatch(t *testing.T) {
	// cursor on punctuation (no word under cursor) → null
	result := runReferences(t, "file:///tmp/none.weft", "fn main { say(1) }", 0, 8)
	if result != nil {
		t.Fatalf("expected null result, got %v", result)
	}
}
