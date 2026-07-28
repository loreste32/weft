package pkgman

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndSignVerify(t *testing.T) {
	kp, err := GenerateKey("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(kp.PublicKey) == 0 || len(kp.PrivateKey) == 0 {
		t.Fatal("empty keys")
	}

	data := []byte("hello weft packages")
	sig, err := Sign(kp, data)
	if err != nil {
		t.Fatal(err)
	}
	if sig == "" {
		t.Fatal("empty sig")
	}

	// Verify with hex public key
	pubHex := fmt.Sprintf("%x", kp.PublicKey)
	if err := Verify(pubHex, data, sig); err != nil {
		t.Fatal(err)
	}

	// Tampered data should fail
	if err := Verify(pubHex, []byte("tampered"), sig); err == nil {
		t.Fatal("tampered data should fail")
	}
}

func TestSignPublicOnlyFails(t *testing.T) {
	kp, _ := GenerateKey("test")
	pubOnly := &KeyPair{PublicKey: kp.PublicKey}
	_, err := Sign(pubOnly, []byte("data"))
	if err == nil {
		t.Fatal("public-only key should not sign")
	}
}

func TestSaveLoadKey(t *testing.T) {
	// Override home for test
	home := t.TempDir()
	t.Setenv("HOME", home)

	kp, _ := GenerateKey("testkey")
	if err := SaveKey("testkey", kp); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadKey("testkey")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", loaded.PublicKey) != fmt.Sprintf("%x", kp.PublicKey) {
		t.Fatal("public key mismatch")
	}

	// Sign with loaded key should work
	sig, err := Sign(loaded, []byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	pubHex := fmt.Sprintf("%x", kp.PublicKey)
	if err := Verify(pubHex, []byte("test"), sig); err != nil {
		t.Fatal(err)
	}
}

func TestLoadKeyNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := LoadKey("nonexistent")
	if err == nil {
		t.Fatal("should error")
	}
}

func TestExportPublicKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	kp, _ := GenerateKey("pub")
	SaveKey("pub", kp)

	pub, err := ExportPublicKey("pub")
	if err != nil {
		t.Fatal(err)
	}
	if pub.SecretHex != "" {
		t.Fatal("export should not include secret")
	}
	if pub.PublicHex == "" {
		t.Fatal("empty public hex")
	}
}

func TestListKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	names, _ := ListKeys()
	if len(names) != 0 {
		t.Fatal("should be empty initially")
	}

	kp, _ := GenerateKey("a")
	SaveKey("a", kp)
	kp2, _ := GenerateKey("b")
	SaveKey("b", kp2)

	names, err := ListKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(names))
	}
}

func TestSignVerifyFile(t *testing.T) {
	kp, _ := GenerateKey("test")
	path := filepath.Join(t.TempDir(), "pkg.tar.gz")
	os.WriteFile(path, []byte("fake archive content"), 0644)

	sig, err := SignFile(kp, path)
	if err != nil {
		t.Fatal(err)
	}

	pubHex := fmt.Sprintf("%x", kp.PublicKey)
	if err := VerifyFile(pubHex, path, sig); err != nil {
		t.Fatal(err)
	}

	// Tamper file
	os.WriteFile(path, []byte("tampered content"), 0644)
	if err := VerifyFile(pubHex, path, sig); err == nil {
		t.Fatal("tampered file should fail verification")
	}
}

func TestVerifyBadHex(t *testing.T) {
	if err := Verify("not_hex", []byte("data"), "also_not_hex"); err == nil {
		t.Fatal("bad hex should error")
	}
}

func TestVerifyBadKeyLength(t *testing.T) {
	if err := Verify("aabb", []byte("data"), "ccdd"); err == nil {
		t.Fatal("short key should error")
	}
}
