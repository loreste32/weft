package pkgman

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, "myapp"); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "myapp" || m.Version != "0.1.0" {
		t.Fatalf("manifest: %+v", m)
	}
}

func TestInitDefaultName(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, ""); err != nil {
		t.Fatal(err)
	}
	m, _ := LoadManifest(dir)
	if m.Name == "" {
		t.Fatal("should use dir basename")
	}
}

func TestInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	Init(dir, "first")
	if err := Init(dir, "second"); err == nil {
		t.Fatal("should error if weft.json exists")
	}
}

func TestLoadSaveManifest(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{Name: "test", Version: "1.0.0", Deps: map[string]DepSpec{"foo": {Path: "./foo"}}}
	if err := SaveManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "test" {
		t.Fatal("name")
	}
	if loaded.Deps["foo"].Path != "./foo" {
		t.Fatal("deps")
	}
}

func TestLoadManifestMissing(t *testing.T) {
	_, err := LoadManifest("/nonexistent")
	if err == nil {
		t.Fatal("should error for missing dir")
	}
}

func TestHashDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.weft"), []byte("fn main { 1 }"), 0644)
	os.WriteFile(filepath.Join(dir, "b.weft"), []byte("fn helper { 2 }"), 0644)

	hash, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" || len(hash) < 10 {
		t.Fatal("empty hash")
	}
	// deterministic
	hash2, _ := HashDir(dir)
	if hash != hash2 {
		t.Fatal("hash not deterministic")
	}
}

func TestHashDirSkipsGitAndVendor(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.weft"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("y"), 0644)
	os.MkdirAll(filepath.Join(dir, "vendor"), 0755)
	os.WriteFile(filepath.Join(dir, "vendor", "v.weft"), []byte("z"), 0644)

	hash, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// hash without .git/vendor dirs should be same
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, "a.weft"), []byte("x"), 0644)
	hash2, _ := HashDir(dir2)
	if hash != hash2 {
		t.Fatal("should skip .git and vendor")
	}
}

func TestDefaultCacheDir(t *testing.T) {
	dir, err := DefaultCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Fatal("empty cache dir")
	}
}

func TestVendorDir(t *testing.T) {
	if VendorDir("/project") != "/project/vendor" {
		t.Fatal("vendor dir")
	}
}

func TestFindPackageEntry(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lib.weft"), []byte("fn helper { 1 }"), 0644)

	entry, err := FindPackageEntry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(entry) != "lib.weft" {
		t.Fatalf("entry = %s", entry)
	}
}

func TestFindPackageEntryFromManifest(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{Name: "mymod", Entry: "main.weft"})
	os.WriteFile(filepath.Join(dir, "main.weft"), []byte("fn main { 1 }"), 0644)

	entry, err := FindPackageEntry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(entry) != "main.weft" {
		t.Fatalf("entry = %s", entry)
	}
}

func TestFindPackageEntryMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := FindPackageEntry(dir)
	if err == nil {
		t.Fatal("should error when no .weft files")
	}
}

func TestFindPackageEntryMissingExplicit(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{Name: "mymod", Entry: "nonexistent.weft"})
	_, err := FindPackageEntry(dir)
	if err == nil {
		t.Fatal("should error when explicit entry missing")
	}
}

func TestFindPackageEntrySingleScript(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "only.weft"), []byte("fn f { 1 }"), 0644)
	// no lib.weft, mod.weft, etc — but single .weft file should be found
	entry, err := FindPackageEntry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(entry) != "only.weft" {
		t.Fatalf("single file entry = %s", entry)
	}
}

func TestFindPackageEntryNamedAfterPkg(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{Name: "greeter"})
	os.WriteFile(filepath.Join(dir, "greeter.weft"), []byte("fn hello { 1 }"), 0644)

	entry, err := FindPackageEntry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(entry) != "greeter.weft" {
		t.Fatalf("entry = %s", entry)
	}
}

func TestResolveSearchPaths(t *testing.T) {
	paths := ResolveSearchPaths("/project")
	if len(paths) < 2 {
		t.Fatal("should have vendor + packages")
	}
	if paths[0] != "/project/vendor" {
		t.Fatalf("first should be vendor: %s", paths[0])
	}
}

func TestFindInstalledPackage(t *testing.T) {
	project := t.TempDir()
	vendor := filepath.Join(project, "vendor", "mypkg")
	os.MkdirAll(vendor, 0755)
	os.WriteFile(filepath.Join(vendor, "lib.weft"), []byte("pub fn hello { 1 }"), 0644)

	dir, entry, err := FindInstalledPackage(project, "mypkg")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "mypkg" {
		t.Fatal("dir")
	}
	if filepath.Base(entry) != "lib.weft" {
		t.Fatal("entry")
	}
}

func TestFindInstalledPackageNotFound(t *testing.T) {
	project := t.TempDir()
	_, _, err := FindInstalledPackage(project, "nonexistent")
	if err == nil {
		t.Fatal("should error")
	}
}

func TestFindInstalledPackageDeclaredPathDependency(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "ml")
	dependency := filepath.Join(root, "warp")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dependency, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dependency, "lib.weft"), []byte("pub fn ok { 1 }"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(project, &Manifest{
		Name: "ml",
		Deps: map[string]DepSpec{"warp": {Path: "../warp"}},
	}); err != nil {
		t.Fatal(err)
	}

	dir, entry, err := FindInstalledPackage(project, "warp")
	if err != nil {
		t.Fatal(err)
	}
	if dir != dependency || entry != filepath.Join(dependency, "lib.weft") {
		t.Fatalf("resolved declared dependency to %q / %q", dir, entry)
	}
}

func TestLockRoundTrip(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{Name: "test", Deps: map[string]DepSpec{}})

	lock := &Lockfile{
		Version: 1,
		Packages: []LockedPkg{
			{Name: "foo", Version: "1.0.0", Sum: "sha256:abc", Dir: "vendor/foo"},
		},
	}
	if err := SaveLock(dir, lock); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Packages) != 1 || loaded.Packages[0].Version != "1.0.0" {
		t.Fatal("lock round-trip")
	}
}
