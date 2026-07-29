package weft_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

// Compatibility corpus: pinned scripts under testdata/compat must produce exact .out.
// This is the language/runtime stability net for 0.4.x — extend carefully; never
// "fix" a golden by weakening semantics without a changelog entry.
func TestCompatCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "compat")
	if _, err := os.Stat(root); err != nil {
		// running from repo root vs package dir
		root = filepath.Join("testdata", "compat")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var cases int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".weft") {
			continue
		}
		cases++
		name := e.Name()
		base := strings.TrimSuffix(name, ".weft")
		srcPath := filepath.Join(root, name)
		outPath := filepath.Join(root, base+".out")
		wantBytes, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("%s: missing %s.out: %v", name, base, err)
		}
		want := string(wantBytes)
		// normalize trailing newline
		if !strings.HasSuffix(want, "\n") && want != "" {
			want += "\n"
		}

		src, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		ctx := weft.New(weft.Options{Stdout: &buf, Stderr: &buf})
		err = ctx.RunSource(context.Background(), srcPath, string(src))
		got := buf.String()
		if err != nil {
			t.Errorf("%s: run error: %v\noutput:\n%s", name, err, got)
			continue
		}
		if got != want {
			t.Errorf("%s: output mismatch\nwant %q\ngot  %q", name, want, got)
		}
	}
	if cases == 0 {
		t.Fatal("no compat cases found")
	}
	t.Logf("compat corpus: %d cases", cases)
}
