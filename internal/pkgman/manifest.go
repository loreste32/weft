// Package pkgman is Weft's package manager: deps without venv hell.
//
// Manifest:  weft.json
// Lockfile:  weft.lock  (JSON, content-addressed sums)
// Install:   vendor/<name>/  (project-local, reproducible)
// Cache:     ~/.weft/cache/  (fetched sources)
package pkgman

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest is weft.json (apps and third-party modules).
type Manifest struct {
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	// Type: "module" (library) or "app" (runnable). Default inferred.
	Type string `json:"type,omitempty"`
	// Entry is the main .weft file for modules (default lib.weft).
	Entry string `json:"entry,omitempty"`
	// Exports lists public API names (documentation + weft mod check).
	Exports []string `json:"exports,omitempty"`
	// License e.g. Apache-2.0, MIT
	License string `json:"license,omitempty"`
	// Authors optional contact list
	Authors []string `json:"authors,omitempty"`
	// Repository source URL
	Repository string `json:"repository,omitempty"`
	// Keywords for discovery (future registry)
	Keywords []string `json:"keywords,omitempty"`
	// Capabilities grants restricted stdlib packages to this module when loaded
	// as a third-party dep (e.g. ["sh"], ["@data"], ["*"] for full host).
	// Tokens starting with @ expand named profiles (see Profiles).
	// Empty → default-deny restricted packages. Apps are unrestricted.
	Capabilities []string `json:"capabilities,omitempty"`
	// CapabilityProfile is a named preset (none|data|net|host|full) merged into
	// capabilities. Prefer explicit packages when possible; profiles are shortcuts.
	CapabilityProfile string `json:"capability_profile,omitempty"`
	// Maturity: experimental|beta|stable|deprecated
	Maturity string             `json:"maturity,omitempty"`
	Deps     map[string]DepSpec `json:"deps,omitempty"`
}

// DepSpec is either a string shorthand or an object.
// String forms:
//
//	"./path" | "../path"
//	"github.com/org/repo@v1.2.3"
//	"https://example.com/pkg.zip"
type DepSpec struct {
	Path    string `json:"path,omitempty"`
	Git     string `json:"git,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Branch  string `json:"branch,omitempty"`
	URL     string `json:"url,omitempty"`
	Version string `json:"version,omitempty"` // for registry shorthand later
	// Raw holds string-form specs before normalize
	raw string
}

func (d *DepSpec) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*d = ParseDepString(s)
		return nil
	}
	type plain DepSpec
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*d = DepSpec(p)
	return nil
}

// ParseDepString parses shorthand dependency specs.
func ParseDepString(s string) DepSpec {
	s = strings.TrimSpace(s)
	if s == "" {
		return DepSpec{}
	}
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") || strings.HasPrefix(s, "/") {
		return DepSpec{Path: s, raw: s}
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if strings.HasSuffix(s, ".git") {
			return DepSpec{Git: s, raw: s}
		}
		return DepSpec{URL: s, raw: s}
	}
	// github.com/org/repo@version or org/repo@version
	ver := ""
	repo := s
	if i := strings.LastIndex(s, "@"); i > 0 {
		repo = s[:i]
		ver = s[i+1:]
	}
	if !strings.Contains(repo, "://") {
		if strings.Count(repo, "/") == 1 {
			repo = "https://github.com/" + repo + ".git"
		} else if strings.HasPrefix(repo, "github.com/") {
			repo = "https://" + repo + ".git"
		}
	}
	return DepSpec{Git: repo, Tag: ver, Version: ver, raw: s}
}

// Lockfile is weft.lock
type Lockfile struct {
	Version  int         `json:"version"`
	Packages []LockedPkg `json:"packages"`
}

// LockedPkg is one resolved dependency.
type LockedPkg struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Version string `json:"version,omitempty"`
	Sum     string `json:"sum"` // sha256 of tree or archive
	Dir     string `json:"dir"` // relative vendor path
}

// LoadManifest reads weft.json (or legacy loom.json) from dir.
func LoadManifest(dir string) (*Manifest, error) {
	path := dir
	if !strings.HasSuffix(dir, "weft.json") && !strings.HasSuffix(dir, "loom.json") {
		path = filepath.Join(dir, "weft.json")
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(dir, "loom.json")
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("weft.json: %w", err)
	}
	if m.Deps == nil {
		m.Deps = map[string]DepSpec{}
	}
	return &m, nil
}

// SaveManifest writes weft.json.
func SaveManifest(dir string, m *Manifest) error {
	if m.Deps == nil {
		m.Deps = map[string]DepSpec{}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(filepath.Join(dir, "weft.json"), b, 0o644)
}

// LoadLock reads weft.lock if present.
func LoadLock(dir string) (*Lockfile, error) {
	b, err := os.ReadFile(filepath.Join(dir, "weft.lock"))
	if err != nil {
		if os.IsNotExist(err) {
			return &Lockfile{Version: 1}, nil
		}
		return nil, err
	}
	var l Lockfile
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, err
	}
	if l.Version == 0 {
		l.Version = 1
	}
	return &l, nil
}

// SaveLock writes weft.lock with stable package order.
func SaveLock(dir string, l *Lockfile) error {
	l.Version = 1
	sort.Slice(l.Packages, func(i, j int) bool {
		return l.Packages[i].Name < l.Packages[j].Name
	})
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	// Replace the lock atomically so a failed write cannot leave a truncated
	// lockfile that disagrees with the vendor tree.
	tmp, err := os.CreateTemp(dir, ".weft-lock-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, filepath.Join(dir, "weft.lock"))
}

// Init creates a minimal weft.json in dir.
func Init(dir, name string) error {
	if name == "" {
		name = filepath.Base(dir)
	}
	path := filepath.Join(dir, "weft.json")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("weft.json already exists")
	}
	return SaveManifest(dir, &Manifest{
		Name:    name,
		Version: "0.1.0",
		Deps:    map[string]DepSpec{},
	})
}

// HashDir computes sha256 of all files under dir (paths relative, sorted).
// Symlinks are rejected (integrity must not follow outside the tree).
func HashDir(dir string) (string, error) {
	h := sha256.New()
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			rel, _ := filepath.Rel(dir, path)
			return fmt.Errorf("symlink not allowed in package tree: %s", rel)
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\n%d\n", rel, len(b))
		h.Write(b)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// DefaultCacheDir is ~/.weft/cache
func DefaultCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".weft", "cache"), nil
}

// VendorDir is project vendor/
func VendorDir(project string) string {
	return filepath.Join(project, "vendor")
}

// FindPackageEntry finds the main .weft (or legacy .loom) file in a package directory.
func FindPackageEntry(dir string) (string, error) {
	base := filepath.Base(dir)
	// Explicit entry from weft.json wins
	if m, err := LoadManifest(dir); err == nil && m.Entry != "" {
		p := filepath.Join(dir, m.Entry)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		return "", fmt.Errorf("weft.json entry %q not found in %s", m.Entry, dir)
	}
	candidates := []string{
		"lib.weft", "mod.weft", "index.weft", base + ".weft",
		"src/lib.weft", "src/mod.weft",
		"lib.loom", "mod.loom", "index.loom", base + ".loom",
		"src/lib.loom", "src/mod.loom",
	}
	if m, err := LoadManifest(dir); err == nil && m.Name != "" {
		candidates = append([]string{
			m.Name + ".weft", "src/" + m.Name + ".weft",
			m.Name + ".loom", "src/" + m.Name + ".loom",
		}, candidates...)
	}
	for _, c := range candidates {
		p := filepath.Join(dir, c)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var scripts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".weft") || strings.HasSuffix(n, ".loom") {
			scripts = append(scripts, filepath.Join(dir, n))
		}
	}
	if len(scripts) == 1 {
		return scripts[0], nil
	}
	return "", fmt.Errorf("no package entry in %s (want lib.weft)", dir)
}

// ResolveSearchPaths returns dirs to search for bare package imports.
func ResolveSearchPaths(projectDir string) []string {
	var paths []string
	// 1. vendor/
	paths = append(paths, VendorDir(projectDir))
	// 2. WEFT_PATH
	if p := os.Getenv("WEFT_PATH"); p != "" {
		for _, part := range strings.Split(p, string(os.PathListSeparator)) {
			if part != "" {
				paths = append(paths, part)
			}
		}
	}
	// 3. project root (for local packages folder)
	paths = append(paths, filepath.Join(projectDir, "packages"))
	return paths
}

// FindInstalledPackage looks up name under search paths.
func FindInstalledPackage(projectDir, name string) (dir string, entry string, err error) {
	for _, root := range ResolveSearchPaths(projectDir) {
		cand := filepath.Join(root, name)
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			entry, err := FindPackageEntry(cand)
			if err == nil {
				return cand, entry, nil
			}
		}
	}
	// A package's declared path dependencies are also valid during source-tree
	// checks. Installation still vendors them for reproducible applications;
	// this fallback lets `mod check --tests` compile a package before install
	// without weakening the package-root path-import restriction.
	if manifest, loadErr := LoadManifest(projectDir); loadErr == nil {
		if spec, ok := manifest.Deps[name]; ok && spec.Path != "" {
			candidate := ResolveDepPaths(spec, projectDir).Path
			if st, statErr := os.Stat(candidate); statErr == nil && st.IsDir() {
				entry, entryErr := FindPackageEntry(candidate)
				if entryErr == nil {
					return candidate, entry, nil
				}
				return "", "", fmt.Errorf("declared dependency %q has no valid package entry: %w", name, entryErr)
			}
			return "", "", fmt.Errorf("declared dependency %q path does not exist: %s", name, candidate)
		}
	}
	// Auto-fetch: registry first, then git for URL paths
	if os.Getenv("WEFT_NO_AUTO_FETCH") != "1" {
		// try git clone for URL-like paths (github.com/user/repo)
		if strings.Contains(name, "/") && strings.Contains(name, ".") {
			if fetched, _ := AutoFetchFromGit(projectDir, name); fetched {
				pkgName := name[strings.LastIndex(name, "/")+1:]
				for _, root := range ResolveSearchPaths(projectDir) {
					cand := filepath.Join(root, pkgName)
					if st, err := os.Stat(cand); err == nil && st.IsDir() {
						entry, err := FindPackageEntry(cand)
						if err == nil {
							return cand, entry, nil
						}
					}
				}
			}
		}
		if fetched, fetchErr := AutoFetchFromRegistry(projectDir, name); fetchErr == nil && fetched {
			for _, root := range ResolveSearchPaths(projectDir) {
				cand := filepath.Join(root, name)
				if st, err := os.Stat(cand); err == nil && st.IsDir() {
					entry, err := FindPackageEntry(cand)
					if err == nil {
						return cand, entry, nil
					}
				}
			}
		}
	}
	return "", "", fmt.Errorf("package %q not found (run: weft registry install %s)", name, name)
}

// AutoFetchFromRegistry downloads a package from the registry into vendor/.
func AutoFetchFromRegistry(projectDir, name string) (bool, error) {
	pkgName := name
	if strings.Contains(pkgName, "/") {
		parts := strings.Split(pkgName, "/")
		pkgName = parts[len(parts)-1]
	}

	registryURL := RegistryURL()
	idx, err := FetchIndex(registryURL)
	if err != nil {
		return false, err
	}
	pkg, err := FindRegistryPackage(idx, pkgName, "")
	if err != nil {
		return false, err
	}

	cacheDir, err := DefaultCacheDir()
	if err != nil {
		return false, err
	}
	archivePath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s.tar.gz", pkg.Name, pkg.Version))
	if err := DownloadAndVerify(registryURL, *pkg, archivePath); err != nil {
		return false, err
	}

	vendorDir := filepath.Join(projectDir, "vendor", pkgName)
	os.RemoveAll(vendorDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return false, err
	}
	if err := ExtractArchive(archivePath, vendorDir); err != nil {
		os.RemoveAll(vendorDir)
		return false, err
	}

	fmt.Fprintf(os.Stderr, "auto-installed %s@%s\n", pkgName, pkg.Version)
	return true, nil
}

// AutoFetchFromGit clones a git URL into vendor/.
// Supports: "github.com/user/repo", "github.com/user/repo/subdir"
func AutoFetchFromGit(projectDir, importPath string) (bool, error) {
	parts := strings.Split(importPath, "/")
	if len(parts) < 3 {
		return false, fmt.Errorf("invalid git import path: %s", importPath)
	}

	// extract repo URL and subdir
	host := parts[0]
	user := parts[1]
	repo := parts[2]
	repoURL := fmt.Sprintf("https://%s/%s/%s.git", host, user, repo)

	// package name is the last part of the path
	pkgName := parts[len(parts)-1]

	// version tag
	version := ""
	if at := strings.Index(repo, "@"); at > 0 {
		version = repo[at+1:]
		repo = repo[:at]
		repoURL = fmt.Sprintf("https://%s/%s/%s.git", host, user, repo)
	}

	vendorDir := filepath.Join(projectDir, "vendor", pkgName)
	if _, err := os.Stat(vendorDir); err == nil {
		return true, nil // already present
	}

	// clone to cache first
	cacheDir, err := DefaultCacheDir()
	if err != nil {
		return false, err
	}
	cloneDir := filepath.Join(cacheDir, "git", fmt.Sprintf("%s_%s_%s", host, user, repo))

	if _, err := os.Stat(cloneDir); err != nil {
		args := []string{"clone", "--depth=1"}
		if version != "" {
			args = append(args, "--branch", version)
		}
		args = append(args, repoURL, cloneDir)
		cmd := exec.Command("git", args...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("git clone %s: %w", repoURL, err)
		}
	}

	// copy the subdir (or root) to vendor/
	srcDir := cloneDir
	if len(parts) > 3 {
		srcDir = filepath.Join(cloneDir, strings.Join(parts[3:], "/"))
	}

	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return false, err
	}

	// simple recursive copy
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if filepath.Base(path) == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(srcDir, path)
		dst := filepath.Join(vendorDir, rel)
		os.MkdirAll(filepath.Dir(dst), 0o755)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		return false, err
	}

	fmt.Fprintf(os.Stderr, "auto-installed %s from %s\n", pkgName, repoURL)
	return true, nil
}
