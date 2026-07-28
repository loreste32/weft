package pkgman

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KeyPair is an ed25519 signing key for package publishing.
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// KeyFile is the on-disk format for ~/.weft/keys/<name>.json.
type KeyFile struct {
	Name      string `json:"name"`
	PublicHex string `json:"public"`
	SecretHex string `json:"secret,omitempty"` // omitted in public exports
}

// GenerateKey creates a new ed25519 keypair.
func GenerateKey(name string) (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &KeyPair{PublicKey: pub, PrivateKey: priv}, nil
}

// SaveKey writes a keypair to ~/.weft/keys/<name>.json.
func SaveKey(name string, kp *KeyPair) error {
	if err := validateKeyName(name); err != nil {
		return err
	}
	dir, err := keysDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	kf := KeyFile{
		Name:      name,
		PublicHex: hex.EncodeToString(kp.PublicKey),
		SecretHex: hex.EncodeToString(kp.PrivateKey),
	}
	b, err := json.MarshalIndent(kf, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, name+".json")
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// LoadKey reads a keypair from ~/.weft/keys/<name>.json.
func LoadKey(name string) (*KeyPair, error) {
	if err := validateKeyName(name); err != nil {
		return nil, err
	}
	dir, err := keysDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("key %q not found (run: weft registry keygen %s)", name, name)
	}
	var kf KeyFile
	if err := json.Unmarshal(b, &kf); err != nil {
		return nil, err
	}
	pub, err := hex.DecodeString(kf.PublicHex)
	if err != nil {
		return nil, fmt.Errorf("bad public key hex: %w", err)
	}
	kp := &KeyPair{PublicKey: ed25519.PublicKey(pub)}
	if kf.SecretHex != "" {
		priv, err := hex.DecodeString(kf.SecretHex)
		if err != nil {
			return nil, fmt.Errorf("bad secret key hex: %w", err)
		}
		kp.PrivateKey = ed25519.PrivateKey(priv)
	}
	return kp, nil
}

// ExportPublicKey returns the KeyFile without the secret (for sharing/registry).
func ExportPublicKey(name string) (*KeyFile, error) {
	kp, err := LoadKey(name)
	if err != nil {
		return nil, err
	}
	return &KeyFile{
		Name:      name,
		PublicHex: hex.EncodeToString(kp.PublicKey),
	}, nil
}

// Sign signs data with the private key.
func Sign(kp *KeyPair, data []byte) (string, error) {
	if kp.PrivateKey == nil {
		return "", fmt.Errorf("no private key (public-only key)")
	}
	sig := ed25519.Sign(kp.PrivateKey, data)
	return hex.EncodeToString(sig), nil
}

// Verify checks a hex signature against a public key.
func Verify(pubHex string, data []byte, sigHex string) error {
	pub, err := hex.DecodeString(pubHex)
	if err != nil {
		return fmt.Errorf("bad public key: %w", err)
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("bad signature: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key length")
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), data, sig) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

// SignFile signs a file and returns the hex signature.
func SignFile(kp *KeyPair, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return Sign(kp, data)
}

// VerifyFile checks a file's signature.
func VerifyFile(pubHex, path, sigHex string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return Verify(pubHex, data, sigHex)
}

// validateKeyName rejects path traversal and shell-hostile characters in key names.
func validateKeyName(name string) error {
	if name == "" {
		return fmt.Errorf("key name required")
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return fmt.Errorf("key name must not contain path separators or '..'")
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, ".") {
		return fmt.Errorf("key name must not start with '-' or '.'")
	}
	if strings.ContainsAny(name, "\x00\r\n") {
		return fmt.Errorf("key name contains illegal characters")
	}
	return nil
}

func keysDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".weft", "keys"), nil
}

// ListKeys returns names of all keys in ~/.weft/keys/.
func ListKeys() ([]string, error) {
	dir, err := keysDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".json") {
			names = append(names, strings.TrimSuffix(n, ".json"))
		}
	}
	return names, nil
}
