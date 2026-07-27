package weft

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/loreste/weft/internal/pkgman"
)

// PkgInit creates weft.json in dir.
func PkgInit(dir, name string) error {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	return pkgman.Init(dir, name)
}

// NewModule scaffolds a third-party Weft module (library).
func NewModule(parentDir, name string, force bool) (string, error) {
	return pkgman.Scaffold(pkgman.ScaffoldOptions{
		Dir: parentDir, Name: name, Kind: "module", Force: force,
	})
}

// NewApp scaffolds a Weft application project.
func NewApp(parentDir, name string, force bool) (string, error) {
	return pkgman.Scaffold(pkgman.ScaffoldOptions{
		Dir: parentDir, Name: name, Kind: "app", Force: force,
	})
}

// NewCLI scaffolds a devops/data CLI tool project.
func NewCLI(parentDir, name string, force bool) (string, error) {
	return pkgman.Scaffold(pkgman.ScaffoldOptions{
		Dir: parentDir, Name: name, Kind: "cli", Force: force,
	})
}

// ModCheckOptions configures weft mod check.
type ModCheckOptions struct {
	// Dir is the package directory (default: cwd).
	Dir string
	// RunTests runs weft test on the package after static validation.
	RunTests bool
	// QuietTests suppresses per-test lines (summary only).
	QuietTests bool
}

// ModCheck validates a module package for publishing.
func ModCheck(dir string) error {
	return ModCheckWith(ModCheckOptions{Dir: dir})
}

// ModCheckWith validates a module; optionally runs unit tests.
func ModCheckWith(opts ModCheckOptions) error {
	dir := opts.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	r, err := pkgman.ValidatePackage(dir)
	if err != nil {
		return err
	}
	fmt.Print(pkgman.FormatValidate(r))
	if !r.OK {
		return fmt.Errorf("module check failed")
	}
	if !opts.RunTests {
		return nil
	}
	fmt.Println()
	fmt.Println("== tests ==")
	rep, err := RunTests(TestOptions{
		Paths:   []string{dir},
		Quiet:   opts.QuietTests,
		Runtime: Options{}, // LLM mock applied inside RunTests when needed
	})
	if err != nil {
		return err
	}
	code := PrintTestReport(rep, opts.QuietTests)
	if code != 0 {
		return fmt.Errorf("module tests failed")
	}
	if rep != nil && rep.Total == 0 {
		fmt.Println("(no tests — add fn test_* in *_test.weft)")
	}
	return nil
}

// ModPack zips a validated module for distribution.
func ModPack(dir, out string) error {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	r, err := pkgman.ValidatePackage(dir)
	if err != nil {
		return err
	}
	if out == "" {
		ver := r.Version
		if ver == "" {
			ver = "0.0.0"
		}
		out = fmt.Sprintf("%s-%s.weftpkg.zip", r.Name, ver)
	}
	if err := pkgman.PackArchive(dir, out); err != nil {
		return err
	}
	abs, _ := filepath.Abs(out)
	fmt.Println("packed", abs)
	return nil
}

// PkgGet adds a dependency: weft get <name> [spec]
// Examples:
//
//	weft get greeter ./packages/greeter
//	weft get util github.com/org/loom-util@v0.1.0
func PkgGet(dir, name, spec string) error {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	if spec == "" {
		spec = name
		// if name looks like a path/url, derive package name
		name = deriveName(spec)
	}
	return pkgman.AddDep(dir, name, spec)
}

func deriveName(spec string) string {
	s := strings.TrimSuffix(spec, "/")
	s = strings.TrimSuffix(s, ".git")
	if i := strings.LastIndex(s, "@"); i > 0 {
		s = s[:i]
	}
	base := filepath.Base(s)
	base = strings.TrimSuffix(base, ".weft")
	if base == "" || base == "." || base == ".." {
		return "dep"
	}
	return strings.ReplaceAll(base, "-", "_")
}

// PkgInstall installs all deps from weft.json.
func PkgInstall(dir string) error {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	lock, err := pkgman.InstallAll(dir)
	if err != nil {
		return err
	}
	fmt.Printf("installed %d package(s)\n", len(lock.Packages))
	for _, p := range lock.Packages {
		fmt.Printf("  %s  %s  %s\n", p.Name, p.Dir, p.Sum[:min(20, len(p.Sum))]+"…")
	}
	return nil
}

// PkgList lists manifest deps and lock status.
func PkgList(dir string) error {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	m, err := pkgman.LoadManifest(dir)
	if err != nil {
		return err
	}
	lock, _ := pkgman.LoadLock(dir)
	locked := map[string]pkgman.LockedPkg{}
	for _, p := range lock.Packages {
		locked[p.Name] = p
	}
	names := make([]string, 0, len(m.Deps))
	for n := range m.Deps {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		fmt.Println("(no dependencies)")
		return nil
	}
	for _, n := range names {
		if lp, ok := locked[n]; ok {
			fmt.Printf("%s\t%s\t%s\n", n, lp.Source, lp.Dir)
		} else {
			fmt.Printf("%s\t(not installed — run weft install)\n", n)
		}
	}
	return nil
}

// CatalogList prints monorepo packages/index.json (optional modules).
// query filters by name/summary when non-empty.
func CatalogList(start string, query ...string) error {
	path, cat, err := pkgman.FindCatalog(start)
	if err != nil {
		return err
	}
	q := ""
	if len(query) > 0 {
		q = query[0]
	}
	fmt.Print(pkgman.FormatCatalogFilter(path, cat, q))
	return nil
}

// CatalogInfo prints one catalog entry.
func CatalogInfo(start, name string) error {
	idx, cat, err := pkgman.FindCatalog(start)
	if err != nil {
		return err
	}
	entry, err := pkgman.FindCatalogEntry(cat, name)
	if err != nil {
		return err
	}
	fmt.Print(pkgman.FormatCatalogEntry(idx, *entry))
	return nil
}

// CatalogGet adds a catalog package by name into the current app (path dep).
// name may be "pkg" or "pkg@^1.2.0" (constraint checked against catalog version).
func CatalogGet(appDir, nameSpec string) error {
	idx, cat, err := pkgman.FindCatalog(appDir)
	if err != nil {
		return err
	}
	name, constraint := pkgman.ParseCatalogGetSpec(nameSpec)
	entry, err := pkgman.FindCatalogEntry(cat, name)
	if err != nil {
		return err
	}
	if err := pkgman.CheckCatalogConstraint(*entry, constraint); err != nil {
		return err
	}
	abs := pkgman.ResolveCatalogPath(idx, entry.Path)
	// prefer a short relative path when the app lives in the same tree
	specPath := abs
	if rel, err := filepath.Rel(appDir, abs); err == nil {
		// avoid ../../../../ noise across volume roots
		if !strings.HasPrefix(rel, ".."+string(filepath.Separator)+".."+string(filepath.Separator)+"..") {
			specPath = rel
			if !strings.HasPrefix(specPath, ".") {
				specPath = "./" + specPath
			}
		}
	}
	// Path dep + pin version from catalog (or explicit constraint) when present.
	ver := entry.Version
	if constraint != "" {
		ver = constraint
	}
	return pkgman.AddDepPathVersion(appDir, entry.Name, specPath, ver)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DetectProjectDir walks up from start for weft.json (or legacy loom.json).
func DetectProjectDir(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "weft.json")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "loom.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}
