package weft

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Build bundles a Weft project into a self-contained .weftapp archive.
// The archive contains the entry script, vendor/, and a manifest.
// Run it with: weft run app.weftapp
func Build(projectDir, entry, output string) error {
	if entry == "" {
		entry = "main.weft"
	}
	entryPath := filepath.Join(projectDir, entry)
	if _, err := os.Stat(entryPath); err != nil {
		return fmt.Errorf("entry file not found: %s", entryPath)
	}

	if output == "" {
		base := strings.TrimSuffix(filepath.Base(entry), ".weft")
		output = base + ".weftapp"
	}

	out, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create %s: %w", output, err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	// add entry file
	if err := addFileToZip(zw, entryPath, entry); err != nil {
		return err
	}

	// add weft.json if exists
	manifest := filepath.Join(projectDir, "weft.json")
	if _, err := os.Stat(manifest); err == nil {
		addFileToZip(zw, manifest, "weft.json")
	}

	// add vendor/ if exists
	vendorDir := filepath.Join(projectDir, "vendor")
	if info, err := os.Stat(vendorDir); err == nil && info.IsDir() {
		err := filepath.Walk(vendorDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(projectDir, path)
			return addFileToZip(zw, path, rel)
		})
		if err != nil {
			return fmt.Errorf("add vendor: %w", err)
		}
	}

	// add lib/ or any other .weft files referenced
	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".weft") {
			return nil
		}
		rel, _ := filepath.Rel(projectDir, path)
		if rel == entry {
			return nil // already added
		}
		return addFileToZip(zw, path, rel)
	})
	if err != nil {
		return fmt.Errorf("walk project: %w", err)
	}

	fmt.Printf("built %s (%s + vendor/)\n", output, entry)
	fmt.Printf("run:  weft run %s\n", output)
	return nil
}

func addFileToZip(zw *zip.Writer, srcPath, name string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}
