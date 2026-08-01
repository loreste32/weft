package weft_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestDoctorReportsCatalog(t *testing.T) {
	// Run from repo root so packages/index.json is discoverable.
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// climb if needed
	for i := 0; i < 4; i++ {
		if _, e := os.Stat(filepath.Join(root, "packages", "index.json")); e == nil {
			break
		}
		root = filepath.Dir(root)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = weft.Doctor()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "weft "+weft.Version) {
		t.Fatalf("version: %s", s)
	}
	if !strings.Contains(s, "catalog") {
		t.Fatalf("missing catalog line:\n%s", s)
	}
	// monorepo should list at least one catalog package name (first few are shown, rest truncated)
	if !strings.Contains(s, "catalog_pkgs") {
		t.Fatalf("expected catalog pkg names:\n%s", s)
	}
	if !strings.Contains(s, "packages list") {
		t.Fatalf("expected packages hint:\n%s", s)
	}
}

func TestDoctorProjectDeps(t *testing.T) {
	// examples/pkg_demo has greeter dep + lock
	root, _ := filepath.Abs("../..")
	for i := 0; i < 4; i++ {
		if _, e := os.Stat(filepath.Join(root, "examples", "pkg_demo", "weft.json")); e == nil {
			break
		}
		root = filepath.Dir(root)
	}
	demo := filepath.Join(root, "examples", "pkg_demo")
	wd, _ := os.Getwd()
	if err := os.Chdir(demo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := weft.Doctor()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "deps") || !strings.Contains(s, "greeter") {
		t.Fatalf("expected deps/greeter:\n%s", s)
	}
	if !strings.Contains(s, "vendor") {
		t.Fatalf("expected vendor line:\n%s", s)
	}
}
