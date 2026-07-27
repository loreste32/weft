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
