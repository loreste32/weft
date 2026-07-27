package stdlib

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenAIToolsToAnthropic(t *testing.T) {
	in := []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name":        "weather",
				"description": "get weather",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{"city": map[string]any{"type": "string"}},
				},
			},
		},
	}
	out := openaiToolsToAnthropic(in)
	if len(out) != 1 || out[0]["name"] != "weather" {
		t.Fatalf("%v", out)
	}
	if out[0]["input_schema"] == nil {
		t.Fatal("missing input_schema")
	}
}

func TestMessagesToAnthropicToolRoundTrip(t *testing.T) {
	msgs := []map[string]any{
		{"role": "system", "content": "sys"},
		{"role": "user", "content": "weather?"},
		{
			"role":    "assistant",
			"content": "",
			"tool_calls": []map[string]any{
				{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "weather",
						"arguments": `{"city":"Paris"}`,
					},
				},
			},
		},
		{"role": "tool", "tool_call_id": "call_1", "content": "clear"},
	}
	sys, turns := messagesToAnthropic(msgs)
	if sys != "sys" {
		t.Fatalf("sys %q", sys)
	}
	if len(turns) < 3 {
		t.Fatalf("turns %v", turns)
	}
	last := turns[len(turns)-1]
	if last["role"] != "user" {
		t.Fatalf("last role %v", last["role"])
	}
	// tool_result batch
	content, ok := last["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content %T %v", last["content"], last["content"])
	}
	if content[0]["type"] != "tool_result" {
		t.Fatalf("%v", content[0])
	}
}

func TestParseAnthropicToolUse(t *testing.T) {
	raw := []byte(`{
	  "content": [
	    {"type":"text","text":"checking"},
	    {"type":"tool_use","id":"toolu_1","name":"weather","input":{"city":"Paris"}}
	  ]
	}`)
	text, calls, err := parseAnthropicResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if text != "checking" || len(calls) != 1 || calls[0].Name != "weather" {
		t.Fatalf("%q %+v", text, calls)
	}
	if !strings.Contains(calls[0].ArgsJSON, "Paris") {
		t.Fatalf("args %s", calls[0].ArgsJSON)
	}
	_ = json.Valid
}
