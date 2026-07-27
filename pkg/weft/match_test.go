package weft_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestMatchLiteralArms(t *testing.T) {
	src := `
fn main {
    k := "text"
    msg := match k {
        "text" { "hi" }
        "done" { "bye" }
        _ { "?" }
    }
    say(msg)

    n := match 2 {
        1 { "one" }
        2 { "two" }
        _ { "other" }
    }
    say(n)

    // no match → unit; prefer trailing _
    d := match "z" {
        "a" { "A" }
        _ { "def" }
    }
    say(d)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "match.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := strings.TrimSpace(out.String())
	if s != "hi\ntwo\ndef" {
		t.Fatalf("%q", s)
	}
}

func TestMatchStreamStyle(t *testing.T) {
	// agent-style event kind switch
	src := `
fn handle(kind, text) {
    match kind {
        "text" { text }
        "done" { "[done]" }
        "error" { "err" }
        _ { "" }
    }
}

fn main {
    say(handle("text", "yo"))
    say(handle("done", ""))
    say(handle("other", "x"))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "match2.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	got := out.String()
	if !strings.HasPrefix(got, "yo\n[done]\n") {
		t.Fatalf("%q", got)
	}
}

func TestMatchBoolNull(t *testing.T) {
	src := `
fn main {
    say(match true {
        true { "T" }
        false { "F" }
        _ { "?" }
    })
    say(match null {
        null { "n" }
        _ { "x" }
    })
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "match3.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if strings.TrimSpace(out.String()) != "T\nn" {
		t.Fatalf("%q", out.String())
	}
}
