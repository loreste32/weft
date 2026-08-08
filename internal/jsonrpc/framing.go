// Package jsonrpc provides bounded JSON-RPC message framing shared by
// the DAP and LSP servers. All limits are enforced before allocation so
// a hostile or malformed client cannot exhaust host memory.
package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Limits for header and body framing. These are intentionally generous
// for local IPC but bounded against malicious input.
const (
	MaxHeaderLine  = 8 << 10  // 8 KiB per header line
	MaxHeaderBytes = 32 << 10 // 32 KiB total header section
	MaxHeaderCount = 64       // max number of header lines
	MaxBodyBytes   = 10 << 20 // 10 MiB message body
)

// ReadMessage reads one length-delimited JSON-RPC message (as used by
// DAP and LSP) from r. It enforces all framing limits before allocating
// memory proportional to any client-controlled value.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	contentLen := -1
	headerCount := 0
	totalHeaderBytes := 0

	for {
		// Use ReadSlice to avoid unbounded allocation. If the line
		// exceeds the bufio buffer we get bufio.ErrBufferFull, but we
		// also track totalHeaderBytes as a hard cap.
		line, err := readBoundedLine(r, MaxHeaderLine)
		if err != nil {
			return nil, fmt.Errorf("header read: %w", err)
		}

		totalHeaderBytes += len(line)
		if totalHeaderBytes > MaxHeaderBytes {
			return nil, fmt.Errorf("headers exceed %d byte limit", MaxHeaderBytes)
		}

		trimmed := strings.TrimRight(string(line), "\r\n")
		if trimmed == "" {
			break // blank line = end of headers
		}

		headerCount++
		if headerCount > MaxHeaderCount {
			return nil, fmt.Errorf("too many headers (%d > %d)", headerCount, MaxHeaderCount)
		}

		if strings.HasPrefix(strings.ToLower(trimmed), "content-length:") {
			if contentLen >= 0 {
				return nil, fmt.Errorf("duplicate Content-Length header")
			}
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("malformed Content-Length: %q", trimmed)
			}
			val := strings.TrimSpace(parts[1])
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q: %w", val, err)
			}
			if n <= 0 {
				return nil, fmt.Errorf("Content-Length must be positive, got %d", n)
			}
			contentLen = n
		}
	}

	if contentLen < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	if contentLen > MaxBodyBytes {
		return nil, fmt.Errorf("Content-Length %d exceeds %d limit", contentLen, MaxBodyBytes)
	}

	body := make([]byte, contentLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("body read: %w", err)
	}
	return body, nil
}

// WriteMessage writes a JSON-RPC message with a Content-Length header.
func WriteMessage(w io.Writer, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

// readBoundedLine reads up to maxLen bytes including the trailing newline.
// Returns an error if the line exceeds the limit before a newline is found.
func readBoundedLine(r *bufio.Reader, maxLen int) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.ReadSlice('\n')
		buf = append(buf, chunk...)
		if err == nil {
			// Found newline
			if len(buf) > maxLen {
				return nil, fmt.Errorf("header line exceeds %d bytes", maxLen)
			}
			return buf, nil
		}
		if err == bufio.ErrBufferFull {
			// Line continues; check accumulated length
			if len(buf) > maxLen {
				return nil, fmt.Errorf("header line exceeds %d bytes", maxLen)
			}
			continue
		}
		return nil, err // io.EOF or other
	}
}
