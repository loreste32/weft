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

func TestLookupIdentAllKeywords(t *testing.T) {
	cases := []struct {
		word string
		want Kind
	}{
		{"fn", Fn}, {"let", Let}, {"const", Const}, {"mut", Mut},
		{"if", If}, {"else", Else}, {"for", For}, {"in", In},
		{"while", While}, {"return", Return}, {"break", Break}, {"continue", Continue},
		{"type", Type}, {"struct", Struct}, {"import", Import}, {"use", Use}, {"as", As},
		{"true", True}, {"false", False}, {"null", Null}, {"pub", Pub}, {"defer", Defer},
		{"say", Say}, {"match", Match}, {"enum", Enum}, {"select", Select},
		{"try", Try}, {"catch", Catch}, {"interface", Interface},
	}
	for _, tc := range cases {
		if got := LookupIdent(tc.word); got != tc.want {
			t.Errorf("LookupIdent(%q) = %v, want %v", tc.word, got, tc.want)
		}
		// Keyword kinds print as their literal spelling.
		if tc.want.String() != tc.word {
			t.Errorf("Kind(%q).String() = %q, want %q", tc.word, tc.want.String(), tc.word)
		}
	}
}

func TestLookupIdentNonKeywords(t *testing.T) {
	// Case-sensitive, and `spawn` is a prelude builtin, not a keyword.
	for _, s := range []string{"", "FN", "Let", "TRUE", "fnx", "let_x", "spawn"} {
		if got := LookupIdent(s); got != Ident {
			t.Errorf("LookupIdent(%q) = %v, want IDENT", s, got)
		}
	}
}

func TestKindStringOperators(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{Comment, "COMMENT"},
		{RawString, "RAWSTRING"},
		{FString, "FSTRING"},
		{Assign, "="},
		{Percent, "%"},
		{Bang, "!"},
		{Lt, "<"},
		{Lte, "<="},
		{Gt, ">"},
		{Gte, ">="},
		{And, "&&"},
		{Or, "||"},
		{NullCoalesce, "??"},
		{Question, "?"},
		{Dot, "."},
		{Comma, ","},
		{Colon, ":"},
		{Semicolon, ";"},
	}
	for _, tc := range cases {
		if tc.k.String() != tc.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tc.k, tc.k.String(), tc.want)
		}
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
