package pkgman

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldCLI(t *testing.T) {
	dir := t.TempDir()
	root, err := Scaffold(ScaffoldOptions{Dir: dir, Name: "mytool", Kind: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "main.weft")); err != nil {
		t.Fatal("missing main.weft")
	}
	m, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "mytool" {
		t.Fatalf("name = %q", m.Name)
	}
}

func TestScaffoldCLIForce(t *testing.T) {
	dir := t.TempDir()
	Scaffold(ScaffoldOptions{Dir: dir, Name: "mytool", Kind: "cli"})
	// second time without force should fail
	_, err := Scaffold(ScaffoldOptions{Dir: dir, Name: "mytool", Kind: "cli"})
	if err == nil {
		t.Fatal("should error without force")
	}
	// with force should succeed
	_, err = Scaffold(ScaffoldOptions{Dir: dir, Name: "mytool", Kind: "cli", Force: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestScaffoldUnknownKind(t *testing.T) {
	dir := t.TempDir()
	_, err := Scaffold(ScaffoldOptions{Dir: dir, Name: "x", Kind: "unknown_kind"})
	if err == nil {
		t.Fatal("unknown kind should error")
	}
}

func TestScaffoldEmptyName(t *testing.T) {
	dir := t.TempDir()
	_, err := Scaffold(ScaffoldOptions{Dir: dir, Name: "", Kind: "module"})
	if err == nil {
		t.Fatal("empty name should error")
	}
}

func TestScaffoldBadName(t *testing.T) {
	dir := t.TempDir()
	_, err := Scaffold(ScaffoldOptions{Dir: dir, Name: "123bad", Kind: "module"})
	if err == nil {
		t.Fatal("invalid name should error")
	}
}
