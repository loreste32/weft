package llmpack

import (
	"testing"
)

func TestExpand(t *testing.T) {
	base := []Example{
		{ID: "e1", Instruction: "Write hello world", Output: `fn main { say("hello") }`},
	}
	expanded := Expand(base)
	if len(expanded) <= len(base) {
		t.Fatalf("expanded should be larger: %d", len(expanded))
	}
	// first should be original
	if expanded[0].ID != "e1" {
		t.Fatal("first should be original")
	}
	// variants should have suffixed IDs
	for _, e := range expanded[1:] {
		if e.ID == "e1" {
			t.Fatal("variants should have different IDs")
		}
		if e.Output != base[0].Output {
			t.Fatal("output should be preserved across variants")
		}
	}
}

func TestExpandEmpty(t *testing.T) {
	expanded := Expand(nil)
	if len(expanded) != 0 {
		t.Fatal("nil should return empty")
	}
}

func TestParaphrase(t *testing.T) {
	variants := paraphrase("Hello world")
	if len(variants) < 3 {
		t.Fatalf("expected at least 3 variants, got %d", len(variants))
	}
	// no duplicates
	seen := map[string]bool{}
	for _, v := range variants {
		if seen[v] {
			t.Fatalf("duplicate: %q", v)
		}
		seen[v] = true
	}
}

func TestParaphraseEmpty(t *testing.T) {
	variants := paraphrase("")
	// empty input might still generate variants with prefix
	for _, v := range variants {
		if v == "" {
			t.Fatal("empty variant should be filtered")
		}
	}
}

func TestLowercaseFirst(t *testing.T) {
	if lowercaseFirst("Hello") != "hello" {
		t.Fatal("lowercase")
	}
	if lowercaseFirst("") != "" {
		t.Fatal("empty")
	}
	if lowercaseFirst("a") != "a" {
		t.Fatal("single")
	}
}

func TestComputeStats(t *testing.T) {
	exs := []Example{
		{ID: "1", Output: "1234567890", Tags: []string{"basic", "io"}},
		{ID: "2", Output: "12345", Tags: []string{"basic"}},
	}
	st := ComputeStats(exs)
	if st.Count != 2 {
		t.Fatal("count")
	}
	if st.AvgOut != 7.5 {
		t.Fatalf("avg = %f", st.AvgOut)
	}
	if st.Tags["basic"] != 2 {
		t.Fatal("tag count")
	}
	if st.Tags["io"] != 1 {
		t.Fatal("io tag")
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	st := ComputeStats(nil)
	if st.Count != 0 || st.AvgOut != 0 {
		t.Fatal("empty stats")
	}
}

func TestValidateAll(t *testing.T) {
	errs := ValidateAll()
	if len(errs) > 0 {
		for _, e := range errs {
			t.Log(e)
		}
		t.Fatalf("%d validation errors", len(errs))
	}
}

func TestValidateExampleEmpty(t *testing.T) {
	err := ValidateExample(Example{ID: "bad", Output: ""})
	if err == nil {
		t.Fatal("empty output should fail")
	}
}

func TestValidateExampleNoMain(t *testing.T) {
	err := ValidateExample(Example{ID: "bad", Output: `say("hello")`})
	if err == nil {
		t.Fatal("missing main should fail")
	}
}

func TestValidateExampleParseError(t *testing.T) {
	err := ValidateExample(Example{ID: "bad", Output: "fn main { @@@invalid }"})
	if err == nil {
		t.Fatal("parse error should fail")
	}
}
