package lsp

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
)

func TestReadMessageBodyLimit(t *testing.T) {
	over := fmt.Sprintf("Content-Length: %d\r\n\r\n", 10<<20+1)
	_, err := readMessage(bufio.NewReader(strings.NewReader(over)))
	if err == nil {
		t.Fatal("expected error for oversized content-length")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadMessageNegativeLength(t *testing.T) {
	msg := "Content-Length: -5\r\n\r\n"
	_, err := readMessage(bufio.NewReader(strings.NewReader(msg)))
	if err == nil {
		t.Fatal("expected error for negative content-length")
	}
}

func TestReadMessageMissingLength(t *testing.T) {
	msg := "Content-Type: application/json\r\n\r\n"
	_, err := readMessage(bufio.NewReader(strings.NewReader(msg)))
	if err == nil {
		t.Fatal("expected error for missing content-length")
	}
}

func TestReadMessageHeaderCountLimit(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < maxHeaders+1; i++ {
		fmt.Fprintf(&sb, "X-Header-%d: value\r\n", i)
	}
	sb.WriteString("\r\n")
	_, err := readMessage(bufio.NewReader(strings.NewReader(sb.String())))
	if err == nil {
		t.Fatal("expected error for too many headers")
	}
}
