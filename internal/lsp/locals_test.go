package lsp

import (
	"strings"
	"testing"
)

func TestFindBindingDefinition(t *testing.T) {
	src := "fn add(a, b) {\n    x := a + b\n    say(x)\n}\n"
	b, ok := findBindingDefinition(src, "x")
	if !ok || b.Kind != "let" {
		t.Fatalf("want let x, got %+v ok=%v", b, ok)
	}
	b, ok = findBindingDefinition(src, "a")
	if !ok || b.Kind != "param" {
		t.Fatalf("want param a, got %+v ok=%v", b, ok)
	}
	b, ok = findBindingDefinition(src, "add")
	if !ok || b.Kind != "fn" {
		t.Fatalf("want fn add, got %+v ok=%v", b, ok)
	}
	names := localBindingNames(src)
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "x") || !strings.Contains(joined, "a") {
		t.Fatalf("localBindingNames: %v", names)
	}
}

func TestDocumentHighlights(t *testing.T) {
	src := "fn add(a, b) {\n    x := a + b\n    say(x)\n}\n"
	// cursor on x in say(x)
	h := documentHighlights(src, 2, 8)
	items, ok := h.([]map[string]any)
	if !ok || len(items) < 2 {
		t.Fatalf("want >=2 highlights, got %#v", h)
	}
}
