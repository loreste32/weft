package llmpack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamples(t *testing.T) {
	exs := Examples()
	if len(exs) < 10 {
		t.Fatalf("expected training rows, got %d", len(exs))
	}
	for _, e := range exs {
		if e.ID == "" {
			t.Fatal("example missing ID")
		}
		if e.Instruction == "" {
			t.Fatalf("%s: missing instruction", e.ID)
		}
		if e.Output == "" {
			t.Fatalf("%s: missing output", e.ID)
		}
	}
}

func TestTrainJSONL(t *testing.T) {
	raw := TrainJSONL()
	if raw == "" {
		t.Fatal("empty train JSONL")
	}
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) < 10 {
		t.Fatalf("too few lines: %d", len(lines))
	}
}

func TestFewShot(t *testing.T) {
	// default (all)
	all := FewShot(0)
	if !strings.Contains(all, "Few-shot") {
		t.Fatal("missing header")
	}
	// limited
	limited := FewShot(2)
	if !strings.Contains(limited, "Few-shot") {
		t.Fatal("missing header")
	}
	// negative
	neg := FewShot(-1)
	if neg == "" {
		t.Fatal("negative should return all")
	}
	// too large
	big := FewShot(99999)
	if big == "" {
		t.Fatal("large n should clamp")
	}
}

func TestChatMessages(t *testing.T) {
	e := Example{
		ID:          "test-1",
		Instruction: "write hello",
		Output:      `fn main { say("hello") }`,
	}
	msgs := ChatMessages(e)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0]["role"] != "system" {
		t.Fatal("first should be system")
	}
	if msgs[1]["role"] != "user" {
		t.Fatal("second should be user")
	}
	if msgs[1]["content"] != "write hello" {
		t.Fatalf("user content = %q", msgs[1]["content"])
	}
	if msgs[2]["role"] != "assistant" {
		t.Fatal("third should be assistant")
	}
}

func TestChatMessagesWithInput(t *testing.T) {
	e := Example{
		ID:          "test-2",
		Instruction: "process this",
		Input:       "data here",
		Output:      `fn main { say("done") }`,
	}
	msgs := ChatMessages(e)
	if !strings.Contains(msgs[1]["content"], "data here") {
		t.Fatal("input should be appended to user message")
	}
}

func TestLoadExamplesFileInstruction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "train.jsonl")
	rows := []string{
		`{"instruction": "write hello", "output": "fn main { say(\"hello\") }"}`,
		`{"instruction": "add nums", "output": "fn main { say(1 + 2) }"}`,
	}
	os.WriteFile(path, []byte(strings.Join(rows, "\n")), 0644)

	exs, err := LoadExamplesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(exs) != 2 {
		t.Fatalf("expected 2 examples, got %d", len(exs))
	}
	if exs[0].ID != "private-1" {
		t.Fatalf("auto ID = %q", exs[0].ID)
	}
}

func TestLoadExamplesFileWithID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "train.jsonl")
	os.WriteFile(path, []byte(`{"id": "custom", "instruction": "hi", "output": "fn main { say(1) }"}`+"\n"), 0644)

	exs, err := LoadExamplesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if exs[0].ID != "custom" {
		t.Fatalf("should preserve ID, got %q", exs[0].ID)
	}
}

func TestLoadExamplesFileChat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat.jsonl")
	row := `{"messages": [{"role": "user", "content": "write hello"}, {"role": "assistant", "content": "fn main { say(\"hello\") }"}]}`
	os.WriteFile(path, []byte(row+"\n"), 0644)

	exs, err := LoadExamplesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(exs) != 1 {
		t.Fatalf("expected 1, got %d", len(exs))
	}
	if exs[0].Instruction != "write hello" {
		t.Fatal("instruction from chat")
	}
	if !strings.Contains(exs[0].Output, "hello") {
		t.Fatal("output from chat")
	}
}

func TestLoadExamplesFileChatMultiUser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat.jsonl")
	row := `{"messages": [{"role": "user", "content": "first"}, {"role": "user", "content": "second"}, {"role": "assistant", "content": "fn main {}"}]}`
	os.WriteFile(path, []byte(row+"\n"), 0644)

	exs, err := LoadExamplesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exs[0].Instruction, "first") || !strings.Contains(exs[0].Instruction, "second") {
		t.Fatal("multi user messages should concatenate")
	}
}

func TestLoadExamplesFileChatMissingAssistant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat.jsonl")
	row := `{"messages": [{"role": "user", "content": "hello"}]}`
	os.WriteFile(path, []byte(row+"\n"), 0644)

	_, err := LoadExamplesFile(path)
	if err == nil {
		t.Fatal("missing assistant should error")
	}
}

func TestLoadExamplesFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(path, []byte(""), 0644)

	_, err := LoadExamplesFile(path)
	if err == nil {
		t.Fatal("empty file should error")
	}
}

func TestLoadExamplesFileBadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	os.WriteFile(path, []byte("not json\n"), 0644)

	_, err := LoadExamplesFile(path)
	if err == nil {
		t.Fatal("bad JSON should error")
	}
}

func TestLoadExamplesFileNotFound(t *testing.T) {
	_, err := LoadExamplesFile("/nonexistent/file.jsonl")
	if err == nil {
		t.Fatal("missing file should error")
	}
}

func TestLoadExamplesFileChatEmptyMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat.jsonl")
	os.WriteFile(path, []byte(`{"messages": []}`+"\n"), 0644)

	_, err := LoadExamplesFile(path)
	if err == nil {
		t.Fatal("empty messages should error")
	}
}
