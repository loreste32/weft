// Package token defines Weft lexical tokens.
package token

// Kind is the lexical token kind.
type Kind int

const (
	Illegal Kind = iota
	EOF
	Comment // kept only if requested; normally skipped

	// Identifiers and literals
	Ident
	Int
	Float
	String
	RawString
	FString // full f"..." as one token; parser re-lexes interpolations

	// Keywords
	Fn
	Let
	Const
	Mut
	If
	Else
	For
	In
	While
	Return
	Break
	Continue
	Type
	Struct
	Import
	Use // preferred package keyword (import is alias)
	As
	True
	False
	Null
	Pub
	Defer
	Say // statement-style print keyword (optional parens)

	// Reserved (later) — lexed as keywords so they cannot be used as idents
	Match
	Enum
	Select
	Try
	Catch
	Interface
	// Note: `spawn` is a prelude builtin (not a keyword) for a non-verbose API.

	// Operators and punctuation
	Assign       // =
	Plus         // +
	Minus        // -
	Star         // *
	Slash        // /
	Percent      // %
	Bang         // !
	Eq           // ==
	Neq          // !=
	Lt           // <
	Lte          // <=
	Gt           // >
	Gte          // >=
	And          // &&
	Or           // ||
	NullCoalesce // ??
	Question     // ?
	Dot          // .
	Comma        // ,
	Colon        // :
	ColonAssign  // :=
	Semicolon    // ;
	Arrow        // ->
	Pipe         // |>  (pipeline, future-friendly; recognized)
	LParen       // (
	RParen       // )
	LBrace       // {
	RBrace       // }
	LBracket     // [
	RBracket     // ]
)

// Pos is a 1-based source position.
type Pos struct {
	Line   int
	Column int
	Offset int // 0-based byte offset
}

// Token is a single lexical token.
type Token struct {
	Kind Kind
	Lit  string
	Pos  Pos
}

var keywords = map[string]Kind{
	"fn":        Fn,
	"let":       Let,
	"const":     Const,
	"mut":       Mut,
	"if":        If,
	"else":      Else,
	"for":       For,
	"in":        In,
	"while":     While,
	"return":    Return,
	"break":     Break,
	"continue":  Continue,
	"type":      Type,
	"struct":    Struct,
	"import":    Import,
	"use":       Use,
	"as":        As,
	"true":      True,
	"false":     False,
	"null":      Null,
	"pub":       Pub,
	"defer":     Defer,
	"say":       Say,
	"match":     Match,
	"enum":      Enum,
	"select":    Select,
	"try":       Try,
	"catch":     Catch,
	"interface": Interface,
}

// LookupIdent returns the keyword kind or Ident.
func LookupIdent(s string) Kind {
	if k, ok := keywords[s]; ok {
		return k
	}
	return Ident
}

func (k Kind) String() string {
	names := map[Kind]string{
		Illegal: "ILLEGAL", EOF: "EOF", Comment: "COMMENT",
		Ident: "IDENT", Int: "INT", Float: "FLOAT", String: "STRING",
		RawString: "RAWSTRING", FString: "FSTRING",
		Fn: "fn", Let: "let", Const: "const", Mut: "mut",
		If: "if", Else: "else", For: "for", In: "in", While: "while",
		Return: "return", Break: "break", Continue: "continue",
		Type: "type", Struct: "struct", Import: "import", Use: "use", As: "as",
		True: "true", False: "false", Null: "null", Pub: "pub", Defer: "defer", Say: "say",
		Match: "match", Enum: "enum", Select: "select",
		Try: "try", Catch: "catch", Interface: "interface",
		Assign: "=", Plus: "+", Minus: "-", Star: "*", Slash: "/", Percent: "%",
		Bang: "!", Eq: "==", Neq: "!=", Lt: "<", Lte: "<=", Gt: ">", Gte: ">=",
		And: "&&", Or: "||", NullCoalesce: "??", Question: "?",
		Dot: ".", Comma: ",", Colon: ":", ColonAssign: ":=", Semicolon: ";", Arrow: "->", Pipe: "|>",
		LParen: "(", RParen: ")", LBrace: "{", RBrace: "}",
		LBracket: "[", RBracket: "]",
	}
	if s, ok := names[k]; ok {
		return s
	}
	return "token(?)"
}
