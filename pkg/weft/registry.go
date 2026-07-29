package weft

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/pkgman"
)

// Publish validates, signs, and uploads a package to the registry.
func Publish(dir, keyName string) error {
	registryURL := pkgman.RegistryURL()
	fmt.Printf("publishing to %s …\n", registryURL)
	if err := pkgman.PublishPackage(dir, keyName, registryURL); err != nil {
		return err
	}
	m, _ := pkgman.LoadManifest(dir)
	if m != nil {
		fmt.Printf("published %s@%s\n", m.Name, m.Version)
	} else {
		fmt.Println("published")
	}
	return nil
}

// RegistrySearch lists packages from the public registry.
func RegistrySearch(query string, showAll bool) error {
	registryURL := pkgman.RegistryURL()
	idx, err := pkgman.FetchIndex(registryURL)
	if err != nil {
		return err
	}
	results := pkgman.SearchRegistry(idx, query)
	if len(results) == 0 {
		if query != "" {
			fmt.Printf("no packages matching %q\n", query)
		} else {
			fmt.Println("registry is empty")
		}
		return nil
	}
	// deduplicate unless --all
	type entry struct {
		name, version, summary string
		signed                 bool
	}
	var display []entry
	if showAll {
		for _, p := range results {
			display = append(display, entry{p.Name, p.Version, p.Summary, p.Signature != ""})
		}
	} else {
		latest := make(map[string]pkgman.RegistryPackage)
		for _, p := range results {
			if cur, ok := latest[p.Name]; !ok || pkgman.VersionGreater(p.Version, cur.Version) {
				latest[p.Name] = p
			}
		}
		for _, p := range latest {
			display = append(display, entry{p.Name, p.Version, p.Summary, p.Signature != ""})
		}
	}
	fmt.Printf("registry  %s (%d packages)\n", registryURL, len(display))
	for _, e := range display {
		sum := e.summary
		if sum == "" {
			sum = "-"
		}
		signed := ""
		if e.signed {
			signed = " [signed]"
		}
		fmt.Printf("  %-16s  %s  %s%s\n", e.name, e.version, sum, signed)
	}
	return nil
}

// RegistryInfo shows details of a registry package.
func RegistryInfo(name string) error {
	registryURL := pkgman.RegistryURL()
	idx, err := pkgman.FetchIndex(registryURL)
	if err != nil {
		return err
	}
	pkg, err := pkgman.FindRegistryPackage(idx, name, "")
	if err != nil {
		return err
	}
	fmt.Printf("name       %s\n", pkg.Name)
	fmt.Printf("version    %s\n", pkg.Version)
	if pkg.Summary != "" {
		fmt.Printf("summary    %s\n", pkg.Summary)
	}
	if pkg.Author != "" {
		fmt.Printf("author     %s\n", pkg.Author)
	}
	if pkg.License != "" {
		fmt.Printf("license    %s\n", pkg.License)
	}
	if pkg.Signature != "" {
		fmt.Printf("signed     yes (ed25519)\n")
		fmt.Printf("pubkey     %s\n", pkg.PublicKey[:16]+"…")
	} else {
		fmt.Printf("signed     no\n")
	}
	if pkg.Published != "" {
		fmt.Printf("published  %s\n", pkg.Published)
	}
	if len(pkg.Deps) > 0 {
		fmt.Printf("deps       %s\n", strings.Join(depsStr(pkg.Deps), ", "))
	}
	// list all available versions
	var versions []string
	for _, p := range idx.Packages {
		if p.Name == name {
			versions = append(versions, p.Version)
		}
	}
	if len(versions) > 1 {
		fmt.Printf("versions   %s\n", strings.Join(versions, ", "))
	}
	fmt.Printf("install    weft registry install %s\n", pkg.Name)
	fmt.Printf("pin ver    weft registry install %s@%s\n", pkg.Name, pkg.Version)
	return nil
}

func depsStr(deps map[string]string) []string {
	var out []string
	for k, v := range deps {
		if v != "" {
			out = append(out, k+"@"+v)
		} else {
			out = append(out, k)
		}
	}
	return out
}

// RegistryInstall downloads and installs a package from the registry into vendor/.
func RegistryInstall(projectDir, spec string) error {
	name, constraint := pkgman.ParseCatalogGetSpec(spec)
	registryURL := pkgman.RegistryURL()

	fmt.Printf("fetching %s from %s …\n", name, registryURL)
	idx, err := pkgman.FetchIndex(registryURL)
	if err != nil {
		return err
	}
	pkg, err := pkgman.FindRegistryPackage(idx, name, constraint)
	if err != nil {
		return err
	}

	// Download to cache
	cacheDir, err := pkgman.DefaultCacheDir()
	if err != nil {
		return err
	}
	archivePath := fmt.Sprintf("%s/%s-%s.tar.gz", cacheDir, pkg.Name, pkg.Version)
	if err := pkgman.DownloadAndVerify(registryURL, *pkg, archivePath); err != nil {
		return err
	}
	if pkg.Signature != "" {
		fmt.Printf("signature verified (%s)\n", pkg.PublicKey[:16]+"…")
	}

	// Extract archive directly to vendor/
	vendorDir := filepath.Join(projectDir, "vendor", name)
	os.RemoveAll(vendorDir) // clean previous version
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return fmt.Errorf("create vendor dir: %w", err)
	}
	if err := pkgman.ExtractArchive(archivePath, vendorDir); err != nil {
		os.RemoveAll(vendorDir)
		return fmt.Errorf("extract %s: %w", pkg.Name, err)
	}

	// Init weft.json if needed
	manifestPath := filepath.Join(projectDir, "weft.json")
	if _, err := os.Stat(manifestPath); err != nil {
		pkgman.Init(projectDir, filepath.Base(projectDir))
	}

	// Update weft.json deps
	m, err := pkgman.LoadManifest(projectDir)
	if err != nil {
		return err
	}
	if m.Deps == nil {
		m.Deps = map[string]pkgman.DepSpec{}
	}
	m.Deps[name] = pkgman.DepSpec{Path: "vendor/" + name, Version: pkg.Version}
	if err := pkgman.SaveManifest(projectDir, m); err != nil {
		return err
	}

	fmt.Printf("installed %s@%s to vendor/%s\n", name, pkg.Version, name)
	return nil
}

// RegistryKeygen generates a new signing keypair.
func RegistryKeygen(name string) error {
	kp, err := pkgman.GenerateKey(name)
	if err != nil {
		return err
	}
	if err := pkgman.SaveKey(name, kp); err != nil {
		return err
	}
	pub, _ := pkgman.ExportPublicKey(name)
	fmt.Printf("key generated: %s\n", name)
	fmt.Printf("public key:    %s\n", pub.PublicHex)
	fmt.Printf("saved to:      ~/.weft/keys/%s.json\n", name)
	fmt.Println("\nShare the public key with the registry. Keep the secret key safe.")
	return nil
}

// RegistryListKeys shows all signing keys.
func RegistryListKeys() error {
	names, err := pkgman.ListKeys()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Println("no keys — run: weft registry keygen")
		return nil
	}
	for _, n := range names {
		pub, err := pkgman.ExportPublicKey(n)
		if err != nil {
			fmt.Printf("  %-16s  (error: %v)\n", n, err)
			continue
		}
		fmt.Printf("  %-16s  %s\n", n, pub.PublicHex[:32]+"…")
	}
	return nil
}

// RegistryTrust adds a public key for a package namespace.
func RegistryTrust(namespace, pubHex, note string) error {
	if err := pkgman.TrustKey(namespace, pubHex, note); err != nil {
		return err
	}
	fmt.Printf("trusted %s for namespace %q\n", pubHex[:min(16, len(pubHex))]+"…", namespace)
	return nil
}

// RegistryTrustLocal imports a local signing key's public half into the trust store.
func RegistryTrustLocal(namespace, keyName string) error {
	if err := pkgman.TrustKeyFromLocalKey(namespace, keyName); err != nil {
		return err
	}
	fmt.Printf("trusted local key %q under namespace %q\n", keyName, namespace)
	return nil
}

// RegistryUntrust removes a key (or whole namespace if pubHex empty).
func RegistryUntrust(namespace, pubHex string) error {
	if err := pkgman.UntrustKey(namespace, pubHex); err != nil {
		return err
	}
	if pubHex == "" {
		fmt.Printf("untrusted namespace %q\n", namespace)
	} else {
		fmt.Printf("removed key from namespace %q\n", namespace)
	}
	return nil
}

// RegistryRotateTrust adds a new key and optionally retires an old one locally.
func RegistryRotateTrust(namespace, newPub, retirePub string) error {
	if err := pkgman.RotateTrust(namespace, newPub, retirePub); err != nil {
		return err
	}
	fmt.Printf("rotated trust for %q (added new key", namespace)
	if retirePub != "" {
		fmt.Printf(", retired old")
	}
	fmt.Println(")")
	return nil
}

// RegistryListTrust prints ~/.weft/trust.json contents.
func RegistryListTrust() error {
	ts, err := pkgman.LoadTrustStore()
	if err != nil {
		return err
	}
	if ts.RequireTrust || os.Getenv("WEFT_REQUIRE_TRUST") == "1" {
		fmt.Println("require_trust: true (installs need trusted namespaces)")
	}
	if len(ts.Namespaces) == 0 {
		fmt.Println("no trusted namespaces — run: weft registry trust <ns> <pubkey>")
		return nil
	}
	for ns, e := range ts.Namespaces {
		fmt.Printf("%s\n", ns)
		if e.Note != "" {
			fmt.Printf("  note: %s\n", e.Note)
		}
		for _, k := range e.PublicKeys {
			short := k
			if len(short) > 32 {
				short = short[:32] + "…"
			}
			fmt.Printf("  key: %s\n", short)
		}
	}
	return nil
}

// RegistryExportPublicKey prints a public key for sharing.
func RegistryExportPublicKey(name string) error {
	pub, err := pkgman.ExportPublicKey(name)
	if err != nil {
		return err
	}
	fmt.Println(pub.PublicHex)
	return nil
}

// RegistryServe starts a local registry server.
func RegistryServe(addr, dataDir, token string) error {
	srv := pkgman.NewRegistryServer(dataDir, token)
	return srv.ListenAndServe(addr)
}
