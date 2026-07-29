// Package diag provides diagnostics (errors/warnings) with positions.
package diag

import (
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/token"
)

// Severity of a diagnostic.
type Severity int

const (
	Error Severity = iota
	Warning
)

// Diagnostic is a single compiler message.
type Diagnostic struct {
	Severity Severity
	Pos      token.Pos
	Message  string
	File     string
	Source   string // source line (set by FormatWithSource)
}

func (d Diagnostic) String() string {
	sev := "error"
	if d.Severity == Warning {
		sev = "warning"
	}
	loc := ""
	if d.File != "" {
		loc = d.File + ":"
	}
	if d.Pos.Line > 0 {
		loc += fmt.Sprintf("%d:%d: ", d.Pos.Line, d.Pos.Column)
	}
	msg := fmt.Sprintf("%s%s: %s", loc, sev, d.Message)
	if d.Source != "" {
		msg += "\n  " + d.Source
		if d.Pos.Column > 0 && d.Pos.Column <= len(d.Source)+1 {
			msg += "\n  " + strings.Repeat(" ", d.Pos.Column-1) + "^"
		}
	}
	return msg
}

// List is a collection of diagnostics.
type List []Diagnostic

func (l List) Error() string {
	if len(l) == 0 {
		return ""
	}
	var b strings.Builder
	for i, d := range l {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(d.String())
	}
	return b.String()
}

func (l List) HasErrors() bool {
	for _, d := range l {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// AttachSource fills in the Source field for each diagnostic using the given source text.
func (l List) AttachSource(src string) List {
	lines := strings.Split(src, "\n")
	for i := range l {
		line := l[i].Pos.Line
		if line > 0 && line <= len(lines) {
			l[i].Source = lines[line-1]
		}
	}
	return l
}

// Errorf appends an error diagnostic.
func Errorf(file string, pos token.Pos, format string, args ...any) Diagnostic {
	return Diagnostic{
		Severity: Error,
		Pos:      pos,
		Message:  fmt.Sprintf(format, args...),
		File:     file,
	}
}
