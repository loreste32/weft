package llmpack

import (
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/parse"
)

// ValidateExample checks that output parses as Weft (required for training).
// Typecheck is soft: undefined args/env are common in short demos.
func ValidateExample(e Example) error {
	src := strings.TrimSpace(e.Output)
	if src == "" {
		return fmt.Errorf("%s: empty output", e.ID)
	}
	if !strings.Contains(src, "fn main") {
		return fmt.Errorf("%s: missing fn main", e.ID)
	}
	_, perrs := parse.ParseFile(e.ID+".weft", src)
	if perrs.HasErrors() {
		return fmt.Errorf("%s: parse: %v", e.ID, perrs)
	}
	return nil
}

// ValidateAll returns errors for every bad training row.
func ValidateAll() []error {
	var errs []error
	for _, e := range Examples() {
		if err := ValidateExample(e); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
