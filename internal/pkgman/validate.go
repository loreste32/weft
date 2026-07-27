package pkgman

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/parse"
)

// ValidateReport is the result of checking a package for third-party consumption.
type ValidateReport struct {
	Dir      string
	Name     string
	Version  string
	Entry    string
	Exports  []string // pub fn / pub type names found
	Declared []string // from weft.json exports
	Deps     []string // dependency names from weft.json
	Files    []string // .weft files in package (multi-file modules)
	OK       bool
	Errors   []string
	Warnings []string
}

// ValidatePackage checks that dir is a publishable Weft module.
func ValidatePackage(dir string) (*ValidateReport, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	r := &ValidateReport{Dir: dir, OK: true}

	m, err := LoadManifest(dir)
	if err != nil {
		// allow bare package without weft.json
		r.Warnings = append(r.Warnings, "no weft.json — consumers can still path-import; add weft.json for git/get")
		m = &Manifest{Name: filepath.Base(dir), Type: "module"}
	} else {
		r.Name = m.Name
		r.Version = m.Version
		r.Declared = append([]string(nil), m.Exports...)
		for _, n := range sortedDepNames(m.Deps) {
			r.Deps = append(r.Deps, n)
		}
		if m.Type == "app" {
			r.Warnings = append(r.Warnings, "type is app — libraries should use \"type\": \"module\"")
		}
	}
	if r.Name == "" {
		r.Name = filepath.Base(dir)
	}

	// multi-file inventory
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == "vendor" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".weft") || strings.HasSuffix(info.Name(), ".loom") {
			rel, _ := filepath.Rel(dir, path)
			r.Files = append(r.Files, filepath.ToSlash(rel))
		}
		return nil
	})

	entry, err := FindPackageEntry(dir)
	if err != nil {
		r.OK = false
		r.Errors = append(r.Errors, err.Error())
		return r, nil
	}
	r.Entry = entry

	src, err := os.ReadFile(entry)
	if err != nil {
		r.OK = false
		r.Errors = append(r.Errors, err.Error())
		return r, nil
	}
	file, perrs := parse.ParseFile(entry, string(src))
	if perrs.HasErrors() {
		r.OK = false
		for _, e := range perrs {
			r.Errors = append(r.Errors, e.String())
		}
		return r, nil
	}

	var pubs []string
	var allFns []string
	hasPub := false
	for _, d := range file.Decls {
		switch n := d.(type) {
		case *ast.FnDecl:
			if n.Name == "main" {
				continue
			}
			allFns = append(allFns, n.Name)
			if n.Pub {
				hasPub = true
				pubs = append(pubs, n.Name)
			}
		case *ast.TypeDecl:
			if n.Pub {
				hasPub = true
				pubs = append(pubs, n.Name)
			}
		case *ast.EnumDecl:
			if n.Pub {
				hasPub = true
				pubs = append(pubs, n.Name)
			}
		}
	}
	if hasPub {
		r.Exports = pubs
	} else {
		r.Exports = allFns
		if len(allFns) > 0 {
			r.Warnings = append(r.Warnings, "no pub fn/type/enum — all non-main functions export; prefer pub for a stable API")
		}
	}
	if len(r.Exports) == 0 {
		r.OK = false
		r.Errors = append(r.Errors, "module exports nothing — add pub fn … (or pub type / pub enum)")
	}

	// declared exports must exist
	exportSet := map[string]bool{}
	for _, e := range r.Exports {
		exportSet[e] = true
	}
	for _, d := range r.Declared {
		if !exportSet[d] {
			r.OK = false
			r.Errors = append(r.Errors, fmt.Sprintf("weft.json exports %q but no pub symbol %s in entry", d, d))
		}
	}
	// warn if pub not listed in exports when exports is non-empty
	if len(r.Declared) > 0 {
		declSet := map[string]bool{}
		for _, d := range r.Declared {
			declSet[d] = true
		}
		for _, e := range r.Exports {
			if !declSet[e] {
				r.Warnings = append(r.Warnings, fmt.Sprintf("pub %s not listed in weft.json exports", e))
			}
		}
	}

	// Parse every non-test .weft in the package (multi-file modules).
	entryBase := filepath.Base(entry)
	var testFiles []string
	for _, rel := range r.Files {
		base := filepath.Base(rel)
		if isTestWeft(base) {
			testFiles = append(testFiles, rel)
			continue
		}
		if base == entryBase {
			continue // already parsed
		}
		path := filepath.Join(dir, rel)
		src2, err := os.ReadFile(path)
		if err != nil {
			r.Warnings = append(r.Warnings, fmt.Sprintf("cannot read %s: %v", rel, err))
			continue
		}
		if _, perrs := parse.ParseFile(path, string(src2)); perrs.HasErrors() {
			r.OK = false
			for _, e := range perrs {
				r.Errors = append(r.Errors, rel+": "+e.String())
			}
		}
	}
	if len(testFiles) > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%d test file(s) — run: weft test %s", len(testFiles), filepath.Base(dir)))
	}

	if m != nil && m.Version == "" {
		r.Warnings = append(r.Warnings, "missing version in weft.json")
	}
	if m != nil && m.Description == "" {
		r.Warnings = append(r.Warnings, "missing description in weft.json")
	}
	if m != nil && m.Type == "" {
		r.Warnings = append(r.Warnings, `missing "type" in weft.json — use "module" for libraries`)
	}
	if m != nil && m.Entry == "" {
		r.Warnings = append(r.Warnings, "missing entry in weft.json (default discovery still works)")
	}
	if m != nil {
		if m.CapabilityProfile != "" {
			_, pwarns := ValidateCapabilityProfile(m.CapabilityProfile)
			r.Warnings = append(r.Warnings, pwarns...)
			if m.CapabilityProfile == "full" || m.CapabilityProfile == "host" {
				r.Warnings = append(r.Warnings, `capability_profile "`+m.CapabilityProfile+`" is broad — review carefully`)
			}
		}
		if len(m.Capabilities) > 0 || m.CapabilityProfile != "" {
			// validate raw tokens (including @profiles)
			raw := append([]string{}, m.Capabilities...)
			if m.CapabilityProfile != "" {
				raw = append(raw, "@"+m.CapabilityProfile)
			}
			cerrs, cwarns := ValidateCapabilities(m.Capabilities)
			for _, e := range cerrs {
				r.OK = false
				r.Errors = append(r.Errors, "capabilities: "+e)
			}
			r.Warnings = append(r.Warnings, cwarns...)
			// expanded grant includes full host?
			expanded := ExpandCapabilities(m.CapabilityProfile, m.Capabilities)
			for _, c := range expanded {
				if c == CapsAll {
					r.Warnings = append(r.Warnings, `capabilities: "*" grants full host (sh/secrets/cli/db/…) — review carefully`)
					break
				}
			}
			_ = raw
		}
	}

	// entry path should be under dir
	rel, err := filepath.Rel(dir, entry)
	if err != nil || strings.HasPrefix(rel, "..") {
		r.OK = false
		r.Errors = append(r.Errors, "entry escapes package directory")
	}

	return r, nil
}

// PrintValidate writes a human report to stdout-like writer via fmt.
func FormatValidate(r *ValidateReport) string {
	var b strings.Builder
	status := "ok"
	if !r.OK {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "module %s  %s\n", r.Name, status)
	fmt.Fprintf(&b, "  dir      %s\n", r.Dir)
	if r.Version != "" {
		fmt.Fprintf(&b, "  version  %s\n", r.Version)
	}
	if r.Entry != "" {
		fmt.Fprintf(&b, "  entry    %s\n", r.Entry)
	}
	if len(r.Exports) > 0 {
		fmt.Fprintf(&b, "  exports  %s\n", strings.Join(r.Exports, ", "))
	}
	if len(r.Deps) > 0 {
		fmt.Fprintf(&b, "  deps     %s\n", strings.Join(r.Deps, ", "))
	}
	if m, err := LoadManifest(r.Dir); err == nil && len(m.Capabilities) > 0 {
		fmt.Fprintf(&b, "  caps     %s\n", strings.Join(m.Capabilities, ", "))
	} else if r.OK {
		fmt.Fprintf(&b, "  caps     (default: no sh/secrets/cli)\n")
	}
	if len(r.Files) > 1 {
		fmt.Fprintf(&b, "  files    %s\n", strings.Join(r.Files, ", "))
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "  warn: %s\n", w)
	}
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "  error: %s\n", e)
	}
	if r.OK {
		fmt.Fprintf(&b, "  consumers: weft get %s <path|git@tag>\n", r.Name)
		fmt.Fprintf(&b, "             use %s   # then %s.<export>(…)\n", r.Name, r.Name)
		base := filepath.Base(r.Dir)
		fmt.Fprintf(&b, "  next:      weft mod check %s --tests\n", base)
		fmt.Fprintf(&b, "             weft test %s\n", base)
		fmt.Fprintf(&b, "             weft mod pack %s\n", base)
		fmt.Fprintf(&b, "             # monorepo catalog: add to packages/index.json then weft packages get %s\n", r.Name)
		fmt.Fprintf(&b, "  expand:    modules add APIs — pure .weft (no native plugins)\n")
	}
	return b.String()
}

func isTestWeft(base string) bool {
	name := strings.TrimSuffix(strings.TrimSuffix(base, ".weft"), ".loom")
	return strings.HasSuffix(name, "_test") || strings.HasPrefix(name, "test_")
}

// PackArchive creates a simple zip of the package (no vendor/.git) for distribution.
func PackArchive(dir, outPath string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	r, err := ValidatePackage(dir)
	if err != nil {
		return err
	}
	if !r.OK {
		return fmt.Errorf("package invalid — fix: weft mod check\n%s", FormatValidate(r))
	}
	if outPath == "" {
		ver := r.Version
		if ver == "" {
			ver = "0.0.0"
		}
		outPath = fmt.Sprintf("%s-%s.weftpkg.zip", r.Name, ver)
	}
	return zipDir(dir, outPath, map[string]bool{
		"vendor": true, ".git": true, "node_modules": true,
	})
}
