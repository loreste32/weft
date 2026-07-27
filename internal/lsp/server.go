// Package lsp is a minimal Language Server Protocol implementation for Weft.
// Supports: initialize, shutdown, textDocument/completion (stdlib + package/enum members),
// textDocument/hover, textDocument/definition (local fns/enums), textDocument/documentSymbol,
// textDocument/formatting (weft fmt), textDocument/publishDiagnostics via didOpen/didChange.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/format"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/types"
)

// Run starts the LSP on r/w (usually stdin/stdout).
func Run(r io.Reader, w io.Writer) error {
	s := &server{w: w, docs: map[string]string{}}
	br := bufio.NewReader(r)
	for {
		msg, err := readMessage(br)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := s.handle(msg); err != nil {
			return err
		}
		if s.exit {
			return nil
		}
	}
}

type server struct {
	w      io.Writer
	docs   map[string]string
	exit   bool
	nextID int
}

func (s *server) handle(raw []byte) error {
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil // ignore garbage
	}
	switch envelope.Method {
	case "initialize":
		return s.reply(envelope.ID, map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync": 1, // full
				"completionProvider": map[string]any{
					"triggerCharacters": []string{".", " "},
				},
				"hoverProvider":              true,
				"definitionProvider":         true,
				"documentSymbolProvider":     true,
				"documentFormattingProvider": true,
				"signatureHelpProvider":      map[string]any{"triggerCharacters": []string{"(", ","}},
			},
			"serverInfo": map[string]any{"name": "weft-lsp", "version": "0.3.30"},
		})
	case "initialized", "textDocument/didSave":
		return nil
	case "shutdown":
		return s.reply(envelope.ID, nil)
	case "exit":
		s.exit = true
		return nil
	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"textDocument"`
		}
		_ = json.Unmarshal(envelope.Params, &p)
		s.docs[p.TextDocument.URI] = p.TextDocument.Text
		s.publishDiags(p.TextDocument.URI, p.TextDocument.Text)
		return nil
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		_ = json.Unmarshal(envelope.Params, &p)
		if len(p.ContentChanges) > 0 {
			s.docs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
			s.publishDiags(p.TextDocument.URI, s.docs[p.TextDocument.URI])
		}
		return nil
	case "textDocument/didClose":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		_ = json.Unmarshal(envelope.Params, &p)
		delete(s.docs, p.TextDocument.URI)
		// clear diagnostics for closed buffer
		_ = s.notify("textDocument/publishDiagnostics", map[string]any{
			"uri": p.TextDocument.URI, "diagnostics": []any{},
		})
		return nil
	case "textDocument/completion":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		_ = json.Unmarshal(envelope.Params, &p)
		return s.reply(envelope.ID, s.completions(s.docs[p.TextDocument.URI], p.Position.Line, p.Position.Character))
	case "textDocument/hover":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		_ = json.Unmarshal(envelope.Params, &p)
		text := s.docs[p.TextDocument.URI]
		word := wordAt(text, p.Position.Line, p.Position.Character)
		// package.member or Enum.Variant hover
		if pkg, mem := dottedAt(text, p.Position.Line, p.Position.Character); pkg != "" {
			if h := hoverForMember(pkg, mem); h != nil {
				return s.reply(envelope.ID, h)
			}
			if h := hoverEnumVariant(text, pkg, mem); h != nil {
				return s.reply(envelope.ID, h)
			}
		}
		if h := hoverLocalEnum(text, word); h != nil {
			return s.reply(envelope.ID, h)
		}
		return s.reply(envelope.ID, hoverFor(word))
	case "textDocument/definition":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		_ = json.Unmarshal(envelope.Params, &p)
		return s.reply(envelope.ID, s.definition(p.TextDocument.URI, p.Position.Line, p.Position.Character))
	case "textDocument/documentSymbol":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		_ = json.Unmarshal(envelope.Params, &p)
		return s.reply(envelope.ID, s.documentSymbols(p.TextDocument.URI))
	case "textDocument/formatting":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		_ = json.Unmarshal(envelope.Params, &p)
		return s.reply(envelope.ID, s.formatDocument(p.TextDocument.URI))
	case "textDocument/signatureHelp":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		_ = json.Unmarshal(envelope.Params, &p)
		return s.reply(envelope.ID, signatureHelp(s.docs[p.TextDocument.URI], p.Position.Line, p.Position.Character))
	default:
		if len(envelope.ID) > 0 && string(envelope.ID) != "null" {
			return s.reply(envelope.ID, nil)
		}
		return nil
	}
}

func (s *server) completions(text string, line, character int) []map[string]any {
	// package member after "pkg." or enum variants after "Status."
	if prefix := nameBeforeDot(text, line, character); prefix != "" {
		if stdlib.IsPackage(prefix) {
			var items []map[string]any
			for _, m := range stdlib.PackageMembers(prefix) {
				item := map[string]any{
					"label":  m,
					"kind":   2, // Method
					"detail": prefix + "." + m,
				}
				if h, ok := lookupMember(prefix, m); ok {
					item["detail"] = h.Sig
					item["documentation"] = map[string]any{
						"kind":  "markdown",
						"value": "**" + h.Sig + "**\n\n" + h.Detail,
					}
				}
				items = append(items, item)
			}
			if len(items) > 0 {
				return items
			}
		}
		if variants := enumVariantsIn(text, prefix); len(variants) > 0 {
			var items []map[string]any
			for _, v := range variants {
				items = append(items, map[string]any{
					"label":  v,
					"kind":   20, // EnumMember
					"detail": prefix + "." + v,
				})
			}
			return items
		}
	}
	var items []map[string]any
	// keywords / control
	for _, n := range []string{
		"fn", "mut", "use", "import", "pub", "type", "const", "enum",
		"match", "if", "else", "while", "for", "in", "return", "defer", "as",
	} {
		items = append(items, map[string]any{"label": n, "kind": 14, "detail": "keyword"}) // Keyword
	}
	// concurrent + pipeline globals
	for _, n := range []string{
		"map", "seq_map", "filter", "seq_filter", "reduce", "each", "par_map",
		"spawn", "parallel", "gather", "race", "timeout", "group",
		"channel", "send", "recv", "close", "select_recv", "try_recv",
		"say", "println", "Ok", "Err", "len", "range", "push",
		"ensure", "bail",
	} {
		items = append(items, map[string]any{"label": n, "kind": 3, "detail": "prelude"})
	}
	// local functions / enums from open buffer
	for _, name := range localFnNames(text) {
		items = append(items, map[string]any{"label": name, "kind": 3, "detail": "local"})
	}
	for _, name := range localEnumNames(text) {
		items = append(items, map[string]any{"label": name, "kind": 13, "detail": "enum"}) // Enum
	}
	for _, n := range stdlib.Names() {
		items = append(items, map[string]any{"label": n, "kind": 9, "detail": "stdlib package"})
	}
	return items
}

func (s *server) formatDocument(uri string) any {
	text := s.docs[uri]
	if text == "" {
		return []any{}
	}
	path := uriToPath(uri)
	formatted, err := format.Source(path, text)
	if err != nil {
		return []any{} // keep buffer if unparseable
	}
	if !strings.HasSuffix(formatted, "\n") && formatted != "" {
		formatted += "\n"
	}
	if formatted == text {
		return []any{}
	}
	lines := strings.Split(text, "\n")
	endLine := len(lines) - 1
	if endLine < 0 {
		endLine = 0
	}
	endChar := 0
	if endLine < len(lines) {
		endChar = len(lines[endLine])
	}
	return []map[string]any{{
		"range": map[string]any{
			"start": map[string]int{"line": 0, "character": 0},
			"end":   map[string]int{"line": endLine, "character": endChar},
		},
		"newText": formatted,
	}}
}

func (s *server) definition(uri string, line, character int) any {
	text := s.docs[uri]
	word := wordAt(text, line, character)
	if word == "" {
		return nil
	}
	path := uriToPath(uri)
	file, errs := parse.ParseFile(path, text)
	if errs.HasErrors() || file == nil {
		// fallback: scan for fn / enum name
		if loc := scanFnLine(text, word); loc >= 0 {
			return location(uri, loc, 0)
		}
		if loc := scanEnumLine(text, word); loc >= 0 {
			return location(uri, loc, 0)
		}
		return nil
	}
	for _, d := range file.Decls {
		switch n := d.(type) {
		case *ast.FnDecl:
			if n.Name == word {
				ln := n.Pos().Line
				if ln < 1 {
					ln = 1
				}
				return location(uri, ln-1, 0)
			}
		case *ast.EnumDecl:
			if n.Name == word {
				ln := n.Pos().Line
				if ln < 1 {
					ln = 1
				}
				return location(uri, ln-1, 0)
			}
		case *ast.TypeDecl:
			if n.Name == word {
				ln := n.Pos().Line
				if ln < 1 {
					ln = 1
				}
				return location(uri, ln-1, 0)
			}
		}
	}
	return nil
}

func (s *server) documentSymbols(uri string) []map[string]any {
	text := s.docs[uri]
	path := uriToPath(uri)
	file, errs := parse.ParseFile(path, text)
	var out []map[string]any
	if errs.HasErrors() || file == nil {
		for _, name := range localFnNames(text) {
			out = append(out, map[string]any{
				"name": name,
				"kind": 12, // Function
				"range": map[string]any{
					"start": map[string]int{"line": 0, "character": 0},
					"end":   map[string]int{"line": 0, "character": 0},
				},
				"selectionRange": map[string]any{
					"start": map[string]int{"line": 0, "character": 0},
					"end":   map[string]int{"line": 0, "character": 0},
				},
			})
		}
		return out
	}
	for _, d := range file.Decls {
		switch n := d.(type) {
		case *ast.FnDecl:
			ln := n.Pos().Line
			if ln < 1 {
				ln = 1
			}
			r := map[string]any{
				"start": map[string]int{"line": ln - 1, "character": 0},
				"end":   map[string]int{"line": ln - 1, "character": len(n.Name) + 3},
			}
			out = append(out, map[string]any{
				"name":           n.Name,
				"kind":           12,
				"range":          r,
				"selectionRange": r,
				"detail":         "fn",
			})
		case *ast.TypeDecl:
			ln := n.Pos().Line
			if ln < 1 {
				ln = 1
			}
			r := map[string]any{
				"start": map[string]int{"line": ln - 1, "character": 0},
				"end":   map[string]int{"line": ln - 1, "character": len(n.Name) + 5},
			}
			out = append(out, map[string]any{
				"name":           n.Name,
				"kind":           5, // Class
				"range":          r,
				"selectionRange": r,
				"detail":         "type",
			})
		case *ast.EnumDecl:
			ln := n.Pos().Line
			if ln < 1 {
				ln = 1
			}
			r := map[string]any{
				"start": map[string]int{"line": ln - 1, "character": 0},
				"end":   map[string]int{"line": ln - 1, "character": len(n.Name) + 5},
			}
			out = append(out, map[string]any{
				"name":           n.Name,
				"kind":           10, // Enum
				"range":          r,
				"selectionRange": r,
				"detail":         "enum",
			})
		}
	}
	return out
}

func location(uri string, line, character int) map[string]any {
	return map[string]any{
		"uri": uri,
		"range": map[string]any{
			"start": map[string]int{"line": line, "character": character},
			"end":   map[string]int{"line": line, "character": character},
		},
	}
}

func hoverFor(word string) any {
	if word == "" {
		return nil
	}
	if stdlib.IsPackage(word) {
		members := stdlib.PackageMembers(word)
		preview := strings.Join(members, ", ")
		if len(preview) > 120 {
			preview = preview[:117] + "…"
		}
		val := fmt.Sprintf("**%s** — Weft stdlib package (`use %s`)", word, word)
		if preview != "" {
			val += "\n\nMembers: `" + preview + "`"
		}
		return map[string]any{
			"contents": map[string]any{"kind": "markdown", "value": val},
		}
	}
	if h, ok := lookupCall(word); ok {
		return map[string]any{
			"contents": map[string]any{
				"kind":  "markdown",
				"value": fmt.Sprintf("**%s**\n\n%s", h.Sig, h.Detail),
			},
		}
	}
	hints := map[string]string{
		"enum":  "enum Name { A, B } — string-tagged variants (Name.A == \"A\")",
		"match": "match x { lit { … } Name.Var { … } _ { … } } — first arm wins",
		"fn":    "fn name(args) { … } or fn() { … } (closure, by-value capture)",
		"defer": "defer expr — runs when the surrounding function returns",
		"use":   "use pkg / use \"./file.weft\" as Alias",
	}
	if h, ok := hints[word]; ok {
		return map[string]any{"contents": map[string]any{"kind": "markdown", "value": h}}
	}
	return nil
}

func hoverForMember(pkg, mem string) any {
	if mem == "" {
		return hoverFor(pkg)
	}
	if !stdlib.IsPackage(pkg) {
		return nil
	}
	if h, ok := lookupMember(pkg, mem); ok {
		return map[string]any{
			"contents": map[string]any{
				"kind":  "markdown",
				"value": fmt.Sprintf("**%s**\n\n%s", h.Sig, h.Detail),
			},
		}
	}
	// confirm member exists
	found := false
	for _, m := range stdlib.PackageMembers(pkg) {
		if m == mem {
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	return map[string]any{
		"contents": map[string]any{
			"kind":  "markdown",
			"value": fmt.Sprintf("**%s.%s** — stdlib member (`weft stdlib %s`)", pkg, mem, pkg),
		},
	}
}

func signatureHelp(text string, line, character int) any {
	// find call name before '('
	name := callNameAt(text, line, character)
	if name == "" {
		return nil
	}
	label := ""
	if h, ok := lookupCall(name); ok {
		label = h.Sig
	} else if pkg, mem, ok := strings.Cut(name, "."); ok && stdlib.IsPackage(pkg) {
		if h, ok := lookupMember(pkg, mem); ok {
			label = h.Sig
		} else {
			label = pkg + "." + mem + "(…)"
		}
	} else {
		return nil
	}
	// activeParameter ≈ commas inside the open call (not nested)
	active := activeParamIndex(text, line, character)
	return map[string]any{
		"signatures": []map[string]any{{
			"label": label,
			"documentation": map[string]any{
				"kind":  "markdown",
				"value": label,
			},
		}},
		"activeSignature": 0,
		"activeParameter": active,
	}
}

// activeParamIndex counts top-level commas between the last '(' and the cursor.
func activeParamIndex(text string, line, character int) int {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return 0
	}
	s := lines[line]
	if character > len(s) {
		character = len(s)
	}
	open := strings.LastIndex(s[:character], "(")
	if open < 0 {
		return 0
	}
	depth := 0
	commas := 0
	for i := open; i < character; i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 1 {
				commas++
			}
		}
	}
	return commas
}

func (s *server) publishDiags(uri, text string) {
	path := uriToPath(uri)
	diags := []map[string]any{}
	file, perrs := parse.ParseFile(path, text)
	if perrs.HasErrors() {
		for _, e := range perrs {
			diags = append(diags, diagItem(e.Pos.Line, e.Pos.Column, e.Message))
		}
	} else {
		if _, cerrs := types.Infer(file); cerrs.HasErrors() {
			for _, e := range cerrs {
				diags = append(diags, diagItem(e.Pos.Line, e.Pos.Column, e.Message))
			}
		}
	}
	_ = s.notify("textDocument/publishDiagnostics", map[string]any{
		"uri":         uri,
		"diagnostics": diags,
	})
}

func diagItem(line, col int, msg string) map[string]any {
	if line < 1 {
		line = 1
	}
	if col < 1 {
		col = 1
	}
	return map[string]any{
		"range": map[string]any{
			"start": map[string]int{"line": line - 1, "character": col - 1},
			"end":   map[string]int{"line": line - 1, "character": col},
		},
		"severity": 1,
		"source":   "weft",
		"message":  msg,
	}
}

func (s *server) reply(id json.RawMessage, result any) error {
	msg := map[string]any{"jsonrpc": "2.0", "id": rawOrNull(id), "result": result}
	return writeMessage(s.w, msg)
}

func (s *server) notify(method string, params any) error {
	msg := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	return writeMessage(s.w, msg)
}

func rawOrNull(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(id, &v); err != nil {
		return nil
	}
	return v
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	contentLen := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &contentLen)
			}
		}
	}
	if contentLen <= 0 {
		return nil, fmt.Errorf("missing content-length")
	}
	buf := make([]byte, contentLen)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func writeMessage(w io.Writer, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

func uriToPath(uri string) string {
	uri = strings.TrimPrefix(uri, "file://")
	return uri
}

func wordAt(text string, line, character int) string {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	s := lines[line]
	if character > len(s) {
		character = len(s)
	}
	i := character
	for i > 0 && isIdentByte(s[i-1]) {
		i--
	}
	j := character
	for j < len(s) && isIdentByte(s[j]) {
		j++
	}
	if i >= j {
		return ""
	}
	return s[i:j]
}

func isIdentByte(c byte) bool {
	return unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c)) || c == '_'
}

// packagePrefix returns "http" when cursor is after "http." (member completion).
func packagePrefix(text string, line, character int) string {
	pkg := nameBeforeDot(text, line, character)
	if stdlib.IsPackage(pkg) {
		return pkg
	}
	return ""
}

// nameBeforeDot returns the identifier immediately before '.' at the cursor
// (stdlib package or local enum name).
func nameBeforeDot(text string, line, character int) string {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	s := lines[line]
	if character > len(s) {
		character = len(s)
	}
	// walk back over partial member then require '.'
	i := character
	for i > 0 && isIdentByte(s[i-1]) {
		i--
	}
	if i == 0 || s[i-1] != '.' {
		return ""
	}
	j := i - 1
	for j > 0 && isIdentByte(s[j-1]) {
		j--
	}
	return s[j : i-1]
}

func enumVariantsIn(text, name string) []string {
	if name == "" {
		return nil
	}
	file, errs := parse.ParseFile("buf.weft", text)
	if !errs.HasErrors() && file != nil {
		for _, d := range file.Decls {
			if e, ok := d.(*ast.EnumDecl); ok && e.Name == name {
				return append([]string{}, e.Variants...)
			}
		}
	}
	// Fallback for incomplete buffers (e.g. "Status." mid-edit).
	return scanEnumVariants(text, name)
}

func localEnumNames(text string) []string {
	file, errs := parse.ParseFile("buf.weft", text)
	if !errs.HasErrors() && file != nil {
		var out []string
		for _, d := range file.Decls {
			if e, ok := d.(*ast.EnumDecl); ok {
				out = append(out, e.Name)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return scanEnumNames(text)
}

// scanEnumVariants finds `enum Name { A, B }` even when the rest of the file doesn't parse.
func scanEnumVariants(text, name string) []string {
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		// optional pub
		line = strings.TrimPrefix(line, "pub ")
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "enum ") {
			continue
		}
		rest := strings.TrimSpace(line[len("enum "):])
		// Name { … } or Name alone with brace on next lines
		ename := rest
		body := ""
		if j := strings.Index(rest, "{"); j >= 0 {
			ename = strings.TrimSpace(rest[:j])
			body = rest[j+1:]
			if k := strings.Index(body, "}"); k >= 0 {
				body = body[:k]
			} else {
				// multi-line variants
				var parts []string
				parts = append(parts, body)
				for i+1 < len(lines) {
					i++
					l := lines[i]
					if k := strings.Index(l, "}"); k >= 0 {
						parts = append(parts, l[:k])
						break
					}
					parts = append(parts, l)
				}
				body = strings.Join(parts, " ")
			}
		} else {
			// enum Name\n{
			ename = strings.TrimSpace(rest)
			// find brace
			for i+1 < len(lines) {
				i++
				l := strings.TrimSpace(lines[i])
				if strings.HasPrefix(l, "{") {
					body = l[1:]
					if k := strings.Index(body, "}"); k >= 0 {
						body = body[:k]
					} else {
						var parts []string
						parts = append(parts, body)
						for i+1 < len(lines) {
							i++
							ll := lines[i]
							if k := strings.Index(ll, "}"); k >= 0 {
								parts = append(parts, ll[:k])
								break
							}
							parts = append(parts, ll)
						}
						body = strings.Join(parts, " ")
					}
					break
				}
			}
		}
		if ename != name {
			continue
		}
		var vars []string
		for _, p := range strings.Split(body, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			// first ident token
			id := ""
			for _, r := range p {
				if unicode.IsLetter(r) || r == '_' || (id != "" && unicode.IsDigit(r)) {
					id += string(r)
				} else if id != "" {
					break
				}
			}
			if id != "" {
				vars = append(vars, id)
			}
		}
		return vars
	}
	return nil
}

func scanEnumNames(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		l := strings.TrimSpace(line)
		l = strings.TrimPrefix(l, "pub ")
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "enum ") {
			continue
		}
		rest := strings.TrimSpace(l[len("enum "):])
		name := rest
		if j := strings.IndexAny(rest, "{ \t"); j >= 0 {
			name = strings.TrimSpace(rest[:j])
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func scanEnumLine(text, name string) int {
	for i, line := range strings.Split(text, "\n") {
		l := strings.TrimSpace(line)
		l = strings.TrimPrefix(l, "pub ")
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "enum ") {
			continue
		}
		rest := strings.TrimSpace(l[len("enum "):])
		ename := rest
		if j := strings.IndexAny(rest, "{ \t"); j >= 0 {
			ename = strings.TrimSpace(rest[:j])
		}
		if ename == name {
			return i
		}
	}
	return -1
}

func hoverEnumVariant(text, enumName, variant string) any {
	vars := enumVariantsIn(text, enumName)
	if len(vars) == 0 {
		return nil
	}
	if variant != "" {
		found := false
		for _, v := range vars {
			if v == variant {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
		return map[string]any{
			"contents": map[string]any{
				"kind":  "markdown",
				"value": fmt.Sprintf("**%s.%s** — enum variant (string tag `%s`)", enumName, variant, variant),
			},
		}
	}
	return nil
}

func hoverLocalEnum(text, name string) any {
	vars := enumVariantsIn(text, name)
	if len(vars) == 0 {
		return nil
	}
	return map[string]any{
		"contents": map[string]any{
			"kind":  "markdown",
			"value": fmt.Sprintf("**%s** — enum\n\nVariants: `%s`", name, strings.Join(vars, "`, `")),
		},
	}
}

// dottedAt returns package and member under cursor (http.get).
func dottedAt(text string, line, character int) (pkg, mem string) {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return "", ""
	}
	s := lines[line]
	if character > len(s) {
		character = len(s)
	}
	// expand full dotted ident
	i := character
	for i > 0 && (isIdentByte(s[i-1]) || s[i-1] == '.') {
		i--
	}
	j := character
	for j < len(s) && (isIdentByte(s[j]) || s[j] == '.') {
		j++
	}
	tok := s[i:j]
	if !strings.Contains(tok, ".") {
		return "", ""
	}
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func callNameAt(text string, line, character int) string {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	s := lines[line]
	if character > len(s) {
		character = len(s)
	}
	// find last '(' before cursor
	open := strings.LastIndex(s[:character], "(")
	if open < 0 {
		return ""
	}
	// walk back name / dotted name
	i := open
	for i > 0 && (isIdentByte(s[i-1]) || s[i-1] == '.') {
		i--
	}
	return s[i:open]
}

func localFnNames(text string) []string {
	var names []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "fn ") && !strings.HasPrefix(line, "pub fn ") {
			continue
		}
		rest := strings.TrimPrefix(line, "pub ")
		rest = strings.TrimPrefix(rest, "fn ")
		name := ""
		for _, c := range rest {
			if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' {
				name += string(c)
			} else {
				break
			}
		}
		if name != "" && name != "main" {
			names = append(names, name)
		}
	}
	return names
}

func scanFnLine(text, name string) int {
	for i, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "fn "+name) || strings.HasPrefix(trim, "pub fn "+name) {
			return i
		}
	}
	return -1
}

// StartStdio is the CLI entry.
func StartStdio() error {
	return Run(os.Stdin, os.Stdout)
}
