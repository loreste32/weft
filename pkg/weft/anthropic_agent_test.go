package weft_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestAnthropicAgentToolUse(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			if body["tools"] == nil {
				t.Errorf("expected tools in body: %s", raw)
			}
			_, _ = w.Write([]byte(`{"content":[{"type":"tool_use","id":"toolu_x","name":"weather","input":{"city":"Paris"}}]}`))
			return
		}
		// second call should include tool_result in messages
		msgs, _ := body["messages"].([]any)
		foundResult := false
		for _, m := range msgs {
			mm, _ := m.(map[string]any)
			if c, ok := mm["content"].([]any); ok {
				for _, block := range c {
					b, _ := block.(map[string]any)
					if b["type"] == "tool_result" {
						foundResult = true
					}
				}
			}
		}
		if !foundResult {
			t.Errorf("expected tool_result in follow-up: %s", raw)
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"clear in Paris"}]}`))
	}))
	defer srv.Close()

	src := `
fn weather(city) { "clear in $city" }

fn main -> Result {
    r := llm.ask("weather?", [llm.tool("weather", weather)])?
    say(r)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{
		Stdout: &out,
		Stderr: &out,
		Environ: map[string]string{
			"WEFT_PROVIDER":      "anthropic",
			"ANTHROPIC_API_KEY":  "sk-ant-test",
			"ANTHROPIC_BASE_URL": srv.URL,
			"ANTHROPIC_MODEL":    "claude-test",
		},
		HTTPClient: srv.Client(),
	})
	if err := ctx.RunSource(context.Background(), "ant.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if hits.Load() < 2 {
		t.Fatalf("want 2 HTTP round-trips, got %d", hits.Load())
	}
	if !strings.Contains(out.String(), "clear in Paris") {
		t.Fatal(out.String())
	}
}
