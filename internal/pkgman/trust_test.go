package pkgman

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageNamespace(t *testing.T) {
	if PackageNamespace("mold") != "mold" {
		t.Fatal(PackageNamespace("mold"))
	}
	if PackageNamespace("acme/billing") != "acme" {
		t.Fatal(PackageNamespace("acme/billing"))
	}
}

func TestTrustStoreRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// keysDir uses UserHomeDir — set on Unix
	os.Setenv("HOME", home)
	// Ensure .weft path under temp home
	_ = os.MkdirAll(filepath.Join(home, ".weft"), 0o700)

	kp, err := GenerateKey("t")
	if err != nil {
		t.Fatal(err)
	}
	pub := hex.EncodeToString(kp.PublicKey)

	if err := TrustKey("mold", pub, "test"); err != nil {
		t.Fatal(err)
	}
	// idempotent
	if err := TrustKey("mold", pub, "test"); err != nil {
		t.Fatal(err)
	}

	ts, err := LoadTrustStore()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := ts.Namespaces["mold"]
	if !ok || len(e.PublicKeys) != 1 {
		t.Fatalf("%+v", ts)
	}

	pkg := RegistryPackage{Name: "mold", Version: "1.0.0", PublicKey: pub, Signature: "00"}
	if err := CheckPackageTrust(pkg); err != nil {
		t.Fatal(err)
	}

	// wrong key
	pkg.PublicKey = hex.EncodeToString(make([]byte, 32))
	if err := CheckPackageTrust(pkg); err == nil {
		t.Fatal("want untrusted key error")
	}

	// require trust for unknown ns
	t.Setenv("WEFT_REQUIRE_TRUST", "1")
	if err := CheckPackageTrust(RegistryPackage{Name: "other", Version: "1.0.0", PublicKey: pub}); err == nil {
		t.Fatal("want require trust error")
	}
	t.Setenv("WEFT_REQUIRE_TRUST", "")

	if err := UntrustKey("mold", ""); err != nil {
		t.Fatal(err)
	}
	ts, _ = LoadTrustStore()
	if _, ok := ts.Namespaces["mold"]; ok {
		t.Fatal("expected removed")
	}
}

func TestTrustKeyBadHex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.Setenv("HOME", home)
	if err := TrustKey("x", "not-hex", ""); err == nil {
		t.Fatal("want bad hex error")
	}
}

func TestRotateTrust(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.Setenv("HOME", home)

	kp1, _ := GenerateKey("a")
	kp2, _ := GenerateKey("b")
	old := hex.EncodeToString(kp1.PublicKey)
	neu := hex.EncodeToString(kp2.PublicKey)
	if err := TrustKey("ns", old, ""); err != nil {
		t.Fatal(err)
	}
	if err := RotateTrust("ns", neu, old); err != nil {
		t.Fatal(err)
	}
	ts, _ := LoadTrustStore()
	e := ts.Namespaces["ns"]
	if len(e.PublicKeys) != 1 || !strings.EqualFold(e.PublicKeys[0], neu) {
		t.Fatalf("keys=%v", e.PublicKeys)
	}
	// both keys after rotate without retire
	if err := TrustKey("ns2", old, ""); err != nil {
		t.Fatal(err)
	}
	if err := RotateTrust("ns2", neu, ""); err != nil {
		t.Fatal(err)
	}
	ts, _ = LoadTrustStore()
	if len(ts.Namespaces["ns2"].PublicKeys) != 2 {
		t.Fatalf("want 2 keys, got %v", ts.Namespaces["ns2"].PublicKeys)
	}
}
