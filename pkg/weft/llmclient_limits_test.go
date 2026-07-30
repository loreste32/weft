package weft

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadLLMResponseRejectsOversizedBody(t *testing.T) {
	_, err := readLLMResponse(bytes.NewReader(bytes.Repeat([]byte{'x'}, maxLLMResponseBytes+1)))
	if err == nil || !strings.Contains(err.Error(), "response too large") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}
