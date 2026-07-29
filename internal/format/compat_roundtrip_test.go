package format_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/format"
	"github.com/loreste/weft/pkg/weft"
)

// Format(src) must parse, format again stably, and run to the same output as the original.
func TestCompatFormatRoundTrip(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "compat")
	if _, err := os.Stat(root); err != nil {
		root = filepath.Join("testdata", "compat")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".weft") {
			continue
		}
		n++
		name := e.Name()
		base := strings.TrimSuffix(name, ".weft")
		srcPath := filepath.Join(root, name)
		outPath := filepath.Join(root, base+".out")
		src, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatal(err)
		}
		wantOut, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatal(err)
		}
		want := string(wantOut)
		if want != "" && !strings.HasSuffix(want, "\n") {
			want += "\n"
		}

		formatted, err := format.Source(srcPath, string(src))
		if err != nil {
			t.Errorf("%s: format: %v", name, err)
			continue
		}
		again, err := format.Source(srcPath, formatted)
		if err != nil {
			t.Errorf("%s: re-format: %v", name, err)
			continue
		}
		if again != formatted {
			t.Errorf("%s: format not idempotent\nfirst:\n%s\nsecond:\n%s", name, formatted, again)
			continue
		}

		var buf bytes.Buffer
		ctx := weft.New(weft.Options{Stdout: &buf, Stderr: &buf})
		if err := ctx.RunSource(context.Background(), srcPath, formatted); err != nil {
			t.Errorf("%s: run formatted: %v\n%s\nsource:\n%s", name, err, buf.String(), formatted)
			continue
		}
		got := buf.String()
		if got != want {
			t.Errorf("%s: formatted output mismatch\nwant %q\ngot  %q", name, want, got)
		}
	}
	if n == 0 {
		t.Fatal("no compat cases")
	}
}
