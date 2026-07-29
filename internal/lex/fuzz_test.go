package lex

import (
	"testing"

	"github.com/loreste/weft/internal/token"
)

func FuzzLex(f *testing.F) {
	for _, s := range []string{
		`fn main { say("hi") }`,
		`1_000 0xFF 0b10 1e-6`,
		`"hello $name ${1+2}"`,
		`// comment`,
		`/* block */`,
		`:= -> ?? |>`,
		string([]byte{0xff, 0xfe, 0x00}),
		``,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// Bound token count to avoid pathological hangs if the lexer stalls.
		const maxTok = 1_000_000
		l := New(src)
		for i := 0; i < maxTok; i++ {
			tok := l.Next()
			if tok.Kind == token.EOF {
				return
			}
		}
	})
}
