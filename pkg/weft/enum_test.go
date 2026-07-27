package weft_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestEnumAndMatchVariants(t *testing.T) {
	src := `
enum Status { Ok, Err, Pending }

fn main {
    say(Status.Ok)
    say(Status.Err)

    s := Status.Pending
    msg := match s {
        Status.Ok { "good" }
        Status.Err { "bad" }
        Status.Pending { "wait" }
        _ { "?" }
    }
    say(msg)

    // string equality still works (variants are strings)
    say(match "Ok" {
        Status.Ok { "yes" }
        _ { "no" }
    })
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "enum.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	got := strings.TrimSpace(out.String())
	if got != "Ok\nErr\nwait\nyes" {
		t.Fatalf("got %q", got)
	}
}

func TestPubEnumExport(t *testing.T) {
	// Enum used only in one file — already covered above.
	// Nested capture + enum:
	src := `
enum Kind { Text, Done }

fn main {
    k := Kind.Text
    label := "evt"
    h := fn(x) {
        match x {
            Kind.Text { label + ":text" }
            Kind.Done { label + ":done" }
            _ { label + ":?" }
        }
    }
    say(h(k))
    say(h(Kind.Done))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "enum2.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	got := strings.TrimSpace(out.String())
	if got != "evt:text\nevt:done" {
		t.Fatalf("got %q", got)
	}
}
