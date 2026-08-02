package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunSourceExecutesCoreLanguageAndCapturesOutput(t *testing.T) {
	output, err := runSource(`
fn main {
    values := [1, 2, 3]
    say(json.stringify(values))
}`, time.Second)
	if err != nil {
		t.Fatalf("runSource returned error: %v", err)
	}
	if got, want := strings.TrimSpace(output), "[1,2,3]"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunSourceReportsParseAndCompileFailures(t *testing.T) {
	for name, source := range map[string]string{
		"parse":   `fn main {`,
		"compile": `fn main { missing_name }`,
	} {
		_, err := runSource(source, time.Second)
		if err == nil || strings.TrimSpace(err.Error()) == "" {
			t.Fatalf("%s error = %v", name, err)
		}
	}
}

func TestRunSourceHonorsTimeout(t *testing.T) {
	_, err := runSource(`fn main { while true { } }`, 10*time.Millisecond)
	if !errors.Is(err, errExecutionTimeout) {
		t.Fatalf("error = %v, want execution timeout", err)
	}
}

func TestRunSourceValidatesLimits(t *testing.T) {
	if _, err := runSource(strings.Repeat("x", maxSourceBytes+1), time.Second); err == nil {
		t.Fatal("oversized source unexpectedly succeeded")
	}
	if _, err := runSource(`fn main { }`, 0); err == nil {
		t.Fatal("zero timeout unexpectedly succeeded")
	}
	if _, err := runSource(`fn main { }`, maxTimeout+time.Millisecond); err == nil {
		t.Fatal("oversized timeout unexpectedly succeeded")
	}
}
