package lsp

import (
	"strings"

	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/token"
)

// bindingDef is a named binding (fn, param, let, for-var, const, type, enum).
type bindingDef struct {
	Name string
	Pos  token.Pos
	Kind string // "fn", "param", "let", "for", "const", "type", "enum"
}

// findBindingDefinition returns the first declaration of name in the buffer.
func findBindingDefinition(text, name string) (bindingDef, bool) {
	if name == "" || isKeyword(name) {
		return bindingDef{}, false
	}
	file, errs := parse.ParseFile("<lsp>", text)
	if errs.HasErrors() || file == nil {
		// Fallback: scan fn/enum/type lines (existing helpers)
		if loc := scanFnLine(text, name); loc >= 0 {
			return bindingDef{Name: name, Pos: token.Pos{Line: loc + 1, Column: 1}, Kind: "fn"}, true
		}
		if loc := scanEnumLine(text, name); loc >= 0 {
			return bindingDef{Name: name, Pos: token.Pos{Line: loc + 1, Column: 1}, Kind: "enum"}, true
		}
		return bindingDef{}, false
	}
	var found *bindingDef
	walkBindings(file, func(b bindingDef) {
		if found == nil && b.Name == name {
			bb := b
			found = &bb
		}
	})
	if found == nil {
		return bindingDef{}, false
	}
	return *found, true
}

// localBindingNames returns params, lets, for-vars, and consts from the buffer.
func localBindingNames(text string) []string {
	file, errs := parse.ParseFile("<lsp>", text)
	if errs.HasErrors() || file == nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	walkBindings(file, func(b bindingDef) {
		if b.Kind == "fn" || b.Kind == "type" || b.Kind == "enum" {
			return // those are offered separately
		}
		if seen[b.Name] || b.Name == "" {
			return
		}
		seen[b.Name] = true
		names = append(names, b.Name)
	})
	return names
}

func walkBindings(file *ast.File, cb func(bindingDef)) {
	for _, d := range file.Decls {
		switch n := d.(type) {
		case *ast.FnDecl:
			cb(bindingDef{Name: n.Name, Pos: n.Pos_, Kind: "fn"})
			for _, p := range n.Params {
				cb(bindingDef{Name: p.Name, Pos: p.Pos_, Kind: "param"})
			}
			walkBlockBindings(n.Body, cb)
		case *ast.ConstDecl:
			cb(bindingDef{Name: n.Name, Pos: n.Pos_, Kind: "const"})
		case *ast.TypeDecl:
			cb(bindingDef{Name: n.Name, Pos: n.Pos_, Kind: "type"})
		case *ast.EnumDecl:
			cb(bindingDef{Name: n.Name, Pos: n.Pos_, Kind: "enum"})
		}
	}
}

func walkBlockBindings(b *ast.Block, cb func(bindingDef)) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		walkStmtBindings(s, cb)
	}
}

func walkStmtBindings(s ast.Stmt, cb func(bindingDef)) {
	switch s := s.(type) {
	case *ast.LetStmt:
		cb(bindingDef{Name: s.Name, Pos: s.Pos_, Kind: "let"})
		// init may contain func lits with their own params
		walkExprBindings(s.Init, cb)
	case *ast.ConstDecl:
		cb(bindingDef{Name: s.Name, Pos: s.Pos_, Kind: "const"})
	case *ast.ExprStmt:
		walkExprBindings(s.X, cb)
	case *ast.ReturnStmt:
		walkExprBindings(s.Value, cb)
	case *ast.AssignStmt:
		walkExprBindings(s.Target, cb)
		walkExprBindings(s.Value, cb)
	case *ast.IfStmt:
		walkExprBindings(s.Cond, cb)
		walkBlockBindings(s.Then, cb)
		if s.Else != nil {
			walkStmtBindings(s.Else, cb)
		}
	case *ast.WhileStmt:
		walkExprBindings(s.Cond, cb)
		walkBlockBindings(s.Body, cb)
	case *ast.ForStmt:
		cb(bindingDef{Name: s.Name, Pos: s.Pos_, Kind: "for"})
		walkExprBindings(s.Iter, cb)
		walkBlockBindings(s.Body, cb)
	case *ast.Block:
		walkBlockBindings(s, cb)
	case *ast.DeferStmt:
		walkExprBindings(s.Call, cb)
	}
}

func walkExprBindings(e ast.Expr, cb func(bindingDef)) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *ast.FuncLit:
		for _, p := range e.Params {
			cb(bindingDef{Name: p.Name, Pos: p.Pos_, Kind: "param"})
		}
		walkBlockBindings(e.Body, cb)
	case *ast.BinaryExpr:
		walkExprBindings(e.X, cb)
		walkExprBindings(e.Y, cb)
	case *ast.UnaryExpr:
		walkExprBindings(e.X, cb)
	case *ast.CallExpr:
		walkExprBindings(e.Fun, cb)
		for _, a := range e.Args {
			walkExprBindings(a, cb)
		}
	case *ast.IndexExpr:
		walkExprBindings(e.X, cb)
		walkExprBindings(e.Index, cb)
	case *ast.FieldExpr:
		walkExprBindings(e.X, cb)
	case *ast.QuestionExpr:
		walkExprBindings(e.X, cb)
	case *ast.ListLit:
		for _, el := range e.Elts {
			walkExprBindings(el, cb)
		}
	case *ast.MapLit:
		for i := range e.Keys {
			walkExprBindings(e.Keys[i], cb)
			walkExprBindings(e.Vals[i], cb)
		}
	case *ast.MatchExpr:
		walkExprBindings(e.Scrutinee, cb)
		for _, arm := range e.Arms {
			for _, b := range arm.Bindings {
				// match payload bindings — no precise pos; skip def location
				_ = b
			}
			walkBlockBindings(arm.Body, cb)
		}
	case *ast.IfExpr:
		walkExprBindings(e.Cond, cb)
		walkBlockBindings(e.Then, cb)
		switch el := e.Else.(type) {
		case *ast.Block:
			walkBlockBindings(el, cb)
		case *ast.IfExpr:
			walkExprBindings(el, cb)
		}
	}
}

// defLocation builds an LSP Location for a binding in uri/text.
func defLocation(uri, text string, b bindingDef) map[string]any {
	line, col := posNameCol(text, b.Pos, b.Name)
	return location(uri, line, col)
}

// posNameCol maps a token.Pos + name to 0-based LSP line/character.
func posNameCol(text string, pos token.Pos, name string) (line, col int) {
	line = pos.Line - 1
	if line < 0 {
		line = 0
	}
	lines := strings.Split(text, "\n")
	if line >= len(lines) {
		return line, 0
	}
	ln := lines[line]
	// Prefer Column when it lands on the name (params, bare binds).
	if pos.Column > 0 {
		c := pos.Column - 1
		if c >= 0 && c < len(ln) && strings.HasPrefix(ln[c:], name) {
			return line, c
		}
	}
	// `mut name` / `let name` / `fn name` — find word on the line.
	col = findIdentCol(ln, name)
	if col < 0 {
		col = 0
	}
	return line, col
}

func findIdentCol(ln, name string) int {
	col := 0
	for col <= len(ln)-len(name) {
		idx := strings.Index(ln[col:], name)
		if idx < 0 {
			return -1
		}
		start := col + idx
		end := start + len(name)
		if (start == 0 || !isIdentChar(ln[start-1])) && (end >= len(ln) || !isIdentChar(ln[end])) {
			return start
		}
		col = end
	}
	return -1
}

// documentHighlights returns DocumentHighlight[] for the identifier at cursor.
func documentHighlights(text string, line, character int) any {
	word := wordAt(text, line, character)
	if word == "" || isKeyword(word) {
		return nil
	}
	// Reuse reference scan; mark definition line as Write.
	defLine := -1
	if b, ok := findBindingDefinition(text, word); ok {
		defLine, _ = posNameCol(text, b.Pos, b.Name)
	}
	raw := findReferences(text, "file:///highlight", line, character)
	locs, ok := raw.([]map[string]any)
	if !ok || len(locs) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(locs))
	for _, loc := range locs {
		r, _ := loc["range"].(map[string]any)
		kind := 1 // Text
		if r != nil {
			if start, ok := r["start"].(map[string]any); ok {
				if ln, ok := start["line"].(int); ok && ln == defLine {
					kind = 3 // Write
				}
				// JSON unmarshaling uses float64 in some paths; ranges we build use int.
				if lnf, ok := start["line"].(float64); ok && int(lnf) == defLine {
					kind = 3
				}
			}
		}
		out = append(out, map[string]any{
			"range": r,
			"kind":  kind,
		})
	}
	return out
}
