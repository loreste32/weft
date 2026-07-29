package dap_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/loreste/weft/internal/dap"
)

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.b.Bytes()...)
}

func writeReq(w io.Writer, id int, method string, params any) {
	body := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		body["params"] = params
	}
	b, _ := json.Marshal(body)
	fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(b), b)
}

// tryMsg attempts to parse one DAP message from buf at *off. Returns ok=false if incomplete.
func tryMsg(buf *safeBuffer, off *int) (map[string]any, bool) {
	data := buf.snapshot()
	if *off >= len(data) {
		return nil, false
	}
	rest := data[*off:]
	headerEnd := bytes.Index(rest, []byte("\r\n\r\n"))
	if headerEnd < 0 {
		return nil, false
	}
	header := string(rest[:headerEnd])
	var cl int
	for _, line := range bytes.Split([]byte(header), []byte("\r\n")) {
		if bytes.HasPrefix(bytes.ToLower(line), []byte("content-length:")) {
			parts := bytes.SplitN(line, []byte(":"), 2)
			if len(parts) == 2 {
				fmt.Sscanf(string(bytes.TrimSpace(parts[1])), "%d", &cl)
			}
		}
	}
	if cl <= 0 || len(rest) < headerEnd+4+cl {
		return nil, false
	}
	body := rest[headerEnd+4 : headerEnd+4+cl]
	*off += headerEnd + 4 + cl
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, false
	}
	return m, true
}

func waitMsg(t *testing.T, buf *safeBuffer, off *int, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m, ok := tryMsg(buf, off); ok {
			return m
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for message; tail=%q", buf.snapshot()[*off:])
	return nil
}

func waitUntil(t *testing.T, buf *safeBuffer, off *int, timeout time.Duration, pred func(map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m, ok := tryMsg(buf, off)
		if !ok {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if pred(m) {
			return m
		}
	}
	t.Fatalf("timeout waiting for matching message; tail=%q", buf.snapshot()[*off:])
	return nil
}

func TestDAPInitializeAndLaunch(t *testing.T) {
	dir := t.TempDir()
	prog := filepath.Join(dir, "t.weft")
	if err := os.WriteFile(prog, []byte("fn main {\n    x := 1\n    say(x)\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	inR, inW := io.Pipe()
	var out safeBuffer
	done := make(chan error, 1)
	go func() { done <- dap.Run(inR, &out, "") }()
	off := 0

	writeReq(inW, 1, "initialize", map[string]any{"adapterID": "weft"})
	m := waitMsg(t, &out, &off, 2*time.Second)
	if m["command"] != "initialize" || m["success"] != true {
		t.Fatalf("init: %#v", m)
	}
	m = waitMsg(t, &out, &off, 2*time.Second)
	if m["event"] != "initialized" {
		t.Fatalf("initialized: %#v", m)
	}

	writeReq(inW, 2, "launch", map[string]any{"program": prog, "stopOnEntry": true})
	m = waitMsg(t, &out, &off, 2*time.Second)
	if m["command"] != "launch" {
		t.Fatalf("launch: %#v", m)
	}

	writeReq(inW, 3, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": prog},
		"breakpoints": []map[string]any{{"line": 3}},
	})
	m = waitMsg(t, &out, &off, 2*time.Second)
	if m["command"] != "setBreakpoints" {
		t.Fatalf("bp: %#v", m)
	}

	writeReq(inW, 4, "configurationDone", map[string]any{})
	gotConfig, gotStopped := false, false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !(gotConfig && gotStopped) {
		m, ok := tryMsg(&out, &off)
		if !ok {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if m["type"] == "response" && m["command"] == "configurationDone" {
			gotConfig = true
		}
		if m["type"] == "event" && m["event"] == "stopped" {
			gotStopped = true
		}
	}
	if !gotConfig || !gotStopped {
		t.Fatalf("configDone=%v stopped=%v", gotConfig, gotStopped)
	}

	writeReq(inW, 5, "threads", nil)
	_ = waitMsg(t, &out, &off, 2*time.Second)

	writeReq(inW, 6, "stackTrace", map[string]any{"threadId": 1})
	m = waitMsg(t, &out, &off, 2*time.Second)
	body, _ := m["body"].(map[string]any)
	frames, _ := body["stackFrames"].([]any)
	if len(frames) < 1 {
		t.Fatalf("stack: %#v", body)
	}

	writeReq(inW, 7, "scopes", map[string]any{"frameId": 1})
	_ = waitMsg(t, &out, &off, 2*time.Second)

	writeReq(inW, 8, "variables", map[string]any{"variablesReference": 1})
	_ = waitMsg(t, &out, &off, 2*time.Second)

	// Resume until terminated (may stop on breakpoint once more).
	for i := 0; i < 4; i++ {
		writeReq(inW, 30+i, "continue", map[string]any{"threadId": 1})
		term := false
		deadline = time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			m, ok := tryMsg(&out, &off)
			if !ok {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			if m["type"] == "event" && m["event"] == "terminated" {
				term = true
				break
			}
		}
		if term {
			break
		}
	}

	writeReq(inW, 99, "disconnect", map[string]any{})
	_ = inW.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("session did not exit")
	}
}

func TestDAPBreakpointHit(t *testing.T) {
	dir := t.TempDir()
	prog := filepath.Join(dir, "bp.weft")
	src := "fn main {\n    a := 10\n    b := 20\n    say(a + b)\n}\n"
	if err := os.WriteFile(prog, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	inR, inW := io.Pipe()
	var out safeBuffer
	done := make(chan error, 1)
	go func() { done <- dap.Run(inR, &out, prog) }()
	off := 0

	writeReq(inW, 1, "initialize", map[string]any{})
	_ = waitMsg(t, &out, &off, 2*time.Second)
	_ = waitMsg(t, &out, &off, 2*time.Second) // initialized event

	writeReq(inW, 2, "launch", map[string]any{"program": prog, "stopOnEntry": false})
	_ = waitMsg(t, &out, &off, 2*time.Second)

	writeReq(inW, 3, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": prog},
		"breakpoints": []map[string]any{{"line": 4}},
	})
	_ = waitMsg(t, &out, &off, 2*time.Second)

	writeReq(inW, 4, "configurationDone", nil)
	waitUntil(t, &out, &off, 3*time.Second, func(m map[string]any) bool {
		return m["type"] == "event" && m["event"] == "stopped"
	})

	writeReq(inW, 5, "variables", map[string]any{"variablesReference": 1})
	m := waitMsg(t, &out, &off, 2*time.Second)
	body, _ := m["body"].(map[string]any)
	vars, _ := body["variables"].([]any)
	t.Logf("locals: %#v", vars)

	writeReq(inW, 6, "continue", map[string]any{"threadId": 1})
	waitUntil(t, &out, &off, 3*time.Second, func(m map[string]any) bool {
		return m["type"] == "event" && m["event"] == "terminated"
	})

	writeReq(inW, 7, "disconnect", nil)
	_ = inW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("hang")
	}
}
