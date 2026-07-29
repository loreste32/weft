package weft

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/loreste/weft/internal/parse"
)

// Doc generates API documentation from Weft source files.
func Doc(paths []string) error {
	if len(paths) == 0 {
		paths = []string{"."}
	}

	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		if info.IsDir() {
			filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					if fi != nil && fi.IsDir() {
						base := filepath.Base(path)
						if base == "vendor" || base == ".git" {
							return filepath.SkipDir
						}
					}
					return err
				}
				if strings.HasSuffix(path, ".weft") && !strings.HasSuffix(path, "_test.weft") {
					files = append(files, path)
				}
				return nil
			})
		} else {
			files = append(files, p)
		}
	}

	for _, f := range files {
		docFile(f)
	}
	return nil
}

func docFile(path string) {
	src, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content := string(src)
	lines := strings.Split(content, "\n")

	// parse to get function names
	file, perrs := parse.ParseFile(path, content)
	if perrs.HasErrors() {
		return
	}
	_ = file

	// extract pub fn declarations with their doc comments
	type funcDoc struct {
		name    string
		sig     string
		comment string
		line    int
	}

	var funcs []funcDoc
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "pub fn ") {
			continue
		}
		// extract signature
		sig := trimmed
		if idx := strings.Index(sig, "{"); idx > 0 {
			sig = strings.TrimSpace(sig[:idx])
		}
		name := strings.TrimPrefix(sig, "pub fn ")
		if idx := strings.Index(name, "("); idx > 0 {
			name = name[:idx]
		}

		// collect doc comment above
		var commentLines []string
		for j := i - 1; j >= 0; j-- {
			cl := strings.TrimSpace(lines[j])
			if strings.HasPrefix(cl, "//") {
				comment := strings.TrimPrefix(cl, "//")
				comment = strings.TrimPrefix(comment, " ")
				commentLines = append([]string{comment}, commentLines...)
			} else {
				break
			}
		}

		funcs = append(funcs, funcDoc{
			name:    name,
			sig:     sig,
			comment: strings.Join(commentLines, "\n"),
			line:    i + 1,
		})
	}

	if len(funcs) == 0 {
		return
	}

	sort.Slice(funcs, func(i, j int) bool {
		return funcs[i].name < funcs[j].name
	})

	rel := path
	if wd, err := os.Getwd(); err == nil {
		if r, err := filepath.Rel(wd, path); err == nil {
			rel = r
		}
	}

	fmt.Printf("── %s (%d exports) ──\n\n", rel, len(funcs))
	for _, f := range funcs {
		fmt.Printf("  %s\n", f.sig)
		if f.comment != "" {
			for _, cl := range strings.Split(f.comment, "\n") {
				fmt.Printf("    %s\n", cl)
			}
		}
		fmt.Println()
	}
}
