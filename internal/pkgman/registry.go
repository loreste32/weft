package pkgman

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loreste/weft/internal/netsafe"
)

// DefaultRegistryURL is the public Weft package registry.
// Override with WEFT_REGISTRY env var.
const DefaultRegistryURL = "https://registry.weftproject.dev"

// RegistryURL returns the active registry URL.
func RegistryURL() string {
	if u := os.Getenv("WEFT_REGISTRY"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return DefaultRegistryURL
}

// RegistryIndex is the root index served at GET /v1/index.json.
type RegistryIndex struct {
	Packages []RegistryPackage `json:"packages"`
}

// RegistryPackage is one published package in the registry.
type RegistryPackage struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Summary     string            `json:"summary,omitempty"`
	Author      string            `json:"author,omitempty"`
	License     string            `json:"license,omitempty"`
	PublicKey   string            `json:"public_key,omitempty"` // hex ed25519
	Signature   string            `json:"signature,omitempty"`  // hex of archive sig
	ArchiveURL  string            `json:"archive_url"`          // relative or absolute
	Sum         string            `json:"sum,omitempty"`        // sha256 of archive
	Keywords    []string          `json:"keywords,omitempty"`
	Deps        map[string]string `json:"deps,omitempty"` // name → version constraint
	Published   string            `json:"published,omitempty"`
	Exports     []string          `json:"exports,omitempty"`     // pub fn/type names
	Description string            `json:"description,omitempty"` // long description
}

// FetchIndex downloads the registry index.
func FetchIndex(registryURL string) (*RegistryIndex, error) {
	url := registryURL + "/v1/index.json"
	if err := netsafe.CheckURL(url); err != nil {
		return nil, fmt.Errorf("registry URL rejected: %w", err)
	}
	client := netsafe.SafeHTTPClient(30 * time.Second)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("registry fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("registry %s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var idx RegistryIndex
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, fmt.Errorf("registry index: %w", err)
	}
	return &idx, nil
}

// SearchRegistry searches the index for packages matching query.
func SearchRegistry(idx *RegistryIndex, query string) []RegistryPackage {
	if query == "" {
		return idx.Packages
	}
	q := strings.ToLower(query)
	var out []RegistryPackage
	for _, p := range idx.Packages {
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Summary), q) {
			out = append(out, p)
		}
	}
	return out
}

// FindRegistryPackage finds a package by exact name, optionally checking version constraint.
func FindRegistryPackage(idx *RegistryIndex, name, constraint string) (*RegistryPackage, error) {
	name = strings.TrimSpace(name)
	var best *RegistryPackage
	for i := range idx.Packages {
		if idx.Packages[i].Name != name {
			continue
		}
		if constraint != "" && constraint != "*" {
			if !MatchConstraint(idx.Packages[i].Version, constraint) {
				continue
			}
		}
		p := &idx.Packages[i]
		if best == nil || versionGreater(p.Version, best.Version) {
			best = p
		}
	}
	if best == nil {
		return nil, fmt.Errorf("package %q not found in registry", name)
	}
	return best, nil
}

func versionGreater(a, b string) bool {
	am, ai, ap, _ := ParseSemver(a)
	bm, bi, bp, _ := ParseSemver(b)
	if am != bm {
		return am > bm
	}
	if ai != bi {
		return ai > bi
	}
	return ap > bp
}

// ResolveArchiveURL builds the full download URL for a package archive.
// Absolute URLs in the index are allowed (CDN hosting) but validated by
// DownloadAndVerify before fetch.
func ResolveArchiveURL(registryURL string, pkg RegistryPackage) string {
	u := pkg.ArchiveURL
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	// Reject path traversal in relative archive URLs
	if strings.Contains(u, "..") {
		return registryURL + "/invalid"
	}
	return registryURL + "/" + strings.TrimPrefix(u, "/")
}

// DownloadAndVerify fetches a package archive, verifies its signature, and saves to dest.
func DownloadAndVerify(registryURL string, pkg RegistryPackage, dest string) error {
	archiveURL := ResolveArchiveURL(registryURL, pkg)

	// SSRF: validate the resolved URL before fetching
	if err := netsafe.CheckURL(archiveURL); err != nil {
		return fmt.Errorf("download %s rejected: %w", pkg.Name, err)
	}

	client := netsafe.SafeHTTPClient(120 * time.Second)
	resp, err := client.Get(archiveURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", pkg.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download %s: HTTP %d", pkg.Name, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxArchiveBytes))
	if err != nil {
		return err
	}

	// Verify hash if present (before sig — cheap check first)
	if pkg.Sum != "" {
		got := "sha256:" + hex.EncodeToString(sha256Sum(data))
		if got != pkg.Sum {
			return fmt.Errorf("checksum mismatch for %s@%s: expected %s, got %s", pkg.Name, pkg.Version, pkg.Sum, got)
		}
	}

	// Verify signature — reject unsigned packages unless WEFT_ALLOW_UNSIGNED=1
	if pkg.PublicKey != "" && pkg.Signature != "" {
		if err := Verify(pkg.PublicKey, data, pkg.Signature); err != nil {
			return fmt.Errorf("signature verification failed for %s@%s: %w", pkg.Name, pkg.Version, err)
		}
	} else if os.Getenv("WEFT_ALLOW_UNSIGNED") != "1" {
		return fmt.Errorf("package %s@%s is unsigned — refusing to install (set WEFT_ALLOW_UNSIGNED=1 to override)", pkg.Name, pkg.Version)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// PublishPackage validates, packs, signs, and uploads a package to the registry.
func PublishPackage(dir, keyName, registryURL string) error {
	// Validate
	report, err := ValidatePackage(dir)
	if err != nil {
		return err
	}
	if !report.OK {
		return fmt.Errorf("package validation failed:\n  %s", strings.Join(report.Errors, "\n  "))
	}

	// Load manifest
	m, err := LoadManifest(dir)
	if err != nil {
		return err
	}
	if m.Name == "" || m.Version == "" {
		return fmt.Errorf("weft.json must have name and version to publish")
	}

	// Sanitize name/version for temp path (no path traversal)
	safeName := strings.ReplaceAll(strings.ReplaceAll(m.Name, "/", "_"), "..", "_")
	safeVer := strings.ReplaceAll(strings.ReplaceAll(m.Version, "/", "_"), "..", "_")
	archivePath := filepath.Join(os.TempDir(), fmt.Sprintf("weft-publish-%s-%s.tar.gz", safeName, safeVer))
	defer os.Remove(archivePath)
	if err := PackArchive(dir, archivePath); err != nil {
		return fmt.Errorf("pack: %w", err)
	}

	// Hash
	sum, err := hashFile(archivePath)
	if err != nil {
		return err
	}

	// Sign
	var pubHex, sigHex string
	if keyName != "" {
		kp, err := LoadKey(keyName)
		if err != nil {
			return err
		}
		sigHex, err = SignFile(kp, archivePath)
		if err != nil {
			return fmt.Errorf("sign: %w", err)
		}
		pubHex = fmt.Sprintf("%x", kp.PublicKey)
	}

	// Build metadata
	meta := RegistryPackage{
		Name:      m.Name,
		Version:   m.Version,
		Summary:   m.Description,
		License:   m.License,
		PublicKey: pubHex,
		Signature: sigHex,
		Sum:       sum,
		Keywords:  m.Keywords,
		Published: time.Now().UTC().Format(time.RFC3339),
	}
	if len(m.Authors) > 0 {
		meta.Author = m.Authors[0]
	}
	if len(m.Exports) > 0 {
		meta.Exports = m.Exports
	}
	// Convert deps
	if len(m.Deps) > 0 {
		meta.Deps = make(map[string]string)
		for k, v := range m.Deps {
			meta.Deps[k] = v.Version
		}
	}

	// Upload
	return uploadPackage(registryURL, meta, archivePath)
}

func uploadPackage(registryURL string, meta RegistryPackage, archivePath string) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	// metadata part
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := w.WriteField("metadata", string(metaJSON)); err != nil {
		return err
	}

	// archive part
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	part, err := w.CreateFormFile("archive", filepath.Base(archivePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	w.Close()

	url := registryURL + "/v1/publish"
	if err := netsafe.CheckURL(url); err != nil {
		return fmt.Errorf("registry URL rejected: %w", err)
	}
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Auth token from env
	if token := os.Getenv("WEFT_REGISTRY_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := netsafe.SafeHTTPClient(120 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("publish %s@%s: HTTP %d: %s", meta.Name, meta.Version, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}
