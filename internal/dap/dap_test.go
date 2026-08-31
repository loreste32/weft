package dap_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// dapSession bundles a running session for the feature tests below.
type dapSession struct {
	t        *testing.T
	inW      *io.PipeWriter
	out      *safeBuffer
	off      *int
	done     chan error
	prog     string
	initBody map[string]any
}

func startSession(t *testing.T, src string) *dapSession {
	t.Helper()
	dir := t.TempDir()
	prog := filepath.Join(dir, "t.weft")
	if err := os.WriteFile(prog, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	inR, inW := io.Pipe()
	var out safeBuffer
	done := make(chan error, 1)
	go func() { done <- dap.Run(inR, &out, "") }()
	off := 0
	s := &dapSession{t: t, inW: inW, out: &out, off: &off, done: done, prog: prog}

	writeReq(inW, 1, "initialize", map[string]any{})
	m := waitMsg(t, &out, &off, 2*time.Second)
	if m["command"] != "initialize" || m["success"] != true {
		t.Fatalf("init: %#v", m)
	}
	s.initBody, _ = m["body"].(map[string]any)
	_ = waitMsg(t, &out, &off, 2*time.Second) // initialized event

	writeReq(inW, 2, "launch", map[string]any{"program": prog, "stopOnEntry": false})
	_ = waitMsg(t, &out, &off, 2*time.Second)
	return s
}

func (s *dapSession) close() {
	writeReq(s.inW, 99, "disconnect", nil)
	_ = s.inW.Close()
	select {
	case err := <-s.done:
		if err != nil {
			s.t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		s.t.Fatal("session did not exit")
	}
}

func (s *dapSession) evaluate(id int, expr string) map[string]any {
	writeReq(s.inW, id, "evaluate", map[string]any{"expression": expr, "frameId": 1})
	return waitMsg(s.t, s.out, s.off, 2*time.Second)
}

func (s *dapSession) evalResult(id int, expr string) string {
	m := s.evaluate(id, expr)
	if m["success"] != true {
		s.t.Fatalf("evaluate %q failed: %#v", expr, m)
	}
	body, _ := m["body"].(map[string]any)
	res, _ := body["result"].(string)
	return res
}

func TestDAPEvaluateExpressions(t *testing.T) {
	src := "fn main {\n" +
		"    a := 10\n" + // line 2
		"    b := 20\n" + // line 3
		"    name := \"weft\"\n" + // line 4
		"    xs := [1, 2, 3]\n" + // line 5
		"    m := {\"k\": \"v\"}\n" + // line 6
		"    say(a)\n" + // line 7
		"}\n"
	s := startSession(t, src)

	// Capabilities: setVariable + exception breakpoint filters advertised.
	if s.initBody["supportsSetVariable"] != true {
		t.Fatalf("supportsSetVariable not advertised: %#v", s.initBody)
	}
	filters, _ := s.initBody["exceptionBreakpointFilters"].([]any)
	if len(filters) != 1 {
		t.Fatalf("exceptionBreakpointFilters: %#v", s.initBody)
	}

	writeReq(s.inW, 3, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": s.prog},
		"breakpoints": []map[string]any{{"line": 7}},
	})
	_ = waitMsg(t, s.out, s.off, 2*time.Second)

	writeReq(s.inW, 4, "configurationDone", nil)
	waitUntil(t, s.out, s.off, 3*time.Second, func(m map[string]any) bool {
		return m["type"] == "event" && m["event"] == "stopped"
	})

	cases := []struct {
		expr string
		want string
	}{
		{"a", "10"},
		{"a + b * 2", "50"},
		{"(a + b) * 2", "60"},
		{"a / 4", "2"},
		{"a % 3", "1"},
		{"-a", "-10"},
		{"a < b", "true"},
		{"a == 10", "true"},
		{"a != 10", "false"},
		{"name + \"!\"", "weft!"},
		{"name", "weft"},
		{"xs[1]", "2"},
		{"xs[a - 9]", "2"},
		{"m.k", "v"},
		{"m[\"k\"]", "v"},
		{"a > 5 && b > 5", "true"},
		{"a > 50 || b > 5", "true"},
		{"!(a > b)", "true"},
		{"[a, b][1]", "20"},
		{"{\"x\": a}.x", "10"},
	}
	for i, c := range cases {
		if got := s.evalResult(10+i, c.expr); got != c.want {
			t.Errorf("evaluate %q = %q, want %q", c.expr, got, c.want)
		}
	}

	// Error cases: undefined name, function calls (unsupported), garbage.
	for i, expr := range []string{"nosuch", "say(a)", "a +"} {
		m := s.evaluate(200+i, expr)
		if m["success"] != false {
			t.Errorf("evaluate %q should fail: %#v", expr, m)
		}
	}

	writeReq(s.inW, 90, "continue", map[string]any{"threadId": 1})
	waitUntil(t, s.out, s.off, 3*time.Second, func(m map[string]any) bool {
		return m["type"] == "event" && m["event"] == "terminated"
	})
	s.close()
}

func TestDAPSetVariable(t *testing.T) {
	src := "fn main {\n" +
		"    a := 10\n" + // line 2
		"    xs := [1, 2, 3]\n" + // line 3
		"    m := {\"k\": \"v\"}\n" + // line 4
		"    say(a)\n" + // line 5
		"    say(xs[0])\n" + // line 6
		"}\n"
	s := startSession(t, src)

	writeReq(s.inW, 3, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": s.prog},
		"breakpoints": []map[string]any{{"line": 5}},
	})
	_ = waitMsg(t, s.out, s.off, 2*time.Second)

	writeReq(s.inW, 4, "configurationDone", nil)
	waitUntil(t, s.out, s.off, 3*time.Second, func(m map[string]any) bool {
		return m["type"] == "event" && m["event"] == "stopped"
	})

	setVar := func(id int, ref int, name, value string) map[string]any {
		writeReq(s.inW, id, "setVariable", map[string]any{
			"variablesReference": ref,
			"name":               name,
			"value":              value,
		})
		return waitMsg(t, s.out, s.off, 2*time.Second)
	}

	// Plain local.
	m := setVar(10, 1, "a", "42")
	if m["success"] != true {
		t.Fatalf("setVariable a: %#v", m)
	}
	body, _ := m["body"].(map[string]any)
	if body["value"] != "42" || body["type"] != "int" {
		t.Fatalf("setVariable a body: %#v", body)
	}
	if got := s.evalResult(11, "a"); got != "42" {
		t.Fatalf("a after set = %q", got)
	}

	// Value may be an expression over other locals.
	if m := setVar(12, 1, "a", "a + 8"); m["success"] != true {
		t.Fatalf("setVariable expr: %#v", m)
	}
	if got := s.evalResult(13, "a"); got != "50" {
		t.Fatalf("a after expr set = %q", got)
	}

	// Index and field elements.
	if m := setVar(14, 1, "xs[0]", "9"); m["success"] != true {
		t.Fatalf("setVariable xs[0]: %#v", m)
	}
	if got := s.evalResult(15, "xs[0]"); got != "9" {
		t.Fatalf("xs[0] after set = %q", got)
	}
	if m := setVar(16, 1, "m.k", "\"w\""); m["success"] != true {
		t.Fatalf("setVariable m.k: %#v", m)
	}
	if got := s.evalResult(17, "m.k"); got != "w" {
		t.Fatalf("m.k after set = %q", got)
	}

	// Error cases: unknown local, unknown scope, unwritable target.
	if m := setVar(18, 1, "nope", "1"); m["success"] != false {
		t.Fatalf("setVariable unknown local should fail: %#v", m)
	}
	if m := setVar(19, 99, "a", "1"); m["success"] != false {
		t.Fatalf("setVariable bad scope should fail: %#v", m)
	}
	if m := setVar(20, 1, "a + 1", "1"); m["success"] != false {
		t.Fatalf("setVariable non-assignable should fail: %#v", m)
	}

	writeReq(s.inW, 90, "continue", map[string]any{"threadId": 1})
	waitUntil(t, s.out, s.off, 3*time.Second, func(m map[string]any) bool {
		return m["type"] == "event" && m["event"] == "terminated"
	})
	s.close()
}

func TestDAPExceptionBreakpoint(t *testing.T) {
	src := "fn main {\n" +
		"    x := 1\n" + // line 2
		"    y := 0\n" + // line 3
		"    z := x / y\n" + // line 4
		"    say(z)\n" + // line 5
		"}\n"
	s := startSession(t, src)

	writeReq(s.inW, 3, "setExceptionBreakpoints", map[string]any{"filters": []string{"all"}})
	m := waitMsg(t, s.out, s.off, 2*time.Second)
	if m["command"] != "setExceptionBreakpoints" || m["success"] != true {
		t.Fatalf("setExceptionBreakpoints: %#v", m)
	}
	body, _ := m["body"].(map[string]any)
	bps, _ := body["breakpoints"].([]any)
	if len(bps) != 1 {
		t.Fatalf("setExceptionBreakpoints body: %#v", body)
	}

	writeReq(s.inW, 4, "configurationDone", nil)
	// Break-on-throw: stop with reason "exception" before the stack unwinds.
	stopped := waitUntil(t, s.out, s.off, 3*time.Second, func(m map[string]any) bool {
		if m["type"] == "event" && m["event"] == "stopped" {
			b, _ := m["body"].(map[string]any)
			return b["reason"] == "exception"
		}
		return false
	})
	sbody, _ := stopped["body"].(map[string]any)
	text, _ := sbody["text"].(string)
	if !strings.Contains(text, "division by zero") {
		t.Fatalf("exception text: %#v", sbody)
	}

	// Locals of the failing frame are inspectable at the raise site.
	if got := s.evalResult(5, "x"); got != "1" {
		t.Fatalf("x at exception = %q", got)
	}
	if got := s.evalResult(6, "y"); got != "0" {
		t.Fatalf("y at exception = %q", got)
	}

	// Continue lets the error propagate and the program terminate.
	writeReq(s.inW, 7, "continue", map[string]any{"threadId": 1})
	waitUntil(t, s.out, s.off, 3*time.Second, func(m map[string]any) bool {
		return m["type"] == "event" && m["event"] == "terminated"
	})
	s.close()
}

func TestDAPNoExceptionBreakpointByDefault(t *testing.T) {
	src := "fn main {\n" +
		"    y := 0\n" + // line 2
		"    z := 1 / y\n" + // line 3
		"}\n"
	s := startSession(t, src)

	// Without setExceptionBreakpoints the VM must never block on the error:
	// terminated arrives with no continue request. (The adapter still emits
	// a stopped/exception notification on error exit — pre-existing
	// behavior — but that is not a blocking pause.)
	writeReq(s.inW, 3, "configurationDone", nil)
	waitUntil(t, s.out, s.off, 3*time.Second, func(m map[string]any) bool {
		return m["type"] == "event" && m["event"] == "terminated"
	})
	s.close()
}
