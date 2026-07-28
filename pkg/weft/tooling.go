package weft

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/format"
	"github.com/loreste/weft/internal/stdlib"
)

// StdlibNames returns sorted stdlib package names.
func StdlibNames() []string { return stdlib.Names() }

// StdlibMembers returns member names of a stdlib package (nil if unknown).
func StdlibMembers(name string) []string {
	if !stdlib.IsPackage(name) {
		return nil
	}
	return stdlib.PackageMembers(name)
}

// FmtFiles pretty-prints Weft files via AST (falls back to whitespace trim on parse error).
// Returns count of files whose on-disk content changed.
func FmtFiles(paths []string) (int, error) {
	changed := 0
	for _, path := range paths {
		st, err := os.Stat(path)
		if err != nil {
			return changed, err
		}
		if st.IsDir() {
			err := filepath.Walk(path, func(f string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					name := info.Name()
					if name == "vendor" || name == ".git" || name == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				n, err := fmtOne(f)
				if err != nil {
					return err
				}
				changed += n
				return nil
			})
			if err != nil {
				return changed, err
			}
			continue
		}
		n, err := fmtOne(path)
		if err != nil {
			return changed, err
		}
		changed += n
	}
	return changed, nil
}

// FmtCheck returns paths that would change (for CI). No files written.
func FmtCheck(paths []string) ([]string, error) {
	var dirty []string
	for _, path := range paths {
		st, err := os.Stat(path)
		if err != nil {
			return dirty, err
		}
		if st.IsDir() {
			err := filepath.Walk(path, func(f string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					name := info.Name()
					if name == "vendor" || name == ".git" || name == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				if changed, err := fmtWouldChange(f); err != nil {
					return err
				} else if changed {
					dirty = append(dirty, f)
				}
				return nil
			})
			if err != nil {
				return dirty, err
			}
			continue
		}
		if changed, err := fmtWouldChange(path); err != nil {
			return dirty, err
		} else if changed {
			dirty = append(dirty, path)
		}
	}
	return dirty, nil
}

func fmtWouldChange(path string) (bool, error) {
	if !strings.HasSuffix(path, ".weft") && !strings.HasSuffix(path, ".loom") {
		return false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	formatted, err := format.Source(path, string(raw))
	if err != nil {
		formatted = string(formatWhitespace(raw))
	}
	if !strings.HasSuffix(formatted, "\n") && formatted != "" {
		formatted += "\n"
	}
	return !bytes.Equal(raw, []byte(formatted)), nil
}

func fmtOne(path string) (int, error) {
	if !strings.HasSuffix(path, ".weft") && !strings.HasSuffix(path, ".loom") {
		return 0, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	formatted, err := format.Source(path, string(raw))
	if err != nil {
		// fallback: whitespace only
		formatted = string(formatWhitespace(raw))
	}
	if !strings.HasSuffix(formatted, "\n") && formatted != "" {
		formatted += "\n"
	}
	b := []byte(formatted)
	if bytes.Equal(raw, b) {
		return 0, nil
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return 0, err
	}
	return 1, nil
}

func formatWhitespace(src []byte) []byte {
	s := string(src)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := strings.Join(lines, "\n")
	if out != "" {
		out += "\n"
	}
	return []byte(out)
}
