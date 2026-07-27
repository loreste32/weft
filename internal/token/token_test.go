package token

import "testing"

func TestLookupIdent(t *testing.T) {
	if LookupIdent("fn") != Fn {
		t.Fatal("fn")
	}
	if LookupIdent("let") != Let {
		t.Fatal("let")
	}
	if LookupIdent("myvar") != Ident {
		t.Fatal("ident")
	}
	if LookupIdent("match") != Match {
		t.Fatal("match")
	}
	if LookupIdent("enum") != Enum {
		t.Fatal("enum")
	}
	if LookupIdent("defer") != Defer {
		t.Fatal("defer")
	}
}

func TestKindString(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{Illegal, "ILLEGAL"},
		{EOF, "EOF"},
		{Ident, "IDENT"},
		{Int, "INT"},
		{Float, "FLOAT"},
		{String, "STRING"},
		{Fn, "fn"},
		{If, "if"},
		{While, "while"},
		{Return, "return"},
		{True, "true"},
		{False, "false"},
		{Null, "null"},
		{Plus, "+"},
		{Minus, "-"},
		{Star, "*"},
		{Slash, "/"},
		{Eq, "=="},
		{Neq, "!="},
		{ColonAssign, ":="},
		{Arrow, "->"},
		{Pipe, "|>"},
		{LParen, "("},
		{RParen, ")"},
		{LBrace, "{"},
		{RBrace, "}"},
		{LBracket, "["},
		{RBracket, "]"},
		{Kind(9999), "token(?)"},
	}
	for _, tc := range cases {
		if tc.k.String() != tc.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tc.k, tc.k.String(), tc.want)
		}
	}
}
