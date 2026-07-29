package weft

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const updateBaseURL = "https://weftproject.dev"

// VersionInfo from the update server.
type VersionInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// CheckUpdate checks if a newer version is available.
func CheckUpdate() (*VersionInfo, error) {
	url := updateBaseURL + "/version.json"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("check update: HTTP %d", resp.StatusCode)
	}
	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("parse version info: %w", err)
	}
	return &info, nil
}

// SelfUpdate downloads and replaces the current binary.
func SelfUpdate() error {
	info, err := CheckUpdate()
	if err != nil {
		return err
	}

	if info.Version == Version {
		fmt.Printf("weft %s is already the latest version\n", Version)
		return nil
	}

	fmt.Printf("updating weft %s → %s ...\n", Version, info.Version)

	// determine binary name for this platform
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	binaryName := fmt.Sprintf("weft-%s-%s", goos, goarch)
	if goos == "windows" {
		binaryName += ".exe"
	}

	dlURL := updateBaseURL + "/dl/" + binaryName
	fmt.Printf("downloading %s ...\n", dlURL)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(dlURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	// write to temp file next to current binary
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, "weft-update-*")
	if err != nil {
		// fallback to system temp if target dir isn't writable
		tmp, err = os.CreateTemp("", "weft-update-*")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
	}
	tmpPath := tmp.Name()

	_, err = io.Copy(tmp, resp.Body)
	tmp.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download: %w", err)
	}

	// make executable
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}

	// macOS: remove quarantine and ad-hoc sign so Gatekeeper doesn't kill it
	if runtime.GOOS == "darwin" {
		exec.Command("xattr", "-cr", tmpPath).Run()
		exec.Command("codesign", "--force", "-s", "-", tmpPath).Run()
	}

	// replace the binary — try rename first, fall back to copy (cross-device or permissions)
	oldPath := exe + ".old"
	os.Remove(oldPath)

	err = os.Rename(exe, oldPath)
	if err == nil {
		err = os.Rename(tmpPath, exe)
		if err != nil {
			os.Rename(oldPath, exe) // restore
			// try copy fallback
			err = copyFile(tmpPath, exe)
		}
		if err == nil {
			os.Remove(oldPath)
		}
	} else {
		// can't rename (permission denied) — try copy with sudo
		err = copyFile(tmpPath, exe)
		if err != nil {
			// last resort: sudo cp
			fmt.Println("installing to", exe, "(requires sudo)...")
			sudoErr := exec.Command("sudo", "cp", tmpPath, exe).Run()
			if sudoErr != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("replace binary: %w (sudo also failed: %v)", err, sudoErr)
			}
			exec.Command("sudo", "chmod", "+x", exe).Run()
			err = nil
		}
	}
	os.Remove(tmpPath)

	if err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	fmt.Printf("\n  weft updated to %s\n\n", info.Version)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, 0o755)
}

// UpgradePackages upgrades installed packages in vendor/ to latest registry versions.
func UpgradePackages(projectDir string) error {
	lockPath := filepath.Join(projectDir, "weft.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return fmt.Errorf("no weft.lock found — run weft install first")
	}

	var lock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Source  string `json:"source"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return fmt.Errorf("parse weft.lock: %w", err)
	}

	if len(lock.Packages) == 0 {
		fmt.Println("no packages to upgrade")
		return nil
	}

	// fetch registry index
	registryURL := os.Getenv("WEFT_REGISTRY")
	if registryURL == "" {
		registryURL = updateBaseURL[:strings.LastIndex(updateBaseURL, "/")] // fallback
		registryURL = "https://registry.weftproject.dev"
	}
	fmt.Printf("checking %s for updates ...\n", registryURL)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(registryURL + "/v1/index.json")
	if err != nil {
		return fmt.Errorf("fetch registry: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("registry: HTTP %d", resp.StatusCode)
	}
	var idx struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return fmt.Errorf("parse index: %w", err)
	}

	// build latest version map
	latest := make(map[string]string)
	for _, p := range idx.Packages {
		if cur, ok := latest[p.Name]; !ok || compareVersionStrings(p.Version, cur) > 0 {
			latest[p.Name] = p.Version
		}
	}

	// check each installed package
	var upgradable []struct{ name, from, to string }
	for _, pkg := range lock.Packages {
		if !strings.HasPrefix(pkg.Source, "path:") {
			// only upgrade registry packages, not local path deps
			if latestVer, ok := latest[pkg.Name]; ok {
				if compareVersionStrings(latestVer, pkg.Version) > 0 {
					upgradable = append(upgradable, struct{ name, from, to string }{pkg.Name, pkg.Version, latestVer})
				}
			}
		}
	}

	if len(upgradable) == 0 {
		fmt.Println("all packages are up to date")
		for _, pkg := range lock.Packages {
			fmt.Printf("  %-16s %s (current)\n", pkg.Name, pkg.Version)
		}
		return nil
	}

	fmt.Printf("\n%d package(s) to upgrade:\n", len(upgradable))
	for _, u := range upgradable {
		fmt.Printf("  %-16s %s → %s\n", u.name, u.from, u.to)
	}
	fmt.Println()

	// install each upgrade
	for _, u := range upgradable {
		fmt.Printf("upgrading %s ...\n", u.name)
		if err := RegistryInstall(projectDir, u.name); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		}
	}

	fmt.Println("\ndone")
	return nil
}

// compareVersionStrings compares semver strings. Returns -1, 0, 1.
func compareVersionStrings(a, b string) int {
	pa := parseVer(a)
	pb := parseVer(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseVer(s string) [3]int {
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	var v [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		p := strings.SplitN(parts[i], "-", 2)[0] // strip pre-release
		fmt.Sscanf(p, "%d", &v[i])
	}
	return v
}
