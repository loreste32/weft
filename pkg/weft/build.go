package weft

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// buildMagic is appended after the zip to identify embedded weft apps.
var buildMagic = []byte("WEFTAPP\x00")

// Build produces a standalone executable: the weft runtime + your script bundled.
// The output binary runs without weft installed on the target machine.
//
//	weft build                    → ./main (from main.weft)
//	weft build . -o myapp        → ./myapp
//	weft build . app.weft -o app → ./app
func Build(projectDir, entry, output string) error {
	if entry == "" {
		entry = "main.weft"
	}
	entryPath := filepath.Join(projectDir, entry)
	if _, err := os.Stat(entryPath); err != nil {
		return fmt.Errorf("entry file not found: %s", entryPath)
	}

	if output == "" {
		output = strings.TrimSuffix(filepath.Base(entry), ".weft")
		if runtime.GOOS == "windows" {
			output += ".exe"
		}
	}

	// 1. copy the current weft binary as the base
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find weft binary: %w", err)
	}
	exe, _ = filepath.EvalSymlinks(exe)

	exeData, err := os.ReadFile(exe)
	if err != nil {
		return fmt.Errorf("read weft binary: %w", err)
	}

	// check if current binary is already a built app — use the original runtime
	if idx := findMagic(exeData); idx >= 0 {
		exeData = exeData[:idx]
	}

	// 2. build the project zip in memory
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	// write entry marker so the runtime knows which file to run
	w, _ := zw.Create(".weftapp-entry")
	w.Write([]byte(entry))

	// add entry file
	if err := addFileToZip(zw, entryPath, entry); err != nil {
		return err
	}

	// add weft.json
	manifest := filepath.Join(projectDir, "weft.json")
	if _, err := os.Stat(manifest); err == nil {
		addFileToZip(zw, manifest, "weft.json")
	}

	// add vendor/
	vendorDir := filepath.Join(projectDir, "vendor")
	if info, err := os.Stat(vendorDir); err == nil && info.IsDir() {
		filepath.Walk(vendorDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(projectDir, path)
			return addFileToZip(zw, path, rel)
		})
	}

	// add all .weft files
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
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
			return nil
		}
		return addFileToZip(zw, path, rel)
	})

	zw.Close()

	// 3. assemble: binary + zip + zip-size(8 bytes) + magic(8 bytes)
	zipData := zipBuf.Bytes()
	var sizeBuf [8]byte
	binary.LittleEndian.PutUint64(sizeBuf[:], uint64(len(zipData)))

	out, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create %s: %w", output, err)
	}
	out.Write(exeData)
	out.Write(zipData)
	out.Write(sizeBuf[:])
	out.Write(buildMagic)
	out.Close()

	os.Chmod(output, 0o755)

	// macOS: ad-hoc sign
	if runtime.GOOS == "darwin" {
		signBinary(output)
	}

	fmt.Printf("built %s (%s + vendor/)\n", output, entry)
	fmt.Printf("run:  ./%s\n", output)
	return nil
}

// CheckEmbeddedApp checks if the current binary has an embedded weft app.
// Returns the temp directory with extracted files and the entry filename, or empty strings.
func CheckEmbeddedApp() (dir string, entry string, ok bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", false
	}
	exe, _ = filepath.EvalSymlinks(exe)

	data, err := os.ReadFile(exe)
	if err != nil {
		return "", "", false
	}

	idx := findMagic(data)
	if idx < 0 {
		return "", "", false
	}

	// read zip size
	sizeStart := idx - 8
	if sizeStart < 0 {
		return "", "", false
	}
	zipSize := binary.LittleEndian.Uint64(data[sizeStart : sizeStart+8])

	zipStart := sizeStart - int(zipSize)
	if zipStart < 0 {
		return "", "", false
	}
	zipData := data[zipStart:sizeStart]

	// extract to temp
	tmpDir, err := os.MkdirTemp("", "weftapp-*")
	if err != nil {
		return "", "", false
	}

	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", false
	}

	entryFile := "main.weft"
	for _, f := range zr.File {
		if f.Name == ".weftapp-entry" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			entryFile = strings.TrimSpace(string(b))
			continue
		}

		outPath := filepath.Join(tmpDir, f.Name)
		os.MkdirAll(filepath.Dir(outPath), 0o755)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		outFile, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}

	return tmpDir, entryFile, true
}

func findMagic(data []byte) int {
	if len(data) < 16 {
		return -1
	}
	tail := data[len(data)-len(buildMagic):]
	if bytes.Equal(tail, buildMagic) {
		return len(data) - len(buildMagic)
	}
	return -1
}

func signBinary(path string) {
	// best-effort ad-hoc sign for macOS Gatekeeper
	exec := findExec("codesign")
	if exec == "" {
		return
	}
	cmd := execCommand(exec, "--force", "-s", "-", path)
	cmd.Run()
}

func findExec(name string) string {
	p, err := execLookPath(name)
	if err != nil {
		return ""
	}
	return p
}

// these are set to os/exec functions to avoid import cycle issues in tests
var execCommand = defaultExecCommand
var execLookPath = defaultExecLookPath

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
