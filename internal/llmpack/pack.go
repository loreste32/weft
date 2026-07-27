// Package llmpack embeds materials for training/serving LLMs that write Weft.
package llmpack

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed SYSTEM.md
var SystemPrompt string

//go:embed train.jsonl
var trainJSONL string

// Example is one supervised training row.
type Example struct {
	ID          string   `json:"id"`
	Instruction string   `json:"instruction"`
	Input       string   `json:"input,omitempty"`
	Output      string   `json:"output"`
	Tags        []string `json:"tags,omitempty"`
}

// SystemCard returns the model system prompt (trimmed).
func SystemCard() string {
	return strings.TrimSpace(SystemPrompt) + "\n"
}

// FewShot returns up to n instruction/output pairs formatted for chat few-shot.
func FewShot(n int) string {
	exs := Examples()
	if n <= 0 || n > len(exs) {
		n = len(exs)
	}
	var b strings.Builder
	b.WriteString("# Few-shot Weft examples\n\n")
	for i := 0; i < n; i++ {
		e := exs[i]
		fmt.Fprintf(&b, "## %s\nUser: %s\n\nAssistant:\n```weft\n%s```\n\n", e.ID, e.Instruction, strings.TrimSpace(e.Output))
	}
	return b.String()
}

// Examples parses the embedded JSONL corpus.
func Examples() []Example {
	var out []Example
	for _, line := range strings.Split(trainJSONL, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e Example
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// TrainJSONL returns the raw corpus.
func TrainJSONL() string {
	return trainJSONL
}

// LoadExamplesFile reads private domain JSONL and converts to Examples.
// Accepts instruction rows {instruction,input,output} or chat rows {messages:[...]}.
func LoadExamplesFile(path string) ([]Example, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Example
	n := 0
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		n++
		var e Example
		if err := json.Unmarshal([]byte(line), &e); err == nil && e.Instruction != "" && e.Output != "" {
			if e.ID == "" {
				e.ID = fmt.Sprintf("private-%d", n)
			}
			out = append(out, e)
			continue
		}
		// chat-style {messages:[{role,content},...]}
		var chat struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal([]byte(line), &chat); err != nil || len(chat.Messages) == 0 {
			return nil, fmt.Errorf("%s:%d: need instruction/output or messages[] row", path, n)
		}
		var user, asst string
		for _, m := range chat.Messages {
			switch strings.ToLower(m.Role) {
			case "user", "human":
				if user == "" {
					user = m.Content
				} else {
					user += "\n" + m.Content
				}
			case "assistant", "gpt":
				asst = m.Content
			}
		}
		if user == "" || asst == "" {
			return nil, fmt.Errorf("%s:%d: chat row needs user + assistant", path, n)
		}
		out = append(out, Example{
			ID:          fmt.Sprintf("private-%d", n),
			Instruction: user,
			Output:      asst,
			Tags:        []string{"private"},
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no examples", path)
	}
	return out, nil
}

// ChatMessages builds OpenAI-style messages for supervised training export.
func ChatMessages(e Example) []map[string]string {
	user := e.Instruction
	if e.Input != "" {
		user = e.Instruction + "\n\n" + e.Input
	}
	return []map[string]string{
		{"role": "system", "content": SystemCard()},
		{"role": "user", "content": user},
		{"role": "assistant", "content": strings.TrimSpace(e.Output)},
	}
}
