package weft_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestCatalogList(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		// module root is repo root when tests run from pkg/weft
		root, _ = filepath.Abs(".")
		// climb to packages/index.json
		for i := 0; i < 5; i++ {
			if _, e := os.Stat(filepath.Join(root, "packages", "index.json")); e == nil {
				break
			}
			root = filepath.Dir(root)
		}
	}
	// find repo root
	dir := root
	for i := 0; i < 6; i++ {
		if _, e := os.Stat(filepath.Join(dir, "packages", "index.json")); e == nil {
			break
		}
		dir = filepath.Dir(dir)
	}
	// capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = weft.CatalogList(filepath.Join(dir, "examples"))
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "tokensave") {
		t.Fatal(s)
	}
}
