package pkgman

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoFetchLocalWinsOverRegistry(t *testing.T) {
	// If a package exists in vendor/, auto-fetch should NOT override it
	dir := t.TempDir()
	vendorDir := filepath.Join(dir, "vendor", "mymod")
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(vendorDir, "weft.json"), []byte(`{"name":"mymod","version":"1.0.0","type":"module","entry":"lib.weft"}`), 0o644)
	os.WriteFile(filepath.Join(vendorDir, "lib.weft"), []byte("pub fn hello { \"local\" }\n"), 0o644)

	foundDir, _, err := FindInstalledPackage(dir, "mymod")
	if err != nil {
		t.Fatal("expected to find local package:", err)
	}
	if foundDir == "" {
		t.Fatal("expected non-empty dir for local package")
	}
}

func TestAutoFetchRejectsRedirectToPrivateIP(t *testing.T) {
	// Set up a server that 302-redirects to a private IP
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not have reached the target")
		w.WriteHeader(200)
	}))
	defer target.Close()

	// Redirector → private IP (169.254.169.254)
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer redirector.Close()

	// download should reject the redirect
	dir := t.TempDir()
	dest := filepath.Join(dir, "pkg.tar.gz")
	err := download(redirector.URL, dest)
	if err == nil {
		t.Fatal("expected error for redirect to private IP, got nil")
	}
}

func TestAutoFetchGitRejectsPrivateURL(t *testing.T) {
	// Git URLs targeting private IPs should be rejected
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "weft.json"), []byte(`{"name":"test","deps":{}}`), 0o644)
	_, err := AutoFetchFromGit(dir, "http://192.168.1.1/evil/repo.git")
	if err == nil {
		t.Fatal("expected rejection of private IP git URL")
	}
}

func TestMaturityField(t *testing.T) {
	dir := t.TempDir()
	wj := filepath.Join(dir, "weft.json")
	os.WriteFile(wj, []byte(`{"name":"test","version":"0.1.0","maturity":"experimental"}`), 0o644)
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Maturity != "experimental" {
		t.Fatalf("maturity = %q, want experimental", m.Maturity)
	}
}
