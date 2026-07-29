package pkgman

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TrustStore is the local key-trust database (~/.weft/trust.json).
// Namespaces map package namespace → allowed ed25519 public keys (hex).
//
// Namespace of a package is the first path segment: "mold" → "mold",
// "acme/billing" → "acme". Flat registry names use the full name as namespace.
type TrustStore struct {
	// Namespaces maps namespace → trust entry.
	Namespaces map[string]NamespaceTrust `json:"namespaces"`
	// RequireTrust: when true, installs fail unless the package namespace is trusted
	// and the signing key matches. Env WEFT_REQUIRE_TRUST=1 also enables this.
	RequireTrust bool `json:"require_trust,omitempty"`
}

// NamespaceTrust lists public keys allowed to sign packages in a namespace.
type NamespaceTrust struct {
	PublicKeys []string `json:"public_keys"`
	Note       string   `json:"note,omitempty"`
	Updated    string   `json:"updated,omitempty"`
}

// PackageNamespace returns the trust namespace for a package name.
func PackageNamespace(pkgName string) string {
	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return ""
	}
	if i := strings.IndexByte(pkgName, '/'); i > 0 {
		return pkgName[:i]
	}
	return pkgName
}

func trustPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".weft", "trust.json"), nil
}

// LoadTrustStore reads ~/.weft/trust.json (empty store if missing).
func LoadTrustStore() (*TrustStore, error) {
	path, err := trustPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TrustStore{Namespaces: map[string]NamespaceTrust{}}, nil
		}
		return nil, err
	}
	var ts TrustStore
	if err := json.Unmarshal(b, &ts); err != nil {
		return nil, fmt.Errorf("trust.json: %w", err)
	}
	if ts.Namespaces == nil {
		ts.Namespaces = map[string]NamespaceTrust{}
	}
	return &ts, nil
}

// SaveTrustStore writes ~/.weft/trust.json (mode 0600).
func SaveTrustStore(ts *TrustStore) error {
	path, err := trustPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if ts.Namespaces == nil {
		ts.Namespaces = map[string]NamespaceTrust{}
	}
	b, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// TrustKey adds a public key (hex) for namespace. Idempotent.
func TrustKey(namespace, pubHex, note string) error {
	namespace = strings.TrimSpace(namespace)
	pubHex = strings.ToLower(strings.TrimSpace(pubHex))
	if namespace == "" {
		return fmt.Errorf("namespace required")
	}
	if err := validateKeyName(namespace); err != nil && strings.Contains(namespace, "/") {
		return fmt.Errorf("invalid namespace: %w", err)
	}
	if pubHex == "" {
		return fmt.Errorf("public key hex required")
	}
	if _, err := decodePubHex(pubHex); err != nil {
		return err
	}
	ts, err := LoadTrustStore()
	if err != nil {
		return err
	}
	entry := ts.Namespaces[namespace]
	for _, k := range entry.PublicKeys {
		if strings.EqualFold(k, pubHex) {
			if note != "" {
				entry.Note = note
				entry.Updated = time.Now().UTC().Format(time.RFC3339)
				ts.Namespaces[namespace] = entry
				return SaveTrustStore(ts)
			}
			return nil // already trusted
		}
	}
	entry.PublicKeys = append(entry.PublicKeys, pubHex)
	if note != "" {
		entry.Note = note
	}
	entry.Updated = time.Now().UTC().Format(time.RFC3339)
	ts.Namespaces[namespace] = entry
	return SaveTrustStore(ts)
}

// UntrustKey removes a public key from a namespace, or the whole namespace if pubHex is empty.
func UntrustKey(namespace, pubHex string) error {
	namespace = strings.TrimSpace(namespace)
	pubHex = strings.ToLower(strings.TrimSpace(pubHex))
	ts, err := LoadTrustStore()
	if err != nil {
		return err
	}
	entry, ok := ts.Namespaces[namespace]
	if !ok {
		return fmt.Errorf("namespace %q is not in the trust store", namespace)
	}
	if pubHex == "" {
		delete(ts.Namespaces, namespace)
		return SaveTrustStore(ts)
	}
	var keys []string
	for _, k := range entry.PublicKeys {
		if !strings.EqualFold(k, pubHex) {
			keys = append(keys, k)
		}
	}
	if len(keys) == len(entry.PublicKeys) {
		return fmt.Errorf("key not found under namespace %q", namespace)
	}
	if len(keys) == 0 {
		delete(ts.Namespaces, namespace)
	} else {
		entry.PublicKeys = keys
		entry.Updated = time.Now().UTC().Format(time.RFC3339)
		ts.Namespaces[namespace] = entry
	}
	return SaveTrustStore(ts)
}

// TrustKeyFromLocalKey trusts the public half of a local signing key under namespace.
func TrustKeyFromLocalKey(namespace, keyName string) error {
	pub, err := ExportPublicKey(keyName)
	if err != nil {
		return err
	}
	note := "local key " + keyName
	return TrustKey(namespace, pub.PublicHex, note)
}

// RotateTrust adds newPubHex and optionally retires oldPubHex for a namespace.
// At least one key must remain after retirement.
func RotateTrust(namespace, newPubHex, retirePubHex string) error {
	namespace = strings.TrimSpace(namespace)
	newPubHex = strings.ToLower(strings.TrimSpace(newPubHex))
	retirePubHex = strings.ToLower(strings.TrimSpace(retirePubHex))
	if namespace == "" || newPubHex == "" {
		return fmt.Errorf("namespace and new public key required")
	}
	if err := TrustKey(namespace, newPubHex, "rotated"); err != nil {
		return err
	}
	if retirePubHex == "" || retirePubHex == newPubHex {
		return nil
	}
	ts, err := LoadTrustStore()
	if err != nil {
		return err
	}
	entry := ts.Namespaces[namespace]
	var keys []string
	for _, k := range entry.PublicKeys {
		if !strings.EqualFold(k, retirePubHex) {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return fmt.Errorf("cannot retire last remaining key for %q", namespace)
	}
	entry.PublicKeys = keys
	entry.Updated = time.Now().UTC().Format(time.RFC3339)
	entry.Note = "rotated"
	ts.Namespaces[namespace] = entry
	return SaveTrustStore(ts)
}

// CheckPackageTrust verifies that pkg's signing key is allowed for its namespace.
// Returns nil if trust is not required and namespace is unlisted (open mode),
// or if the key is trusted.
func CheckPackageTrust(pkg RegistryPackage) error {
	ts, err := LoadTrustStore()
	if err != nil {
		return err
	}
	require := ts.RequireTrust || os.Getenv("WEFT_REQUIRE_TRUST") == "1"
	ns := PackageNamespace(pkg.Name)
	entry, known := ts.Namespaces[ns]
	if !known {
		if require {
			return fmt.Errorf("package %s@%s: namespace %q is not trusted (run: weft registry trust %s <pubkey>)",
				pkg.Name, pkg.Version, ns, ns)
		}
		return nil
	}
	if pkg.PublicKey == "" {
		return fmt.Errorf("package %s@%s: unsigned but namespace %q requires a trusted key", pkg.Name, pkg.Version, ns)
	}
	for _, k := range entry.PublicKeys {
		if strings.EqualFold(k, pkg.PublicKey) {
			return nil
		}
	}
	return fmt.Errorf("package %s@%s: signing key is not trusted for namespace %q", pkg.Name, pkg.Version, ns)
}

func decodePubHex(pubHex string) ([]byte, error) {
	b, err := hex.DecodeString(pubHex)
	if err != nil {
		return nil, fmt.Errorf("bad public key: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length (want %d bytes)", ed25519.PublicKeySize)
	}
	return b, nil
}
