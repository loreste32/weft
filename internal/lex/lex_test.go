package lex

import (
	"testing"

	"github.com/loreste/weft/internal/token"
)

func kinds(src string) []token.Kind {
	toks := TokenizeAll(src)
	ks := make([]token.Kind, len(toks))
	for i, t := range toks {
		ks[i] = t.Kind
	}
	return ks
}

func TestHello(t *testing.T) {
	src := `fn main() {
    println("hello, weft")
}`
	got := kinds(src)
	want := []token.Kind{
		token.Fn, token.Ident, token.LParen, token.RParen, token.LBrace,
		token.Ident, token.LParen, token.String, token.RParen,
		token.RBrace,
	}
	if len(got) != len(want) {
		t.Fatalf("len got %d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %v want %v", i, got[i], want[i])
		}
	}
}

func TestKeywordsAndOps(t *testing.T) {
	src := `let mut x = 1 + 2 * 3; if x >= 1 && x != 0 || true { return x? }`
	toks := TokenizeAll(src)
	if len(toks) < 10 {
		t.Fatalf("too few tokens: %v", toks)
	}
	if toks[0].Kind != token.Let || toks[1].Kind != token.Mut {
		t.Fatalf("expected let mut, got %v %v", toks[0], toks[1])
	}
}

func TestNumbers(t *testing.T) {
	toks := TokenizeAll("42 3.14")
	if toks[0].Kind != token.Int || toks[0].Lit != "42" {
		t.Fatalf("int: %+v", toks[0])
	}
	if toks[1].Kind != token.Float || toks[1].Lit != "3.14" {
		t.Fatalf("float: %+v", toks[1])
	}
}

func TestHexInts(t *testing.T) {
	cases := []struct {
		src string
		lit string
	}{
		{"0xff", "0xff"},
		{"0xFF", "0xFF"},
		{"0x0", "0x0"},
		{"0Xdead", "0Xdead"},
		{"0xde_ad", "0xde_ad"},
	}
	for _, c := range cases {
		toks := TokenizeAll(c.src)
		if len(toks) < 1 || toks[0].Kind != token.Int || toks[0].Lit != c.lit {
			t.Fatalf("%q → %+v want Int %q", c.src, toks, c.lit)
		}
	}
	// incomplete 0x
	toks := TokenizeAll("0x")
	if toks[0].Kind != token.Illegal {
		t.Fatalf("0x: %+v", toks[0])
	}
}

func TestBinOctAndUnderscores(t *testing.T) {
	cases := []struct {
		src  string
		kind token.Kind
		lit  string
	}{
		{"0b1010", token.Int, "0b1010"},
		{"0B11", token.Int, "0B11"},
		{"0b1010_0001", token.Int, "0b1010_0001"},
		{"0o755", token.Int, "0o755"},
		{"0O644", token.Int, "0O644"},
		{"1_000", token.Int, "1_000"},
		{"1_000_000", token.Int, "1_000_000"},
		{"3.14_15", token.Float, "3.14_15"},
		{"1_000e-3", token.Float, "1_000e-3"},
	}
	for _, c := range cases {
		toks := TokenizeAll(c.src)
		if len(toks) < 1 || toks[0].Kind != c.kind || toks[0].Lit != c.lit {
			t.Fatalf("%q → %+v want %v %q", c.src, toks, c.kind, c.lit)
		}
	}
	for _, bad := range []string{"0b", "0o", "1_"} {
		toks := TokenizeAll(bad)
		if toks[0].Kind != token.Illegal {
			t.Fatalf("%q want Illegal, got %+v", bad, toks[0])
		}
	}
}

func TestScientificFloats(t *testing.T) {
	cases := []struct {
		src string
		lit string
	}{
		{"1e6", "1e6"},
		{"1E6", "1E6"},
		{"2.5e-3", "2.5e-3"},
		{"1.0E+2", "1.0E+2"},
		{"6.02e23", "6.02e23"},
	}
	for _, c := range cases {
		toks := TokenizeAll(c.src)
		if len(toks) < 1 || toks[0].Kind != token.Float || toks[0].Lit != c.lit {
			t.Fatalf("%q → %+v want Float %q", c.src, toks, c.lit)
		}
	}
	// "1 else" must not swallow else into the number
	toks := TokenizeAll("1 else")
	if toks[0].Kind != token.Int || toks[0].Lit != "1" {
		t.Fatalf("1 else first: %+v", toks[0])
	}
	if toks[1].Kind != token.Else {
		t.Fatalf("1 else second: %+v", toks[1])
	}
}

func TestComments(t *testing.T) {
	toks := TokenizeAll("// hi\n1 /* block */ 2")
	if len(toks) != 2 || toks[0].Lit != "1" || toks[1].Lit != "2" {
		t.Fatalf("got %v", toks)
	}
}

func TestFString(t *testing.T) {
	toks := TokenizeAll(`f"hello {name}"`)
	if len(toks) != 1 || toks[0].Kind != token.FString {
		t.Fatalf("got %v", toks)
	}
	if toks[0].Lit != "hello {name}" {
		t.Fatalf("lit %q", toks[0].Lit)
	}
}

func TestStringEscapes(t *testing.T) {
	toks := TokenizeAll(`"a\nb"`)
	if toks[0].Lit != "a\nb" {
		t.Fatalf("got %q", toks[0].Lit)
	}
}

func TestImportAndArrow(t *testing.T) {
	toks := TokenizeAll(`import http fn f() -> int`)
	if toks[0].Kind != token.Import || toks[1].Kind != token.Ident {
		t.Fatalf("%v", toks)
	}
	// find arrow
	found := false
	for _, tk := range toks {
		if tk.Kind == token.Arrow {
			found = true
		}
	}
	if !found {
		t.Fatal("missing arrow")
	}
}
