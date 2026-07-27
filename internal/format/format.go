package format

import (
	"github.com/loreste/weft/internal/parse"
)

// Source parses and pretty-prints Weft source. On parse error, returns
// whitespace-normalized original (caller may fall back).
func Source(path, src string) (string, error) {
	file, errs := parse.ParseFile(path, src)
	if errs.HasErrors() {
		return "", errs
	}
	return File(file, Options{}), nil
}
