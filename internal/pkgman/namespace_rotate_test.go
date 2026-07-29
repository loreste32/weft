package pkgman

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNamespaceKeyRotation(t *testing.T) {
	srv := NewRegistryServer(t.TempDir(), testToken)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// first publish pins key A
	publishSignedToServer(t, ts, "rot", "1.0.0", testToken)

	// second version with different key must fail before rotation
	pubB, privB, _ := ed25519.GenerateKey(rand.Reader)
	archiveData := []byte("new key archive")
	sig := ed25519.Sign(privB, archiveData)
	meta := RegistryPackage{
		Name: "rot", Version: "1.1.0",
		PublicKey: hex.EncodeToString(pubB),
		Signature: hex.EncodeToString(sig),
	}
	metaJSON, _ := json.Marshal(meta)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("metadata", string(metaJSON))
	part, _ := w.CreateFormFile("archive", "rot-1.1.0.tar.gz")
	_, _ = part.Write(archiveData)
	_ = w.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("want 403 before rotate, got %d %s", resp.StatusCode, b)
	}
	if !strings.Contains(string(b), "rotate") && !strings.Contains(string(b), "signing key") {
		t.Fatalf("unexpected body: %s", b)
	}

	// rotate: add key B
	rotBody, _ := json.Marshal(map[string]string{"public_key": hex.EncodeToString(pubB)})
	req, _ = http.NewRequest("POST", ts.URL+"/v1/namespaces/rot/keys", bytes.NewReader(rotBody))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		bb, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("rotate status %d %s", resp.StatusCode, bb)
	}
	resp.Body.Close()

	// publish with B should work now
	body.Reset()
	w = multipart.NewWriter(&body)
	_ = w.WriteField("metadata", string(metaJSON))
	part, _ = w.CreateFormFile("archive", "rot-1.1.0.tar.gz")
	_, _ = part.Write(archiveData)
	_ = w.Close()
	req, _ = http.NewRequest("POST", ts.URL+"/v1/publish", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("publish after rotate: %d %s", resp.StatusCode, bb)
	}
	resp.Body.Close()
}
