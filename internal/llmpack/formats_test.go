package llmpack

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

var testExamples = []Example{
	{ID: "t1", Instruction: "write hello", Output: `fn main { say("hello") }`},
	{ID: "t2", Instruction: "add", Input: "1 and 2", Output: `fn main { say(1 + 2) }`},
}

func TestWriteInstructionJSONL(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteInstructionJSONL(&buf, testExamples); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var e Example
	json.Unmarshal([]byte(lines[0]), &e)
	if e.ID != "t1" || e.Instruction != "write hello" {
		t.Fatalf("first row: %+v", e)
	}
}

func TestWriteChatJSONL(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteChatJSONL(&buf, testExamples); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var row struct {
		Messages []map[string]string `json:"messages"`
	}
	json.Unmarshal([]byte(lines[0]), &row)
	if len(row.Messages) != 3 {
		t.Fatalf("messages: %d", len(row.Messages))
	}
	if row.Messages[0]["role"] != "system" {
		t.Fatal("first should be system")
	}
}

func TestWriteAlpacaJSONL(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAlpacaJSONL(&buf, testExamples); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var row map[string]string
	json.Unmarshal([]byte(lines[0]), &row)
	if row["instruction"] != "write hello" {
		t.Fatal("instruction")
	}
	if row["system"] == "" {
		t.Fatal("missing system")
	}
}

func TestWriteShareGPTJSONL(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteShareGPTJSONL(&buf, testExamples); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var row struct {
		Conversations []map[string]string `json:"conversations"`
	}
	json.Unmarshal([]byte(lines[0]), &row)
	if len(row.Conversations) != 3 {
		t.Fatal("conversations")
	}
	if row.Conversations[0]["from"] != "system" {
		t.Fatal("system")
	}
	if row.Conversations[1]["from"] != "human" {
		t.Fatal("human")
	}
	if row.Conversations[2]["from"] != "gpt" {
		t.Fatal("gpt")
	}
}

func TestWriteShareGPTWithInput(t *testing.T) {
	var buf bytes.Buffer
	WriteShareGPTJSONL(&buf, testExamples[1:2]) // "add" has Input
	var row struct {
		Conversations []map[string]string `json:"conversations"`
	}
	json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &row)
	if !strings.Contains(row.Conversations[1]["value"], "1 and 2") {
		t.Fatal("input should be in human message")
	}
}

func TestWriteCompletionsJSONL(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCompletionsJSONL(&buf, testExamples); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var row map[string]string
	json.Unmarshal([]byte(lines[0]), &row)
	if row["prompt"] == "" || row["completion"] == "" {
		t.Fatal("missing prompt/completion")
	}
}

func TestWriteCompletionsWithInput(t *testing.T) {
	var buf bytes.Buffer
	WriteCompletionsJSONL(&buf, testExamples[1:2])
	var row map[string]string
	json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &row)
	if !strings.Contains(row["prompt"], "1 and 2") {
		t.Fatal("input in prompt")
	}
}

func TestDatasetCard(t *testing.T) {
	st := Stats{Count: 10, AvgOut: 50.5, Tags: map[string]int{"basic": 5, "agent": 3}}
	card := DatasetCard(st)
	if !strings.Contains(card, "Examples: **10**") {
		t.Fatal("count")
	}
	if !strings.Contains(card, "basic") {
		t.Fatal("tags")
	}
	if !strings.Contains(card, "apache-2.0") {
		t.Fatal("license")
	}
}
