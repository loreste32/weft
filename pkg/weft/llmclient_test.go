package weft

import (
	"strings"
	"testing"
)

func TestExtractWeftCodeFenced(t *testing.T) {
	reply := "Sure!\n```weft\nfn main {\n    say(\"hi\")\n}\n```\n"
	got := ExtractWeftCode(reply)
	if got != "fn main {\n    say(\"hi\")\n}" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractWeftCodeLegacyFence(t *testing.T) {
	reply := "```loom\nfn main { say(1) }\n```"
	got := ExtractWeftCode(reply)
	if got != "fn main { say(1) }" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractWeftCodeGenericFence(t *testing.T) {
	reply := "Here:\n```\nfn main { say(2) }\n```\n"
	got := ExtractWeftCode(reply)
	if !strings.Contains(got, "fn main") {
		t.Fatalf("got %q", got)
	}
}

func TestExtractWeftCodeBare(t *testing.T) {
	reply := "fn main {\n    say(\"x\")\n}"
	got := ExtractWeftCode(reply)
	if got != reply {
		t.Fatalf("got %q", got)
	}
}

func TestValidateWeftSource(t *testing.T) {
	if err := validateWeftSource(`fn main { say("ok") }`); err != nil {
		t.Fatal(err)
	}
	if err := validateWeftSource(`say("no main")`); err == nil {
		t.Fatal("expected error for missing main")
	}
	if err := validateWeftSource(``); err == nil {
		t.Fatal("expected error for empty")
	}
	if err := validateWeftSource(`fn main { !!! }`); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestVersion(t *testing.T) {
	if Version == "" || !strings.HasPrefix(Version, "0.") {
		t.Fatalf("unexpected Version %q", Version)
	}
}
