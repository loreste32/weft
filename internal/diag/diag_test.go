package diag

import (
	"testing"

	"github.com/loreste/weft/internal/token"
)

func TestDiagnosticString(t *testing.T) {
	d := Diagnostic{
		Severity: Error,
		File:     "test.weft",
		Pos:      token.Pos{Line: 5, Column: 3},
		Message:  "bad thing",
	}
	s := d.String()
	if s != "test.weft:5:3: error: bad thing" {
		t.Fatalf("got %q", s)
	}
}

func TestDiagnosticStringWarning(t *testing.T) {
	d := Diagnostic{
		Severity: Warning,
		File:     "x.weft",
		Pos:      token.Pos{Line: 1, Column: 1},
		Message:  "unused",
	}
	if d.String() != "x.weft:1:1: warning: unused" {
		t.Fatalf("got %q", d.String())
	}
}

func TestDiagnosticStringNoFile(t *testing.T) {
	d := Diagnostic{
		Severity: Error,
		Pos:      token.Pos{Line: 3, Column: 2},
		Message:  "oops",
	}
	if d.String() != "3:2: error: oops" {
		t.Fatalf("got %q", d.String())
	}
}

func TestDiagnosticStringNoPos(t *testing.T) {
	d := Diagnostic{
		Severity: Error,
		File:     "x.weft",
		Message:  "oops",
	}
	if d.String() != "x.weft:error: oops" {
		t.Fatalf("got %q", d.String())
	}
}

func TestListError(t *testing.T) {
	l := List{
		Errorf("a.weft", token.Pos{Line: 1, Column: 1}, "first"),
		Errorf("b.weft", token.Pos{Line: 2, Column: 1}, "second"),
	}
	s := l.Error()
	if s == "" {
		t.Fatal("should not be empty")
	}
}

func TestListErrorEmpty(t *testing.T) {
	l := List{}
	if l.Error() != "" {
		t.Fatal("empty list should return empty string")
	}
}

func TestHasErrors(t *testing.T) {
	l := List{
		{Severity: Warning, Message: "warn"},
	}
	if l.HasErrors() {
		t.Fatal("warnings only should not have errors")
	}
	l = append(l, Diagnostic{Severity: Error, Message: "err"})
	if !l.HasErrors() {
		t.Fatal("should have errors")
	}
}

func TestErrorf(t *testing.T) {
	d := Errorf("f.weft", token.Pos{Line: 1, Column: 1}, "bad %d", 42)
	if d.Message != "bad 42" {
		t.Fatal("Errorf formatting")
	}
	if d.Severity != Error {
		t.Fatal("Errorf severity")
	}
}

func TestWarnf(t *testing.T) {
	d := Warnf("f.weft", token.Pos{Line: 2, Column: 5}, "maybe %s", "wrong")
	if d.Severity != Warning {
		t.Fatal("Warnf severity")
	}
	if d.Message != "maybe wrong" {
		t.Fatalf("Warnf message: %q", d.Message)
	}
	if d.File != "f.weft" || d.Pos.Line != 2 || d.Pos.Column != 5 {
		t.Fatal("Warnf file/pos")
	}
}

func TestDiagnosticStringWithSource(t *testing.T) {
	d := Diagnostic{
		Severity: Error,
		File:     "x.weft",
		Pos:      token.Pos{Line: 2, Column: 4},
		Message:  "bad",
		Source:   "let x = 1",
	}
	want := "x.weft:2:4: error: bad\n  let x = 1\n     ^"
	if d.String() != want {
		t.Fatalf("got %q want %q", d.String(), want)
	}
}

func TestDiagnosticStringSourceCaretOutOfRange(t *testing.T) {
	// Column beyond the source line: source shown, caret line omitted.
	d := Diagnostic{
		Severity: Error,
		File:     "x.weft",
		Pos:      token.Pos{Line: 1, Column: 99},
		Message:  "bad",
		Source:   "say(1)",
	}
	want := "x.weft:1:99: error: bad\n  say(1)"
	if d.String() != want {
		t.Fatalf("got %q want %q", d.String(), want)
	}
}

func TestListErrorJoinsWithNewline(t *testing.T) {
	l := List{
		Errorf("a.weft", token.Pos{Line: 1, Column: 1}, "first"),
		Errorf("b.weft", token.Pos{Line: 2, Column: 1}, "second"),
	}
	want := "a.weft:1:1: error: first\nb.weft:2:1: error: second"
	if l.Error() != want {
		t.Fatalf("got %q want %q", l.Error(), want)
	}
}

func TestAttachSource(t *testing.T) {
	src := "first line\nsecond line"
	l := List{
		Errorf("x.weft", token.Pos{Line: 2, Column: 1}, "in range"),
		Errorf("x.weft", token.Pos{Line: 99, Column: 1}, "out of range"),
		Errorf("x.weft", token.Pos{}, "no pos"),
	}
	l = l.AttachSource(src)
	if l[0].Source != "second line" {
		t.Fatalf("AttachSource in range: %q", l[0].Source)
	}
	if l[1].Source != "" {
		t.Fatalf("AttachSource out of range should stay empty: %q", l[1].Source)
	}
	if l[2].Source != "" {
		t.Fatalf("AttachSource no pos should stay empty: %q", l[2].Source)
	}
}
