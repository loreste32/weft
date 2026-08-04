package accelerator

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDisabledByEnv(t *testing.T) {
	t.Setenv(EnvDisablePlugins, "1")
	t.Setenv(EnvAllowlist, "")
	t.Setenv(EnvRequireChecksum, "")
	t.Setenv(EnvChecksum, "")
	if _, err := Load("/tmp/does-not-matter.so"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Load with disable env = %v, want disabled error", err)
	}
}

func TestAllowlistRejectsOutsidePath(t *testing.T) {
	dir := t.TempDir()
	allowed := filepath.Join(dir, "allowed")
	if err := os.MkdirAll(allowed, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside.so")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvDisablePlugins, "")
	t.Setenv(EnvAllowlist, allowed)
	t.Setenv(EnvRequireChecksum, "")
	t.Setenv(EnvChecksum, "")
	if err := enforceAllowlist(outside); err == nil || !strings.Contains(err.Error(), "ALLOWLIST") {
		t.Fatalf("enforceAllowlist outside = %v", err)
	}
	inside := filepath.Join(allowed, "plugin.so")
	if err := os.WriteFile(inside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enforceAllowlist(inside); err != nil {
		t.Fatalf("enforceAllowlist inside = %v", err)
	}
}

func TestChecksumRequiredAndVerified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.so")
	payload := []byte("trusted-plugin-bytes")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])

	t.Setenv(EnvDisablePlugins, "")
	t.Setenv(EnvAllowlist, "")
	t.Setenv(EnvRequireChecksum, "1")
	t.Setenv(EnvChecksum, "")
	if err := enforceChecksum(path); err == nil || !strings.Contains(err.Error(), "checksum required") {
		t.Fatalf("require without checksum = %v", err)
	}

	t.Setenv(EnvChecksum, hexSum)
	if err := enforceChecksum(path); err != nil {
		t.Fatalf("matching env checksum = %v", err)
	}

	t.Setenv(EnvChecksum, "")
	sidecar := path + ".sha256"
	if err := os.WriteFile(sidecar, []byte(hexSum+"  plugin.so\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enforceChecksum(path); err != nil {
		t.Fatalf("matching sidecar checksum = %v", err)
	}

	if err := os.WriteFile(sidecar, []byte(strings.Repeat("ab", 32)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enforceChecksum(path); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("bad sidecar checksum = %v", err)
	}
}

func TestNormalizeHexChecksum(t *testing.T) {
	if _, err := normalizeHexChecksum("deadbeef"); err == nil {
		t.Fatal("short checksum accepted")
	}
	good := strings.Repeat("ab", 32)
	got, err := normalizeHexChecksum(strings.ToUpper(good))
	if err != nil || got != good {
		t.Fatalf("normalize = %q, %v", got, err)
	}
}
