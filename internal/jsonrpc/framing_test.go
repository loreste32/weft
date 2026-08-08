package jsonrpc

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"
)

func reader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func TestReadMessage_BodyLength(t *testing.T) {
	tests := []struct {
		name    string
		input   func() *bufio.Reader
		wantErr string // empty = expect success
		wantLen int    // expected body length on success
	}{
		{
			name:    "valid small JSON",
			input:   func() *bufio.Reader { return reader("Content-Length: 2\r\n\r\n{}") },
			wantLen: 2,
		},
		{
			name: "exactly 10 MiB accepted",
			input: func() *bufio.Reader {
				body := strings.Repeat("x", MaxBodyBytes)
				return reader(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", MaxBodyBytes, body))
			},
			wantLen: MaxBodyBytes,
		},
		{
			name: "10 MiB plus one rejected",
			input: func() *bufio.Reader {
				return reader(fmt.Sprintf("Content-Length: %d\r\n\r\n", MaxBodyBytes+1))
			},
			wantErr: "exceeds",
		},
		{
			name:    "zero rejected",
			input:   func() *bufio.Reader { return reader("Content-Length: 0\r\n\r\n") },
			wantErr: "must be positive",
		},
		{
			name:    "negative rejected",
			input:   func() *bufio.Reader { return reader("Content-Length: -5\r\n\r\n") },
			wantErr: "must be positive",
		},
		{
			name:    "missing rejected",
			input:   func() *bufio.Reader { return reader("Content-Type: json\r\n\r\n") },
			wantErr: "missing Content-Length",
		},
		{
			name:    "nonnumeric rejected",
			input:   func() *bufio.Reader { return reader("Content-Length: abc\r\n\r\n") },
			wantErr: "invalid Content-Length",
		},
		{
			name:    "partially numeric rejected",
			input:   func() *bufio.Reader { return reader("Content-Length: 123junk\r\n\r\n") },
			wantErr: "invalid Content-Length",
		},
		{
			name:    "integer overflow rejected",
			input:   func() *bufio.Reader { return reader("Content-Length: 99999999999999999999\r\n\r\n") },
			wantErr: "invalid Content-Length",
		},
		{
			name: "truncated body returns read error",
			input: func() *bufio.Reader {
				return reader("Content-Length: 100\r\n\r\nshort")
			},
			wantErr: "body read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := ReadMessage(tt.input())
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(body) != tt.wantLen {
				t.Fatalf("body length = %d, want %d", len(body), tt.wantLen)
			}
		})
	}
}

func TestReadMessage_Headers(t *testing.T) {
	tests := []struct {
		name    string
		input   func() *bufio.Reader
		wantErr string
	}{
		{
			name: "64 headers accepted",
			input: func() *bufio.Reader {
				var sb strings.Builder
				for i := 0; i < MaxHeaderCount-1; i++ {
					fmt.Fprintf(&sb, "X-H-%d: v\r\n", i)
				}
				sb.WriteString("Content-Length: 2\r\n\r\n{}")
				return reader(sb.String())
			},
		},
		{
			name: "65 headers rejected",
			input: func() *bufio.Reader {
				var sb strings.Builder
				for i := 0; i < MaxHeaderCount; i++ {
					fmt.Fprintf(&sb, "X-H-%d: v\r\n", i)
				}
				sb.WriteString("Content-Length: 2\r\n\r\n{}")
				return reader(sb.String())
			},
			wantErr: "too many headers",
		},
		{
			name: "8 KiB header line accepted",
			input: func() *bufio.Reader {
				// header: "X-Big: " (7) + padding + "\r\n" (2) = MaxHeaderLine
				pad := strings.Repeat("a", MaxHeaderLine-7-2)
				return reader("X-Big: " + pad + "\r\nContent-Length: 2\r\n\r\n{}")
			},
		},
		{
			name: "over-limit header line rejected",
			input: func() *bufio.Reader {
				pad := strings.Repeat("a", MaxHeaderLine+1)
				return reader("X-Big: " + pad + "\r\nContent-Length: 2\r\n\r\n{}")
			},
			wantErr: "header line exceeds",
		},
		{
			name: "total header bytes at limit accepted",
			input: func() *bufio.Reader {
				// Use fewer but larger headers to stay under 64 count but near 32 KiB total
				// Each line ~1100 bytes, 28 lines = ~30800 bytes, plus Content-Length line
				line := "X-Pad: " + strings.Repeat("p", 1090) + "\r\n"
				var sb strings.Builder
				for i := 0; i < 28; i++ {
					sb.WriteString(line)
				}
				sb.WriteString("Content-Length: 2\r\n\r\n{}")
				return reader(sb.String())
			},
		},
		{
			name: "total header bytes over limit rejected",
			input: func() *bufio.Reader {
				// Use 32 lines of ~1100 bytes each to exceed 32 KiB before hitting 64-header cap
				line := "X-Pad: " + strings.Repeat("p", 1090) + "\r\n"
				var sb strings.Builder
				for i := 0; i < 32; i++ {
					sb.WriteString(line)
				}
				sb.WriteString("Content-Length: 2\r\n\r\n{}")
				return reader(sb.String())
			},
			wantErr: "byte limit",
		},
		{
			name: "unterminated oversized header rejected",
			input: func() *bufio.Reader {
				// A line longer than MaxHeaderLine that ends with a newline
				huge := strings.Repeat("X", MaxHeaderLine+100) + "\n"
				return bufio.NewReaderSize(strings.NewReader(huge), MaxHeaderLine+200)
			},
			wantErr: "header line exceeds",
		},
		{
			name: "duplicate Content-Length rejected",
			input: func() *bufio.Reader {
				return reader("Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}")
			},
			wantErr: "duplicate Content-Length",
		},
		{
			name: "conflicting duplicate Content-Length rejected",
			input: func() *bufio.Reader {
				return reader("Content-Length: 2\r\nContent-Length: 5\r\n\r\n{}")
			},
			wantErr: "duplicate Content-Length",
		},
		{
			name: "mixed-case Content-Length recognized",
			input: func() *bufio.Reader {
				return reader("content-LENGTH: 2\r\n\r\n{}")
			},
		},
		{
			name:    "123junk rejected",
			input:   func() *bufio.Reader { return reader("Content-Length: 123junk\r\n\r\n") },
			wantErr: "invalid Content-Length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := ReadMessage(tt.input())
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			_ = body
		})
	}
}

func TestWriteMessage(t *testing.T) {
	var sb strings.Builder
	err := WriteMessage(&sb, map[string]string{"ok": "yes"})
	if err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	if !strings.HasPrefix(out, "Content-Length: ") {
		t.Fatalf("missing header: %q", out)
	}
	// Should be parseable back
	body, err := ReadMessage(bufio.NewReader(strings.NewReader(out)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"ok"`) {
		t.Fatalf("round-trip failed: %s", body)
	}
}

func TestReadBoundedLine_EOF(t *testing.T) {
	// Empty reader should return EOF
	_, err := readBoundedLine(bufio.NewReader(strings.NewReader("")), 1024)
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}
