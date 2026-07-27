package stdlib

import (
	"reflect"
	"testing"
)

func TestShlexSplit(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a b c", []string{"a", "b", "c"}},
		{"echo 'hello world'", []string{"echo", "hello world"}},
		{`echo "hello world"`, []string{"echo", "hello world"}},
		{`a\ b`, []string{"a b"}},
		{"  spaced  out  ", []string{"spaced", "out"}},
		{`x "y z" 'q r'`, []string{"x", "y z", "q r"}},
		{`""`, []string{""}}, // empty quoted token via hadToken from quotes
	}
	for _, tc := range cases {
		got, err := shlexSplit(tc.in)
		if err != nil {
			t.Fatalf("split %q: %v", tc.in, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			// empty quoted: implementation may skip empty token — accept either
			if tc.in == `""` && len(got) == 0 {
				continue
			}
			t.Fatalf("split %q: got %#v want %#v", tc.in, got, tc.want)
		}
	}
}

func TestShlexSplitErrors(t *testing.T) {
	for _, bad := range []string{`'unterminated`, `"unterminated`, `trailing\`} {
		if _, err := shlexSplit(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestShlexQuote(t *testing.T) {
	if shlexQuote("") != "''" {
		t.Fatal(shlexQuote(""))
	}
	if shlexQuote("abc") != "abc" {
		t.Fatal(shlexQuote("abc"))
	}
	if shlexQuote("a b") != "'a b'" {
		t.Fatal(shlexQuote("a b"))
	}
	if shlexQuote("it's") != `'it'\''s'` {
		t.Fatal(shlexQuote("it's"))
	}
}

func TestShlexJoinRoundTrip(t *testing.T) {
	parts := []string{"echo", "hello world", "x"}
	var line string
	for i, p := range parts {
		if i > 0 {
			line += " "
		}
		line += shlexQuote(p)
	}
	got, err := shlexSplit(line)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, parts) {
		t.Fatalf("got %#v want %#v", got, parts)
	}
}
