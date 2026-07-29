package lsp

import (
	"fmt"
	"strings"
	"unicode"
)

// codeActions returns refactor actions for the selection (extract function).
func (s *server) codeActions(uri string, startLine, startChar, endLine, endChar int) any {
	text := s.docs[uri]
	if text == "" {
		return []any{}
	}
	// Require a non-empty selection spanning at least one non-whitespace char.
	sel := selectionText(text, startLine, startChar, endLine, endChar)
	if strings.TrimSpace(sel) == "" {
		return []any{}
	}
	// Don't offer on pure whitespace or single-token one-liners that are just an identifier.
	if startLine == endLine && !strings.Contains(sel, "\n") && !strings.ContainsAny(sel, " \t(){}:=+") {
		// still allow multi-token expressions on one line
		if len(strings.Fields(sel)) < 2 && !strings.Contains(sel, "(") {
			return []any{}
		}
	}
	edit := extractFunctionEdit(uri, text, startLine, startChar, endLine, endChar, sel)
	if edit == nil {
		return []any{}
	}
	return []map[string]any{{
		"title":       "Extract function",
		"kind":        "refactor.extract.function",
		"edit":        edit,
		"isPreferred": true,
	}}
}

func selectionText(text string, startLine, startChar, endLine, endChar int) string {
	lines := strings.Split(text, "\n")
	if startLine < 0 || endLine >= len(lines) || startLine > endLine {
		return ""
	}
	if startLine == endLine {
		ln := lines[startLine]
		if startChar > len(ln) {
			startChar = len(ln)
		}
		if endChar > len(ln) {
			endChar = len(ln)
		}
		if startChar > endChar {
			return ""
		}
		return ln[startChar:endChar]
	}
	var b strings.Builder
	// first line
	ln := lines[startLine]
	if startChar > len(ln) {
		startChar = len(ln)
	}
	b.WriteString(ln[startChar:])
	b.WriteByte('\n')
	for i := startLine + 1; i < endLine; i++ {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	ln = lines[endLine]
	if endChar > len(ln) {
		endChar = len(ln)
	}
	b.WriteString(ln[:endChar])
	return b.String()
}

// extractFunctionEdit builds a WorkspaceEdit that:
//  1. inserts `fn extracted_N(params) { … }` before the enclosing top-level fn (or file start)
//  2. replaces the selection with `extracted_N(args)` (or statement form)
func extractFunctionEdit(uri, text string, startLine, startChar, endLine, endChar int, sel string) any {
	body := strings.TrimRight(sel, "\n")
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}

	// Free variables: simple scan of identifiers not bound inside the selection.
	params := freeVars(body)
	name := nextExtractName(text)

	// Indent body with 4 spaces per line
	var bodyIndented strings.Builder
	for _, ln := range strings.Split(body, "\n") {
		bodyIndented.WriteString("    ")
		bodyIndented.WriteString(strings.TrimRight(ln, "\r"))
		bodyIndented.WriteByte('\n')
	}

	paramList := strings.Join(params, ", ")
	fnText := fmt.Sprintf("fn %s(%s) {\n%s}\n\n", name, paramList, bodyIndented.String())

	// Insert before the first top-level fn that contains startLine, else at file start.
	insertLine := insertLineForExtract(text, startLine)

	// Replacement: if selection looks like a statement block, call as statement;
	// if expression-like, use as expression (call returns value).
	call := name + "(" + paramList + ")"
	isExpr := looksLikeExpr(body)
	replaceText := call
	if !isExpr {
		// statement: keep as bare call
		replaceText = call
	}

	// Full-file rewrite is simpler and avoids offset math bugs for multi-line inserts.
	newText := applyExtract(text, startLine, startChar, endLine, endChar, insertLine, fnText, replaceText)
	if newText == text {
		return nil
	}
	lines := strings.Split(text, "\n")
	endL := len(lines) - 1
	if endL < 0 {
		endL = 0
	}
	endC := 0
	if endL < len(lines) {
		endC = len(lines[endL])
	}
	return map[string]any{
		"changes": map[string]any{
			uri: []map[string]any{{
				"range": map[string]any{
					"start": map[string]int{"line": 0, "character": 0},
					"end":   map[string]int{"line": endL, "character": endC},
				},
				"newText": newText,
			}},
		},
	}
}

func applyExtract(text string, startLine, startChar, endLine, endChar, insertLine int, fnText, replaceText string) string {
	lines := strings.Split(text, "\n")
	// Replace selection first (working on a copy of lines).
	if startLine < 0 || endLine >= len(lines) {
		return text
	}
	if startLine == endLine {
		ln := lines[startLine]
		if startChar > len(ln) {
			startChar = len(ln)
		}
		if endChar > len(ln) {
			endChar = len(ln)
		}
		lines[startLine] = ln[:startChar] + replaceText + ln[endChar:]
	} else {
		first := lines[startLine]
		if startChar > len(first) {
			startChar = len(first)
		}
		last := lines[endLine]
		if endChar > len(last) {
			endChar = len(last)
		}
		merged := first[:startChar] + replaceText + last[endChar:]
		// collapse middle lines
		newLines := append([]string{}, lines[:startLine]...)
		newLines = append(newLines, merged)
		newLines = append(newLines, lines[endLine+1:]...)
		lines = newLines
		// insertLine may shift if insert was after selection — handle below carefully
		if insertLine > endLine {
			insertLine -= endLine - startLine
		}
	}
	// Insert function text at insertLine
	if insertLine < 0 {
		insertLine = 0
	}
	if insertLine > len(lines) {
		insertLine = len(lines)
	}
	fnLines := strings.Split(strings.TrimSuffix(fnText, "\n"), "\n")
	// ensure blank line after if needed
	out := append([]string{}, lines[:insertLine]...)
	out = append(out, fnLines...)
	out = append(out, lines[insertLine:]...)
	return strings.Join(out, "\n")
}

func insertLineForExtract(text string, selStart int) int {
	// Place new fn just before the top-level `fn` whose body contains selStart.
	lines := strings.Split(text, "\n")
	lastFnLine := 0
	for i, ln := range lines {
		trim := strings.TrimSpace(ln)
		if strings.HasPrefix(trim, "fn ") {
			if i <= selStart {
				lastFnLine = i
			}
		}
	}
	return lastFnLine
}

func nextExtractName(text string) string {
	for i := 1; i < 1000; i++ {
		name := fmt.Sprintf("extracted_%d", i)
		if !strings.Contains(text, "fn "+name) && !strings.Contains(text, name+"(") {
			return name
		}
	}
	return "extracted"
}

func looksLikeExpr(body string) bool {
	t := strings.TrimSpace(body)
	if strings.Contains(t, "\n") {
		return false
	}
	if strings.HasPrefix(t, "return ") || strings.HasPrefix(t, "say ") ||
		strings.HasPrefix(t, "mut ") || strings.HasPrefix(t, "let ") ||
		strings.Contains(t, ":=") {
		return false
	}
	return true
}

// freeVars returns identifier names used but not assigned in body (rough).
func freeVars(body string) []string {
	assigned := map[string]bool{}
	used := map[string]bool{}
	// assignment / bind: name :=  or mut name :=
	for _, ln := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(ln)
		if strings.HasPrefix(trim, "mut ") {
			trim = strings.TrimSpace(strings.TrimPrefix(trim, "mut "))
		}
		if i := strings.Index(trim, ":="); i > 0 {
			name := strings.TrimSpace(trim[:i])
			if j := strings.Index(name, ":"); j > 0 {
				name = strings.TrimSpace(name[:j])
			}
			if isIdent(name) {
				assigned[name] = true
			}
		}
	}
	// scan idents
	for _, id := range scanIdents(body) {
		if isKeyword(id) || stdlibPackage(id) {
			continue
		}
		if assigned[id] {
			continue
		}
		used[id] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, id := range scanIdents(body) {
		if !used[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func scanIdents(s string) []string {
	var out []string
	i := 0
	for i < len(s) {
		r := rune(s[i])
		if unicode.IsLetter(r) || r == '_' {
			j := i + 1
			for j < len(s) {
				rj := rune(s[j])
				if unicode.IsLetter(rj) || unicode.IsDigit(rj) || rj == '_' {
					j++
					continue
				}
				break
			}
			out = append(out, s[i:j])
			i = j
			continue
		}
		// skip strings
		if s[i] == '"' {
			i++
			for i < len(s) {
				if s[i] == '\\' {
					i += 2
					continue
				}
				if s[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		}
		i++
	}
	return out
}

func stdlibPackage(name string) bool {
	// avoid import cycle with stdlib in tests — use local keyword-like filter
	switch name {
	case "http", "json", "fs", "str", "math", "time", "env", "cli", "llm", "web",
		"yaml", "toml", "csv", "db", "log", "re", "os", "path", "crypto", "base64",
		"hex", "uuid", "sha", "md5", "test", "fmt", "io", "net", "url", "regex":
		return true
	}
	return false
}
