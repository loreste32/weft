package pkgman

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	// monorepo relative from this package's module root
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	idx := filepath.Join(root, "packages", "index.json")
	c, err := LoadCatalog(idx)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Packages) < 2 {
		t.Fatalf("want ml+tokensave, got %+v", c.Packages)
	}
	s := FormatCatalog(idx, c)
	if !strings.Contains(s, "tokensave") || !strings.Contains(s, "weft get") {
		t.Fatal(s)
	}
}

func TestFindCatalog(t *testing.T) {
	root, _ := filepath.Abs("../..")
	start := filepath.Join(root, "examples", "tokensave_demo")
	path, cat, err := FindCatalog(start)
	if err != nil {
		t.Fatal(err)
	}
	if cat == nil || !strings.Contains(path, "index.json") {
		t.Fatalf("%s %+v", path, cat)
	}
	abs := ResolveCatalogPath(path, "./tokensave")
	if _, err := os.Stat(abs); err != nil {
		t.Fatal(abs, err)
	}
}

func TestCatalogSearchAndFind(t *testing.T) {
	root, _ := filepath.Abs("../..")
	idx := filepath.Join(root, "packages", "index.json")
	c, err := LoadCatalog(idx)
	if err != nil {
		t.Fatal(err)
	}
	hits := SearchCatalog(c, "embed")
	if len(hits) != 1 || hits[0].Name != "ml" {
		t.Fatalf("%+v", hits)
	}
	e, err := FindCatalogEntry(c, "tokensave")
	if err != nil || e.Version == "" {
		t.Fatal(err, e)
	}
	_, err = FindCatalogEntry(c, "token")
	if err == nil || !strings.Contains(err.Error(), "did you mean") {
		t.Fatalf("want suggestion, got %v", err)
	}
	name, cons := ParseCatalogGetSpec("tokensave@^0.5.0")
	if name != "tokensave" || cons != "^0.5.0" {
		t.Fatalf("%q %q", name, cons)
	}
	if err := CheckCatalogConstraint(*e, "^0.5.0"); err != nil {
		t.Fatal(err)
	}
	if err := CheckCatalogConstraint(*e, "^9.0.0"); err == nil {
		t.Fatal("expected constraint fail")
	}
	info := FormatCatalogEntry(idx, *e)
	if !strings.Contains(info, "tokensave") || !strings.Contains(info, "weft packages get") {
		t.Fatal(info)
	}
	filtered := FormatCatalogFilter(idx, c, "token")
	if !strings.Contains(filtered, "tokensave") || strings.Contains(filtered, "  ml ") {
		// ml shouldn't match "token"
		if strings.Contains(filtered, "ml            ") {
			t.Fatal(filtered)
		}
	}
}
