package pkgman

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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

const testToken = "test-secret-token"

// testKeypair generates an ephemeral ed25519 keypair for signing test packages.
func testKeypair() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return pub, priv
}

func publishSignedToServer(t *testing.T, ts *httptest.Server, name, version, token string) {
	t.Helper()
	pub, priv := testKeypair()
	archiveData := []byte("test archive content for " + name + "@" + version)
	sig := ed25519.Sign(priv, archiveData)
	h := sha256.Sum256(archiveData)

	meta := RegistryPackage{
		Name:      name,
		Version:   version,
		Summary:   "test package",
		PublicKey: hex.EncodeToString(pub),
		Signature: hex.EncodeToString(sig),
		Sum:       "sha256:" + hex.EncodeToString(h[:]),
	}
	metaJSON, _ := json.Marshal(meta)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("metadata", string(metaJSON))
	part, _ := w.CreateFormFile("archive", name+"-"+version+".tar.gz")
	part.Write(archiveData)
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

func TestRegistryServerHealth(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), testToken)
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
	srv := NewRegistryServer(t.TempDir(), testToken)
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

func TestRegistryServerPublishAndIndex(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	publishSignedToServer(t, ts, "mylib", "1.0.0", testToken)

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
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	publishSignedToServer(t, ts, "dl", "0.1.0", testToken)

	resp, err := http.Get(ts.URL + "/v1/packages/dl-0.1.0.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("download failed")
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "test archive content") {
		t.Fatal("wrong content")
	}
}

func TestRegistryServerAuth(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), "secret123")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Without token — should fail
	pub, priv := testKeypair()
	archiveData := []byte("data")
	sig := ed25519.Sign(priv, archiveData)
	meta := RegistryPackage{
		Name:      "authtest",
		Version:   "1.0.0",
		PublicKey: hex.EncodeToString(pub),
		Signature: hex.EncodeToString(sig),
	}
	metaJSON, _ := json.Marshal(meta)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("metadata", string(metaJSON))
	part, _ := w.CreateFormFile("archive", "authtest-1.0.0.tar.gz")
	part.Write(archiveData)
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("should require auth, got %d", resp.StatusCode)
	}

	// With correct token — should succeed
	publishSignedToServer(t, ts, "authtest", "1.0.0", "secret123")
}

func TestRegistryServerRejectUnsigned(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Try publishing without signature
	meta := RegistryPackage{Name: "unsigned", Version: "1.0.0", Summary: "no sig"}
	metaJSON, _ := json.Marshal(meta)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("metadata", string(metaJSON))
	part, _ := w.CreateFormFile("archive", "unsigned-1.0.0.tar.gz")
	part.Write([]byte("data"))
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("should reject unsigned, got %d: %s", resp.StatusCode, string(b))
	}
	if !strings.Contains(string(b), "signature required") {
		t.Fatalf("wrong error: %s", string(b))
	}
}

func TestRegistryServerRejectBadSignature(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	pub, _ := testKeypair()
	_, otherPriv := testKeypair() // sign with wrong key
	archiveData := []byte("data")
	sig := ed25519.Sign(otherPriv, archiveData)

	meta := RegistryPackage{
		Name:      "badsig",
		Version:   "1.0.0",
		PublicKey: hex.EncodeToString(pub), // pub doesn't match priv
		Signature: hex.EncodeToString(sig),
	}
	metaJSON, _ := json.Marshal(meta)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("metadata", string(metaJSON))
	part, _ := w.CreateFormFile("archive", "badsig-1.0.0.tar.gz")
	part.Write(archiveData)
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("should reject bad signature, got %d", resp.StatusCode)
	}
}

func TestRegistryServerRejectVersionOverwrite(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	publishSignedToServer(t, ts, "immutable", "1.0.0", testToken)

	// Try publishing same version again — should 409
	pub, priv := testKeypair()
	archiveData := []byte("trojanized")
	sig := ed25519.Sign(priv, archiveData)
	meta := RegistryPackage{
		Name:      "immutable",
		Version:   "1.0.0",
		PublicKey: hex.EncodeToString(pub),
		Signature: hex.EncodeToString(sig),
	}
	metaJSON, _ := json.Marshal(meta)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("metadata", string(metaJSON))
	part, _ := w.CreateFormFile("archive", "immutable-1.0.0.tar.gz")
	part.Write(archiveData)
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("should reject overwrite, got %d: %s", resp.StatusCode, string(b))
	}
}

func TestRegistryServerRejectReservedName(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	pub, priv := testKeypair()
	archiveData := []byte("data")
	sig := ed25519.Sign(priv, archiveData)
	meta := RegistryPackage{
		Name:      "http", // stdlib name
		Version:   "1.0.0",
		PublicKey: hex.EncodeToString(pub),
		Signature: hex.EncodeToString(sig),
	}
	metaJSON, _ := json.Marshal(meta)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("metadata", string(metaJSON))
	part, _ := w.CreateFormFile("archive", "http-1.0.0.tar.gz")
	part.Write(archiveData)
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("should reject reserved name, got %d", resp.StatusCode)
	}
}

func TestRegistryServerPathTraversal(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/v1/packages/../../etc/passwd")
	resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("path traversal should be blocked")
	}
}

func TestRegistryServerBadPublish(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), testToken)
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
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	publishSignedToServer(t, ts, "lib", "1.0.0", testToken)
	publishSignedToServer(t, ts, "lib", "1.1.0", testToken)
	publishSignedToServer(t, ts, "other", "0.1.0", testToken)

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
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/v1/packages/nonexistent.tar.gz")
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatal("should 404")
	}
}

func TestRegistryEndToEnd(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewRegistryServer(dataDir, "tok")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	t.Setenv("HOME", t.TempDir())
	kp, _ := GenerateKey("test")
	SaveKey("test", kp)

	pkgDir := t.TempDir()
	SaveManifest(pkgDir, &Manifest{Name: "hello", Version: "0.1.0"})
	os.WriteFile(filepath.Join(pkgDir, "lib.weft"), []byte("pub fn greet { \"hi\" }\n"), 0644)

	archivePath := filepath.Join(t.TempDir(), "hello-0.1.0.tar.gz")
	PackArchive(pkgDir, archivePath)
	sig, _ := SignFile(kp, archivePath)
	pubHex := fmt.Sprintf("%x", kp.PublicKey)

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
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("publish failed: %d %s", resp.StatusCode, string(b))
	}

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

	dest := filepath.Join(t.TempDir(), "hello.tar.gz")
	if err := DownloadAndVerify(ts.URL, *pkg, dest); err != nil {
		t.Fatal(err)
	}
}
