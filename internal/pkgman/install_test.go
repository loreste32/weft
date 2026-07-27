package pkgman_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/loreste/weft/internal/pkgman"
)

func TestInstallTransitiveDeps(t *testing.T) {
	root := t.TempDir()
	// leaf: mathx (multi-file)
	mathx := filepath.Join(root, "packages", "mathx")
	_ = os.MkdirAll(mathx, 0o755)
	_ = pkgman.SaveManifest(mathx, &pkgman.Manifest{
		Name: "mathx", Version: "0.1.0", Type: "module", Entry: "lib.weft",
		Exports: []string{"double"},
	})
	_ = os.WriteFile(filepath.Join(mathx, "lib.weft"), []byte(`
use "./helpers.weft" as h
pub fn double(x) { h.times(x, 2) }
`), 0o644)
	_ = os.WriteFile(filepath.Join(mathx, "helpers.weft"), []byte(`
pub fn times(a, b) { a * b }
`), 0o644)

	// mid: stringx depends on mathx via relative path
	stringx := filepath.Join(root, "packages", "stringx")
	_ = os.MkdirAll(stringx, 0o755)
	_ = pkgman.SaveManifest(stringx, &pkgman.Manifest{
		Name: "stringx", Version: "0.1.0", Type: "module", Entry: "lib.weft",
		Exports: []string{"pad"},
		Deps: map[string]pkgman.DepSpec{
			"mathx": {Path: "../mathx"},
		},
	})
	_ = os.WriteFile(filepath.Join(stringx, "lib.weft"), []byte(`
use mathx
pub fn pad(s, n) {
    mut out := s
    mut i := 0
    while i < mathx.double(0) {
        i = i + 1
    }
    // pad to at least n by repeating
    while len(out) < n {
        out = out + s
    }
    out
}
`), 0o644)

	// app only depends on stringx — mathx must install transitively
	app := filepath.Join(root, "app")
	_ = os.MkdirAll(app, 0o755)
	_ = pkgman.SaveManifest(app, &pkgman.Manifest{
		Name: "app", Version: "0.1.0", Type: "app",
		Deps: map[string]pkgman.DepSpec{
			"stringx": {Path: "../packages/stringx"},
		},
	})

	lock, err := pkgman.InstallAll(app)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) < 2 {
		t.Fatalf("want transitive install of mathx+stringx, got %d: %+v", len(lock.Packages), lock.Packages)
	}
	names := map[string]bool{}
	for _, p := range lock.Packages {
		names[p.Name] = true
	}
	if !names["mathx"] || !names["stringx"] {
		t.Fatalf("packages %v", names)
	}
	// both vendored
	if _, err := os.Stat(filepath.Join(app, "vendor", "mathx", "lib.weft")); err != nil {
		t.Fatal("mathx not vendored:", err)
	}
	if _, err := os.Stat(filepath.Join(app, "vendor", "stringx", "helpers.weft")); err == nil {
		// stringx has no helpers — ok
	}
	if _, err := os.Stat(filepath.Join(app, "vendor", "mathx", "helpers.weft")); err != nil {
		t.Fatal("mathx multi-file helpers not copied:", err)
	}
}

func TestInstallCycleOK(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	_ = os.MkdirAll(a, 0o755)
	_ = os.MkdirAll(b, 0o755)
	_ = pkgman.SaveManifest(a, &pkgman.Manifest{
		Name: "a", Type: "module", Entry: "lib.weft",
		Deps: map[string]pkgman.DepSpec{"b": {Path: "../b"}},
	})
	_ = os.WriteFile(filepath.Join(a, "lib.weft"), []byte("pub fn fa() { 1 }\n"), 0o644)
	_ = pkgman.SaveManifest(b, &pkgman.Manifest{
		Name: "b", Type: "module", Entry: "lib.weft",
		Deps: map[string]pkgman.DepSpec{"a": {Path: "../a"}},
	})
	_ = os.WriteFile(filepath.Join(b, "lib.weft"), []byte("pub fn fb() { 2 }\n"), 0o644)

	app := filepath.Join(root, "app")
	_ = os.MkdirAll(app, 0o755)
	_ = pkgman.SaveManifest(app, &pkgman.Manifest{
		Name: "app",
		Deps: map[string]pkgman.DepSpec{"a": {Path: "../a"}},
	})
	lock, err := pkgman.InstallAll(app)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) != 2 {
		t.Fatalf("got %d packages", len(lock.Packages))
	}
}

func TestResolveDepPaths(t *testing.T) {
	spec := pkgman.ResolveDepPaths(pkgman.DepSpec{Path: "../mathx"}, "/repo/packages/stringx")
	if !filepath.IsAbs(spec.Path) {
		t.Fatalf("want abs path, got %q", spec.Path)
	}
	want := filepath.Clean("/repo/packages/mathx")
	if spec.Path != want {
		t.Fatalf("got %q want %q", spec.Path, want)
	}
}
