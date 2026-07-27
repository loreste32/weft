package weft_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/pkgman"
	"github.com/loreste/weft/pkg/weft"
)

func TestModCheckWithTests(t *testing.T) {
	parent := t.TempDir()
	root, err := pkgman.Scaffold(pkgman.ScaffoldOptions{
		Dir: parent, Name: "chkmod", Kind: "module",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Capture stdout from ModCheckWith
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = weft.ModCheckWith(weft.ModCheckOptions{Dir: root, RunTests: true, QuietTests: true})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "module chkmod") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "== tests ==") && !strings.Contains(out, "passed") {
		t.Fatalf("expected test section:\n%s", out)
	}
	// static-only still works
	if err := weft.ModCheck(root); err != nil {
		t.Fatal(err)
	}
}

func TestModCheckTestsFail(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("pub fn n() { 1 }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "n_test.weft"), []byte(`
use "./lib.weft" as m
fn test_bad {
    test.eq(m.n(), 99)
}
`), 0o644)
	_ = pkgman.SaveManifest(dir, &pkgman.Manifest{
		Name: "n", Version: "0.1.0", Type: "module", Entry: "lib.weft",
		Exports: []string{"n"}, Description: "n",
	})

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := weft.ModCheckWith(weft.ModCheckOptions{Dir: dir, RunTests: true, QuietTests: true})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if err == nil {
		t.Fatalf("expected test failure, got:\n%s", buf.String())
	}
	if !strings.Contains(err.Error(), "tests failed") {
		t.Fatal(err)
	}
}
