package stdlib

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func newMLTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skipf("sandbox does not allow loopback listeners: %v", err)
		}
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func TestMLInferValidationHelpers(t *testing.T) {
	for _, rawURL := range []string{"", "file:///tmp/model", "localhost:8000/predict"} {
		if err := validateInferenceURL(rawURL); err == nil {
			t.Errorf("validateInferenceURL(%q) accepted an invalid URL", rawURL)
		}
	}
	for _, rawURL := range []string{"http://127.0.0.1:8000/predict", "https://model.example/predict"} {
		if err := validateInferenceURL(rawURL); err != nil {
			t.Errorf("validateInferenceURL(%q) = %v", rawURL, err)
		}
	}

	if got := responsePreview([]byte("short")); got != "short" {
		t.Fatalf("short response preview = %q", got)
	}
	long := make([]byte, 4097)
	for i := range long {
		long[i] = 'x'
	}
	if got := responsePreview(long); len(got) != 4099 || !strings.HasSuffix(got, "…") {
		t.Fatalf("long response preview was not bounded: len=%d", len(got))
	}
}

func resultObject(t *testing.T, value runtime.Value) *runtime.ResultObj {
	t.Helper()
	if value.Kind != runtime.KindResult {
		t.Fatalf("expected Result, got %v", value.Kind)
	}
	return value.Obj.(*runtime.ResultObj)
}

func TestMLInferPreservesJSONAndRejectsBadResponses(t *testing.T) {
	var request any
	server := newMLTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	value, err := doInferenceRequest(server.URL, []any{"text", float64(2)}, map[string]string{"Content-Type": "application/json"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result := resultObject(t, value); !result.Ok {
		t.Fatalf("request returned Err: %v", result.Err)
	}
	items, ok := request.([]any)
	if !ok || len(items) != 2 || items[0] != "text" || items[1] != float64(2) {
		t.Fatalf("request = %#v, want JSON array preserving values", request)
	}

	value, err = doInferenceRequest("file:///tmp/model", map[string]any{}, nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resultObject(t, value).Ok {
		t.Fatal("non-HTTP URL should fail")
	}

	badJSON := newMLTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	value, err = doInferenceRequest(badJSON.URL, map[string]any{}, nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resultObject(t, value).Ok {
		t.Fatal("invalid JSON response should fail")
	}
}

func TestMLInferRejectsHTTPErrorsAndInvalidGetResponses(t *testing.T) {
	server := newMLTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream unavailable"}`))
	}))

	value, err := doInferenceRequest(server.URL, map[string]any{}, nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resultObject(t, value).Ok {
		t.Fatal("HTTP error should fail")
	}

	value, err = doGet(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resultObject(t, value).Ok {
		t.Fatal("GET HTTP error should fail")
	}
}
