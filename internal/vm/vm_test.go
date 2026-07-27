package vm_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/vm"
	"github.com/loreste/weft/pkg/weft"
)

func TestStackTraceOnError(t *testing.T) {
	src := `
fn inner() {
    1 / 0
}
fn mid() {
    inner()
}
fn main {
    mid()
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	err := ctx.RunSource(context.Background(), "stack.weft", src)
	if err == nil {
		t.Fatal("want div zero error")
	}
	es := err.Error()
	// should mention stack / functions
	if !strings.Contains(es, "inner") && !strings.Contains(es, "stack") && !strings.Contains(es, "division") {
		t.Fatalf("want stackful error, got %q", es)
	}
	// RuntimeError type
	if re, ok := err.(*vm.RuntimeError); ok {
		if len(re.Stack) < 2 {
			t.Fatalf("want multi-frame stack, got %+v", re.Stack)
		}
	} else {
		// may be wrapped - still require division message
		if !strings.Contains(es, "division") {
			t.Fatalf("%T %q", err, es)
		}
	}
}

func TestUndefinedNameHasLocation(t *testing.T) {
	ctx := weft.New(weft.Options{})
	err := ctx.RunSource(context.Background(), "u.weft", `fn main { missing_name }`)
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "missing_name") {
		t.Fatal(err)
	}
}
