package ast

import (
	"testing"

	"github.com/loreste/weft/internal/token"
)

func TestFilePos(t *testing.T) {
	f := &File{Path: "test.weft"}
	p := f.Pos()
	if p.Line != 0 {
		t.Fatal("empty file pos")
	}
	f.Decls = []Decl{&FnDecl{Pos_: token.Pos{Line: 1, Column: 1}}}
	if f.Pos().Line != 1 {
		t.Fatal("file pos with decls")
	}
}

func TestDeclPos(t *testing.T) {
	pos := token.Pos{Line: 5, Column: 3}
	cases := []Decl{
		&ImportDecl{Pos_: pos},
		&TypeDecl{Pos_: pos},
		&FnDecl{Pos_: pos},
		&ConstDecl{Pos_: pos},
		&EnumDecl{Pos_: pos},
	}
	for _, d := range cases {
		if d.Pos().Line != 5 {
			t.Fatal("decl pos")
		}
	}
}

func TestTypeExprPos(t *testing.T) {
	pos := token.Pos{Line: 3}
	types := []TypeExpr{
		&NamedType{Pos_: pos},
		&ListType{Pos_: pos},
		&MapType{Pos_: pos},
		&ResultType{Pos_: pos},
		&OptionalType{Pos_: pos},
		&StructType{Pos_: pos},
		&FnType{Pos_: pos},
	}
	for _, te := range types {
		if te.Pos().Line != 3 {
			t.Fatal("type expr pos")
		}
	}
}

func TestStmtPos(t *testing.T) {
	pos := token.Pos{Line: 7}
	stmts := []Stmt{
		&LetStmt{Pos_: pos},
		&ExprStmt{Pos_: pos},
		&ReturnStmt{Pos_: pos},
		&AssignStmt{Pos_: pos},
		&IfStmt{Pos_: pos},
		&WhileStmt{Pos_: pos},
		&ForStmt{Pos_: pos},
		&BreakStmt{Pos_: pos},
		&ContinueStmt{Pos_: pos},
		&Block{Pos_: pos},
		&ConstDecl{Pos_: pos},
		&DeferStmt{Pos_: pos},
	}
	for _, s := range stmts {
		if s.Pos().Line != 7 {
			t.Fatal("stmt pos")
		}
	}
}

func TestExprPos(t *testing.T) {
	pos := token.Pos{Line: 2}
	exprs := []Expr{
		&BasicLit{Pos_: pos},
		&Ident{Pos_: pos},
		&BinaryExpr{Pos_: pos},
		&UnaryExpr{Pos_: pos},
		&CallExpr{Pos_: pos},
		&IndexExpr{Pos_: pos},
		&FieldExpr{Pos_: pos},
		&QuestionExpr{Pos_: pos},
		&ListLit{Pos_: pos},
		&MapLit{Pos_: pos},
		&StructLit{Pos_: pos},
		&FStringExpr{Pos_: pos},
		&FuncLit{Pos_: pos},
		&IfExpr{Pos_: pos},
		&MatchExpr{Pos_: pos},
	}
	for _, e := range exprs {
		if e.Pos().Line != 2 {
			t.Fatal("expr pos")
		}
	}
}

func TestFieldPos(t *testing.T) {
	f := &Field{Pos_: token.Pos{Line: 1}}
	if f.Pos().Line != 1 {
		t.Fatal("field pos")
	}
}

func TestParamPos(t *testing.T) {
	p := &Param{Pos_: token.Pos{Line: 1}}
	if p.Pos().Line != 1 {
		t.Fatal("param pos")
	}
}

func TestStructFieldType(t *testing.T) {
	sf := StructField{Name: "x"}
	if sf.Name != "x" {
		t.Fatal("struct field name")
	}
}

func TestMatchArmType(t *testing.T) {
	a := MatchArm{Pos_: token.Pos{Line: 1}, IsWildcard: true}
	if !a.IsWildcard {
		t.Fatal("match arm wildcard")
	}
}

func TestFilePosFirstDecl(t *testing.T) {
	f := &File{
		Path: "test.weft",
		Decls: []Decl{
			&FnDecl{Pos_: token.Pos{Line: 3, Column: 1}},
			&FnDecl{Pos_: token.Pos{Line: 9, Column: 1}},
		},
	}
	if f.Pos().Line != 3 {
		t.Fatalf("file pos should be first decl, got line %d", f.Pos().Line)
	}
}

func TestConstDeclDualInterface(t *testing.T) {
	// ConstDecl is both a top-level declaration and a statement.
	pos := token.Pos{Line: 4, Column: 2}
	c := &ConstDecl{Pos_: pos, Name: "x"}
	var d Decl = c
	var s Stmt = c
	if d.Pos() != pos {
		t.Fatal("ConstDecl as Decl pos")
	}
	if s.Pos() != pos {
		t.Fatal("ConstDecl as Stmt pos")
	}
}

func TestEnumVariant(t *testing.T) {
	unit := EnumVariant{Name: "A"}
	if unit.Fields != nil {
		t.Fatal("unit variant should have nil fields")
	}
	payload := EnumVariant{Name: "B", Fields: []string{"x", "y"}}
	if len(payload.Fields) != 2 || payload.Fields[0] != "x" || payload.Fields[1] != "y" {
		t.Fatalf("payload fields: %v", payload.Fields)
	}
}

func TestFStringPart(t *testing.T) {
	text := FStringPart{Text: "hello"}
	if text.Expr != nil {
		t.Fatal("text part should have nil expr")
	}
	expr := FStringPart{Expr: &Ident{Name: "x"}}
	if expr.Text != "" || expr.Expr == nil {
		t.Fatal("expr part should have empty text and non-nil expr")
	}
}
