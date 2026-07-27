package pkgman_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/pkgman"
)

func TestScaffoldModuleAndCheck(t *testing.T) {
	parent := t.TempDir()
	root, err := pkgman.Scaffold(pkgman.ScaffoldOptions{
		Dir: parent, Name: "coolkit", Kind: "module",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "lib.weft")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "weft.json")); err != nil {
		t.Fatal(err)
	}
	r, err := pkgman.ValidatePackage(root)
	if err != nil {
		t.Fatal(err)
	}
	if !r.OK {
		t.Fatal(pkgman.FormatValidate(r))
	}
	if r.Name != "coolkit" {
		t.Fatalf("name %q", r.Name)
	}
	found := false
	for _, e := range r.Exports {
		if e == "hello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("exports %v", r.Exports)
	}
}

func TestScaffoldApp(t *testing.T) {
	parent := t.TempDir()
	root, err := pkgman.Scaffold(pkgman.ScaffoldOptions{
		Dir: parent, Name: "myapp", Kind: "app",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "main.weft")); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMissingPub(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("fn only_private() { 1 }\n"), 0o644)
	r, err := pkgman.ValidatePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	// no pub → all fns export, so still OK with warning
	if !r.OK {
		t.Fatal(pkgman.FormatValidate(r))
	}
	if len(r.Warnings) == 0 {
		t.Fatal("expected warning about pub")
	}
}

func TestValidateMultiFileParseError(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("pub fn ok() { 1 }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "broken.weft"), []byte("fn bad { ??? }\n"), 0o644)
	r, err := pkgman.ValidatePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.OK {
		t.Fatal("expected fail on broken sibling file")
	}
	out := pkgman.FormatValidate(r)
	if !strings.Contains(out, "broken.weft") {
		t.Fatal(out)
	}
}

func TestValidatePubEnumAndNextSteps(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "lib.weft"), []byte(`
pub enum Kind { A, B }
pub fn use_kind(k) { k }
`), 0o644)
	_ = pkgman.SaveManifest(dir, &pkgman.Manifest{
		Name: "kindy", Version: "0.2.0", Type: "module", Entry: "lib.weft",
		Exports: []string{"Kind", "use_kind"}, Description: "kinds",
	})
	r, err := pkgman.ValidatePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !r.OK {
		t.Fatal(pkgman.FormatValidate(r))
	}
	found := false
	for _, e := range r.Exports {
		if e == "Kind" {
			found = true
		}
	}
	if !found {
		t.Fatalf("exports %v", r.Exports)
	}
	out := pkgman.FormatValidate(r)
	if !strings.Contains(out, "weft test") || !strings.Contains(out, "weft mod pack") {
		t.Fatal(out)
	}
}

func TestValidateBadExport(t *testing.T) {
	dir := t.TempDir()
	_ = pkgman.SaveManifest(dir, &pkgman.Manifest{
		Name: "x", Version: "0.1.0", Type: "module", Entry: "lib.weft",
		Exports: []string{"missing"},
	})
	_ = os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("pub fn real() { 1 }\n"), 0o644)
	r, err := pkgman.ValidatePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.OK {
		t.Fatal("expected fail for missing export")
	}
}

func TestPackArchive(t *testing.T) {
	parent := t.TempDir()
	root, err := pkgman.Scaffold(pkgman.ScaffoldOptions{
		Dir: parent, Name: "zipme", Kind: "module",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(parent, "zipme.weftpkg.zip")
	if err := pkgman.PackArchive(root, out); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(out)
	if err != nil || st.Size() < 10 {
		t.Fatalf("pack: %v size=%v", err, st)
	}
}

func TestInvalidName(t *testing.T) {
	_, err := pkgman.Scaffold(pkgman.ScaffoldOptions{
		Dir: t.TempDir(), Name: "Bad-Name!", Kind: "module",
	})
	// Bad-Name! → bad_name! after replace? "-" becomes "_" → "bad_name!" still invalid
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid name") {
		t.Fatal(err)
	}
}
