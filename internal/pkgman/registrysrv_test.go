package pkgman

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryServerHealth(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("health check failed")
	}
}

func TestRegistryServerEmptyIndex(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/index.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var idx RegistryIndex
	json.NewDecoder(resp.Body).Decode(&idx)
	if len(idx.Packages) != 0 {
		t.Fatal("should be empty")
	}
}

func publishToServer(t *testing.T, ts *httptest.Server, name, version, token string) {
	t.Helper()
	meta := RegistryPackage{
		Name:    name,
		Version: version,
		Summary: "test package",
	}
	metaJSON, _ := json.Marshal(meta)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("metadata", string(metaJSON))
	part, _ := w.CreateFormFile("archive", name+"-"+version+".tar.gz")
	part.Write([]byte("fake archive content for " + name))
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("publish failed: %d %s", resp.StatusCode, string(b))
	}
}

func TestRegistryServerPublishAndIndex(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	publishToServer(t, ts, "mylib", "1.0.0", "")

	// Check index
	resp, err := http.Get(ts.URL + "/v1/index.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var idx RegistryIndex
	json.NewDecoder(resp.Body).Decode(&idx)
	if len(idx.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(idx.Packages))
	}
	if idx.Packages[0].Name != "mylib" || idx.Packages[0].Version != "1.0.0" {
		t.Fatal("wrong package")
	}
}

func TestRegistryServerDownload(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	publishToServer(t, ts, "dl", "0.1.0", "")

	resp, err := http.Get(ts.URL + "/v1/packages/dl-0.1.0.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("download failed")
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "fake archive") {
		t.Fatal("wrong content")
	}
}

func TestRegistryServerAuth(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "secret123")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Without token
	meta := RegistryPackage{Name: "x", Version: "1.0.0"}
	metaJSON, _ := json.Marshal(meta)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("metadata", string(metaJSON))
	part, _ := w.CreateFormFile("archive", "x-1.0.0.tar.gz")
	part.Write([]byte("data"))
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatal("should require auth")
	}

	// With token
	publishToServer(t, ts, "x", "1.0.0", "secret123")
}

func TestRegistryServerPathTraversal(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/v1/packages/../../etc/passwd")
	resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("path traversal should be blocked")
	}
}

func TestRegistryServerBadPublish(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Missing metadata
	resp, _ := http.Post(ts.URL+"/v1/publish", "application/json", strings.NewReader("{}"))
	resp.Body.Close()
	if resp.StatusCode < 400 {
		t.Fatal("should reject bad request")
	}

	// GET on publish
	resp, _ = http.Get(ts.URL + "/v1/publish")
	resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Fatal("should reject GET")
	}
}

func TestRegistryServerMultipleVersions(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	publishToServer(t, ts, "lib", "1.0.0", "")
	publishToServer(t, ts, "lib", "1.1.0", "")
	publishToServer(t, ts, "other", "0.1.0", "")

	resp, err := http.Get(ts.URL + "/v1/index.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var idx RegistryIndex
	json.NewDecoder(resp.Body).Decode(&idx)
	if len(idx.Packages) != 3 {
		t.Fatalf("expected 3, got %d", len(idx.Packages))
	}
}

func TestRegistryServerDownloadNotFound(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/v1/packages/nonexistent.tar.gz")
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatal("should 404")
	}
}

func TestRegistryEndToEnd(t *testing.T) {
	// Full flow: publish with signing, download and verify
	dataDir := t.TempDir()
	srv := NewRegistryServer(dataDir, "tok")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Create a key
	t.Setenv("HOME", t.TempDir())
	kp, _ := GenerateKey("test")
	SaveKey("test", kp)

	// Create a package
	pkgDir := t.TempDir()
	SaveManifest(pkgDir, &Manifest{Name: "hello", Version: "0.1.0"})
	os.WriteFile(filepath.Join(pkgDir, "lib.weft"), []byte("pub fn greet { \"hi\" }\n"), 0644)

	// Pack + sign
	archivePath := filepath.Join(t.TempDir(), "hello-0.1.0.tar.gz")
	PackArchive(pkgDir, archivePath)
	sig, _ := SignFile(kp, archivePath)
	pubHex := fmt.Sprintf("%x", kp.PublicKey)

	// Publish
	meta := RegistryPackage{
		Name:      "hello",
		Version:   "0.1.0",
		Summary:   "greetings",
		PublicKey: pubHex,
		Signature: sig,
	}
	metaJSON, _ := json.Marshal(meta)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("metadata", string(metaJSON))
	f, _ := os.Open(archivePath)
	part, _ := w.CreateFormFile("archive", "hello-0.1.0.tar.gz")
	io.Copy(part, f)
	f.Close()
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer tok")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatal("publish failed")
	}

	// Fetch index and verify
	idx, err := FetchIndex(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := FindRegistryPackage(idx, "hello", "")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Signature == "" {
		t.Fatal("should be signed")
	}

	// Download and verify signature
	dest := filepath.Join(t.TempDir(), "hello.tar.gz")
	if err := DownloadAndVerify(ts.URL, *pkg, dest); err != nil {
		t.Fatal(err)
	}
}
