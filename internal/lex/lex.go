// Package lex implements the Weft lexer.
package lex

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/loreste/weft/internal/token"
)

// Lexer tokenizes Weft source.
type Lexer struct {
	src  string
	pos  int // current byte offset
	line int
	col  int
	ch   rune
}

// New creates a lexer over src.
func New(src string) *Lexer {
	l := &Lexer{src: src, line: 1, col: 0}
	l.advance()
	return l
}

func (l *Lexer) advance() {
	if l.pos >= len(l.src) {
		l.ch = 0
		return
	}
	r, size := utf8.DecodeRuneInString(l.src[l.pos:])
	if r == utf8.RuneError && size == 1 {
		l.ch = r
		l.pos += size
		l.col++
		return
	}
	if l.ch == '\n' {
		l.line++
		l.col = 0
	}
	l.pos += size
	l.col++
	l.ch = r
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.pos:])
	return r
}

func (l *Lexer) curPos() token.Pos {
	// col is the column of current char; for token start we want current position
	return token.Pos{Line: l.line, Column: l.col, Offset: l.pos - runeSize(l.ch)}
}

func runeSize(r rune) int {
	if r == 0 {
		return 0
	}
	return utf8.RuneLen(r)
}

// Next returns the next non-comment token.
func (l *Lexer) Next() token.Token {
	for {
		l.skipSpace()
		if l.ch == 0 {
			return token.Token{Kind: token.EOF, Pos: token.Pos{Line: l.line, Column: l.col, Offset: l.pos}}
		}
		// line comment
		if l.ch == '/' && l.peek() == '/' {
			l.skipLineComment()
			continue
		}
		// block comment
		if l.ch == '/' && l.peek() == '*' {
			if err := l.skipBlockComment(); err != nil {
				return token.Token{Kind: token.Illegal, Lit: err.Error(), Pos: l.curPos()}
			}
			continue
		}
		break
	}

	start := token.Pos{Line: l.line, Column: l.col, Offset: l.pos - runeSize(l.ch)}

	switch l.ch {
	case '(':
		l.advance()
		return token.Token{Kind: token.LParen, Lit: "(", Pos: start}
	case ')':
		l.advance()
		return token.Token{Kind: token.RParen, Lit: ")", Pos: start}
	case '{':
		l.advance()
		return token.Token{Kind: token.LBrace, Lit: "{", Pos: start}
	case '}':
		l.advance()
		return token.Token{Kind: token.RBrace, Lit: "}", Pos: start}
	case '[':
		l.advance()
		return token.Token{Kind: token.LBracket, Lit: "[", Pos: start}
	case ']':
		l.advance()
		return token.Token{Kind: token.RBracket, Lit: "]", Pos: start}
	case ',':
		l.advance()
		return token.Token{Kind: token.Comma, Lit: ",", Pos: start}
	case ':':
		if l.peek() == '=' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.ColonAssign, Lit: ":=", Pos: start}
		}
		l.advance()
		return token.Token{Kind: token.Colon, Lit: ":", Pos: start}
	case '|':
		if l.peek() == '>' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.Pipe, Lit: "|>", Pos: start}
		}
		if l.peek() == '|' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.Or, Lit: "||", Pos: start}
		}
		return token.Token{Kind: token.Illegal, Lit: "|", Pos: start}
	case ';':
		l.advance()
		return token.Token{Kind: token.Semicolon, Lit: ";", Pos: start}
	case '.':
		l.advance()
		return token.Token{Kind: token.Dot, Lit: ".", Pos: start}
	case '+':
		l.advance()
		return token.Token{Kind: token.Plus, Lit: "+", Pos: start}
	case '*':
		l.advance()
		return token.Token{Kind: token.Star, Lit: "*", Pos: start}
	case '%':
		l.advance()
		return token.Token{Kind: token.Percent, Lit: "%", Pos: start}
	case '-':
		if l.peek() == '>' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.Arrow, Lit: "->", Pos: start}
		}
		l.advance()
		return token.Token{Kind: token.Minus, Lit: "-", Pos: start}
	case '=':
		if l.peek() == '=' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.Eq, Lit: "==", Pos: start}
		}
		l.advance()
		return token.Token{Kind: token.Assign, Lit: "=", Pos: start}
	case '!':
		if l.peek() == '=' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.Neq, Lit: "!=", Pos: start}
		}
		l.advance()
		return token.Token{Kind: token.Bang, Lit: "!", Pos: start}
	case '<':
		if l.peek() == '=' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.Lte, Lit: "<=", Pos: start}
		}
		l.advance()
		return token.Token{Kind: token.Lt, Lit: "<", Pos: start}
	case '>':
		if l.peek() == '=' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.Gte, Lit: ">=", Pos: start}
		}
		l.advance()
		return token.Token{Kind: token.Gt, Lit: ">", Pos: start}
	case '&':
		if l.peek() == '&' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.And, Lit: "&&", Pos: start}
		}
		return token.Token{Kind: token.Illegal, Lit: "&", Pos: start}

	case '?':
		if l.peek() == '?' {
			l.advance()
			l.advance()
			return token.Token{Kind: token.NullCoalesce, Lit: "??", Pos: start}
		}
		l.advance()
		return token.Token{Kind: token.Question, Lit: "?", Pos: start}
	case '/':
		l.advance()
		return token.Token{Kind: token.Slash, Lit: "/", Pos: start}
	case '"':
		return l.scanString(start)
	case '`':
		return l.scanRawString(start)
	case 'f':
		// f-string: f"..."
		if l.peek() == '"' {
			return l.scanFString(start)
		}
		return l.scanIdent(start)
	}

	if isIdentStart(l.ch) {
		return l.scanIdent(start)
	}
	if unicode.IsDigit(l.ch) {
		return l.scanNumber(start)
	}

	ch := l.ch
	l.advance()
	return token.Token{Kind: token.Illegal, Lit: string(ch), Pos: start}
}

func (l *Lexer) skipSpace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.advance()
	}
}

func (l *Lexer) skipLineComment() {
	for l.ch != 0 && l.ch != '\n' {
		l.advance()
	}
}

func (l *Lexer) skipBlockComment() error {
	l.advance() // /
	l.advance() // *
	for {
		if l.ch == 0 {
			return fmt.Errorf("unterminated block comment")
		}
		if l.ch == '*' && l.peek() == '/' {
			l.advance()
			l.advance()
			return nil
		}
		l.advance()
	}
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (l *Lexer) mark() int {
	if l.ch == 0 {
		return l.pos
	}
	return l.pos - runeSize(l.ch)
}

func (l *Lexer) sliceFrom(begin int) string {
	end := l.mark()
	if end < begin {
		end = begin
	}
	return l.src[begin:end]
}

func (l *Lexer) scanIdent(start token.Pos) token.Token {
	begin := l.mark()
	for isIdentPart(l.ch) {
		l.advance()
	}
	lit := l.sliceFrom(begin)
	return token.Token{Kind: token.LookupIdent(lit), Lit: lit, Pos: start}
}

func (l *Lexer) scanNumber(start token.Pos) token.Token {
	begin := l.mark()
	// Prefixed integers: 0x hex, 0b binary, 0o octal (optional _ between digits).
	if l.ch == '0' {
		p := l.peek()
		switch p {
		case 'x', 'X':
			return l.scanPrefixedInt(start, begin, isHexDigit, "hex")
		case 'b', 'B':
			return l.scanPrefixedInt(start, begin, isBinDigit, "binary")
		case 'o', 'O':
			return l.scanPrefixedInt(start, begin, isOctDigit, "octal")
		}
	}

	isFloat := false
	_ = l.scanDigitsWithSep(unicode.IsDigit)
	if l.ch == '.' && unicode.IsDigit(l.peek()) {
		// 3.14 — fraction must start with a digit (not 3._14)
		isFloat = true
		l.advance() // .
		l.scanDigitsWithSep(unicode.IsDigit)
	}
	// Scientific exponent: 1e6, 2.5E-3, 1_000e+3 (always a float).
	if l.ch == 'e' || l.ch == 'E' {
		pos, line, col, ch := l.pos, l.line, l.col, l.ch
		l.advance() // e/E
		if l.ch == '+' || l.ch == '-' {
			l.advance()
		}
		if l.scanDigitsWithSep(unicode.IsDigit) {
			isFloat = true
		} else {
			// not a valid exponent — leave 'e' for the next token (e.g. 1 else)
			l.pos, l.line, l.col, l.ch = pos, line, col, ch
		}
	}
	// trailing underscore after digits is illegal (1_)
	if l.ch == '_' {
		return token.Token{Kind: token.Illegal, Lit: "number cannot end with _", Pos: start}
	}
	lit := l.sliceFrom(begin)
	kind := token.Int
	if isFloat {
		kind = token.Float
	}
	return token.Token{Kind: kind, Lit: lit, Pos: start}
}

func (l *Lexer) scanPrefixedInt(start token.Pos, begin int, digit func(rune) bool, kind string) token.Token {
	l.advance() // 0
	l.advance() // x/b/o
	if !l.scanDigitsWithSep(digit) {
		return token.Token{Kind: token.Illegal, Lit: "incomplete " + kind + " number", Pos: start}
	}
	lit := l.sliceFrom(begin)
	if strings.HasSuffix(lit, "_") {
		return token.Token{Kind: token.Illegal, Lit: "number cannot end with _", Pos: start}
	}
	return token.Token{Kind: token.Int, Lit: lit, Pos: start}
}

// scanDigitsWithSep consumes digit ( _ digit )*  — underscores only between digits.
func (l *Lexer) scanDigitsWithSep(digit func(rune) bool) bool {
	got := false
	for {
		if digit(l.ch) {
			got = true
			l.advance()
			continue
		}
		if l.ch == '_' && digit(l.peek()) {
			l.advance() // skip _
			continue
		}
		break
	}
	return got
}

func isHexDigit(r rune) bool {
	return unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isBinDigit(r rune) bool { return r == '0' || r == '1' }

func isOctDigit(r rune) bool { return r >= '0' && r <= '7' }

func (l *Lexer) scanString(start token.Pos) token.Token {
	l.advance() // opening "
	var b []byte
	for {
		if l.ch == 0 || l.ch == '\n' {
			return token.Token{Kind: token.Illegal, Lit: "unterminated string", Pos: start}
		}
		if l.ch == '"' {
			l.advance()
			return token.Token{Kind: token.String, Lit: string(b), Pos: start}
		}
		if l.ch == '\\' {
			l.advance()
			switch l.ch {
			case 'n':
				b = append(b, '\n')
			case 't':
				b = append(b, '\t')
			case 'r':
				b = append(b, '\r')
			case '\\', '"':
				b = append(b, byte(l.ch))
			case '{', '}':
				// literal braces in interpolating strings
				b = append(b, byte(l.ch))
			case '0':
				b = append(b, 0)
			default:
				b = append(b, '\\')
				if l.ch != 0 {
					// append rune as utf8
					var buf [utf8.UTFMax]byte
					n := utf8.EncodeRune(buf[:], l.ch)
					b = append(b, buf[:n]...)
				}
			}
			l.advance()
			continue
		}
		var buf [utf8.UTFMax]byte
		n := utf8.EncodeRune(buf[:], l.ch)
		b = append(b, buf[:n]...)
		l.advance()
	}
}

func (l *Lexer) scanRawString(start token.Pos) token.Token {
	l.advance() // `
	var b []byte
	for {
		if l.ch == 0 {
			return token.Token{Kind: token.Illegal, Lit: "unterminated raw string", Pos: start}
		}
		if l.ch == '`' {
			l.advance()
			return token.Token{Kind: token.RawString, Lit: string(b), Pos: start}
		}
		var buf [utf8.UTFMax]byte
		n := utf8.EncodeRune(buf[:], l.ch)
		b = append(b, buf[:n]...)
		l.advance()
	}
}

func (l *Lexer) scanFString(start token.Pos) token.Token {
	// consume f
	l.advance()
	// then scan like string but keep as FString with raw content including braces
	if l.ch != '"' {
		return token.Token{Kind: token.Illegal, Lit: "expected \" after f", Pos: start}
	}
	l.advance()
	var b []byte
	for {
		if l.ch == 0 || l.ch == '\n' {
			return token.Token{Kind: token.Illegal, Lit: "unterminated f-string", Pos: start}
		}
		if l.ch == '"' {
			l.advance()
			return token.Token{Kind: token.FString, Lit: string(b), Pos: start}
		}
		if l.ch == '\\' {
			l.advance()
			switch l.ch {
			case 'n':
				b = append(b, '\n')
			case 't':
				b = append(b, '\t')
			case 'r':
				b = append(b, '\r')
			case '\\', '"', '{', '}':
				b = append(b, byte(l.ch))
			default:
				b = append(b, '\\')
				if l.ch != 0 {
					var buf [utf8.UTFMax]byte
					n := utf8.EncodeRune(buf[:], l.ch)
					b = append(b, buf[:n]...)
				}
			}
			l.advance()
			continue
		}
		var buf [utf8.UTFMax]byte
		n := utf8.EncodeRune(buf[:], l.ch)
		b = append(b, buf[:n]...)
		l.advance()
	}
}

// TokenizeAll returns all tokens until EOF (excluding EOF).
func TokenizeAll(src string) []token.Token {
	l := New(src)
	var out []token.Token
	for {
		t := l.Next()
		if t.Kind == token.EOF {
			break
		}
		out = append(out, t)
		if t.Kind == token.Illegal {
			break
		}
	}
	return out
}
