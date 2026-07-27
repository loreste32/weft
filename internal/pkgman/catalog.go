package pkgman

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PackageCatalog is packages/index.json (monorepo discovery — not a public registry).
type PackageCatalog struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Packages    []CatalogEntry `json:"packages"`
}

// CatalogEntry is one optional module in the catalog.
type CatalogEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// FindCatalog walks up from dir (or uses WEFT_PACKAGES / WEFT_CATALOG_URL) for packages/index.json.
// WEFT_CATALOG_URL may be an https://…/index.json for remote discovery (still path/git install).
func FindCatalog(start string) (catalogPath string, cat *PackageCatalog, err error) {
	if u := os.Getenv("WEFT_CATALOG_URL"); u != "" {
		c, e := LoadCatalogURL(u)
		if e == nil {
			return u, c, nil
		}
		// fall through to local if remote fails
	}
	if p := os.Getenv("WEFT_PACKAGES"); p != "" {
		idx := p
		if !strings.HasSuffix(p, "index.json") {
			idx = filepath.Join(p, "index.json")
		}
		c, e := LoadCatalog(idx)
		if e == nil {
			return idx, c, nil
		}
	}
	if start == "" {
		start, _ = os.Getwd()
	}
	dir := start
	for i := 0; i < 12; i++ {
		idx := filepath.Join(dir, "packages", "index.json")
		if c, e := LoadCatalog(idx); e == nil {
			return idx, c, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", nil, fmt.Errorf("no packages/index.json found (set WEFT_PACKAGES or run from monorepo)")
}

// LoadCatalog reads index.json from disk.
func LoadCatalog(path string) (*PackageCatalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseCatalog(b, path)
}

// LoadCatalogURL fetches a remote index.json (HTTP/HTTPS).
func LoadCatalogURL(rawURL string) (*PackageCatalog, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("catalog HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	return parseCatalog(b, rawURL)
}

func parseCatalog(b []byte, src string) (*PackageCatalog, error) {
	var c PackageCatalog
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if len(c.Packages) == 0 {
		return nil, fmt.Errorf("empty catalog %s", src)
	}
	return &c, nil
}

// FormatCatalog is a short human listing. Optional query filters by name/summary.
func FormatCatalog(path string, c *PackageCatalog) string {
	return FormatCatalogFilter(path, c, "")
}

// FormatCatalogFilter lists packages; query matches name or summary (case-insensitive).
func FormatCatalogFilter(path string, c *PackageCatalog, query string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "catalog  %s\n", path)
	if c.Description != "" {
		fmt.Fprintf(&b, "%s\n", c.Description)
	}
	entries := c.Packages
	if query != "" {
		entries = SearchCatalog(c, query)
		if len(entries) == 0 {
			fmt.Fprintf(&b, "(no packages matching %q)\n", query)
			return b.String()
		}
	}
	for _, p := range entries {
		sum := p.Summary
		if sum == "" {
			sum = "-"
		}
		ver := p.Version
		if ver == "" {
			ver = "-"
		}
		spec := catalogSpec(path, p.Path)
		fmt.Fprintf(&b, "  %-12s  %s  %s\n", p.Name, ver, sum)
		fmt.Fprintf(&b, "               weft packages get %s\n", p.Name)
		fmt.Fprintf(&b, "               weft get %s %s\n", p.Name, spec)
	}
	return b.String()
}

// FormatCatalogEntry is a detailed view of one package.
func FormatCatalogEntry(indexPath string, e CatalogEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "name     %s\n", e.Name)
	if e.Version != "" {
		fmt.Fprintf(&b, "version  %s\n", e.Version)
	}
	if e.Summary != "" {
		fmt.Fprintf(&b, "summary  %s\n", e.Summary)
	}
	fmt.Fprintf(&b, "path     %s\n", e.Path)
	abs := ResolveCatalogPath(indexPath, e.Path)
	fmt.Fprintf(&b, "abs      %s\n", abs)
	fmt.Fprintf(&b, "install  weft packages get %s\n", e.Name)
	if e.Version != "" {
		fmt.Fprintf(&b, "         weft packages get %s@^%s\n", e.Name, strings.TrimPrefix(e.Version, "v"))
	}
	return b.String()
}

func catalogSpec(indexPath, entryPath string) string {
	abs := ResolveCatalogPath(indexPath, entryPath)
	spec := entryPath
	if strings.HasPrefix(indexPath, "http://") || strings.HasPrefix(indexPath, "https://") {
		return entryPath
	}
	if root := filepath.Dir(filepath.Dir(indexPath)); root != "" {
		if rel, err := filepath.Rel(root, abs); err == nil {
			spec = rel
			if !strings.HasPrefix(spec, ".") {
				spec = "./" + spec
			}
		}
	}
	return spec
}

// ResolveCatalogPath turns catalog entry path into absolute path next to index.
func ResolveCatalogPath(indexPath, entryPath string) string {
	if filepath.IsAbs(entryPath) {
		return entryPath
	}
	if strings.HasPrefix(indexPath, "http://") || strings.HasPrefix(indexPath, "https://") {
		// remote catalog: path is informational / relative to monorepo convention
		return entryPath
	}
	base := filepath.Dir(indexPath)
	// paths in index are like ./ml relative to packages/
	return filepath.Clean(filepath.Join(base, entryPath))
}

// SearchCatalog filters entries by substring in name or summary (case-insensitive).
func SearchCatalog(c *PackageCatalog, query string) []CatalogEntry {
	if c == nil || query == "" {
		if c == nil {
			return nil
		}
		return append([]CatalogEntry{}, c.Packages...)
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var out []CatalogEntry
	for _, p := range c.Packages {
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Summary), q) {
			out = append(out, p)
		}
	}
	return out
}

// FindCatalogEntry looks up a package by exact name (case-sensitive).
// On miss, error includes up to 3 close name suggestions.
func FindCatalogEntry(c *PackageCatalog, name string) (*CatalogEntry, error) {
	if c == nil {
		return nil, fmt.Errorf("empty catalog")
	}
	name = strings.TrimSpace(name)
	for i := range c.Packages {
		if c.Packages[i].Name == name {
			return &c.Packages[i], nil
		}
	}
	// case-insensitive exact
	lower := strings.ToLower(name)
	for i := range c.Packages {
		if strings.ToLower(c.Packages[i].Name) == lower {
			return &c.Packages[i], nil
		}
	}
	// suggestions: substring / prefix
	var sug []string
	for _, p := range c.Packages {
		ln := strings.ToLower(p.Name)
		if strings.Contains(ln, lower) || strings.HasPrefix(lower, ln) || strings.HasPrefix(ln, lower) {
			sug = append(sug, p.Name)
		}
		if len(sug) >= 3 {
			break
		}
	}
	msg := fmt.Sprintf("package %q not in catalog — weft packages list", name)
	if len(sug) > 0 {
		msg += fmt.Sprintf(" (did you mean: %s?)", strings.Join(sug, ", "))
	}
	return nil, fmt.Errorf("%s", msg)
}

// ParseCatalogGetSpec splits "name" or "name@^1.2.0" / "name@>=0.5".
func ParseCatalogGetSpec(spec string) (name, constraint string) {
	spec = strings.TrimSpace(spec)
	if i := strings.LastIndex(spec, "@"); i > 0 {
		// avoid treating scoped paths; catalog names are simple identifiers
		rest := spec[i+1:]
		if rest != "" && (rest[0] == '^' || rest[0] == '~' || rest[0] == '>' || rest[0] == '=' ||
			(rest[0] >= '0' && rest[0] <= '9') || rest[0] == 'v') {
			return spec[:i], rest
		}
	}
	return spec, ""
}

// CheckCatalogConstraint verifies entry.Version against a lite constraint.
// Empty constraint always matches. Missing entry version only matches empty/"*".
func CheckCatalogConstraint(entry CatalogEntry, constraint string) error {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return nil
	}
	if entry.Version == "" {
		return fmt.Errorf("catalog entry %q has no version; cannot check constraint %s", entry.Name, constraint)
	}
	if !MatchConstraint(entry.Version, constraint) {
		return fmt.Errorf("catalog %s@%s does not satisfy %s", entry.Name, entry.Version, constraint)
	}
	return nil
}
