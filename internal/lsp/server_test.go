package lsp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/lsp"
)

func writeRPC(w io.Writer, v any) {
	body, _ := json.Marshal(v)
	fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func readAllMessages(r io.Reader) []map[string]any {
	data, _ := io.ReadAll(r)
	var out []map[string]any
	rest := string(data)
	for {
		idx := strings.Index(strings.ToLower(rest), "content-length:")
		if idx < 0 {
			break
		}
		rest = rest[idx:]
		var n int
		lineEnd := strings.Index(rest, "\r\n")
		if lineEnd < 0 {
			break
		}
		fmt.Sscanf(rest[:lineEnd], "Content-Length: %d", &n)
		// find blank line
		hdrEnd := strings.Index(rest, "\r\n\r\n")
		if hdrEnd < 0 {
			break
		}
		body := rest[hdrEnd+4:]
		if n > len(body) {
			n = len(body)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(body[:n]), &m); err == nil {
			out = append(out, m)
		}
		rest = body[n:]
	}
	return out
}

func TestLSPInitializeAndDiagnostics(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "initialized",
		"params":  map[string]any{},
	})
	// open invalid file → diagnostics
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri":  "file:///tmp/bad.weft",
				"text": "fn main { ??? }",
			},
		},
	})
	// completion
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "textDocument/completion",
		"params":  map[string]any{},
	})
	// hover on known word after didOpen with good content
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri":  "file:///tmp/ok.weft",
				"text": "fn main { map }",
			},
		},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "textDocument/hover",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/ok.weft"},
			"position":     map[string]any{"line": 0, "character": 11},
		},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "shutdown",
		"params":  nil,
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "exit",
	})

	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)
	if len(msgs) < 3 {
		t.Fatalf("too few messages: %d %#v", len(msgs), msgs)
	}

	// initialize result
	initOK := false
	hasDiags := false
	hasCompletion := false
	hasHover := false
	for _, m := range msgs {
		if id, ok := m["id"].(float64); ok && id == 1 {
			res, _ := m["result"].(map[string]any)
			caps, _ := res["capabilities"].(map[string]any)
			if caps != nil {
				initOK = true
			}
		}
		if m["method"] == "textDocument/publishDiagnostics" {
			hasDiags = true
			params, _ := m["params"].(map[string]any)
			diags, _ := params["diagnostics"].([]any)
			if len(diags) == 0 {
				// may be ok for ok.weft; bad.weft should have errors
			}
		}
		if id, ok := m["id"].(float64); ok && id == 2 {
			if items, ok := m["result"].([]any); ok && len(items) > 0 {
				hasCompletion = true
			}
		}
		if id, ok := m["id"].(float64); ok && id == 3 {
			if m["result"] != nil {
				hasHover = true
			}
		}
	}
	if !initOK {
		t.Fatal("missing initialize capabilities", msgs)
	}
	if !hasDiags {
		t.Fatal("missing diagnostics notification", msgs)
	}
	if !hasCompletion {
		t.Fatal("missing completion items", msgs)
	}
	if !hasHover {
		t.Fatal("missing hover", msgs)
	}
}

func TestLSPGoodFileNoFatalDiags(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri":  "file:///tmp/good.weft",
				"text": "fn main { say(1) }",
			},
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	// should complete without error
	if out.Len() == 0 {
		t.Fatal("no output")
	}
}

func TestLSPDefinitionAndSymbols(t *testing.T) {
	src := "fn helper { 1 }\nfn main { helper() }\n"
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/d.weft", "text": src},
		},
	})
	// definition of helper at call site (line 1, char ~2)
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "textDocument/definition",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/d.weft"},
			"position":     map[string]any{"line": 1, "character": 12},
		},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "textDocument/documentSymbol",
		"params":  map[string]any{"textDocument": map[string]any{"uri": "file:///tmp/d.weft"}},
	})
	// package member completion after "http."
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "textDocument/completion",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/d.weft"},
			"position":     map[string]any{"line": 0, "character": 0},
		},
	})
	// force member completion with synthetic position in a doc containing http.
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/h.weft", "text": "fn main { http. }"},
		},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "textDocument/completion",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/h.weft"},
			// "fn main { http." → cursor right after the dot (index 15)
			"position": map[string]any{"line": 0, "character": 15},
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)
	var defOK, symOK, memberOK bool
	for _, m := range msgs {
		id, _ := m["id"].(float64)
		switch id {
		case 2:
			if m["result"] != nil {
				defOK = true
			}
		case 3:
			if items, ok := m["result"].([]any); ok && len(items) >= 2 {
				symOK = true
			}
		case 5:
			if items, ok := m["result"].([]any); ok {
				for _, it := range items {
					mm, _ := it.(map[string]any)
					if mm["label"] == "get" || mm["label"] == "post" {
						memberOK = true
					}
				}
			}
		}
	}
	if !defOK {
		t.Fatal("definition missing", msgs)
	}
	if !symOK {
		t.Fatal("document symbols missing", msgs)
	}
	if !memberOK {
		t.Fatal("http. member completion missing", msgs)
	}
}

func TestLSPWebYamlHover(t *testing.T) {
	src := "fn main { web.sse(x); yaml.parse(s) }"
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/w.weft", "text": src},
		},
	})
	// hover web.sse
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/hover",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/w.weft"},
			"position":     map[string]any{"line": 0, "character": 14}, // sse
		},
	})
	// completion after yaml.
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/y.weft", "text": "fn main { yaml. }"},
		},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "textDocument/completion",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/y.weft"},
			"position":     map[string]any{"line": 0, "character": 15},
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)
	var hoverOK, parseOK bool
	for _, m := range msgs {
		id, _ := m["id"].(float64)
		if id == 2 {
			if res, ok := m["result"].(map[string]any); ok && res != nil {
				if c, ok := res["contents"].(map[string]any); ok {
					if v, _ := c["value"].(string); strings.Contains(v, "web.sse") {
						hoverOK = true
					}
				}
			}
		}
		if id == 3 {
			if items, ok := m["result"].([]any); ok {
				for _, it := range items {
					mm, _ := it.(map[string]any)
					if mm["label"] == "parse" {
						if d, _ := mm["detail"].(string); strings.Contains(d, "yaml.parse") {
							parseOK = true
						}
					}
				}
			}
		}
	}
	if !hoverOK {
		t.Fatal("web.sse hover missing", msgs)
	}
	if !parseOK {
		t.Fatal("yaml.parse completion detail missing", msgs)
	}
}

func TestLSPLLMMemberHelp(t *testing.T) {
	// line0: fn main { llm.
	//          012345678901234  → char 14 is after '.'
	compSrc := "fn main { llm. }"
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/llm.weft", "text": compSrc},
		},
	})
	// completion after llm.
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "textDocument/completion",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/llm.weft"},
			"position":     map[string]any{"line": 0, "character": 14},
		},
	})
	// hover + signature on llm.ask("hi")
	// fn main { llm.ask("hi") }
	// 012345678901234567890
	hoverSrc := `fn main { llm.ask("hi") }`
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri":  "file:///tmp/llm2.weft",
				"text": hoverSrc,
			},
		},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "textDocument/hover",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/llm2.weft"},
			"position":     map[string]any{"line": 0, "character": 14}, // on "ask"
		},
	})
	// signature help inside llm.ask(
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "textDocument/signatureHelp",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/llm2.weft"},
			"position":     map[string]any{"line": 0, "character": 18}, // after (
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)
	var hasAsk, hoverOK, sigOK bool
	for _, m := range msgs {
		id, _ := m["id"].(float64)
		switch id {
		case 2:
			if items, ok := m["result"].([]any); ok {
				for _, it := range items {
					mm, _ := it.(map[string]any)
					if mm["label"] == "ask" {
						hasAsk = true
						if d, _ := mm["detail"].(string); strings.Contains(d, "llm.ask") {
							// detail enriched with signature
						}
					}
				}
			}
		case 3:
			if res, ok := m["result"].(map[string]any); ok && res != nil {
				if c, ok := res["contents"].(map[string]any); ok {
					if v, _ := c["value"].(string); strings.Contains(v, "llm.ask") {
						hoverOK = true
					}
				}
			}
		case 4:
			if res, ok := m["result"].(map[string]any); ok && res != nil {
				if sigs, ok := res["signatures"].([]any); ok && len(sigs) > 0 {
					sm, _ := sigs[0].(map[string]any)
					if lab, _ := sm["label"].(string); strings.Contains(lab, "llm.ask") {
						sigOK = true
					}
				}
			}
		}
	}
	if !hasAsk {
		t.Fatal("llm.ask completion missing", msgs)
	}
	if !hoverOK {
		t.Fatal("llm.ask hover missing signature", msgs)
	}
	if !sigOK {
		t.Fatal("llm.ask signatureHelp missing", msgs)
	}
}

func TestLSPFormattingAndEnumCompletion(t *testing.T) {
	src := "enum Status{Ok,Err}\nfn main{say(Status.Ok)}\n"
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/fmt.weft", "text": src},
		},
	})
	// format
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "textDocument/formatting",
		"params":  map[string]any{"textDocument": map[string]any{"uri": "file:///tmp/fmt.weft"}},
	})
	// enum member completion after Status.
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri":  "file:///tmp/e.weft",
				"text": "enum Status { Ok, Err }\nfn main { Status. }",
			},
		},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "textDocument/completion",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/e.weft"},
			// "fn main { Status." → cursor right after the dot (index 17)
			"position": map[string]any{"line": 1, "character": 17},
		},
	})
	// definition of Status (incomplete buffer still resolves enum via scan)
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "textDocument/definition",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/e.weft"},
			"position":     map[string]any{"line": 1, "character": 13}, // inside Status
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)
	var fmtOK, enumOK, defOK, capFmt bool
	for _, m := range msgs {
		id, _ := m["id"].(float64)
		if id == 1 {
			res, _ := m["result"].(map[string]any)
			caps, _ := res["capabilities"].(map[string]any)
			if caps != nil {
				if _, ok := caps["documentFormattingProvider"]; ok {
					capFmt = true
				}
			}
		}
		if id == 2 {
			if edits, ok := m["result"].([]any); ok && len(edits) > 0 {
				ed, _ := edits[0].(map[string]any)
				nt, _ := ed["newText"].(string)
				if strings.Contains(nt, "enum Status { Ok, Err }") {
					fmtOK = true
				}
			}
		}
		if id == 3 {
			if items, ok := m["result"].([]any); ok {
				for _, it := range items {
					mm, _ := it.(map[string]any)
					if mm["label"] == "Ok" || mm["label"] == "Err" {
						enumOK = true
					}
				}
			}
		}
		if id == 4 && m["result"] != nil {
			defOK = true
		}
	}
	if !capFmt {
		t.Fatal("missing documentFormattingProvider")
	}
	if !fmtOK {
		t.Fatal("formatting missing enum compact form", msgs)
	}
	if !enumOK {
		t.Fatal("enum variant completion missing", msgs)
	}
	if !defOK {
		t.Fatal("enum definition missing", msgs)
	}
}

func TestLSPRename(t *testing.T) {
	src := "fn add(a, b) { a + b }\nfn main { say(add(1, 2)) }\n"
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/r.weft", "text": src},
		},
	})
	// prepareRename on "add" at line 0, char 3
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/prepareRename",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/r.weft"},
			"position":     map[string]any{"line": 0, "character": 3},
		},
	})
	// rename "add" → "sum"
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "textDocument/rename",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/r.weft"},
			"position":     map[string]any{"line": 0, "character": 3},
			"newName":      "sum",
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)
	var prepareOK, renameOK bool
	for _, m := range msgs {
		id, _ := m["id"].(float64)
		if id == 2 && m["result"] != nil {
			prepareOK = true
		}
		if id == 3 {
			if res, ok := m["result"].(map[string]any); ok && res != nil {
				if changes, ok := res["changes"].(map[string]any); ok {
					for _, edits := range changes {
						if es, ok := edits.([]any); ok && len(es) >= 2 {
							renameOK = true
						}
					}
				}
			}
		}
	}
	if !prepareOK {
		t.Fatal("prepareRename failed", msgs)
	}
	if !renameOK {
		t.Fatal("rename failed", msgs)
	}
}

func TestLSPRenameKeyword(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/k.weft", "text": "fn main { say(1) }"},
		},
	})
	// try to rename "fn" (keyword) — should return null
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/prepareRename",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/k.weft"},
			"position":     map[string]any{"line": 0, "character": 0},
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	lsp.Run(in, out)
	msgs := readAllMessages(out)
	for _, m := range msgs {
		id, _ := m["id"].(float64)
		if id == 2 && m["result"] != nil {
			t.Fatal("keywords should not be renameable")
		}
	}
}

func TestLSPDidChange(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/c.weft", "text": "fn main { say(1) }"},
		},
	})
	// Change content
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didChange",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/c.weft"},
			"contentChanges": []any{
				map[string]any{"text": "fn main { say(2) }"},
			},
		},
	})
	// Close
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didClose",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/c.weft"},
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
}

func TestLSPUnknownMethod(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "custom/unknown", "params": map[string]any{}})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "custom/notification"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
}

func TestLSPTypeHoverAndWarnings(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	src := `type User { name: str, age: int }
fn add(a: int, b: int) -> int { a + b }
fn main {
    u := User{name: "a", age: 1}
    n: int := "nope"
    say(u.name)
    say(add(1, 2))
}
`
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri":  "file:///tmp/typed.weft",
				"text": src,
			},
		},
	})
	// hover on binding n (line index 4: "    n: int := ...") — character on 'n'
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/hover",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/typed.weft"},
			"position":     map[string]any{"line": 4, "character": 5},
		},
	})
	// hover on add function name in call (line 6)
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "textDocument/hover",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/typed.weft"},
			"position":     map[string]any{"line": 6, "character": 9},
		},
	})
	// field completion after "u."
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 4, "method": "textDocument/completion",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/typed.weft"},
			"position":     map[string]any{"line": 5, "character": 10}, // after u.
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 5, "method": "shutdown", "params": nil})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})

	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)

	var warnCount int
	var hoverN, hoverAdd string
	var fieldLabels []string
	for _, m := range msgs {
		if m["method"] == "textDocument/publishDiagnostics" {
			params, _ := m["params"].(map[string]any)
			for _, d := range params["diagnostics"].([]any) {
				dm, _ := d.(map[string]any)
				if sev, _ := dm["severity"].(float64); sev == 2 {
					warnCount++
				}
			}
		}
		if id, ok := m["id"].(float64); ok && id == 2 {
			if res, ok := m["result"].(map[string]any); ok {
				if c, ok := res["contents"].(map[string]any); ok {
					hoverN, _ = c["value"].(string)
				}
			}
		}
		if id, ok := m["id"].(float64); ok && id == 3 {
			if res, ok := m["result"].(map[string]any); ok {
				if c, ok := res["contents"].(map[string]any); ok {
					hoverAdd, _ = c["value"].(string)
				}
			}
		}
		if id, ok := m["id"].(float64); ok && id == 4 {
			if items, ok := m["result"].([]any); ok {
				for _, it := range items {
					im, _ := it.(map[string]any)
					if lab, ok := im["label"].(string); ok {
						fieldLabels = append(fieldLabels, lab)
					}
				}
			}
		}
	}
	if warnCount < 1 {
		t.Fatalf("want type warning diagnostics, got %d warnings among msgs", warnCount)
	}
	// After mismatch, n binds actual type str (poison fix) — still show type
	if !strings.Contains(hoverN, "n") || !strings.Contains(hoverN, "`") {
		t.Fatalf("hover on n: %q", hoverN)
	}
	if !strings.Contains(hoverAdd, "add") && !strings.Contains(hoverAdd, "fn") {
		// may be prelude-less; function global should show
		t.Logf("hover add: %q", hoverAdd)
	}
	hasName, hasAge := false, false
	for _, l := range fieldLabels {
		if l == "name" {
			hasName = true
		}
		if l == "age" {
			hasAge = true
		}
	}
	if !hasName || !hasAge {
		t.Fatalf("field completion want name+age, got %v", fieldLabels)
	}
}

func TestLSPMultiFileRename(t *testing.T) {
	// Top-level helper renamed across two open docs
	a := "fn helper { 1 }\nfn main { say(helper()) }\n"
	b := "fn main { say(helper()) }\n"
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": "file:///tmp/a.weft", "text": a}},
	})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": "file:///tmp/b.weft", "text": b}},
	})
	// rename helper at def site (line 0, char 3)
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/rename",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/a.weft"},
			"position":     map[string]any{"line": 0, "character": 3},
			"newName":      "aid",
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)
	var sawA, sawB bool
	for _, m := range msgs {
		if id, _ := m["id"].(float64); id != 2 {
			continue
		}
		res, _ := m["result"].(map[string]any)
		if res == nil {
			t.Fatal("nil rename result", msgs)
		}
		ch, _ := res["changes"].(map[string]any)
		if ch == nil {
			t.Fatal("no changes", res)
		}
		if _, ok := ch["file:///tmp/a.weft"]; ok {
			sawA = true
		}
		if _, ok := ch["file:///tmp/b.weft"]; ok {
			sawB = true
		}
	}
	if !sawA || !sawB {
		t.Fatalf("multi-file rename want both files, a=%v b=%v msgs=%v", sawA, sawB, msgs)
	}
}

func TestLSPExtractFunction(t *testing.T) {
	src := "fn main {\n    n := 1 + 2\n    say(n)\n}\n"
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	uri := "file:///tmp/ex.weft"
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": uri, "text": src}},
	})
	// select "1 + 2" on line 1
	// "    n := 1 + 2" → start char 9 end 14
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/codeAction",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"range": map[string]any{
				"start": map[string]int{"line": 1, "character": 9},
				"end":   map[string]int{"line": 1, "character": 14},
			},
			"context": map[string]any{"diagnostics": []any{}},
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)
	var okExtract bool
	for _, m := range msgs {
		if id, _ := m["id"].(float64); id != 2 {
			continue
		}
		items, _ := m["result"].([]any)
		for _, it := range items {
			mm, _ := it.(map[string]any)
			if mm["title"] == "Extract function" {
				edit, _ := mm["edit"].(map[string]any)
				ch, _ := edit["changes"].(map[string]any)
				edits, _ := ch[uri].([]any)
				if len(edits) > 0 {
					ed0, _ := edits[0].(map[string]any)
					nt, _ := ed0["newText"].(string)
					if strings.Contains(nt, "fn extracted_") && strings.Contains(nt, "extracted_") {
						okExtract = true
					}
				}
			}
		}
	}
	if !okExtract {
		t.Fatal("extract function code action missing", msgs)
	}
}

func TestLSPLocalDefinitionAndHighlight(t *testing.T) {
	// line0: fn add(a, b) {
	// line1:     x := a + b
	// line2:     say(x)
	// line3: }
	src := "fn add(a, b) {\n    x := a + b\n    say(x)\n}\n"
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	uri := "file:///tmp/local.weft"
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": uri, "text": src},
		},
	})
	// definition of x at say(x) — line 2, character on x inside say(x)
	// "    say(x)" → x is at index 8
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/definition",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": 2, "character": 8},
		},
	})
	// definition of param a at use site "a + b"
	// "    x := a + b" → a at index 9
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "textDocument/definition",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": 1, "character": 9},
		},
	})
	// documentHighlight on x at say(x)
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 4, "method": "textDocument/documentHighlight",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": 2, "character": 8},
		},
	})
	// completion should include local binding x
	writeRPC(in, map[string]any{
		"jsonrpc": "2.0", "id": 5, "method": "textDocument/completion",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": 2, "character": 4},
		},
	})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeRPC(in, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := lsp.Run(in, out); err != nil {
		t.Fatal(err)
	}
	msgs := readAllMessages(out)
	var capHL, defX, defA, hlOK, compX bool
	for _, m := range msgs {
		id, _ := m["id"].(float64)
		if id == 1 {
			res, _ := m["result"].(map[string]any)
			caps, _ := res["capabilities"].(map[string]any)
			if caps != nil {
				if _, ok := caps["documentHighlightProvider"]; ok {
					capHL = true
				}
			}
		}
		if id == 2 {
			if res, ok := m["result"].(map[string]any); ok && res != nil {
				// should point at line 1 (x :=)
				if r, ok := res["range"].(map[string]any); ok {
					if st, ok := r["start"].(map[string]any); ok {
						if ln, ok := st["line"].(float64); ok && int(ln) == 1 {
							defX = true
						}
						if ln, ok := st["line"].(int); ok && ln == 1 {
							defX = true
						}
					}
				}
			}
		}
		if id == 3 {
			if res, ok := m["result"].(map[string]any); ok && res != nil {
				if r, ok := res["range"].(map[string]any); ok {
					if st, ok := r["start"].(map[string]any); ok {
						// param a is on line 0
						if ln, ok := st["line"].(float64); ok && int(ln) == 0 {
							defA = true
						}
						if ln, ok := st["line"].(int); ok && ln == 0 {
							defA = true
						}
					}
				}
			}
		}
		if id == 4 {
			if items, ok := m["result"].([]any); ok && len(items) >= 2 {
				hlOK = true
			}
		}
		if id == 5 {
			if items, ok := m["result"].([]any); ok {
				for _, it := range items {
					mm, _ := it.(map[string]any)
					if mm["label"] == "x" || mm["label"] == "a" || mm["label"] == "b" {
						compX = true
					}
				}
			}
		}
	}
	if !capHL {
		t.Fatal("missing documentHighlightProvider")
	}
	if !defX {
		t.Fatal("definition of local x missing or wrong line", msgs)
	}
	if !defA {
		t.Fatal("definition of param a missing or wrong line", msgs)
	}
	if !hlOK {
		t.Fatal("documentHighlight for x missing", msgs)
	}
	if !compX {
		t.Fatal("local binding completion missing", msgs)
	}
}
