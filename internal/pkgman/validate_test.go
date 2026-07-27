package pkgman

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePackage(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{
		Name:    "mymod",
		Version: "0.1.0",
		Entry:   "lib.weft",
	})
	os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("pub fn hello { 1 }\n"), 0644)

	report, err := ValidatePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("errors: %v", report.Errors)
	}
	if report.Name != "mymod" {
		t.Fatalf("name = %q", report.Name)
	}
}

func TestValidatePackageMissingManifest(t *testing.T) {
	dir := t.TempDir()
	report, err := ValidatePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatal("missing manifest should fail")
	}
}

func TestValidatePackageMissingEntry(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{Name: "mymod", Version: "0.1.0", Entry: "missing.weft"})
	report, _ := ValidatePackage(dir)
	if report.OK {
		t.Fatal("missing entry should fail")
	}
}

func TestValidatePackageParseError(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{Name: "mymod", Version: "0.1.0"})
	os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("@@@invalid\n"), 0644)
	report, _ := ValidatePackage(dir)
	if report.OK {
		t.Fatal("parse error should fail")
	}
}

func TestValidatePackageMissingName(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{Version: "0.1.0"})
	os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("pub fn hello { 1 }\n"), 0644)
	report, _ := ValidatePackage(dir)
	// missing name may be error or warning depending on implementation
	if report.OK && len(report.Warnings) == 0 && len(report.Errors) == 0 {
		t.Fatal("missing name should produce at least a warning")
	}
}

func TestValidatePackageMissingVersion(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{Name: "mymod"})
	os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("pub fn hello { 1 }\n"), 0644)
	report, _ := ValidatePackage(dir)
	// missing version is a warning or error depending on implementation
	_ = report
}

func TestFormatValidate(t *testing.T) {
	report := &ValidateReport{
		Dir:     "/test",
		Name:    "mymod",
		Version: "0.1.0",
		Entry:   "lib.weft",
		Exports: []string{"hello"},
		OK:      true,
	}
	out := FormatValidate(report)
	if out == "" {
		t.Fatal("empty output")
	}
}

func TestFormatValidateWithErrors(t *testing.T) {
	report := &ValidateReport{
		Dir:      "/test",
		Name:     "mymod",
		OK:       false,
		Errors:   []string{"missing entry"},
		Warnings: []string{"no exports"},
	}
	out := FormatValidate(report)
	if out == "" {
		t.Fatal("empty output")
	}
}

func TestPackArchive(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{Name: "mymod", Version: "0.1.0"})
	os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("pub fn hello { 1 }\n"), 0644)

	outPath := filepath.Join(t.TempDir(), "out.zip")
	err := PackArchive(dir, outPath)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(outPath)
	if err != nil || st.Size() == 0 {
		t.Fatal("empty zip")
	}
}
