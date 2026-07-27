// Package format pretty-prints Weft AST (weft fmt).
package format

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/token"
)

// Options controls printing.
type Options struct {
	Indent string // default "    " (4 spaces)
}

// File formats an AST file to source text ending with a newline.
func File(f *ast.File, opts Options) string {
	if opts.Indent == "" {
		opts.Indent = "    "
	}
	p := &printer{indent: opts.Indent}
	for i, d := range f.Decls {
		if i > 0 {
			p.newline()
			// blank line between top-level decls
			p.newline()
		}
		p.decl(d)
	}
	if len(f.Decls) > 0 {
		p.newline()
	}
	return p.buf.String()
}

type printer struct {
	buf    bytes.Buffer
	indent string
	depth  int
	bol    bool // beginning of line
}

func (p *printer) write(s string) {
	if p.bol && s != "\n" && s != "" {
		for i := 0; i < p.depth; i++ {
			p.buf.WriteString(p.indent)
		}
		p.bol = false
	}
	p.buf.WriteString(s)
	if strings.HasSuffix(s, "\n") {
		p.bol = true
	}
}

func (p *printer) newline() {
	p.buf.WriteByte('\n')
	p.bol = true
}

func (p *printer) decl(d ast.Decl) {
	switch d := d.(type) {
	case *ast.ImportDecl:
		// Prefer Weft `use`
		p.write("use ")
		if d.IsPath {
			p.write(strconv.Quote(d.Path))
		} else {
			p.write(d.Path)
		}
		if d.Alias != "" {
			p.write(" as ")
			p.write(d.Alias)
		}
		p.newline()
	case *ast.ConstDecl:
		p.write("const ")
		p.write(d.Name)
		if d.Type != nil {
			p.write(": ")
			p.typeExpr(d.Type)
		}
		p.write(" = ")
		p.expr(d.Value, 0)
		p.newline()
	case *ast.TypeDecl:
		if d.Pub {
			p.write("pub ")
		}
		p.write("type ")
		p.write(d.Name)
		if d.Alias != nil {
			p.write(" = ")
			p.typeExpr(d.Alias)
			p.newline()
			return
		}
		p.write(" ")
		p.write("{")
		p.newline()
		p.depth++
		for _, f := range d.Fields {
			p.write(f.Name)
			if f.Type != nil {
				p.write(": ")
				p.typeExpr(f.Type)
			}
			if f.Default != nil {
				p.write(" = ")
				p.expr(f.Default, 0)
			}
			p.newline()
		}
		p.depth--
		p.write("}")
		p.newline()
	case *ast.FnDecl:
		if d.Pub {
			p.write("pub ")
		}
		p.write("fn ")
		p.write(d.Name)
		if len(d.Params) > 0 {
			p.write("(")
			for i, par := range d.Params {
				if i > 0 {
					p.write(", ")
				}
				p.write(par.Name)
				if par.Type != nil {
					p.write(": ")
					p.typeExpr(par.Type)
				}
			}
			p.write(")")
		}
		if d.Ret != nil {
			p.write(" -> ")
			p.typeExpr(d.Ret)
		}
		p.write(" ")
		p.block(d.Body)
		p.newline()
	case *ast.EnumDecl:
		if d.Pub {
			p.write("pub ")
		}
		p.write("enum ")
		p.write(d.Name)
		p.write(" {")
		// short enums stay on one line: enum Status { Ok, Err, Pending }
		oneLine := len(d.Variants) > 0 && len(d.Variants) <= 8
		if oneLine {
			n := len(d.Name) + 8
			for _, v := range d.Variants {
				n += len(v) + 2
			}
			if n > 72 {
				oneLine = false
			}
		}
		if len(d.Variants) == 0 {
			p.write("}")
			p.newline()
			return
		}
		if oneLine {
			p.write(" ")
			for i, v := range d.Variants {
				if i > 0 {
					p.write(", ")
				}
				p.write(v)
			}
			p.write(" }")
			p.newline()
			return
		}
		p.newline()
		p.depth++
		for i, v := range d.Variants {
			p.write(v)
			if i < len(d.Variants)-1 {
				p.write(",")
			}
			p.newline()
		}
		p.depth--
		p.write("}")
		p.newline()
	default:
		p.write(fmt.Sprintf("/* unhandled decl %T */", d))
		p.newline()
	}
}

func (p *printer) typeExpr(t ast.TypeExpr) {
	if t == nil {
		return
	}
	switch t := t.(type) {
	case *ast.NamedType:
		p.write(t.Name)
	case *ast.ListType:
		p.write("[")
		p.typeExpr(t.Element)
		p.write("]")
	case *ast.MapType:
		p.write("{")
		p.typeExpr(t.Key)
		p.write(": ")
		p.typeExpr(t.Value)
		p.write("}")
	case *ast.ResultType:
		p.write("Result")
		if t.Ok != nil {
			p.write("[")
			p.typeExpr(t.Ok)
			p.write("]")
		}
	case *ast.OptionalType:
		p.typeExpr(t.Element)
		p.write("?")
	case *ast.StructType:
		p.write("struct {")
		for i, f := range t.Fields {
			if i > 0 {
				p.write("; ")
			}
			p.write(f.Name)
			if f.Type != nil {
				p.write(": ")
				p.typeExpr(f.Type)
			}
		}
		p.write("}")
	default:
		p.write("any")
	}
}

func (p *printer) block(b *ast.Block) {
	if b == nil {
		p.write("{}")
		return
	}
	p.write("{")
	if len(b.Stmts) == 0 {
		p.write("}")
		return
	}
	// Single simple expression → { expr } on one line (match arms, short fns).
	if x, ok := simpleBlockExpr(b); ok {
		p.write(" ")
		p.expr(x, 0)
		p.write(" }")
		return
	}
	p.newline()
	p.depth++
	for _, s := range b.Stmts {
		p.stmt(s)
	}
	p.depth--
	p.write("}")
}

// simpleBlockExpr is true for a one-statement expression block (no nested control).
func simpleBlockExpr(b *ast.Block) (ast.Expr, bool) {
	if b == nil || len(b.Stmts) != 1 {
		return nil, false
	}
	es, ok := b.Stmts[0].(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	switch es.X.(type) {
	case *ast.MatchExpr, *ast.IfExpr, *ast.FuncLit:
		return nil, false
	}
	return es.X, true
}

func (p *printer) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.LetStmt:
		if s.Mut {
			p.write("mut ")
		}
		p.write(s.Name)
		if s.Type != nil {
			p.write(": ")
			p.typeExpr(s.Type)
		}
		p.write(" := ")
		p.expr(s.Init, 0)
		p.newline()
	case *ast.ConstDecl:
		p.write("const ")
		p.write(s.Name)
		p.write(" = ")
		p.expr(s.Value, 0)
		p.newline()
	case *ast.AssignStmt:
		p.expr(s.Target, 0)
		p.write(" = ")
		p.expr(s.Value, 0)
		p.newline()
	case *ast.ReturnStmt:
		p.write("return")
		if s.Value != nil {
			p.write(" ")
			p.expr(s.Value, 0)
		}
		p.newline()
	case *ast.IfStmt:
		p.write("if ")
		p.expr(s.Cond, 0)
		p.write(" ")
		p.block(s.Then)
		if s.Else != nil {
			p.write(" else ")
			switch e := s.Else.(type) {
			case *ast.IfStmt:
				p.stmt(e)
				return // stmt already newline
			case *ast.Block:
				p.block(e)
			default:
				p.write("{}")
			}
		}
		p.newline()
	case *ast.WhileStmt:
		p.write("while ")
		p.expr(s.Cond, 0)
		p.write(" ")
		p.block(s.Body)
		p.newline()
	case *ast.ForStmt:
		p.write("for ")
		p.write(s.Name)
		p.write(" in ")
		p.expr(s.Iter, 0)
		p.write(" ")
		p.block(s.Body)
		p.newline()
	case *ast.BreakStmt:
		p.write("break")
		p.newline()
	case *ast.ContinueStmt:
		p.write("continue")
		p.newline()
	case *ast.DeferStmt:
		p.write("defer ")
		p.expr(s.Call, 0)
		p.newline()
	case *ast.Block:
		p.block(s)
		p.newline()
	case *ast.ExprStmt:
		// prefer say for println(...)
		if call, ok := s.X.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "println" {
				p.write("say")
				if len(call.Args) == 1 {
					p.write("(")
					p.expr(call.Args[0], 0)
					p.write(")")
				} else {
					p.write("(")
					for i, a := range call.Args {
						if i > 0 {
							p.write(", ")
						}
						p.expr(a, 0)
					}
					p.write(")")
				}
				p.newline()
				return
			}
		}
		p.expr(s.X, 0)
		p.newline()
	default:
		p.write(fmt.Sprintf("/* stmt %T */", s))
		p.newline()
	}
}

// precedence for parenthesization (higher binds tighter)
const (
	precLowest = iota
	precPipe
	precOr
	precAnd
	precCmp
	precAdd
	precMul
	precUnary
	precPost // call, index, field, ?
	precPrimary
)

func (p *printer) expr(e ast.Expr, parentPrec int) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *ast.Ident:
		p.write(e.Name)
	case *ast.BasicLit:
		switch e.Kind {
		case token.String:
			p.write(strconv.Quote(e.Value))
		case token.RawString:
			p.write("`")
			p.write(e.Value)
			p.write("`")
		case token.True:
			p.write("true")
		case token.False:
			p.write("false")
		case token.Null:
			p.write("null")
		default:
			p.write(e.Value)
		}
	case *ast.BinaryExpr:
		prec := binPrec(e.Op)
		if prec < parentPrec {
			p.write("(")
		}
		p.expr(e.X, prec)
		p.write(" ")
		p.write(e.Op.String())
		p.write(" ")
		p.expr(e.Y, prec+1)
		if prec < parentPrec {
			p.write(")")
		}
	case *ast.UnaryExpr:
		if precUnary < parentPrec {
			p.write("(")
		}
		p.write(e.Op.String())
		p.expr(e.X, precUnary)
		if precUnary < parentPrec {
			p.write(")")
		}
	case *ast.CallExpr:
		p.expr(e.Fun, precPost)
		p.write("(")
		for i, a := range e.Args {
			if i > 0 {
				p.write(", ")
			}
			p.expr(a, 0)
		}
		p.write(")")
	case *ast.IndexExpr:
		p.expr(e.X, precPost)
		p.write("[")
		p.expr(e.Index, 0)
		p.write("]")
	case *ast.FieldExpr:
		p.expr(e.X, precPost)
		p.write(".")
		p.write(e.Name)
	case *ast.QuestionExpr:
		p.expr(e.X, precPost)
		p.write("?")
	case *ast.ListLit:
		p.listLit(e)
	case *ast.MapLit:
		p.mapLit(e)
	case *ast.StructLit:
		p.write(e.Name)
		p.write("{")
		if structNeedsMulti(e) {
			p.newline()
			p.depth++
			for _, f := range e.Fields {
				p.write(f.Name)
				p.write(": ")
				p.expr(f.Value, 0)
				p.write(",")
				p.newline()
			}
			p.depth--
			p.write("}")
		} else {
			for i, f := range e.Fields {
				if i > 0 {
					p.write(", ")
				}
				p.write(f.Name)
				p.write(": ")
				p.expr(f.Value, 0)
			}
			p.write("}")
		}
	case *ast.FStringExpr:
		// print as "…" with $ / ${} when simple
		p.write("\"")
		for _, part := range e.Parts {
			if part.Expr != nil {
				if id, ok := part.Expr.(*ast.Ident); ok {
					p.write("$")
					p.write(id.Name)
				} else {
					p.write("${")
					p.expr(part.Expr, 0)
					p.write("}")
				}
			} else {
				p.write(escapeInterp(part.Text))
			}
		}
		p.write("\"")
	case *ast.IfExpr:
		p.write("if ")
		p.expr(e.Cond, 0)
		p.write(" ")
		p.block(e.Then)
		if e.Else != nil {
			p.write(" else ")
			switch el := e.Else.(type) {
			case *ast.Block:
				p.block(el)
			case *ast.IfExpr:
				p.expr(el, 0)
			case *ast.IfStmt:
				// rare
				p.write("if ")
				p.expr(el.Cond, 0)
				p.write(" ")
				p.block(el.Then)
			}
		}
	case *ast.MatchExpr:
		p.write("match ")
		p.expr(e.Scrutinee, 0)
		p.write(" {")
		p.newline()
		p.depth++
		for _, arm := range e.Arms {
			if arm.IsWildcard {
				p.write("_ ")
			} else {
				p.expr(arm.Pattern, 0)
				p.write(" ")
			}
			p.block(arm.Body)
			p.newline()
		}
		p.depth--
		p.write("}")
	case *ast.FuncLit:
		// Always print params list for anonymous fns so `fn() { … }` is unambiguous.
		p.write("fn(")
		for i, par := range e.Params {
			if i > 0 {
				p.write(", ")
			}
			p.write(par.Name)
			if par.Type != nil {
				p.write(": ")
				p.typeExpr(par.Type)
			}
		}
		p.write(")")
		if e.Ret != nil {
			p.write(" -> ")
			p.typeExpr(e.Ret)
		}
		p.write(" ")
		p.block(e.Body)
	default:
		p.write(fmt.Sprintf("/*expr %T*/", e))
	}
}

func (p *printer) mapLit(e *ast.MapLit) {
	if len(e.Keys) == 0 {
		p.write("{}")
		return
	}
	multi := mapNeedsMulti(e)
	p.write("{")
	if multi {
		p.newline()
		p.depth++
		for i := range e.Keys {
			p.mapKey(e.Keys[i])
			p.write(": ")
			p.expr(e.Vals[i], 0)
			p.write(",")
			p.newline()
		}
		p.depth--
		p.write("}")
		return
	}
	for i := range e.Keys {
		if i > 0 {
			p.write(", ")
		}
		p.mapKey(e.Keys[i])
		p.write(": ")
		p.expr(e.Vals[i], 0)
	}
	p.write("}")
}

func (p *printer) mapKey(k ast.Expr) {
	// String keys must stay quoted — bare idents are variable lookups at runtime.
	if lit, ok := k.(*ast.BasicLit); ok && lit.Kind == token.String {
		p.write(strconv.Quote(lit.Value))
		return
	}
	p.expr(k, 0)
}

func (p *printer) listLit(e *ast.ListLit) {
	if len(e.Elts) == 0 {
		p.write("[]")
		return
	}
	if listNeedsMulti(e) {
		p.write("[")
		p.newline()
		p.depth++
		for _, el := range e.Elts {
			p.expr(el, 0)
			p.write(",")
			p.newline()
		}
		p.depth--
		p.write("]")
		return
	}
	p.write("[")
	for i, el := range e.Elts {
		if i > 0 {
			p.write(", ")
		}
		p.expr(el, 0)
	}
	p.write("]")
}

// mapNeedsMulti: nested values, many keys, or would be a long one-liner.
func mapNeedsMulti(e *ast.MapLit) bool {
	if e == nil || len(e.Keys) == 0 {
		return false
	}
	if len(e.Keys) >= 3 {
		return true
	}
	for _, v := range e.Vals {
		if complexExpr(v) {
			return true
		}
	}
	// rough width: keys + values as strings if literals
	n := 2
	for i := range e.Keys {
		n += approxWidth(e.Keys[i]) + approxWidth(e.Vals[i]) + 4
	}
	return n > 60
}

func listNeedsMulti(e *ast.ListLit) bool {
	if e == nil || len(e.Elts) == 0 {
		return false
	}
	if len(e.Elts) >= 6 {
		return true
	}
	for _, el := range e.Elts {
		if complexExpr(el) {
			return true
		}
	}
	n := 2
	for _, el := range e.Elts {
		n += approxWidth(el) + 2
	}
	return n > 72
}

func structNeedsMulti(e *ast.StructLit) bool {
	if e == nil || len(e.Fields) == 0 {
		return false
	}
	if len(e.Fields) >= 3 {
		return true
	}
	for _, f := range e.Fields {
		if complexExpr(f.Value) {
			return true
		}
	}
	return false
}

func complexExpr(e ast.Expr) bool {
	switch e.(type) {
	case *ast.MapLit, *ast.ListLit, *ast.FuncLit, *ast.MatchExpr, *ast.IfExpr, *ast.StructLit:
		return true
	default:
		return false
	}
}

func approxWidth(e ast.Expr) int {
	if e == nil {
		return 4
	}
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.String {
			return len(x.Value) + 2
		}
		return len(x.Value)
	case *ast.Ident:
		return len(x.Name)
	case *ast.FieldExpr:
		return approxWidth(x.X) + 1 + len(x.Name)
	case *ast.UnaryExpr:
		return 1 + approxWidth(x.X)
	case *ast.BinaryExpr:
		return approxWidth(x.X) + approxWidth(x.Y) + 3
	case *ast.CallExpr:
		n := approxWidth(x.Fun) + 2
		for _, a := range x.Args {
			n += approxWidth(a) + 2
		}
		return n
	case *ast.MapLit:
		return 20 + len(x.Keys)*8
	case *ast.ListLit:
		return 10 + len(x.Elts)*6
	default:
		return 12
	}
}

func binPrec(op token.Kind) int {
	switch op {
	case token.Pipe:
		return precPipe
	case token.Or:
		return precOr
	case token.And:
		return precAnd
	case token.Eq, token.Neq, token.Lt, token.Lte, token.Gt, token.Gte:
		return precCmp
	case token.Plus, token.Minus:
		return precAdd
	case token.Star, token.Slash, token.Percent:
		return precMul
	case token.NullCoalesce:
		return precOr
	default:
		return precLowest
	}
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
		} else if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func escapeInterp(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "$", "\\$")
	return s
}
