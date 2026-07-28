package pkgman

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/loreste/weft/internal/netsafe"
	"github.com/loreste/weft/internal/stdlib"
)

// InstallAll installs every dep in the manifest into vendor/ and updates lock.
// Transitive dependencies declared in each package's weft.json are installed too
// (flattened into the project's vendor/). Path deps resolve relative to the
// package that declared them (monorepo-friendly).
//
// Diamond conflicts (same name, different identity) fail install — first-wins is
// not silent. Names that collide with the stdlib are rejected.
//
// Install is atomic at the vendor/ tree level: packages land in a staging dir,
// then vendor/ is swapped. A failed install does not leave a half-updated vendor.
func InstallAll(projectDir string) (*Lockfile, error) {
	m, err := LoadManifest(projectDir)
	if err != nil {
		return nil, fmt.Errorf("no weft.json — run: weft init (%w)", err)
	}

	stageVendor := filepath.Join(projectDir, ".weft-vendor-stage")
	_ = os.RemoveAll(stageVendor)
	if err := os.MkdirAll(stageVendor, 0o755); err != nil {
		return nil, err
	}
	// Always clean stage on exit unless committed (renamed away).
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stageVendor)
		}
	}()

	lock := &Lockfile{Version: 1}
	seen := map[string]string{} // name -> depIdentity
	var walk func(name string, spec DepSpec, baseDir string) error
	walk = func(name string, spec DepSpec, baseDir string) error {
		if name == "" {
			return fmt.Errorf("empty dependency name")
		}
		if isReservedPackageName(name) {
			return fmt.Errorf("%q is reserved (stdlib or prelude) — choose another import name", name)
		}
		resolved := ResolveDepPaths(spec, baseDir)
		id := depIdentity(resolved)
		if prev, ok := seen[name]; ok {
			if prev != id {
				return fmt.Errorf("dependency conflict: %q required as %s and as %s (pin one source)", name, prev, id)
			}
			return nil
		}
		seen[name] = id

		lp, err := installOneInto(projectDir, stageVendor, name, resolved)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		lock.Packages = append(lock.Packages, *lp)

		// Nested path deps resolve against the *source* of this package
		// (original monorepo path, or staged copy for git/url installs).
		nestedBase := filepath.Join(stageVendor, name)
		if resolved.Path != "" {
			nestedBase = resolved.Path
		}

		vm, err := LoadManifest(filepath.Join(stageVendor, name))
		if err != nil || len(vm.Deps) == 0 {
			return nil
		}
		for _, n := range sortedDepNames(vm.Deps) {
			if err := walk(n, vm.Deps[n], nestedBase); err != nil {
				return err
			}
		}
		return nil
	}

	for _, name := range sortedDepNames(m.Deps) {
		if err := walk(name, m.Deps[name], projectDir); err != nil {
			return nil, err
		}
	}

	// Swap staging → vendor/ (atomic as rename allows on this OS).
	finalVendor := VendorDir(projectDir)
	bakVendor := filepath.Join(projectDir, ".weft-vendor-bak")
	_ = os.RemoveAll(bakVendor)
	if _, err := os.Stat(finalVendor); err == nil {
		if err := os.Rename(finalVendor, bakVendor); err != nil {
			return nil, fmt.Errorf("vendor swap (backup): %w", err)
		}
	}
	if err := os.Rename(stageVendor, finalVendor); err != nil {
		// rollback
		if _, err2 := os.Stat(bakVendor); err2 == nil {
			_ = os.Rename(bakVendor, finalVendor)
		}
		return nil, fmt.Errorf("vendor swap: %w", err)
	}
	committed = true
	_ = os.RemoveAll(bakVendor)

	if err := SaveLock(projectDir, lock); err != nil {
		return nil, err
	}
	return lock, nil
}

// depIdentity is a stable key for conflict detection (not the lock source string).
func depIdentity(spec DepSpec) string {
	switch {
	case spec.Path != "":
		return "path:" + filepath.Clean(spec.Path)
	case spec.Git != "":
		ref := spec.Tag
		if ref == "" {
			ref = spec.Branch
		}
		if ref == "" {
			ref = "HEAD"
		}
		return "git:" + spec.Git + "@" + ref
	case spec.URL != "":
		return "url:" + spec.URL
	default:
		return "empty"
	}
}

// isReservedPackageName blocks third-party packages from shadowing stdlib imports
// or common prelude globals that would break language semantics.
func isReservedPackageName(name string) bool {
	if name == "" {
		return true
	}
	if stdlib.IsPackage(name) {
		return true
	}
	// prelude globals often used bare — packaging them would clobber call sites
	switch name {
	case "map", "filter", "reduce", "each", "find", "any", "all", "flat_map", "par_map",
		"print", "println", "len", "push", "pop", "range", "slice", "concat",
		"contains", "keys", "values", "delete",
		"Ok", "Err", "Error", "int", "unit", "spawn", "parallel", "group",
		"channel", "send", "recv", "try_recv", "close", "select_recv", "args", "os":
		return true
	}
	return false
}

// VerifyLock checks weft.lock sums against vendor/ on disk (tamper detection).
// No-op if weft.lock is missing. Call before run when integrity matters.
func VerifyLock(projectDir string) error {
	lock, err := LoadLock(projectDir)
	if err != nil {
		return err
	}
	if len(lock.Packages) == 0 {
		// empty lock or missing file treated as no packages
		if _, err := os.Stat(filepath.Join(projectDir, "weft.lock")); err != nil {
			return nil
		}
	}
	for _, p := range lock.Packages {
		dir := filepath.Join(projectDir, filepath.FromSlash(p.Dir))
		if p.Dir == "" {
			dir = filepath.Join(VendorDir(projectDir), p.Name)
		}
		sum, err := HashDir(dir)
		if err != nil {
			return fmt.Errorf("vendor integrity: %s: %w", p.Name, err)
		}
		if p.Sum != "" && sum != p.Sum {
			return fmt.Errorf("vendor integrity: %s sum mismatch (weft.lock has %s, disk %s) — re-run: weft install",
				p.Name, truncateSum(p.Sum), truncateSum(sum))
		}
	}
	return nil
}

func truncateSum(s string) string {
	if len(s) > 20 {
		return s[:20] + "…"
	}
	return s
}

// ResolveDepPaths makes path deps absolute relative to baseDir.
func ResolveDepPaths(spec DepSpec, baseDir string) DepSpec {
	out := spec
	if out.Path != "" && !filepath.IsAbs(out.Path) {
		out.Path = filepath.Clean(filepath.Join(baseDir, out.Path))
	}
	return out
}

func sortedDepNames(deps map[string]DepSpec) []string {
	names := make([]string, 0, len(deps))
	for n := range deps {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// AddDep adds a dependency to the manifest, installs it, updates lock.
func AddDep(projectDir, name, specStr string) error {
	m, err := LoadManifest(projectDir)
	if err != nil {
		if err := Init(projectDir, filepath.Base(projectDir)); err != nil {
			return err
		}
		m, err = LoadManifest(projectDir)
		if err != nil {
			return err
		}
	}
	if name == "" {
		return fmt.Errorf("dependency name required")
	}
	spec := ParseDepString(specStr)
	if m.Deps == nil {
		m.Deps = map[string]DepSpec{}
	}
	m.Deps[name] = spec
	if err := SaveManifest(projectDir, m); err != nil {
		return err
	}
	_, err = InstallAll(projectDir)
	return err
}

// AddDepPathVersion adds a path dependency with an optional version constraint
// (stored in weft.json and checked on install against the package weft.json).
func AddDepPathVersion(projectDir, name, pathSpec, version string) error {
	m, err := LoadManifest(projectDir)
	if err != nil {
		if err := Init(projectDir, filepath.Base(projectDir)); err != nil {
			return err
		}
		m, err = LoadManifest(projectDir)
		if err != nil {
			return err
		}
	}
	if name == "" {
		return fmt.Errorf("dependency name required")
	}
	spec := ParseDepString(pathSpec)
	if version != "" {
		spec.Version = version
	}
	if m.Deps == nil {
		m.Deps = map[string]DepSpec{}
	}
	m.Deps[name] = spec
	if err := SaveManifest(projectDir, m); err != nil {
		return err
	}
	_, err = InstallAll(projectDir)
	return err
}

// installOneInto materializes a package under vendorRoot/<name>.
func installOneInto(projectDir, vendorRoot, name string, spec DepSpec) (*LockedPkg, error) {
	if err := os.MkdirAll(vendorRoot, 0o755); err != nil {
		return nil, err
	}
	// Stage then rename so a failed extract never leaves a partial package dir.
	stage := filepath.Join(vendorRoot, ".staging-"+name+"-"+shortHash(fmt.Sprintf("%d", os.Getpid())))
	_ = os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(stage) }() // no-op after successful rename

	source := ""
	version := spec.Version
	if version == "" {
		version = spec.Tag
	}

	switch {
	case spec.Path != "":
		src := spec.Path
		if !filepath.IsAbs(src) {
			src = filepath.Join(projectDir, src)
		}
		src = filepath.Clean(src)
		if st, err := os.Stat(src); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("path dep %q not a directory: %s", name, src)
		}
		if err := copyDir(src, stage); err != nil {
			return nil, err
		}
		source = "path:" + src
		if rel, err := filepath.Rel(projectDir, src); err == nil && !strings.HasPrefix(rel, "..") {
			source = "path:" + filepath.ToSlash(rel)
		}

	case spec.Git != "":
		cache, err := DefaultCacheDir()
		if err != nil {
			return nil, err
		}
		key := shortHash(spec.Git + "@" + spec.Tag + spec.Branch)
		cached := filepath.Join(cache, "git", key)
		// Tags are treated as immutable cache keys. Branches / HEAD always re-clone.
		needClone := true
		if _, err := os.Stat(filepath.Join(cached, ".git")); err == nil {
			if spec.Tag != "" && spec.Branch == "" {
				needClone = false
			}
		}
		if needClone {
			if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
				return nil, err
			}
			_ = os.RemoveAll(cached)
			if err := gitClone(spec.Git, cached, spec.Tag, spec.Branch); err != nil {
				return nil, err
			}
		}
		if err := copyDir(cached, stage); err != nil {
			return nil, err
		}
		_ = os.RemoveAll(filepath.Join(stage, ".git"))
		source = "git:" + spec.Git
		if spec.Tag != "" {
			source += "@" + spec.Tag
		} else if spec.Branch != "" {
			source += "@" + spec.Branch
		}

	case spec.URL != "":
		cache, err := DefaultCacheDir()
		if err != nil {
			return nil, err
		}
		key := shortHash(spec.URL)
		archive := filepath.Join(cache, "url", key+extOf(spec.URL))
		if _, err := os.Stat(archive); err != nil {
			if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
				return nil, err
			}
			if err := download(spec.URL, archive); err != nil {
				return nil, err
			}
		}
		if err := extractArchive(archive, stage); err != nil {
			return nil, err
		}
		source = "url:" + spec.URL

	default:
		return nil, fmt.Errorf("empty dependency spec for %q (use path, git, url, or github.com/org/repo@tag)", name)
	}

	sum, err := HashDir(stage)
	if err != nil {
		return nil, err
	}
	if _, err := FindPackageEntry(stage); err != nil {
		return nil, err
	}

	// Optional version constraint from weft.json (e.g. "^1.2.0", "~0.3.1", ">=0.2").
	// Git branch/tag names that are not semver-like are not checked as constraints.
	if m, err := LoadManifest(stage); err == nil && m.Version != "" {
		if constraint := strings.TrimSpace(spec.Version); constraint != "" && looksLikeVersionConstraint(constraint) {
			if !MatchConstraint(m.Version, constraint) {
				return nil, fmt.Errorf("package %q version %s does not satisfy constraint %s", name, m.Version, constraint)
			}
		}
		if version == "" || !looksLikeVersionConstraint(version) ||
			strings.HasPrefix(version, "^") || strings.HasPrefix(version, "~") || strings.HasPrefix(version, ">=") {
			// Prefer the package's declared version in the lock when we only had a range/branch.
			if version == "" || strings.HasPrefix(version, "^") || strings.HasPrefix(version, "~") || strings.HasPrefix(version, ">=") {
				version = m.Version
			}
		}
	}

	dest := filepath.Join(vendorRoot, name)
	_ = os.RemoveAll(dest)
	if err := os.Rename(stage, dest); err != nil {
		// cross-device fallback
		if err := copyDir(stage, dest); err != nil {
			return nil, err
		}
		_ = os.RemoveAll(stage)
	}

	return &LockedPkg{
		Name:    name,
		Source:  source,
		Version: version,
		Sum:     sum,
		Dir:     filepath.ToSlash(filepath.Join("vendor", name)),
	}, nil
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

func gitClone(repo, dest, tag, branch string) error {
	// Reject dash-leading args so repo/tag/branch cannot become git flags.
	if err := gitSafeArg("repo", repo); err != nil {
		return err
	}
	if err := gitSafeArg("tag", tag); err != nil {
		return err
	}
	if err := gitSafeArg("branch", branch); err != nil {
		return err
	}
	// Optional SSRF-ish check for http(s) git URLs
	if strings.HasPrefix(repo, "http://") || strings.HasPrefix(repo, "https://") {
		if err := netsafe.CheckURL(repo); err != nil {
			return fmt.Errorf("git clone rejected: %w", err)
		}
	}
	args := []string{"clone", "--depth", "1"}
	if tag != "" {
		args = append(args, "--branch", tag)
	} else if branch != "" {
		args = append(args, "--branch", branch)
	}
	// "--" ends option parsing so the repo URL is never treated as a flag
	args = append(args, "--", repo, dest)
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, out)
	}
	return nil
}

func gitSafeArg(kind, v string) error {
	if v == "" {
		return nil
	}
	if strings.HasPrefix(v, "-") {
		return fmt.Errorf("git %s must not start with '-'", kind)
	}
	if strings.ContainsAny(v, "\x00\r\n") {
		return fmt.Errorf("git %s contains illegal characters", kind)
	}
	return nil
}

// MaxArchiveBytes caps package downloads (zip/tar) to limit zip-bomb disk abuse.
const MaxArchiveBytes = 100 << 20 // 100 MiB

func download(rawURL, dest string) error {
	if err := netsafe.CheckURL(rawURL); err != nil {
		return fmt.Errorf("download rejected: %w", err)
	}
	client := netsafe.SafeHTTPClient(120 * time.Second)
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: HTTP %d", rawURL, resp.StatusCode)
	}
	if resp.ContentLength > MaxArchiveBytes {
		return fmt.Errorf("download too large: %d bytes (max %d)", resp.ContentLength, MaxArchiveBytes)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	// Cap actual bytes even if Content-Length is missing/wrong.
	n, err := io.Copy(f, io.LimitReader(resp.Body, MaxArchiveBytes+1))
	if err != nil {
		return err
	}
	if n > MaxArchiveBytes {
		_ = os.Remove(dest)
		return fmt.Errorf("download exceeded %d bytes", MaxArchiveBytes)
	}
	return nil
}

func extOf(url string) string {
	u := url
	if i := strings.Index(u, "?"); i >= 0 {
		u = u[:i]
	}
	switch {
	case strings.HasSuffix(u, ".tar.gz"), strings.HasSuffix(u, ".tgz"):
		return ".tar.gz"
	case strings.HasSuffix(u, ".zip"):
		return ".zip"
	case strings.HasSuffix(u, ".tar"):
		return ".tar"
	default:
		return ".bin"
	}
}

// ExtractArchive extracts a .zip, .tar.gz, or .tar archive to dest.
func ExtractArchive(archive, dest string) error {
	return extractArchive(archive, dest)
}

func extractArchive(archive, dest string) error {
	switch {
	case strings.HasSuffix(archive, ".zip"):
		return unzip(archive, dest)
	case strings.HasSuffix(archive, ".tar.gz"), strings.HasSuffix(archive, ".tgz"):
		return untarGz(archive, dest)
	case strings.HasSuffix(archive, ".tar"):
		return untar(archive, dest)
	default:
		// single file package?
		return fmt.Errorf("unsupported archive type: %s", archive)
	}
}

// MaxExtractedBytes limits total uncompressed size extracted from one archive.
const MaxExtractedBytes = 200 << 20 // 200 MiB

// MaxExtractedFiles limits file count per archive.
const MaxExtractedFiles = 50_000

func unzip(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	var total int64
	var files int
	for _, f := range r.File {
		mode := f.Mode()
		if mode&os.ModeSymlink != 0 {
			return fmt.Errorf("illegal symlink in zip: %s", f.Name)
		}
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		files++
		if files > MaxExtractedFiles {
			return fmt.Errorf("archive has too many files (>%d)", MaxExtractedFiles)
		}
		if f.UncompressedSize64 > uint64(MaxExtractedBytes) {
			return fmt.Errorf("archive entry too large: %s", f.Name)
		}
		total += int64(f.UncompressedSize64)
		if total > MaxExtractedBytes {
			return fmt.Errorf("archive uncompressed size exceeds %d bytes", MaxExtractedBytes)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, MaxExtractedBytes+1))
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
		if n > MaxExtractedBytes {
			return fmt.Errorf("archive entry exceeded size cap: %s", f.Name)
		}
	}
	return flattenSingleRoot(dest)
}

func untarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	return untarReader(gz, dest)
}

func untar(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	return untarReader(f, dest)
}

func untarReader(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	var total int64
	var files int
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		case tar.TypeSymlink, tar.TypeLink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			return fmt.Errorf("illegal entry type in tar: %s (%q)", hdr.Name, string(hdr.Typeflag))
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			files++
			if files > MaxExtractedFiles {
				return fmt.Errorf("archive has too many files (>%d)", MaxExtractedFiles)
			}
			if hdr.Size > MaxExtractedBytes {
				return fmt.Errorf("archive entry too large: %s", hdr.Name)
			}
			total += hdr.Size
			if total > MaxExtractedBytes {
				return fmt.Errorf("archive uncompressed size exceeds %d bytes", MaxExtractedBytes)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			n, err := io.Copy(out, io.LimitReader(tr, MaxExtractedBytes+1))
			out.Close()
			if err != nil {
				return err
			}
			if n > MaxExtractedBytes {
				return fmt.Errorf("archive entry exceeded size cap: %s", hdr.Name)
			}
		default:
			return fmt.Errorf("unsupported tar type %d for %s", hdr.Typeflag, hdr.Name)
		}
	}
	return flattenSingleRoot(dest)
}

// flattenSingleRoot if dest contains one directory and no files, move children up.
func flattenSingleRoot(dest string) error {
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}
	sub := filepath.Join(dest, entries[0].Name())
	tmp := dest + ".flat"
	_ = os.RemoveAll(tmp)
	if err := os.Rename(sub, tmp); err != nil {
		return err
	}
	_ = os.RemoveAll(dest)
	return os.Rename(tmp, dest)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (info.Name() == ".git" || info.Name() == "vendor") {
			return filepath.SkipDir
		}
		// Refuse symlinks (could point outside the package tree).
		if info.Mode()&os.ModeSymlink != 0 {
			rel, _ := filepath.Rel(src, path)
			return fmt.Errorf("refusing symlink in package: %s", rel)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		out.Close()
		return err
	})
}

// zipDir writes a zip of dir, skipping top-level names in skip.
func zipDir(dir, outPath string, skip map[string]bool) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil && filepath.Dir(outPath) != "." {
		// ok if dir is .
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// skip top-level dirs
		top := strings.Split(filepath.ToSlash(rel), "/")[0]
		if skip[top] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, in)
		in.Close()
		return err
	})
}
