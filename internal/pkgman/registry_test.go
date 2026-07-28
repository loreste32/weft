package pkgman

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryURL(t *testing.T) {
	t.Setenv("WEFT_REGISTRY", "")
	if RegistryURL() != DefaultRegistryURL {
		t.Fatal("default")
	}
	t.Setenv("WEFT_REGISTRY", "https://custom.example.com/")
	if RegistryURL() != "https://custom.example.com" {
		t.Fatalf("custom: %q", RegistryURL())
	}
}

func testIndex() *RegistryIndex {
	return &RegistryIndex{
		Packages: []RegistryPackage{
			{Name: "foo", Version: "1.0.0", Summary: "A foo package"},
			{Name: "foo", Version: "1.1.0", Summary: "A foo package (newer)"},
			{Name: "bar", Version: "0.2.0", Summary: "Bar utilities"},
		},
	}
}

func TestSearchRegistry(t *testing.T) {
	idx := testIndex()
	all := SearchRegistry(idx, "")
	if len(all) != 3 {
		t.Fatalf("all: %d", len(all))
	}
	foos := SearchRegistry(idx, "foo")
	if len(foos) != 2 {
		t.Fatalf("foo: %d", len(foos))
	}
	none := SearchRegistry(idx, "nonexistent")
	if len(none) != 0 {
		t.Fatal("should be empty")
	}
	// search by summary
	bars := SearchRegistry(idx, "utilities")
	if len(bars) != 1 {
		t.Fatal("summary search")
	}
}

func TestFindRegistryPackage(t *testing.T) {
	idx := testIndex()
	// latest version
	pkg, err := FindRegistryPackage(idx, "foo", "")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Version != "1.1.0" {
		t.Fatalf("should pick latest: %s", pkg.Version)
	}
	// with constraint
	pkg, err = FindRegistryPackage(idx, "foo", "^1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Version != "1.1.0" {
		t.Fatal("^1.0.0 should match 1.1.0")
	}
	// not found
	_, err = FindRegistryPackage(idx, "baz", "")
	if err == nil {
		t.Fatal("should error")
	}
}

func TestResolveArchiveURL(t *testing.T) {
	pkg := RegistryPackage{ArchiveURL: "v1/packages/foo-1.0.0.tar.gz"}
	url := ResolveArchiveURL("https://registry.example.com", pkg)
	if url != "https://registry.example.com/v1/packages/foo-1.0.0.tar.gz" {
		t.Fatal(url)
	}
	// absolute URL
	pkg.ArchiveURL = "https://cdn.example.com/foo.tar.gz"
	url = ResolveArchiveURL("https://registry.example.com", pkg)
	if url != "https://cdn.example.com/foo.tar.gz" {
		t.Fatal(url)
	}
}

func TestFetchIndex(t *testing.T) {
	idx := testIndex()
	b, _ := json.Marshal(idx)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/index.json" {
			w.Write(b)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	fetched, err := FetchIndex(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched.Packages) != 3 {
		t.Fatalf("packages: %d", len(fetched.Packages))
	}
}

func TestFetchIndexBadURL(t *testing.T) {
	_, err := FetchIndex("http://127.0.0.1:1") // nothing listening
	if err == nil {
		t.Fatal("should error on unreachable")
	}
}

func TestFetchIndexBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	_, err := FetchIndex(srv.URL)
	if err == nil {
		t.Fatal("should error on 500")
	}
}

func TestDownloadAndVerify(t *testing.T) {
	// Generate key and sign fake archive
	kp, _ := GenerateKey("test")
	archiveData := []byte("fake archive data for testing")
	sig, _ := Sign(kp, archiveData)
	pubHex := fmt.Sprintf("%x", kp.PublicKey)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData)
	}))
	defer srv.Close()

	pkg := RegistryPackage{
		Name:       "testpkg",
		Version:    "1.0.0",
		ArchiveURL: srv.URL + "/testpkg-1.0.0.tar.gz",
		PublicKey:  pubHex,
		Signature:  sig,
	}

	dest := filepath.Join(t.TempDir(), "testpkg-1.0.0.tar.gz")
	if err := DownloadAndVerify(srv.URL, pkg, dest); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(dest)
	if string(data) != string(archiveData) {
		t.Fatal("content mismatch")
	}
}

func TestDownloadAndVerifyBadSig(t *testing.T) {
	kp, _ := GenerateKey("test")
	pubHex := fmt.Sprintf("%x", kp.PublicKey)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("tampered data"))
	}))
	defer srv.Close()

	// Sign different data
	sig, _ := Sign(kp, []byte("original data"))

	pkg := RegistryPackage{
		Name:       "bad",
		Version:    "1.0.0",
		ArchiveURL: srv.URL + "/bad.tar.gz",
		PublicKey:  pubHex,
		Signature:  sig,
	}

	dest := filepath.Join(t.TempDir(), "bad.tar.gz")
	if err := DownloadAndVerify(srv.URL, pkg, dest); err == nil {
		t.Fatal("tampered archive should fail signature check")
	}
}

func TestDownloadUnsigned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("unsigned archive"))
	}))
	defer srv.Close()

	pkg := RegistryPackage{
		Name:       "unsigned",
		Version:    "1.0.0",
		ArchiveURL: srv.URL + "/unsigned.tar.gz",
	}

	dest := filepath.Join(t.TempDir(), "unsigned.tar.gz")
	if err := DownloadAndVerify(srv.URL, pkg, dest); err != nil {
		t.Fatal("unsigned should still download")
	}
}

func TestResolveArchiveURLPathTraversal(t *testing.T) {
	pkg := RegistryPackage{ArchiveURL: "../../../etc/passwd"}
	url := ResolveArchiveURL("https://registry.example.com", pkg)
	if strings.Contains(url, "..") {
		t.Fatal("path traversal should be sanitized")
	}
}

func TestDownloadRejectsPrivateIP(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	pkg := RegistryPackage{
		Name:       "evil",
		Version:    "1.0.0",
		ArchiveURL: "http://169.254.169.254/latest/meta-data/",
	}
	dest := filepath.Join(t.TempDir(), "evil.tar.gz")
	err := DownloadAndVerify("https://registry.example.com", pkg, dest)
	if err == nil {
		t.Fatal("metadata IP should be rejected")
	}
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("actual content"))
	}))
	defer srv.Close()

	pkg := RegistryPackage{
		Name:       "badsum",
		Version:    "1.0.0",
		ArchiveURL: srv.URL + "/pkg.tar.gz",
		Sum:        "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	dest := filepath.Join(t.TempDir(), "pkg.tar.gz")
	err := DownloadAndVerify(srv.URL, pkg, dest)
	if err == nil {
		t.Fatal("bad checksum should fail")
	}
}

func TestVersionGreater(t *testing.T) {
	if !versionGreater("2.0.0", "1.0.0") {
		t.Fatal("2 > 1")
	}
	if !versionGreater("1.1.0", "1.0.0") {
		t.Fatal("1.1 > 1.0")
	}
	if !versionGreater("1.0.1", "1.0.0") {
		t.Fatal("1.0.1 > 1.0.0")
	}
	if versionGreater("1.0.0", "1.0.0") {
		t.Fatal("equal")
	}
}
