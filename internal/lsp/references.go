package lsp

import (
	"strings"
)

// findReferences returns all locations where the identifier at cursor is used.
func findReferences(text, uri string, line, character int) any {
	word := wordAt(text, line, character)
	if word == "" || isKeyword(word) {
		return nil
	}

	lines := strings.Split(text, "\n")
	var locations []map[string]any

	for lineIdx, ln := range lines {
		col := 0
		for col < len(ln) {
			idx := strings.Index(ln[col:], word)
			if idx < 0 {
				break
			}
			start := col + idx
			end := start + len(word)

			// word boundaries
			if start > 0 && isIdentChar(ln[start-1]) {
				col = end
				continue
			}
			if end < len(ln) && isIdentChar(ln[end]) {
				col = end
				continue
			}
			// skip strings
			if inString(ln, start) {
				col = end
				continue
			}

			locations = append(locations, map[string]any{
				"uri":   uri,
				"range": makeRange(lineIdx, start, lineIdx, end),
			})
			col = end
		}
	}

	if len(locations) == 0 {
		return nil
	}
	return locations
}
