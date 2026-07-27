// Package ast defines the Weft abstract syntax tree.
package ast

import "github.com/loreste/weft/internal/token"

// Node is any AST node.
type Node interface {
	Pos() token.Pos
}

// File is a complete Weft source file.
type File struct {
	Path  string
	Decls []Decl
}

func (f *File) Pos() token.Pos {
	if len(f.Decls) == 0 {
		return token.Pos{}
	}
	return f.Decls[0].Pos()
}

// Decl is a top-level declaration.
type Decl interface {
	Node
	declNode()
}

// ImportDecl: import http | import "./x.weft" [as name]
type ImportDecl struct {
	Pos_   token.Pos
	Path   string // bare package name or path string
	IsPath bool   // true if string path
	Alias  string // optional
}

func (d *ImportDecl) Pos() token.Pos { return d.Pos_ }
func (d *ImportDecl) declNode()      {}

// TypeDecl: type Name { fields } | type Name = TypeExpr
type TypeDecl struct {
	Pos_   token.Pos
	Pub    bool
	Name   string
	Alias  TypeExpr // if non-nil, alias form
	Fields []*Field // if struct form
}

func (d *TypeDecl) Pos() token.Pos { return d.Pos_ }
func (d *TypeDecl) declNode()      {}

// Field is a struct field.
type Field struct {
	Pos_    token.Pos
	Name    string
	Type    TypeExpr
	Default Expr // optional
}

func (f *Field) Pos() token.Pos { return f.Pos_ }

// FnDecl is a function declaration.
type FnDecl struct {
	Pos_   token.Pos
	Pub    bool
	Name   string
	Params []*Param
	Ret    TypeExpr // optional
	Body   *Block
}

func (d *FnDecl) Pos() token.Pos { return d.Pos_ }
func (d *FnDecl) declNode()      {}

// Param is a function parameter.
type Param struct {
	Pos_ token.Pos
	Name string
	Type TypeExpr
}

func (p *Param) Pos() token.Pos { return p.Pos_ }

// ConstDecl is a top-level or statement-level const.
type ConstDecl struct {
	Pos_  token.Pos
	Name  string
	Type  TypeExpr
	Value Expr
}

func (d *ConstDecl) Pos() token.Pos { return d.Pos_ }
func (d *ConstDecl) declNode()      {}
func (d *ConstDecl) stmtNode()      {}

// EnumDecl: enum Name { A, B, C } — string-tagged names as a map (Name.A == "A").
type EnumDecl struct {
	Pos_     token.Pos
	Pub      bool
	Name     string
	Variants []string
}

func (d *EnumDecl) Pos() token.Pos { return d.Pos_ }
func (d *EnumDecl) declNode()      {}

// --- Types ---

// TypeExpr is a type expression.
type TypeExpr interface {
	Node
	typeNode()
}

type NamedType struct {
	Pos_ token.Pos
	Name string
}

func (t *NamedType) Pos() token.Pos { return t.Pos_ }
func (t *NamedType) typeNode()      {}

type ListType struct {
	Pos_    token.Pos
	Element TypeExpr
}

func (t *ListType) Pos() token.Pos { return t.Pos_ }
func (t *ListType) typeNode()      {}

type MapType struct {
	Pos_  token.Pos
	Key   TypeExpr
	Value TypeExpr
}

func (t *MapType) Pos() token.Pos { return t.Pos_ }
func (t *MapType) typeNode()      {}

type ResultType struct {
	Pos_ token.Pos
	Ok   TypeExpr
}

func (t *ResultType) Pos() token.Pos { return t.Pos_ }
func (t *ResultType) typeNode()      {}

type OptionalType struct {
	Pos_    token.Pos
	Element TypeExpr
}

func (t *OptionalType) Pos() token.Pos { return t.Pos_ }
func (t *OptionalType) typeNode()      {}

type StructType struct {
	Pos_   token.Pos
	Fields []*Field
}

func (t *StructType) Pos() token.Pos { return t.Pos_ }
func (t *StructType) typeNode()      {}

// --- Statements ---

// Stmt is a statement.
type Stmt interface {
	Node
	stmtNode()
}

// Block is a brace block.
type Block struct {
	Pos_  token.Pos
	Stmts []Stmt
}

func (b *Block) Pos() token.Pos { return b.Pos_ }
func (b *Block) stmtNode()      {}

// LetStmt: let [mut] name [: type] = expr
type LetStmt struct {
	Pos_ token.Pos
	Mut  bool
	Name string
	Type TypeExpr
	Init Expr
}

func (s *LetStmt) Pos() token.Pos { return s.Pos_ }
func (s *LetStmt) stmtNode()      {}

// AssignStmt: target = expr
type AssignStmt struct {
	Pos_   token.Pos
	Target Expr // Ident, Field, Index
	Value  Expr
}

func (s *AssignStmt) Pos() token.Pos { return s.Pos_ }
func (s *AssignStmt) stmtNode()      {}

// ReturnStmt
type ReturnStmt struct {
	Pos_  token.Pos
	Value Expr // optional
}

func (s *ReturnStmt) Pos() token.Pos { return s.Pos_ }
func (s *ReturnStmt) stmtNode()      {}

// IfStmt
type IfStmt struct {
	Pos_ token.Pos
	Cond Expr
	Then *Block
	Else Stmt // *Block or *IfStmt or nil
}

func (s *IfStmt) Pos() token.Pos { return s.Pos_ }
func (s *IfStmt) stmtNode()      {}

// WhileStmt
type WhileStmt struct {
	Pos_ token.Pos
	Cond Expr
	Body *Block
}

func (s *WhileStmt) Pos() token.Pos { return s.Pos_ }
func (s *WhileStmt) stmtNode()      {}

// ForStmt: for name in expr { }
type ForStmt struct {
	Pos_ token.Pos
	Name string
	Iter Expr
	Body *Block
}

func (s *ForStmt) Pos() token.Pos { return s.Pos_ }
func (s *ForStmt) stmtNode()      {}

// BreakStmt / ContinueStmt
type BreakStmt struct{ Pos_ token.Pos }

func (s *BreakStmt) Pos() token.Pos { return s.Pos_ }
func (s *BreakStmt) stmtNode()      {}

type ContinueStmt struct{ Pos_ token.Pos }

func (s *ContinueStmt) Pos() token.Pos { return s.Pos_ }
func (s *ContinueStmt) stmtNode()      {}

// DeferStmt
type DeferStmt struct {
	Pos_ token.Pos
	Call Expr
}

func (s *DeferStmt) Pos() token.Pos { return s.Pos_ }
func (s *DeferStmt) stmtNode()      {}

// ExprStmt
type ExprStmt struct {
	Pos_ token.Pos
	X    Expr
}

func (s *ExprStmt) Pos() token.Pos { return s.Pos_ }
func (s *ExprStmt) stmtNode()      {}

// --- Expressions ---

// Expr is an expression.
type Expr interface {
	Node
	exprNode()
}

// Ident
type Ident struct {
	Pos_ token.Pos
	Name string
}

func (e *Ident) Pos() token.Pos { return e.Pos_ }
func (e *Ident) exprNode()      {}

// BasicLit
type BasicLit struct {
	Pos_  token.Pos
	Kind  token.Kind // Int, Float, String, RawString, True, False, Null
	Value string
}

func (e *BasicLit) Pos() token.Pos { return e.Pos_ }
func (e *BasicLit) exprNode()      {}

// BinaryExpr
type BinaryExpr struct {
	Pos_ token.Pos
	Op   token.Kind
	X, Y Expr
}

func (e *BinaryExpr) Pos() token.Pos { return e.Pos_ }
func (e *BinaryExpr) exprNode()      {}

// UnaryExpr
type UnaryExpr struct {
	Pos_ token.Pos
	Op   token.Kind
	X    Expr
}

func (e *UnaryExpr) Pos() token.Pos { return e.Pos_ }
func (e *UnaryExpr) exprNode()      {}

// CallExpr
type CallExpr struct {
	Pos_ token.Pos
	Fun  Expr
	Args []Expr
}

func (e *CallExpr) Pos() token.Pos { return e.Pos_ }
func (e *CallExpr) exprNode()      {}

// IndexExpr
type IndexExpr struct {
	Pos_  token.Pos
	X     Expr
	Index Expr
}

func (e *IndexExpr) Pos() token.Pos { return e.Pos_ }
func (e *IndexExpr) exprNode()      {}

// FieldExpr: x.field
type FieldExpr struct {
	Pos_ token.Pos
	X    Expr
	Name string
}

func (e *FieldExpr) Pos() token.Pos { return e.Pos_ }
func (e *FieldExpr) exprNode()      {}

// QuestionExpr: x?
type QuestionExpr struct {
	Pos_ token.Pos
	X    Expr
}

func (e *QuestionExpr) Pos() token.Pos { return e.Pos_ }
func (e *QuestionExpr) exprNode()      {}

// ListLit
type ListLit struct {
	Pos_ token.Pos
	Elts []Expr
}

func (e *ListLit) Pos() token.Pos { return e.Pos_ }
func (e *ListLit) exprNode()      {}

// MapLit
type MapLit struct {
	Pos_ token.Pos
	Keys []Expr
	Vals []Expr
}

func (e *MapLit) Pos() token.Pos { return e.Pos_ }
func (e *MapLit) exprNode()      {}

// StructLit: Name{ field: expr, ... }
type StructLit struct {
	Pos_   token.Pos
	Name   string // type name
	Fields []StructField
}

// StructField is a named field init.
type StructField struct {
	Pos_  token.Pos
	Name  string
	Value Expr
}

func (e *StructLit) Pos() token.Pos { return e.Pos_ }
func (e *StructLit) exprNode()      {}

// FStringExpr: interpolations of text and expressions
type FStringExpr struct {
	Pos_  token.Pos
	Parts []FStringPart
}

// FStringPart is either literal text or an expression.
type FStringPart struct {
	Text string // if non-empty, literal
	Expr Expr   // if non-nil, expression
}

func (e *FStringExpr) Pos() token.Pos { return e.Pos_ }
func (e *FStringExpr) exprNode()      {}

// IfExpr: if cond { … } else { … } as a value (RHS of :=, args, …).
// Then/Else yield last expression; Else may be *Block, *IfExpr, or nil.
type IfExpr struct {
	Pos_ token.Pos
	Cond Expr
	Then *Block
	Else any
}

func (e *IfExpr) Pos() token.Pos { return e.Pos_ }
func (e *IfExpr) exprNode()      {}

// MatchExpr: match scrut { pat { body } … } as a value (also fine as a statement).
// Patterns: literals, `_`, idents/consts, or field access (enum Status.Ok). First match wins.
type MatchExpr struct {
	Pos_      token.Pos
	Scrutinee Expr
	Arms      []MatchArm
}

func (e *MatchExpr) Pos() token.Pos { return e.Pos_ }
func (e *MatchExpr) exprNode()      {}

// MatchArm is one pattern → body branch.
type MatchArm struct {
	Pos_       token.Pos
	Pattern    Expr // nil when Wildcard
	Body       *Block
	IsWildcard bool
}

// FuncLit: anonymous fn
type FuncLit struct {
	Pos_   token.Pos
	Params []*Param
	Ret    TypeExpr
	Body   *Block
}

func (e *FuncLit) Pos() token.Pos { return e.Pos_ }
func (e *FuncLit) exprNode()      {}
