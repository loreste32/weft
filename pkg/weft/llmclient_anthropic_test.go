package weft

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatAnthropic(t *testing.T) {
	var gotPath string
	var gotKey, gotVer string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-api-key")
		gotVer = r.Header.Get("anthropic-version")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"fn main { say(\"hi\") }"}]}`))
	}))
	defer srv.Close()

	c := &LLMClient{
		BaseURL:  srv.URL,
		APIKey:   "sk-ant-x",
		Model:    "claude-test",
		Provider: ProviderAnthropic,
		HTTP:     srv.Client(),
	}
	text, err := c.Chat([]ChatMessage{
		{Role: "system", Content: "You write Weft."},
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "fn main") {
		t.Fatalf("text %q", text)
	}
	if gotKey != "sk-ant-x" || gotVer == "" {
		t.Fatalf("headers key=%q ver=%q", gotKey, gotVer)
	}
	if !strings.Contains(gotPath, "messages") {
		t.Fatalf("path %q", gotPath)
	}
	if gotBody["system"] != "You write Weft." {
		t.Fatalf("system %v", gotBody["system"])
	}
}

func TestParseAnthropicMessages(t *testing.T) {
	s, err := parseAnthropicMessages([]byte(`{"content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}`))
	if err != nil || s != "ab" {
		t.Fatalf("%q %v", s, err)
	}
}
