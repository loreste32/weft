package pkgman

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoinRejectsTraversal(t *testing.T) {
	dest := t.TempDir()
	cases := []string{
		"../outside.txt",
		"../../etc/passwd",
		"/etc/passwd",
		`..\windows\system32`,
		"foo/../../etc/passwd",
		"C:/windows/system32",
		"//unc/share/file",
		"a/\x00/b",
	}
	for _, name := range cases {
		if _, err := safeJoin(dest, name); err == nil {
			t.Errorf("expected reject %q", name)
		}
	}
	ok, err := safeJoin(dest, "lib.weft")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ok, filepath.Clean(dest)) {
		t.Fatalf("got %q", ok)
	}
}

func TestUnzipRejectsSlip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../pwned.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("pwned"))
	// also a legit file so structure is valid-looking
	w2, _ := zw.Create("lib.weft")
	_, _ = w2.Write([]byte("pub fn hi() { 1 }\n"))
	_ = zw.Close()
	_ = f.Close()

	dest := filepath.Join(dir, "out")
	_ = os.MkdirAll(dest, 0o755)
	err = unzip(zipPath, dest)
	if err == nil || !strings.Contains(err.Error(), "illegal") && !strings.Contains(err.Error(), "traversal") {
		t.Fatalf("want zip-slip reject, got %v", err)
	}
	// must not create file outside dest
	if _, err := os.Stat(filepath.Join(dir, "pwned.txt")); err == nil {
		t.Fatal("zip slip wrote outside dest")
	}
}

func TestUntarRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "evil.tar")
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	hdr := &tar.Header{
		Name:     "link",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     0o777,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = f.Close()

	dest := filepath.Join(dir, "out")
	_ = os.MkdirAll(dest, 0o755)
	err = untar(tarPath, dest)
	if err == nil {
		t.Fatal("expected symlink reject")
	}
}

func TestCapabilitiesAllows(t *testing.T) {
	if Allows(nil, "json") {
		// default allow non-restricted
	} else {
		t.Fatal("json should be allowed by default")
	}
	if Allows(nil, "sh") {
		t.Fatal("sh denied by default")
	}
	if !Allows([]string{"sh"}, "sh") {
		t.Fatal("explicit grant")
	}
	if !Allows([]string{"*"}, "secrets") {
		t.Fatal("* grants all")
	}
}

func TestAtomicInstallLeavesOldVendorOnFailure(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good")
	_ = os.MkdirAll(good, 0o755)
	_ = os.WriteFile(filepath.Join(good, "lib.weft"), []byte("pub fn ok() { 1 }\n"), 0o644)
	app := filepath.Join(root, "app")
	_ = os.MkdirAll(app, 0o755)
	// pre-seed vendor
	_ = os.MkdirAll(filepath.Join(app, "vendor", "good"), 0o755)
	_ = os.WriteFile(filepath.Join(app, "vendor", "good", "lib.weft"), []byte("pub fn ok() { 1 }\n"), 0o644)
	_ = SaveManifest(app, &Manifest{
		Name: "app",
		Deps: map[string]DepSpec{
			"good": {Path: "../good"},
			"bad":  {Path: "../missing-nope"},
		},
	})
	_, err := InstallAll(app)
	if err == nil {
		t.Fatal("expected failure")
	}
	// old good package should still be present (atomic swap never committed)
	b, err := os.ReadFile(filepath.Join(app, "vendor", "good", "lib.weft"))
	if err != nil {
		t.Fatalf("vendor destroyed on failed install: %v", err)
	}
	if !bytes.Contains(b, []byte("ok")) {
		t.Fatalf("content %q", b)
	}
	// no leftover stage
	if _, err := os.Stat(filepath.Join(app, ".weft-vendor-stage")); err == nil {
		t.Fatal("stage dir leaked")
	}
}

func TestVerifyLockRejectsPathEscape(t *testing.T) {
	root := t.TempDir()
	if err := SaveLock(root, &Lockfile{Packages: []LockedPkg{{
		Name: "safe",
		Dir:  "../../outside",
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLock(root); err == nil {
		t.Fatal("expected lockfile path escape to be rejected")
	}
}

func TestInstallAllRejectsPathLikeDependencyName(t *testing.T) {
	root := t.TempDir()
	if err := SaveManifest(root, &Manifest{
		Name: "app",
		Deps: map[string]DepSpec{"../escape": {Path: "."}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallAll(root); err == nil {
		t.Fatal("expected path-like dependency name to be rejected")
	}
}

func TestZipDirRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	secret := filepath.Join(root, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := zipDir(root, filepath.Join(t.TempDir(), "pkg.zip"), nil); err == nil {
		t.Fatal("expected package archive to reject symlink")
	}
}
