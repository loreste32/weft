package weft_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These tests are deliberately named wire fixtures: they validate byte-level
// ESL framing only. The real Weft parser/dispatcher path is covered by
// TestESLBlackBoxProcess, which launches the actual Weft client process.
const wireFixtureTimeout = 2 * time.Second

type wireFrame struct {
	headers map[string]string
	body    string
}

func listenWireFixture(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln, ln.Addr().String()
}

func closeWireFixtureListener(t *testing.T, ln net.Listener) {
	t.Helper()
	if err := ln.Close(); err != nil {
		t.Errorf("close listener: %v", err)
	}
}

func runWireFixtureServer(ln net.Listener, handler func(net.Conn) error) (err error) {
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close connection: %w", closeErr)
		}
	}()
	if err := conn.SetDeadline(time.Now().Add(wireFixtureTimeout)); err != nil {
		return fmt.Errorf("set server deadline: %w", err)
	}
	return handler(conn)
}

func writeWire(conn net.Conn, data []byte) error {
	n, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("write %d bytes: %w", len(data), err)
	}
	if n != len(data) {
		return fmt.Errorf("write %d bytes: wrote %d: %w", len(data), n, io.ErrShortWrite)
	}
	return nil
}

func readWireBlock(br *bufio.Reader) ([]byte, error) {
	var block bytes.Buffer
	for block.Len() <= 64<<10 {
		line, err := br.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		_, _ = block.Write(line)
		if bytes.HasSuffix(block.Bytes(), []byte("\n\n")) || bytes.HasSuffix(block.Bytes(), []byte("\r\n\r\n")) {
			return block.Bytes(), nil
		}
	}
	return nil, errors.New("ESL wire block exceeds 64KB")
}

func readWireCommand(br *bufio.Reader) (string, error) {
	block, err := readWireBlock(br)
	if err != nil {
		return "", err
	}
	return string(block), nil
}

func readWireFrame(br *bufio.Reader) (wireFrame, error) {
	block, err := readWireBlock(br)
	if err != nil {
		return wireFrame{}, err
	}
	text := string(block)
	if i := strings.Index(text, "\r\n\r\n"); i >= 0 {
		text = text[:i]
	} else if i := strings.Index(text, "\n\n"); i >= 0 {
		text = text[:i]
	} else {
		return wireFrame{}, errors.New("ESL wire block has no delimiter")
	}

	frame := wireFrame{headers: make(map[string]string)}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return wireFrame{}, fmt.Errorf("malformed ESL header %q", line)
		}
		name := strings.TrimSpace(parts[0])
		if _, exists := frame.headers[name]; exists {
			return wireFrame{}, fmt.Errorf("duplicate ESL header %q", name)
		}
		frame.headers[name] = strings.TrimSpace(parts[1])
	}

	bodyLength := 0
	if rawLength, ok := frame.headers["Content-Length"]; ok {
		bodyLength, err = strconv.Atoi(rawLength)
		if err != nil || bodyLength < 0 {
			return wireFrame{}, fmt.Errorf("invalid Content-Length %q", rawLength)
		}
	}
	body := make([]byte, bodyLength)
	if _, err := io.ReadFull(br, body); err != nil {
		return wireFrame{}, err
	}
	frame.body = string(body)
	return frame, nil
}

func wireReply(body string, crlf bool) []byte {
	line := "\n"
	if crlf {
		line = "\r\n"
	}
	frame := "Content-Type: command/reply" + line + "Reply-Text: +OK accepted" + line
	if body != "" {
		frame += "Content-Length: " + strconv.Itoa(len(body)) + line
	}
	return []byte(frame + line + body)
}

func wireEvent(body string, crlf bool) []byte {
	line := "\n"
	if crlf {
		line = "\r\n"
	}
	return []byte("Content-Type: text/event-json" + line +
		"Content-Length: " + strconv.Itoa(len(body)) + line + line + body)
}

func serveWireHandshake(conn net.Conn, crlf bool, afterAuth func(*bufio.Reader, net.Conn) error) error {
	line := "\n"
	if crlf {
		line = "\r\n"
	}
	if err := writeWire(conn, []byte("Content-Type: auth/request"+line+line)); err != nil {
		return err
	}
	br := bufio.NewReader(conn)
	command, err := readWireCommand(br)
	if err != nil {
		return fmt.Errorf("read auth command: %w", err)
	}
	if strings.TrimSpace(command) != "auth pw" {
		return fmt.Errorf("unexpected auth command %q", command)
	}
	if err := writeWire(conn, wireReply("", crlf)); err != nil {
		return err
	}
	return afterAuth(br, conn)
}

func dialWireFixture(t *testing.T, address string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	frame, err := readWireFrame(br)
	if err != nil {
		_ = conn.Close()
		t.Fatalf("read auth request: %v", err)
	}
	if frame.headers["Content-Type"] != "auth/request" || frame.body != "" {
		_ = conn.Close()
		t.Fatalf("unexpected auth request: %#v", frame)
	}
	if err := writeWire(conn, []byte("auth pw\n\n")); err != nil {
		_ = conn.Close()
		t.Fatalf("write auth command: %v", err)
	}
	if frame, err = readWireFrame(br); err != nil {
		_ = conn.Close()
		t.Fatalf("read auth reply: %v", err)
	} else if frame.headers["Reply-Text"] != "+OK accepted" {
		_ = conn.Close()
		t.Fatalf("unexpected auth reply: %#v", frame)
	}
	return conn, br
}

func waitWireFixtureServer(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(wireFixtureTimeout):
		t.Fatal("wire fixture server did not finish")
	}
}

func TestESLWireFixtureContentLength(t *testing.T) {
	ln, address := listenWireFixture(t)
	defer closeWireFixtureListener(t, ln)
	done := make(chan error, 1)
	go func() {
		done <- runWireFixtureServer(ln, func(conn net.Conn) error {
			return serveWireHandshake(conn, false, func(_ *bufio.Reader, conn net.Conn) error {
				return writeWire(conn, wireEvent(`{"Event":"TEST"}`, false))
			})
		})
	}()

	conn, br := dialWireFixture(t, address)
	frame, err := readWireFrame(br)
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	if frame.headers["Content-Length"] != "16" || frame.body != `{"Event":"TEST"}` {
		t.Fatalf("unexpected event frame: %#v", frame)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	waitWireFixtureServer(t, done)
}

func TestESLWireFixtureCRLF(t *testing.T) {
	ln, address := listenWireFixture(t)
	defer closeWireFixtureListener(t, ln)
	done := make(chan error, 1)
	go func() {
		done <- runWireFixtureServer(ln, func(conn net.Conn) error {
			return serveWireHandshake(conn, true, func(_ *bufio.Reader, conn net.Conn) error {
				return writeWire(conn, wireEvent(`{"Event":"CRLF"}`, true))
			})
		})
	}()

	conn, br := dialWireFixture(t, address)
	frame, err := readWireFrame(br)
	if err != nil {
		t.Fatalf("read CRLF event: %v", err)
	}
	if frame.headers["Content-Type"] != "text/event-json" || frame.body != `{"Event":"CRLF"}` {
		t.Fatalf("unexpected CRLF event frame: %#v", frame)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	waitWireFixtureServer(t, done)
}

func TestESLWireFixtureCoalescedFrames(t *testing.T) {
	ln, address := listenWireFixture(t)
	defer closeWireFixtureListener(t, ln)
	done := make(chan error, 1)
	go func() {
		done <- runWireFixtureServer(ln, func(conn net.Conn) error {
			return serveWireHandshake(conn, false, func(_ *bufio.Reader, conn net.Conn) error {
				first := wireEvent(`{"Event":"FIRST"}`, false)
				second := wireEvent(`{"Event":"SECOND"}`, false)
				return writeWire(conn, append(first, second...))
			})
		})
	}()

	conn, br := dialWireFixture(t, address)
	first, err := readWireFrame(br)
	if err != nil {
		t.Fatalf("read first coalesced frame: %v", err)
	}
	second, err := readWireFrame(br)
	if err != nil {
		t.Fatalf("read second coalesced frame: %v", err)
	}
	if first.body != `{"Event":"FIRST"}` || second.body != `{"Event":"SECOND"}` {
		t.Fatalf("coalesced frames were not separated: %#v %#v", first, second)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	waitWireFixtureServer(t, done)
}

func TestESLWireFixtureDeadlineHonored(t *testing.T) {
	ln, address := listenWireFixture(t)
	defer closeWireFixtureListener(t, ln)
	done := make(chan error, 1)
	go func() {
		done <- runWireFixtureServer(ln, func(conn net.Conn) error {
			return serveWireHandshake(conn, false, func(_ *bufio.Reader, conn net.Conn) error {
				buffer := make([]byte, 1)
				_, err := conn.Read(buffer)
				if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
					return nil
				}
				return fmt.Errorf("read after client deadline: %w", err)
			})
		})
	}()

	conn, br := dialWireFixture(t, address)
	if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	_, err := readWireFrame(br)
	if err == nil {
		t.Fatal("read unexpectedly succeeded without a frame")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected timeout, got %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	waitWireFixtureServer(t, done)
}
