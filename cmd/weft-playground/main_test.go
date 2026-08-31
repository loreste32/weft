package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func doRun(t *testing.T, method, body string) (*httptest.ResponseRecorder, runResponse) {
	t.Helper()
	r := httptest.NewRequest(method, "/api/run", strings.NewReader(body))
	w := httptest.NewRecorder()
	handleRun(w, r)
	var resp runResponse
	if method != "OPTIONS" {
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response not JSON: %q", w.Body.String())
		}
	}
	return w, resp
}

func TestHandleRunOptions(t *testing.T) {
	r := httptest.NewRequest("OPTIONS", "/api/run", nil)
	w := httptest.NewRecorder()
	handleRun(w, r)
	if w.Code != 204 {
		t.Fatalf("OPTIONS status = %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://weftproject.dev" {
		t.Fatalf("CORS origin = %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Fatalf("CORS methods = %q", got)
	}
}

func TestHandleRunMethodNotAllowed(t *testing.T) {
	_, resp := doRun(t, "GET", "")
	if resp.Error != "method not allowed" {
		t.Fatalf("error = %q", resp.Error)
	}
}

func TestHandleRunBadJSON(t *testing.T) {
	_, resp := doRun(t, "POST", "{not json")
	if resp.Error != "bad request" {
		t.Fatalf("error = %q", resp.Error)
	}
}

func TestHandleRunCodeTooLarge(t *testing.T) {
	body, _ := json.Marshal(runRequest{Code: strings.Repeat(" ", 10001)})
	_, resp := doRun(t, "POST", string(body))
	if resp.Error != "code too large (max 10KB)" {
		t.Fatalf("error = %q", resp.Error)
	}
}

func TestHandleRunSuccess(t *testing.T) {
	body, _ := json.Marshal(runRequest{Code: `fn main { say("hello") }`, Timeout: 5})
	w, resp := doRun(t, "POST", string(body))
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type = %q", ct)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
	if !strings.Contains(resp.Output, "hello") {
		t.Fatalf("output = %q, want it to contain %q", resp.Output, "hello")
	}
}

func TestHandleRunError(t *testing.T) {
	body, _ := json.Marshal(runRequest{Code: `fn main { 1 / 0 }`, Timeout: 5})
	_, resp := doRun(t, "POST", string(body))
	if resp.Error == "" {
		t.Fatal("expected runtime error, got none")
	}
}

func TestHandleRunTimeoutClamped(t *testing.T) {
	// Out-of-range timeout values fall back to the default; the run still succeeds.
	for _, timeout := range []int{0, -3, 999} {
		body, _ := json.Marshal(runRequest{Code: `fn main { say("ok") }`, Timeout: timeout})
		_, resp := doRun(t, "POST", string(body))
		if resp.Error != "" || !strings.Contains(resp.Output, "ok") {
			t.Fatalf("timeout %d: output=%q error=%q", timeout, resp.Output, resp.Error)
		}
	}
}

// Sanity: a code payload at the limit is accepted.
func TestHandleRunCodeAtLimit(t *testing.T) {
	code := `fn main { say("x") } //` + strings.Repeat("x", 10000)
	code = code[:10000]
	body, _ := json.Marshal(runRequest{Code: code, Timeout: 5})
	var buf bytes.Buffer
	buf.Write(body)
	_, resp := doRun(t, "POST", buf.String())
	if resp.Error != "" && strings.Contains(resp.Error, "code too large") {
		t.Fatalf("10KB code should be accepted: %q", resp.Error)
	}
}
