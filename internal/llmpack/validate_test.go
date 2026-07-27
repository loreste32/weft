package llmpack

import (
	"testing"
)

func TestCorpusParses(t *testing.T) {
	exs := Examples()
	if len(exs) < 10 {
		t.Fatalf("expected training rows, got %d", len(exs))
	}
	var failed int
	for _, e := range exs {
		if err := ValidateExample(e); err != nil {
			t.Log(err)
			failed++
		}
	}
	if failed > 0 {
		t.Fatalf("%d/%d training examples failed validation", failed, len(exs))
	}
}

func TestSystemCardNonEmpty(t *testing.T) {
	if len(SystemCard()) < 500 {
		t.Fatal("system card too short")
	}
}
