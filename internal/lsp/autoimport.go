package lsp

import (
	"strings"

	"github.com/loreste/weft/internal/stdlib"
)

// autoImportActions detects unimported packages used on a line and offers to add `use` statements.
func (s *server) autoImportActions(uri, text string, line int) []map[string]any {
	lines := strings.Split(text, "\n")
	if line >= len(lines) {
		return nil
	}

	// find existing imports
	imported := map[string]bool{}
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "use ") {
			pkg := strings.TrimPrefix(t, "use ")
			pkg = strings.TrimSpace(pkg)
			pkg = strings.Trim(pkg, "\"")
			if i := strings.LastIndex(pkg, "/"); i >= 0 {
				pkg = pkg[i+1:]
			}
			if i := strings.Index(pkg, " "); i >= 0 {
				pkg = pkg[:i] // handle "use foo as bar"
			}
			imported[pkg] = true
		}
	}

	// scan the current line for pkg.member patterns
	currentLine := lines[line]
	var actions []map[string]any
	seen := map[string]bool{}

	for i := 0; i < len(currentLine)-1; i++ {
		if currentLine[i] == '.' && i > 0 {
			// extract package name before the dot
			end := i
			start := end - 1
			for start > 0 && isIdentChar(currentLine[start-1]) {
				start--
			}
			pkg := currentLine[start:end]
			if pkg == "" || seen[pkg] || imported[pkg] {
				continue
			}

			// check if it's a known stdlib package or registry package
			if !stdlib.IsPackage(pkg) && !isRegistryPackage(pkg) {
				continue
			}
			seen[pkg] = true

			// find the first line to insert (after any existing use statements)
			insertLine := findInsertLine(lines)

			actions = append(actions, map[string]any{
				"title": "Add `use " + pkg + "`",
				"kind":  "quickfix",
				"edit": map[string]any{
					"changes": map[string]any{
						uri: []map[string]any{
							{
								"range": map[string]any{
									"start": map[string]int{"line": insertLine, "character": 0},
									"end":   map[string]int{"line": insertLine, "character": 0},
								},
								"newText": "use " + pkg + "\n",
							},
						},
					},
				},
			})
		}
	}

	return actions
}

func findInsertLine(lines []string) int {
	lastUse := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "use ") {
			lastUse = i
		}
		// stop scanning after first non-use, non-empty, non-comment line
		if lastUse >= 0 && t != "" && !strings.HasPrefix(t, "use ") && !strings.HasPrefix(t, "//") {
			break
		}
	}
	if lastUse >= 0 {
		return lastUse + 1
	}
	return 0
}

// known registry packages
var registryPackages = map[string]bool{
	"mold": true, "ml": true, "tokensave": true, "warp": true,
	"telecom": true, "retry": true, "semver": true, "color": true,
	"cache": true, "jwt": true, "http_router": true, "template": true,
	"validate": true, "cron": true,
}

func isRegistryPackage(name string) bool {
	return registryPackages[name]
}
