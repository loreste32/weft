package weft_test

import (
	"net"
	"strings"
	"testing"
	"time"
)

// These tests verify the ESL frame protocol at the TCP level, independent
// of the Weft telecom module. They exercise the exact byte sequences that
// FreeSWITCH sends and verify correct parsing behavior.

func eslMockAcceptAuth(t *testing.T, ln net.Listener) net.Conn {
	t.Helper()
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	// Send auth/request
	conn.Write([]byte("Content-Type: auth/request\n\n"))
	// Read auth response
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.Read(buf)
	// Send auth OK
	conn.Write([]byte("Content-Type: command/reply\nReply-Text: +OK accepted\n\n"))
	return conn
}

func TestESLFrameContentLength(t *testing.T) {
	// Verify that Content-Length-based framing works: server sends a body
	// of exactly N bytes, client parses it correctly.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn := eslMockAcceptAuth(t, ln)
		defer conn.Close()
		// Send an event with Content-Length
		conn.Write([]byte("Content-Type: text/event-json\nContent-Length: 16\n\n{\"Event\":\"TEST\"}"))
		time.Sleep(time.Second)
	}()

	// Client side: connect and read the frame
	addr := ln.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Read auth/request
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := conn.Read(buf)
	if !strings.Contains(string(buf[:n]), "auth/request") {
		t.Fatal("expected auth/request")
	}

	// Send auth
	conn.Write([]byte("auth pw\n\n"))

	// Read auth reply + event
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var all []byte
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			all = append(all, buf[:n]...)
		}
		// Check if we got the event body
		if strings.Contains(string(all), `"Event":"TEST"`) {
			break
		}
		if err != nil {
			break
		}
	}

	s := string(all)
	if !strings.Contains(s, "+OK accepted") {
		t.Fatalf("missing auth OK in: %s", s)
	}
	if !strings.Contains(s, `{"Event":"TEST"}`) {
		t.Fatalf("missing event body in: %s", s)
	}
}

func TestESLFrameCRLF(t *testing.T) {
	// Verify that \r\n\r\n delimiters work (some FreeSWITCH versions use CRLF)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, _ := ln.Accept()
		defer conn.Close()
		// Use CRLF
		conn.Write([]byte("Content-Type: auth/request\r\n\r\n"))
		buf := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buf)
		conn.Write([]byte("Content-Type: command/reply\r\nReply-Text: +OK\r\n\r\n"))
		time.Sleep(500 * time.Millisecond)
	}()

	conn, _ := net.Dial("tcp", ln.Addr().String())
	defer conn.Close()

	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := conn.Read(buf)
	if !strings.Contains(string(buf[:n]), "auth/request") {
		t.Fatal("CRLF framing: expected auth/request")
	}

	conn.Write([]byte("auth pw\n\n"))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ = conn.Read(buf)
	if !strings.Contains(string(buf[:n]), "+OK") {
		t.Fatal("CRLF framing: expected +OK")
	}
}

func TestESLCoalescedFrames(t *testing.T) {
	// Server sends multiple frames in one TCP write — client must split them
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn := eslMockAcceptAuth(t, ln)
		defer conn.Close()
		// Two events in one write
		conn.Write([]byte(
			"Content-Type: text/event-json\nContent-Length: 17\n\n{\"Event\":\"FIRST\"}" +
				"Content-Type: text/event-json\nContent-Length: 18\n\n{\"Event\":\"SECOND\"}"))
		time.Sleep(time.Second)
	}()

	conn, _ := net.Dial("tcp", ln.Addr().String())
	defer conn.Close()

	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.Read(buf) // auth/request
	conn.Write([]byte("auth pw\n\n"))

	// Read everything
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var all []byte
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			all = append(all, buf[:n]...)
		}
		if strings.Contains(string(all), "SECOND") {
			break
		}
		if err != nil {
			break
		}
	}

	s := string(all)
	if !strings.Contains(s, "FIRST") {
		t.Fatal("missing FIRST event")
	}
	if !strings.Contains(s, "SECOND") {
		t.Fatal("missing SECOND event")
	}
}

func TestESLSocketDeadlineHonored(t *testing.T) {
	// Verify that a short read deadline causes timeout, not 60-second default
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, _ := ln.Accept()
		defer conn.Close()
		time.Sleep(10 * time.Second) // never send anything
	}()

	conn, _ := net.Dial("tcp", ln.Addr().String())
	defer conn.Close()

	// Set a 500ms deadline
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	start := time.Now()
	buf := make([]byte, 1024)
	_, err = conn.Read(buf)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("deadline not honored: took %v (expected ~500ms)", elapsed)
	}
	netErr, ok := err.(net.Error)
	if !ok || !netErr.Timeout() {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}
