package pkgman

import (
	"fmt"
	"path/filepath"
	"strings"
)

// safeJoin joins dest with a relative archive member name, rejecting zip-slip
// and absolute/UNC/drive paths. Returns cleaned absolute target under dest.
func safeJoin(dest, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty archive path")
	}
	// normalize to slash for checks
	n := strings.ReplaceAll(name, "\\", "/")
	if strings.Contains(n, "\x00") {
		return "", fmt.Errorf("illegal path (null byte): %q", name)
	}
	// reject absolute / UNC / drive-letter
	if strings.HasPrefix(n, "/") || strings.HasPrefix(n, "//") {
		return "", fmt.Errorf("illegal absolute path in archive: %q", name)
	}
	if len(n) >= 2 && n[1] == ':' { // C:...
		return "", fmt.Errorf("illegal drive path in archive: %q", name)
	}
	// clean and reject ".."
	parts := strings.Split(n, "/")
	var clean []string
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			return "", fmt.Errorf("illegal path traversal in archive: %q", name)
		}
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return "", fmt.Errorf("empty archive path after clean: %q", name)
	}
	rel := filepath.Join(clean...)
	target := filepath.Join(dest, rel)
	// final containment (handles OS quirks)
	destClean := filepath.Clean(dest)
	targetClean := filepath.Clean(target)
	sep := string(filepath.Separator)
	if targetClean != destClean && !strings.HasPrefix(targetClean, destClean+sep) {
		return "", fmt.Errorf("illegal path escapes dest: %q", name)
	}
	return targetClean, nil
}
