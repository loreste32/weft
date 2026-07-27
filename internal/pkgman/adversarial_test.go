package pkgman_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/pkgman"
	"github.com/loreste/weft/internal/stdlib"
)

func TestInstallRejectsStdlibName(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "http_pkg")
	_ = os.MkdirAll(pkg, 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "lib.weft"), []byte("pub fn get() { 1 }\n"), 0o644)
	app := filepath.Join(root, "app")
	_ = os.MkdirAll(app, 0o755)
	_ = pkgman.SaveManifest(app, &pkgman.Manifest{
		Name: "app",
		Deps: map[string]pkgman.DepSpec{
			"http": {Path: "../http_pkg"}, // reserved stdlib name
		},
	})
	_, err := pkgman.InstallAll(app)
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("want reserved error, got %v", err)
	}
	// every stdlib name is reserved
	if !stdlib.IsPackage("http") {
		t.Fatal("precondition")
	}
}

func TestInstallRejectsMapPreludeName(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "m")
	_ = os.MkdirAll(pkg, 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "lib.weft"), []byte("pub fn x() { 1 }\n"), 0o644)
	app := filepath.Join(root, "app")
	_ = os.MkdirAll(app, 0o755)
	_ = pkgman.SaveManifest(app, &pkgman.Manifest{
		Name: "app",
		Deps: map[string]pkgman.DepSpec{"map": {Path: "../m"}},
	})
	_, err := pkgman.InstallAll(app)
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("want reserved, got %v", err)
	}
}

func TestInstallDiamondConflict(t *testing.T) {
	root := t.TempDir()
	b1 := filepath.Join(root, "b1")
	b2 := filepath.Join(root, "b2")
	a := filepath.Join(root, "a")
	c := filepath.Join(root, "c")
	app := filepath.Join(root, "app")
	for _, d := range []string{b1, b2, a, c, app} {
		_ = os.MkdirAll(d, 0o755)
	}
	_ = os.WriteFile(filepath.Join(b1, "lib.weft"), []byte("pub fn id() { \"1\" }\n"), 0o644)
	_ = pkgman.SaveManifest(b1, &pkgman.Manifest{Name: "b", Entry: "lib.weft", Exports: []string{"id"}})
	_ = os.WriteFile(filepath.Join(b2, "lib.weft"), []byte("pub fn id() { \"2\" }\n"), 0o644)
	_ = pkgman.SaveManifest(b2, &pkgman.Manifest{Name: "b", Entry: "lib.weft", Exports: []string{"id"}})
	_ = os.WriteFile(filepath.Join(a, "lib.weft"), []byte("use b\npub fn fa() { b.id() }\n"), 0o644)
	_ = pkgman.SaveManifest(a, &pkgman.Manifest{
		Name: "a", Entry: "lib.weft", Exports: []string{"fa"},
		Deps: map[string]pkgman.DepSpec{"b": {Path: "../b1"}},
	})
	_ = os.WriteFile(filepath.Join(c, "lib.weft"), []byte("use b\npub fn fc() { b.id() }\n"), 0o644)
	_ = pkgman.SaveManifest(c, &pkgman.Manifest{
		Name: "c", Entry: "lib.weft", Exports: []string{"fc"},
		Deps: map[string]pkgman.DepSpec{"b": {Path: "../b2"}},
	})
	_ = pkgman.SaveManifest(app, &pkgman.Manifest{
		Name: "app",
		Deps: map[string]pkgman.DepSpec{
			"a": {Path: "../a"},
			"c": {Path: "../c"},
		},
	})
	_, err := pkgman.InstallAll(app)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("want diamond conflict, got %v", err)
	}
}

func TestVerifyLockDetectsTamper(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "p")
	_ = os.MkdirAll(pkg, 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "lib.weft"), []byte("pub fn id(x) { x }\n"), 0o644)
	app := filepath.Join(root, "app")
	_ = os.MkdirAll(app, 0o755)
	_ = pkgman.SaveManifest(app, &pkgman.Manifest{
		Name: "app",
		Deps: map[string]pkgman.DepSpec{"p": {Path: "../p"}},
	})
	if _, err := pkgman.InstallAll(app); err != nil {
		t.Fatal(err)
	}
	// tamper vendor
	_ = os.WriteFile(filepath.Join(app, "vendor", "p", "lib.weft"), []byte("pub fn id(x) { \"evil\" }\n"), 0o644)
	err := pkgman.VerifyLock(app)
	if err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("want sum mismatch, got %v", err)
	}
}
