package parse

import "testing"

func FuzzParseFile(f *testing.F) {
	seeds := []string{
		`fn main { say(1) }`,
		`fn add(a: int, b: int) -> int { a + b }`,
		`type U { name: str, age: int? }`,
		`fn main -> Result { json.parse("{}")? }`,
		`fn main { for x in [1,2] { say(x) } }`,
		`fn main { match x { 1 { } _ { } } }`,
		`<<<garbage`,
		``,
		string([]byte{0, 1, 2, 255}),
		`fn main { "unterminated`,
		`pub fn f() { }`,
		`use http\nfn main { }`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// Must not panic; errors are fine.
		_, _ = ParseFile("fuzz.weft", src)
	})
}
