package lsp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/stdlib"
)

// prepareRename returns the range of the identifier at the cursor, or null if not renameable.
func prepareRename(text string, line, character int) any {
	word := wordAt(text, line, character)
	if word == "" {
		return nil
	}
	// Don't rename keywords or stdlib packages
	if isKeyword(word) {
		return nil
	}
	lines := strings.Split(text, "\n")
	if line >= len(lines) {
		return nil
	}
	col := character
	ln := lines[line]
	// Find word boundaries
	start := col
	for start > 0 && isIdentChar(ln[start-1]) {
		start--
	}
	end := col
	for end < len(ln) && isIdentChar(ln[end]) {
		end++
	}
	return map[string]any{
		"range": makeRange(line, start, line, end),
	}
}

// rename finds occurrences of the identifier at cursor and returns a WorkspaceEdit.
// Top-level fn/type/enum/const renames apply across open buffers and (when rootURI
// is set) on-disk .weft files under the workspace. Locals stay single-file.
func rename(text, uri string, line, character int, newName string) any {
	return renameInDocs(map[string]string{uri: text}, "", uri, line, character, newName)
}

// renameInDocs is the multi-document rename implementation.
func renameInDocs(docs map[string]string, rootURI, uri string, line, character int, newName string) any {
	text := docs[uri]
	oldName := wordAt(text, line, character)
	if oldName == "" || isKeyword(oldName) {
		return nil
	}
	if newName == "" || newName == oldName {
		return nil
	}
	// Never rename stdlib package names workspace-wide.
	if stdlib.IsPackage(oldName) {
		return nil
	}

	topLevel := isTopLevelBinding(text, oldName)
	changes := map[string]any{}

	// Always rename in the origin file.
	if edits := renameInText(text, oldName, newName); len(edits) > 0 {
		changes[uri] = edits
	}

	if topLevel {
		// Other open buffers
		for u, t := range docs {
			if u == uri {
				continue
			}
			if edits := renameInText(t, oldName, newName); len(edits) > 0 {
				changes[u] = edits
			}
		}
		// On-disk workspace .weft files (skip already-open URIs)
		if rootURI != "" {
			root := uriToPath(rootURI)
			_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					if d != nil && (d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules") {
						return filepath.SkipDir
					}
					return nil
				}
				if !strings.HasSuffix(path, ".weft") && !strings.HasSuffix(path, ".loom") {
					return nil
				}
				u := pathToURI(path)
				if _, open := docs[u]; open {
					return nil
				}
				// also match without double-slash quirks
				for ou := range docs {
					if uriToPath(ou) == path {
						return nil
					}
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				t := string(data)
				if !strings.Contains(t, oldName) {
					return nil
				}
				if edits := renameInText(t, oldName, newName); len(edits) > 0 {
					changes[u] = edits
				}
				return nil
			})
		}
	}

	if len(changes) == 0 {
		return nil
	}
	return map[string]any{"changes": changes}
}

func (s *server) rename(uri string, line, character int, newName string) any {
	return renameInDocs(s.docs, s.rootURI, uri, line, character, newName)
}

func isTopLevelBinding(text, name string) bool {
	b, ok := findBindingDefinition(text, name)
	if !ok {
		// Fallback: top-level fn/type/enum/const line scans
		return scanFnLine(text, name) >= 0 || scanEnumLine(text, name) >= 0 ||
			strings.Contains(text, "type "+name) || strings.Contains(text, "const "+name+" ")
	}
	switch b.Kind {
	case "fn", "type", "enum", "const":
		return true
	default:
		return false
	}
}

func renameInText(text, oldName, newName string) []map[string]any {
	file, errs := parse.ParseFile("<rename>", text)
	var edits []map[string]any
	if !errs.HasErrors() && file != nil {
		collectIdents(file, oldName, func(pos int, endPos int, line int) {
			lines := strings.Split(text, "\n")
			if line-1 >= len(lines) || line < 1 {
				return
			}
			ln := lines[line-1]
			col := findColInLine(ln, oldName, pos-lineOffset(text, line))
			if col < 0 {
				return
			}
			edits = append(edits, map[string]any{
				"range":   makeRange(line-1, col, line-1, col+len(oldName)),
				"newText": newName,
			})
		})
	}
	if len(edits) == 0 {
		edits = findAllOccurrences(text, oldName, newName)
	}
	return edits
}

// findAllOccurrences does text-level rename of identifier occurrences.
func findAllOccurrences(text, oldName, newName string) []map[string]any {
	lines := strings.Split(text, "\n")
	var edits []map[string]any
	for lineIdx, ln := range lines {
		col := 0
		for col < len(ln) {
			idx := strings.Index(ln[col:], oldName)
			if idx < 0 {
				break
			}
			start := col + idx
			end := start + len(oldName)
			// Check word boundaries
			if start > 0 && isIdentChar(ln[start-1]) {
				col = end
				continue
			}
			if end < len(ln) && isIdentChar(ln[end]) {
				col = end
				continue
			}
			// Skip occurrences inside strings
			if inString(ln, start) {
				col = end
				continue
			}
			edits = append(edits, map[string]any{
				"range":   makeRange(lineIdx, start, lineIdx, end),
				"newText": newName,
			})
			col = end
		}
	}
	return edits
}

func inString(line string, pos int) bool {
	inStr := false
	esc := false
	for i := 0; i < pos && i < len(line); i++ {
		if esc {
			esc = false
			continue
		}
		if line[i] == '\\' {
			esc = true
			continue
		}
		if line[i] == '"' {
			inStr = !inStr
		}
	}
	return inStr
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func isKeyword(w string) bool {
	switch w {
	case "fn", "let", "mut", "if", "else", "for", "in", "while", "return",
		"break", "continue", "type", "struct", "import", "use", "as",
		"true", "false", "null", "pub", "defer", "say", "match", "enum",
		"const":
		return true
	}
	return false
}

func makeRange(startLine, startChar, endLine, endChar int) map[string]any {
	return map[string]any{
		"start": map[string]any{"line": startLine, "character": startChar},
		"end":   map[string]any{"line": endLine, "character": endChar},
	}
}

// collectIdents walks the AST finding Ident nodes with the given name.
// callback receives (byte offset, end offset, 1-based line).
func collectIdents(file *ast.File, name string, cb func(pos, end, line int)) {
	for _, d := range file.Decls {
		walkDeclIdents(d, name, cb)
	}
}

func walkDeclIdents(d ast.Decl, name string, cb func(pos, end, line int)) {
	switch d := d.(type) {
	case *ast.FnDecl:
		if d.Name == name {
			cb(d.Pos_.Offset, d.Pos_.Offset+len(name), d.Pos_.Line)
		}
		for _, p := range d.Params {
			if p.Name == name {
				cb(p.Pos_.Offset, p.Pos_.Offset+len(name), p.Pos_.Line)
			}
		}
		if d.Body != nil {
			walkBlockIdents(d.Body, name, cb)
		}
	}
}

func walkBlockIdents(b *ast.Block, name string, cb func(pos, end, line int)) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		walkStmtIdents(s, name, cb)
	}
}

func walkStmtIdents(s ast.Stmt, name string, cb func(pos, end, line int)) {
	switch s := s.(type) {
	case *ast.LetStmt:
		if s.Name == name {
			cb(s.Pos_.Offset, s.Pos_.Offset+len(name), s.Pos_.Line)
		}
		walkExprIdents(s.Init, name, cb)
	case *ast.ExprStmt:
		walkExprIdents(s.X, name, cb)
	case *ast.ReturnStmt:
		if s.Value != nil {
			walkExprIdents(s.Value, name, cb)
		}
	case *ast.AssignStmt:
		walkExprIdents(s.Target, name, cb)
		walkExprIdents(s.Value, name, cb)
	case *ast.IfStmt:
		walkExprIdents(s.Cond, name, cb)
		walkBlockIdents(s.Then, name, cb)
		if s.Else != nil {
			if b, ok := s.Else.(*ast.Block); ok {
				walkBlockIdents(b, name, cb)
			}
			if ifs, ok := s.Else.(*ast.IfStmt); ok {
				walkStmtIdents(ifs, name, cb)
			}
		}
	case *ast.WhileStmt:
		walkExprIdents(s.Cond, name, cb)
		walkBlockIdents(s.Body, name, cb)
	case *ast.ForStmt:
		if s.Name == name {
			cb(s.Pos_.Offset, s.Pos_.Offset+len(name), s.Pos_.Line)
		}
		walkExprIdents(s.Iter, name, cb)
		walkBlockIdents(s.Body, name, cb)
	case *ast.Block:
		walkBlockIdents(s, name, cb)
	case *ast.DeferStmt:
		walkExprIdents(s.Call, name, cb)
	}
}

func walkExprIdents(e ast.Expr, name string, cb func(pos, end, line int)) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *ast.Ident:
		if e.Name == name {
			cb(e.Pos_.Offset, e.Pos_.Offset+len(name), e.Pos_.Line)
		}
	case *ast.BinaryExpr:
		walkExprIdents(e.X, name, cb)
		walkExprIdents(e.Y, name, cb)
	case *ast.UnaryExpr:
		walkExprIdents(e.X, name, cb)
	case *ast.CallExpr:
		walkExprIdents(e.Fun, name, cb)
		for _, a := range e.Args {
			walkExprIdents(a, name, cb)
		}
	case *ast.IndexExpr:
		walkExprIdents(e.X, name, cb)
		walkExprIdents(e.Index, name, cb)
	case *ast.FieldExpr:
		walkExprIdents(e.X, name, cb)
	case *ast.QuestionExpr:
		walkExprIdents(e.X, name, cb)
	case *ast.ListLit:
		for _, el := range e.Elts {
			walkExprIdents(el, name, cb)
		}
	case *ast.MapLit:
		for i := range e.Keys {
			walkExprIdents(e.Keys[i], name, cb)
			walkExprIdents(e.Vals[i], name, cb)
		}
	case *ast.FuncLit:
		for _, p := range e.Params {
			if p.Name == name {
				cb(p.Pos_.Offset, p.Pos_.Offset+len(name), p.Pos_.Line)
			}
		}
		walkBlockIdents(e.Body, name, cb)
	case *ast.IfExpr:
		walkExprIdents(e.Cond, name, cb)
		walkBlockIdents(e.Then, name, cb)
	case *ast.MatchExpr:
		walkExprIdents(e.Scrutinee, name, cb)
		for _, arm := range e.Arms {
			walkExprIdents(arm.Pattern, name, cb)
			walkBlockIdents(arm.Body, name, cb)
		}
	case *ast.FStringExpr:
		for _, p := range e.Parts {
			if p.Expr != nil {
				walkExprIdents(p.Expr, name, cb)
			}
		}
	}
}

func lineOffset(text string, oneBased int) int {
	line := 1
	for i := 0; i < len(text); i++ {
		if line == oneBased {
			return i
		}
		if text[i] == '\n' {
			line++
		}
	}
	return len(text)
}

func findColInLine(ln, name string, byteOff int) int {
	idx := strings.Index(ln[byteOff:], name)
	if idx < 0 {
		// try from start
		idx = strings.Index(ln, name)
		if idx < 0 {
			return -1
		}
		return idx
	}
	return byteOff + idx
}
