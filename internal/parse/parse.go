// Package parse implements a recursive-descent parser for Weft.
package parse

import (
	"fmt"

	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/diag"
	"github.com/loreste/weft/internal/lex"
	"github.com/loreste/weft/internal/token"
)

// Parser holds parse state.
type Parser struct {
	file  string
	lex   *lex.Lexer
	tok   token.Token
	peek  token.Token
	peek2 token.Token
	errs  diag.List
	// for f-string expression re-parse we use nested parsers
}

// ParseFile parses source into an AST file.
func ParseFile(path, src string) (*ast.File, diag.List) {
	p := &Parser{file: path, lex: lex.New(src)}
	p.next()
	p.next()
	p.next()
	f := &ast.File{Path: path}
	for p.tok.Kind != token.EOF {
		d := p.parseDecl()
		if d != nil {
			f.Decls = append(f.Decls, d)
		}
		if p.tok.Kind == token.Illegal {
			p.errorf(p.tok.Pos, "%s", p.tok.Lit)
			break
		}
		// recover: if no progress, advance
		if len(p.errs) > 20 {
			break
		}
	}
	return f, p.errs.AttachSource(src)
}

func (p *Parser) next() {
	p.tok = p.peek
	p.peek = p.peek2
	p.peek2 = p.lex.Next()
}

func (p *Parser) errorf(pos token.Pos, format string, args ...any) {
	p.errs = append(p.errs, diag.Errorf(p.file, pos, format, args...))
}

func (p *Parser) expect(k token.Kind) token.Token {
	t := p.tok
	if t.Kind != k {
		p.errorf(t.Pos, "expected %s, got %s", k, t.Kind)
	} else {
		p.next()
	}
	return t
}

func (p *Parser) parseDecl() ast.Decl {
	switch p.tok.Kind {
	case token.Enum:
		return p.parseEnum(false)
	case token.Import, token.Use:
		return p.parseImport()
	case token.Pub:
		p.next()
		switch p.tok.Kind {
		case token.Fn:
			return p.parseFn(true)
		case token.Type:
			return p.parseTypeDecl(true)
		case token.Enum:
			return p.parseEnum(true)
		default:
			p.errorf(p.tok.Pos, "expected fn, type, or enum after pub")
			p.next()
			return nil
		}
	case token.Fn:
		return p.parseFn(false)
	case token.Type:
		return p.parseTypeDecl(false)
	case token.Const:
		return p.parseConstDecl()
	default:
		p.errorf(p.tok.Pos, "expected declaration, got %s", p.tok.Kind)
		p.next()
		return nil
	}
}

func (p *Parser) parseImport() *ast.ImportDecl {
	pos := p.tok.Pos
	p.next() // import | use
	d := &ast.ImportDecl{Pos_: pos}
	switch p.tok.Kind {
	case token.Ident:
		d.Path = p.tok.Lit
		d.IsPath = false
		p.next()
	case token.String, token.RawString:
		d.Path = p.tok.Lit
		d.IsPath = true
		p.next()
	default:
		p.errorf(p.tok.Pos, "expected package name or path string after import")
		return d
	}
	if p.tok.Kind == token.As {
		p.next()
		if p.tok.Kind != token.Ident {
			p.errorf(p.tok.Pos, "expected alias after as")
		} else {
			d.Alias = p.tok.Lit
			p.next()
		}
	}
	return d
}

func (p *Parser) parseTypeDecl(pub bool) *ast.TypeDecl {
	pos := p.tok.Pos
	p.next() // type
	if p.tok.Kind != token.Ident {
		p.errorf(p.tok.Pos, "expected type name")
		return &ast.TypeDecl{Pos_: pos, Pub: pub}
	}
	name := p.tok.Lit
	p.next()
	d := &ast.TypeDecl{Pos_: pos, Pub: pub, Name: name}
	if p.tok.Kind == token.Assign {
		p.next()
		d.Alias = p.parseTypeExpr()
		return d
	}
	if p.tok.Kind == token.LBrace {
		d.Fields = p.parseFieldList()
		return d
	}
	p.errorf(p.tok.Pos, "expected = or { after type name")
	return d
}

func (p *Parser) parseFieldList() []*ast.Field {
	p.expect(token.LBrace)
	var fields []*ast.Field
	for p.tok.Kind != token.RBrace && p.tok.Kind != token.EOF {
		f := p.parseField()
		if f != nil {
			fields = append(fields, f)
		}
		if p.tok.Kind == token.Comma {
			p.next()
		}
	}
	p.expect(token.RBrace)
	return fields
}

func (p *Parser) parseField() *ast.Field {
	if p.tok.Kind != token.Ident {
		p.errorf(p.tok.Pos, "expected field name")
		p.next()
		return nil
	}
	f := &ast.Field{Pos_: p.tok.Pos, Name: p.tok.Lit}
	p.next()
	p.expect(token.Colon)
	f.Type = p.parseTypeExpr()
	if p.tok.Kind == token.Assign {
		p.next()
		f.Default = p.parseExpr()
	}
	return f
}

func (p *Parser) parseFn(pub bool) *ast.FnDecl {
	pos := p.tok.Pos
	p.next() // fn
	if p.tok.Kind != token.Ident {
		p.errorf(p.tok.Pos, "expected function name")
		return &ast.FnDecl{Pos_: pos, Pub: pub}
	}
	name := p.tok.Lit
	p.next()
	d := &ast.FnDecl{Pos_: pos, Pub: pub, Name: name}
	// Weft: () optional when no parameters — `fn main {` or `fn main -> Result {`
	if p.tok.Kind == token.LParen {
		d.Params = p.parseParams()
	}
	if p.tok.Kind == token.Arrow {
		p.next()
		d.Ret = p.parseTypeExpr()
	}
	d.Body = p.parseBlock()
	return d
}

func (p *Parser) parseParams() []*ast.Param {
	p.expect(token.LParen)
	var params []*ast.Param
	for p.tok.Kind != token.RParen && p.tok.Kind != token.EOF {
		if p.tok.Kind != token.Ident {
			p.errorf(p.tok.Pos, "expected parameter name")
			break
		}
		pr := &ast.Param{Pos_: p.tok.Pos, Name: p.tok.Lit}
		p.next()
		if p.tok.Kind == token.Colon {
			p.next()
			pr.Type = p.parseTypeExpr()
		}
		params = append(params, pr)
		if p.tok.Kind == token.Comma {
			p.next()
		} else {
			break
		}
	}
	p.expect(token.RParen)
	return params
}

func (p *Parser) parseConstDecl() *ast.ConstDecl {
	pos := p.tok.Pos
	p.next() // const
	if p.tok.Kind != token.Ident {
		p.errorf(p.tok.Pos, "expected const name")
		return &ast.ConstDecl{Pos_: pos}
	}
	name := p.tok.Lit
	p.next()
	d := &ast.ConstDecl{Pos_: pos, Name: name}
	if p.tok.Kind == token.Colon {
		p.next()
		d.Type = p.parseTypeExpr()
	}
	p.expect(token.Assign)
	d.Value = p.parseExpr()
	return d
}

// enum Name { A, B, C }
func (p *Parser) parseEnum(pub bool) *ast.EnumDecl {
	pos := p.tok.Pos
	p.next() // enum
	if p.tok.Kind != token.Ident {
		p.errorf(p.tok.Pos, "expected enum name")
		return &ast.EnumDecl{Pos_: pos, Pub: pub}
	}
	name := p.tok.Lit
	p.next()
	d := &ast.EnumDecl{Pos_: pos, Pub: pub, Name: name}
	p.expect(token.LBrace)
	for p.tok.Kind != token.RBrace && p.tok.Kind != token.EOF {
		if p.tok.Kind != token.Ident {
			p.errorf(p.tok.Pos, "expected enum variant name")
			p.next()
			continue
		}
		vname := p.tok.Lit
		p.next()
		d.Variants = append(d.Variants, vname)

		// Check for payload: Circle(radius) or Rect(w, h)
		var fields []string
		if p.tok.Kind == token.LParen {
			p.next() // (
			for p.tok.Kind != token.RParen && p.tok.Kind != token.EOF {
				if p.tok.Kind != token.Ident {
					p.errorf(p.tok.Pos, "expected field name in enum variant")
					break
				}
				fields = append(fields, p.tok.Lit)
				p.next()
				if p.tok.Kind == token.Comma {
					p.next()
				}
			}
			p.expect(token.RParen)
		}
		d.Payloads = append(d.Payloads, ast.EnumVariant{Name: vname, Fields: fields})

		if p.tok.Kind == token.Comma {
			p.next()
		}
	}
	p.expect(token.RBrace)
	return d
}

func (p *Parser) parseTypeExpr() ast.TypeExpr {
	t := p.parseTypePrimary()
	if p.tok.Kind == token.Question {
		pos := p.tok.Pos
		p.next()
		t = &ast.OptionalType{Pos_: pos, Element: t}
	}
	return t
}

func (p *Parser) parseTypePrimary() ast.TypeExpr {
	pos := p.tok.Pos
	switch p.tok.Kind {
	case token.Ident:
		name := p.tok.Lit
		p.next()
		if name == "Map" && p.tok.Kind == token.LBracket {
			p.next()
			k := p.parseTypeExpr()
			p.expect(token.Comma)
			v := p.parseTypeExpr()
			p.expect(token.RBracket)
			return &ast.MapType{Pos_: pos, Key: k, Value: v}
		}
		if name == "Result" {
			// Result or Result[T] — bare Result means Result[any] (less verbose)
			if p.tok.Kind == token.LBracket {
				p.next()
				ok := p.parseTypeExpr()
				p.expect(token.RBracket)
				return &ast.ResultType{Pos_: pos, Ok: ok}
			}
			return &ast.ResultType{Pos_: pos, Ok: &ast.NamedType{Pos_: pos, Name: "any"}}
		}
		if name == "struct" && p.tok.Kind == token.LBrace {
			fields := p.parseFieldList()
			return &ast.StructType{Pos_: pos, Fields: fields}
		}
		return &ast.NamedType{Pos_: pos, Name: name}
	case token.LBracket:
		p.next()
		el := p.parseTypeExpr()
		p.expect(token.RBracket)
		return &ast.ListType{Pos_: pos, Element: el}
	case token.Struct:
		p.next()
		fields := p.parseFieldList()
		return &ast.StructType{Pos_: pos, Fields: fields}
	default:
		p.errorf(pos, "expected type, got %s", p.tok.Kind)
		p.next()
		return &ast.NamedType{Pos_: pos, Name: "any"}
	}
}

func (p *Parser) parseBlock() *ast.Block {
	pos := p.tok.Pos
	p.expect(token.LBrace)
	b := &ast.Block{Pos_: pos}
	for p.tok.Kind != token.RBrace && p.tok.Kind != token.EOF {
		s := p.parseStmt()
		if s != nil {
			b.Stmts = append(b.Stmts, s)
		}
	}
	p.expect(token.RBrace)
	return b
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.tok.Kind {
	case token.Let:
		return p.parseLet()
	case token.Mut:
		// mut x := expr
		return p.parseColonBind(true)
	case token.Say:
		return p.parseSay()
	case token.Const:
		return p.parseConstDecl()
	case token.Return:
		pos := p.tok.Pos
		p.next()
		s := &ast.ReturnStmt{Pos_: pos}
		if p.tok.Kind != token.RBrace && p.tok.Kind != token.EOF {
			// return may be bare or with expr; if next is `}` no expr
			// hard: we parse expr if not ending block
			if p.tok.Kind != token.RBrace {
				s.Value = p.parseExpr()
			}
		}
		return s
	case token.If:
		return p.parseIf()
	case token.While:
		return p.parseWhile()
	case token.For:
		return p.parseFor()
	case token.Break:
		pos := p.tok.Pos
		p.next()
		return &ast.BreakStmt{Pos_: pos}
	case token.Continue:
		pos := p.tok.Pos
		p.next()
		return &ast.ContinueStmt{Pos_: pos}
	case token.Defer:
		pos := p.tok.Pos
		p.next()
		return &ast.DeferStmt{Pos_: pos, Call: p.parseExpr()}
	case token.LBrace:
		// Disambiguate map literal `{ "k": v }` / `{ k: v }` from a nested block.
		if p.isMapLitStart() {
			pos := p.tok.Pos
			ex := p.parseExpr()
			if p.tok.Kind == token.Assign {
				p.next()
				return &ast.AssignStmt{Pos_: pos, Target: ex, Value: p.parseExpr()}
			}
			return &ast.ExprStmt{Pos_: pos, X: ex}
		}
		return p.parseBlock()
	default:
		// x := expr  (Weft bind)
		if p.tok.Kind == token.Ident && p.peek.Kind == token.ColonAssign {
			return p.parseColonBind(false)
		}
		// expression or assignment
		pos := p.tok.Pos
		ex := p.parseExpr()
		if p.tok.Kind == token.Assign {
			p.next()
			val := p.parseExpr()
			return &ast.AssignStmt{Pos_: pos, Target: ex, Value: val}
		}
		return &ast.ExprStmt{Pos_: pos, X: ex}
	}
}

// parseColonBind: [mut] name := expr
func (p *Parser) parseColonBind(mutAlready bool) *ast.LetStmt {
	pos := p.tok.Pos
	mut := mutAlready
	if mutAlready {
		p.next() // mut
	}
	if p.tok.Kind != token.Ident {
		p.errorf(p.tok.Pos, "expected name before :=")
		return &ast.LetStmt{Pos_: pos, Mut: mut}
	}
	name := p.tok.Lit
	p.next()
	if p.tok.Kind == token.Colon {
		// name: Type := expr  optional
		p.next()
		typ := p.parseTypeExpr()
		if p.tok.Kind != token.ColonAssign && p.tok.Kind != token.Assign {
			// was `name: type` only in wrong place — still need :=
			p.errorf(p.tok.Pos, "expected := after type")
		}
		s := &ast.LetStmt{Pos_: pos, Mut: mut, Name: name, Type: typ}
		if p.tok.Kind == token.ColonAssign || p.tok.Kind == token.Assign {
			p.next()
			s.Init = p.parseExpr()
		}
		return s
	}
	if p.tok.Kind != token.ColonAssign {
		p.errorf(p.tok.Pos, "expected :=")
		return &ast.LetStmt{Pos_: pos, Mut: mut, Name: name}
	}
	p.next()
	return &ast.LetStmt{Pos_: pos, Mut: mut, Name: name, Init: p.parseExpr()}
}

// say "hi"  or  say(a, b)  → println
func (p *Parser) parseSay() ast.Stmt {
	pos := p.tok.Pos
	p.next() // say
	var args []ast.Expr
	if p.tok.Kind == token.LParen {
		p.next()
		for p.tok.Kind != token.RParen && p.tok.Kind != token.EOF {
			args = append(args, p.parseExpr())
			if p.tok.Kind == token.Comma {
				p.next()
			} else {
				break
			}
		}
		p.expect(token.RParen)
	} else {
		// say expr  (single argument, no parens)
		args = append(args, p.parseExpr())
	}
	call := &ast.CallExpr{
		Pos_: pos,
		Fun:  &ast.Ident{Pos_: pos, Name: "println"},
		Args: args,
	}
	return &ast.ExprStmt{Pos_: pos, X: call}
}

func (p *Parser) parseLet() *ast.LetStmt {
	pos := p.tok.Pos
	p.next() // let
	mut := false
	if p.tok.Kind == token.Mut {
		mut = true
		p.next()
	}
	if p.tok.Kind != token.Ident {
		p.errorf(p.tok.Pos, "expected name after let")
		return &ast.LetStmt{Pos_: pos, Mut: mut}
	}
	name := p.tok.Lit
	p.next()
	s := &ast.LetStmt{Pos_: pos, Mut: mut, Name: name}
	if p.tok.Kind == token.Colon {
		p.next()
		s.Type = p.parseTypeExpr()
	}
	// accept := or =
	if p.tok.Kind == token.ColonAssign || p.tok.Kind == token.Assign {
		p.next()
	} else {
		p.errorf(p.tok.Pos, "expected = or :=")
	}
	s.Init = p.parseExpr()
	return s
}

func (p *Parser) parseIf() *ast.IfStmt {
	pos := p.tok.Pos
	p.next() // if
	s := &ast.IfStmt{Pos_: pos, Cond: p.parseExpr(), Then: p.parseBlock()}
	if p.tok.Kind == token.Else {
		p.next()
		if p.tok.Kind == token.If {
			s.Else = p.parseIf()
		} else {
			s.Else = p.parseBlock()
		}
	}
	return s
}

// parseIfExpr: if cond { … } [else if … | else { … }] as an expression value.
func (p *Parser) parseIfExpr() *ast.IfExpr {
	pos := p.tok.Pos
	p.next() // if
	e := &ast.IfExpr{Pos_: pos, Cond: p.parseExpr(), Then: p.parseBlock()}
	if p.tok.Kind == token.Else {
		p.next()
		if p.tok.Kind == token.If {
			e.Else = p.parseIfExpr()
		} else {
			e.Else = p.parseBlock()
		}
	}
	return e
}

// parseMatchExpr: match scrut { pat { body } … }
// Patterns: literals, `_`, ident (const/global), or field access (Status.Ok).
func (p *Parser) parseMatchExpr() *ast.MatchExpr {
	pos := p.tok.Pos
	p.next() // match
	e := &ast.MatchExpr{Pos_: pos, Scrutinee: p.parseExpr()}
	if p.tok.Kind != token.LBrace {
		p.errorf(p.tok.Pos, "expected { after match expression")
		return e
	}
	p.next() // {
	for p.tok.Kind != token.RBrace && p.tok.Kind != token.EOF {
		armPos := p.tok.Pos
		arm := ast.MatchArm{Pos_: armPos}
		switch p.tok.Kind {
		case token.Ident:
			if p.tok.Lit == "_" {
				arm.IsWildcard = true
				p.next()
			} else {
				// Status or Status.Ok or Shape.Circle(r, ...)
				arm.Pattern = p.parseMatchPattern()
				// Check for destructuring bindings: Pattern(a, b, ...)
				if p.tok.Kind == token.LParen {
					p.next() // (
					for p.tok.Kind != token.RParen && p.tok.Kind != token.EOF {
						if p.tok.Kind != token.Ident {
							p.errorf(p.tok.Pos, "expected binding name in match pattern")
							break
						}
						arm.Bindings = append(arm.Bindings, p.tok.Lit)
						p.next()
						if p.tok.Kind == token.Comma {
							p.next()
						}
					}
					p.expect(token.RParen)
				}
			}
		case token.String, token.RawString, token.Int, token.Float, token.True, token.False, token.Null:
			t := p.tok
			p.next()
			arm.Pattern = &ast.BasicLit{Pos_: armPos, Kind: t.Kind, Value: t.Lit}
		default:
			p.errorf(p.tok.Pos, "match pattern must be a literal, name, field, or _")
			// recover: skip until { or }
			for p.tok.Kind != token.LBrace && p.tok.Kind != token.RBrace && p.tok.Kind != token.EOF {
				p.next()
			}
		}
		if p.tok.Kind == token.LBrace {
			arm.Body = p.parseBlock()
		} else {
			p.errorf(p.tok.Pos, "expected { after match pattern")
		}
		e.Arms = append(e.Arms, arm)
	}
	if p.tok.Kind == token.RBrace {
		p.next()
	} else {
		p.errorf(p.tok.Pos, "expected } to close match")
	}
	return e
}

// parseMatchPattern: Ident or Ident.Field… (evaluated against env for equality).
func (p *Parser) parseMatchPattern() ast.Expr {
	pos := p.tok.Pos
	name := p.tok.Lit
	p.next()
	var x ast.Expr = &ast.Ident{Pos_: pos, Name: name}
	for p.tok.Kind == token.Dot {
		p.next()
		if p.tok.Kind != token.Ident {
			p.errorf(p.tok.Pos, "expected field name in match pattern")
			break
		}
		fname := p.tok.Lit
		fpos := p.tok.Pos
		p.next()
		x = &ast.FieldExpr{Pos_: fpos, X: x, Name: fname}
	}
	return x
}

func (p *Parser) parseWhile() *ast.WhileStmt {
	pos := p.tok.Pos
	p.next()
	return &ast.WhileStmt{Pos_: pos, Cond: p.parseExpr(), Body: p.parseBlock()}
}

func (p *Parser) parseFor() *ast.ForStmt {
	pos := p.tok.Pos
	p.next() // for
	if p.tok.Kind != token.Ident {
		p.errorf(p.tok.Pos, "expected loop variable")
		return &ast.ForStmt{Pos_: pos}
	}
	name := p.tok.Lit
	p.next()
	p.expect(token.In)
	// Bare `for x in xs {` must not parse `xs{...}` as a struct literal —
	// the `{` starts the loop body. Only treat Name{ as struct when not followed
	// by a statement-looking body (handled in parseForIter).
	return &ast.ForStmt{Pos_: pos, Name: name, Iter: p.parseForIter(), Body: p.parseBlock()}
}

// parseForIter parses the iterator expression of a for-in loop.
func (p *Parser) parseForIter() ast.Expr {
	// for x in name {  →  name is Ident, not Name{struct}
	if p.tok.Kind == token.Ident && p.peek.Kind == token.LBrace {
		pos := p.tok.Pos
		n := p.tok.Lit
		p.next()
		return &ast.Ident{Pos_: pos, Name: n}
	}
	return p.parseExpr()
}

// --- Expressions (precedence climbing) ---

func (p *Parser) parseExpr() ast.Expr {
	return p.parsePipe()
}

func (p *Parser) parseNullCoalesce() ast.Expr {
	left := p.parseOr()
	for p.tok.Kind == token.NullCoalesce {
		pos := p.tok.Pos
		p.next()
		right := p.parseOr()
		left = &ast.BinaryExpr{Pos_: pos, Op: token.NullCoalesce, X: left, Y: right}
	}
	return left
}

func (p *Parser) parseOr() ast.Expr {
	left := p.parseAnd()
	for p.tok.Kind == token.Or {
		pos := p.tok.Pos
		p.next()
		right := p.parseAnd()
		left = &ast.BinaryExpr{Pos_: pos, Op: token.Or, X: left, Y: right}
	}
	return left
}

func (p *Parser) parseAnd() ast.Expr {
	left := p.parseEquality()
	for p.tok.Kind == token.And {
		pos := p.tok.Pos
		p.next()
		right := p.parseEquality()
		left = &ast.BinaryExpr{Pos_: pos, Op: token.And, X: left, Y: right}
	}
	return left
}

func (p *Parser) parseEquality() ast.Expr {
	left := p.parseComparison()
	for p.tok.Kind == token.Eq || p.tok.Kind == token.Neq {
		pos := p.tok.Pos
		op := p.tok.Kind
		p.next()
		right := p.parseComparison()
		left = &ast.BinaryExpr{Pos_: pos, Op: op, X: left, Y: right}
	}
	return left
}

func (p *Parser) parseComparison() ast.Expr {
	left := p.parseTerm()
	for p.tok.Kind == token.Lt || p.tok.Kind == token.Lte || p.tok.Kind == token.Gt || p.tok.Kind == token.Gte {
		pos := p.tok.Pos
		op := p.tok.Kind
		p.next()
		right := p.parseTerm()
		left = &ast.BinaryExpr{Pos_: pos, Op: op, X: left, Y: right}
	}
	return left
}

func (p *Parser) parseTerm() ast.Expr {
	left := p.parseFactor()
	for p.tok.Kind == token.Plus || p.tok.Kind == token.Minus {
		pos := p.tok.Pos
		op := p.tok.Kind
		p.next()
		right := p.parseFactor()
		left = &ast.BinaryExpr{Pos_: pos, Op: op, X: left, Y: right}
	}
	return left
}

func (p *Parser) parseFactor() ast.Expr {
	left := p.parseUnary()
	for p.tok.Kind == token.Star || p.tok.Kind == token.Slash || p.tok.Kind == token.Percent {
		pos := p.tok.Pos
		op := p.tok.Kind
		p.next()
		right := p.parseUnary()
		left = &ast.BinaryExpr{Pos_: pos, Op: op, X: left, Y: right}
	}
	return left
}

func (p *Parser) parseUnary() ast.Expr {
	if p.tok.Kind == token.Minus || p.tok.Kind == token.Bang {
		pos := p.tok.Pos
		op := p.tok.Kind
		p.next()
		return &ast.UnaryExpr{Pos_: pos, Op: op, X: p.parseUnary()}
	}
	return p.parsePostfix()
}

func (p *Parser) parsePostfix() ast.Expr {
	ex := p.parsePrimary()
	for {
		switch p.tok.Kind {
		case token.LParen:
			pos := p.tok.Pos
			p.next()
			var args []ast.Expr
			for p.tok.Kind != token.RParen && p.tok.Kind != token.EOF {
				args = append(args, p.parseExpr())
				if p.tok.Kind == token.Comma {
					p.next()
				} else {
					break
				}
			}
			p.expect(token.RParen)
			ex = &ast.CallExpr{Pos_: pos, Fun: ex, Args: args}
		case token.LBracket:
			pos := p.tok.Pos
			p.next()
			idx := p.parseExpr()
			p.expect(token.RBracket)
			ex = &ast.IndexExpr{Pos_: pos, X: ex, Index: idx}
		case token.Dot:
			pos := p.tok.Pos
			p.next()
			if p.tok.Kind != token.Ident {
				p.errorf(p.tok.Pos, "expected field name after .")
				return ex
			}
			name := p.tok.Lit
			p.next()
			ex = &ast.FieldExpr{Pos_: pos, X: ex, Name: name}
		case token.Question:
			pos := p.tok.Pos
			p.next()
			ex = &ast.QuestionExpr{Pos_: pos, X: ex}
		default:
			return ex
		}
	}
}

func (p *Parser) parsePrimary() ast.Expr {
	pos := p.tok.Pos
	switch p.tok.Kind {
	case token.If:
		// if as value: x := if c { a } else { b }
		return p.parseIfExpr()
	case token.Match:
		return p.parseMatchExpr()
	case token.Ident:
		name := p.tok.Lit
		p.next()
		// StructLit: Name{ ... } — only if next is { and looks like fields (ident: or })
		if p.tok.Kind == token.LBrace && p.isStructLit() {
			return p.parseStructLit(pos, name)
		}
		return &ast.Ident{Pos_: pos, Name: name}
	case token.Int, token.Float, token.RawString, token.True, token.False, token.Null:
		t := p.tok
		p.next()
		return &ast.BasicLit{Pos_: pos, Kind: t.Kind, Value: t.Lit}
	case token.String:
		// Weft: "hi $name" / "sum=${a+b}" — not brace-f-strings (JSON-safe)
		lit := p.tok.Lit
		p.next()
		if containsDollarInterp(lit) {
			return p.parseDollarString(pos, lit)
		}
		return &ast.BasicLit{Pos_: pos, Kind: token.String, Value: lit}
	case token.FString:
		lit := p.tok.Lit
		p.next()
		return p.parseFString(pos, lit)
	case token.LParen:
		p.next()
		// empty unit ()
		if p.tok.Kind == token.RParen {
			p.next()
			return &ast.Ident{Pos_: pos, Name: "unit"} // unit value placeholder; compiler maps unit{}
		}
		ex := p.parseExpr()
		p.expect(token.RParen)
		return ex
	case token.LBracket:
		return p.parseListLit()
	case token.LBrace:
		return p.parseMapLit()
	case token.Fn:
		return p.parseFuncLit()
	case token.Say:
		// Expression form: allow `|> say` and higher-order use (prelude builtin).
		// Statement form remains parseSay() for `say x` / `say(x)`.
		p.next()
		return &ast.Ident{Pos_: pos, Name: "say"}
	default:
		p.errorf(pos, "expected expression, got %s", p.tok.Kind)
		p.next()
		return &ast.BasicLit{Pos_: pos, Kind: token.Null, Value: "null"}
	}
}

func (p *Parser) isStructLit() bool {
	// After Name, tok is `{`. True struct: `Name{}` or `Name{ field: value`.
	// Not struct: `while x < n {` / `for x in xs {` where `{` starts a block.
	if p.tok.Kind != token.LBrace {
		return false
	}
	if p.peek.Kind == token.RBrace {
		return true
	}
	// Require field: value (Ident then Colon) — uses peek2
	if p.peek.Kind == token.Ident && p.peek2.Kind == token.Colon {
		return true
	}
	return false
}

// isMapLitStart reports whether `{` begins a map literal statement (not a block).
func (p *Parser) isMapLitStart() bool {
	if p.tok.Kind != token.LBrace {
		return false
	}
	// {"key": ...}
	if p.peek.Kind == token.String {
		return true
	}
	// {key: ...} bare ident key
	if p.peek.Kind == token.Ident && p.peek2.Kind == token.Colon {
		return true
	}
	return false
}

func (p *Parser) parseStructLit(pos token.Pos, name string) *ast.StructLit {
	p.expect(token.LBrace)
	s := &ast.StructLit{Pos_: pos, Name: name}
	for p.tok.Kind != token.RBrace && p.tok.Kind != token.EOF {
		if p.tok.Kind != token.Ident {
			p.errorf(p.tok.Pos, "expected field name in struct literal")
			break
		}
		fp := p.tok.Pos
		fn := p.tok.Lit
		p.next()
		p.expect(token.Colon)
		val := p.parseExpr()
		s.Fields = append(s.Fields, ast.StructField{Pos_: fp, Name: fn, Value: val})
		if p.tok.Kind == token.Comma {
			p.next()
		} else {
			break
		}
	}
	p.expect(token.RBrace)
	return s
}

func (p *Parser) parseListLit() *ast.ListLit {
	pos := p.tok.Pos
	p.next() // [
	l := &ast.ListLit{Pos_: pos}
	for p.tok.Kind != token.RBracket && p.tok.Kind != token.EOF {
		l.Elts = append(l.Elts, p.parseExpr())
		if p.tok.Kind == token.Comma {
			p.next()
		} else {
			break
		}
	}
	p.expect(token.RBracket)
	return l
}

func (p *Parser) parseMapLit() *ast.MapLit {
	pos := p.tok.Pos
	p.next() // {
	m := &ast.MapLit{Pos_: pos}
	for p.tok.Kind != token.RBrace && p.tok.Kind != token.EOF {
		k := p.parseExpr()
		p.expect(token.Colon)
		v := p.parseExpr()
		m.Keys = append(m.Keys, k)
		m.Vals = append(m.Vals, v)
		if p.tok.Kind == token.Comma {
			p.next()
		} else {
			break
		}
	}
	p.expect(token.RBrace)
	return m
}

func (p *Parser) parseFuncLit() *ast.FuncLit {
	pos := p.tok.Pos
	p.next() // fn
	f := &ast.FuncLit{Pos_: pos}
	if p.tok.Kind == token.LParen {
		f.Params = p.parseParams()
	}
	if p.tok.Kind == token.Arrow {
		p.next()
		f.Ret = p.parseTypeExpr()
	}
	f.Body = p.parseBlock()
	return f
}

// Pipeline: expr |> f  →  f(expr)   (lowest precedence)
func (p *Parser) parsePipe() ast.Expr {
	left := p.parseNullCoalesce()
	for p.tok.Kind == token.Pipe {
		pos := p.tok.Pos
		p.next()
		right := p.parseNullCoalesce()
		switch r := right.(type) {
		case *ast.CallExpr:
			args := append([]ast.Expr{left}, r.Args...)
			left = &ast.CallExpr{Pos_: pos, Fun: r.Fun, Args: args}
		default:
			left = &ast.CallExpr{Pos_: pos, Fun: right, Args: []ast.Expr{left}}
		}
	}
	return left
}

func (p *Parser) parseFString(pos token.Pos, lit string) ast.Expr {
	// Split lit into text and {expr} parts; re-parse expressions.
	parts := splitFString(lit)
	fs := &ast.FStringExpr{Pos_: pos}
	for _, part := range parts {
		if part.isExpr {
			sub, errs := ParseExpr(p.file, part.text)
			if len(errs) > 0 {
				p.errs = append(p.errs, errs...)
			}
			fs.Parts = append(fs.Parts, ast.FStringPart{Expr: sub})
		} else {
			fs.Parts = append(fs.Parts, ast.FStringPart{Text: part.text})
		}
	}
	return fs
}

type fpart struct {
	text   string
	isExpr bool
}

func splitFString(s string) []fpart {
	var parts []fpart
	var buf []byte
	i := 0
	for i < len(s) {
		if s[i] == '{' {
			if i+1 < len(s) && s[i+1] == '{' {
				buf = append(buf, '{')
				i += 2
				continue
			}
			if len(buf) > 0 {
				parts = append(parts, fpart{text: string(buf)})
				buf = buf[:0]
			}
			i++
			start := i
			depth := 1
			for i < len(s) && depth > 0 {
				if s[i] == '{' {
					depth++
				} else if s[i] == '}' {
					depth--
				}
				if depth > 0 {
					i++
				}
			}
			if depth != 0 {
				parts = append(parts, fpart{text: s[start:], isExpr: true})
				return parts
			}
			parts = append(parts, fpart{text: s[start:i], isExpr: true})
			i++ // skip }
			continue
		}
		if s[i] == '}' && i+1 < len(s) && s[i+1] == '}' {
			buf = append(buf, '}')
			i += 2
			continue
		}
		buf = append(buf, s[i])
		i++
	}
	if len(buf) > 0 {
		parts = append(parts, fpart{text: string(buf)})
	}
	return parts
}

func containsDollarInterp(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) {
			n := s[i+1]
			if n == '{' || n == '_' || (n >= 'a' && n <= 'z') || (n >= 'A' && n <= 'Z') {
				return true
			}
		}
	}
	return false
}

// parseDollarString splits "a $x b ${y+1}" into FStringExpr parts.
func (p *Parser) parseDollarString(pos token.Pos, lit string) ast.Expr {
	fs := &ast.FStringExpr{Pos_: pos}
	i := 0
	for i < len(lit) {
		if lit[i] == '$' && i+1 < len(lit) {
			if lit[i+1] == '$' {
				// $$ → literal $
				fs.Parts = append(fs.Parts, ast.FStringPart{Text: "$"})
				i += 2
				continue
			}
			if lit[i+1] == '{' {
				// ${expr}
				depth := 1
				j := i + 2
				for j < len(lit) && depth > 0 {
					if lit[j] == '{' {
						depth++
					} else if lit[j] == '}' {
						depth--
					}
					if depth > 0 {
						j++
					}
				}
				if depth != 0 {
					p.errorf(pos, "unterminated ${...} in string")
					return fs
				}
				sub := lit[i+2 : j]
				ex, errs := ParseExpr(p.file, sub)
				p.errs = append(p.errs, errs...)
				fs.Parts = append(fs.Parts, ast.FStringPart{Expr: ex})
				i = j + 1
				continue
			}
			// $ident
			if isIdentStartByte(lit[i+1]) {
				j := i + 1
				for j < len(lit) && isIdentPartByte(lit[j]) {
					j++
				}
				name := lit[i+1 : j]
				fs.Parts = append(fs.Parts, ast.FStringPart{
					Expr: &ast.Ident{Pos_: pos, Name: name},
				})
				i = j
				continue
			}
		}
		// plain text until next $
		j := i
		for j < len(lit) {
			if lit[j] == '$' {
				break
			}
			j++
		}
		if j > i {
			fs.Parts = append(fs.Parts, ast.FStringPart{Text: lit[i:j]})
		}
		i = j
	}
	return fs
}

func isIdentStartByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isIdentPartByte(b byte) bool {
	return isIdentStartByte(b) || (b >= '0' && b <= '9')
}

// ParseExpr parses a single expression (for f-string interpolation).
func ParseExpr(file, src string) (ast.Expr, diag.List) {
	p := &Parser{file: file, lex: lex.New(src)}
	p.next()
	p.next()
	p.next()
	ex := p.parseExpr()
	return ex, p.errs
}

// Ensure unused import safety for fmt in tests etc.
var _ = fmt.Sprintf
