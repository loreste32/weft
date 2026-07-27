package llmpack

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// WriteInstructionJSONL: {id, instruction, input, output, tags}
func WriteInstructionJSONL(w io.Writer, exs []Example) error {
	enc := json.NewEncoder(w)
	for _, e := range exs {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// WriteChatJSONL: OpenAI fine-tune style {messages:[...]}
func WriteChatJSONL(w io.Writer, exs []Example) error {
	enc := json.NewEncoder(w)
	for _, e := range exs {
		row := map[string]any{"messages": ChatMessages(e)}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

// WriteAlpacaJSONL: {instruction, input, output, system}
func WriteAlpacaJSONL(w io.Writer, exs []Example) error {
	enc := json.NewEncoder(w)
	sys := SystemCard()
	for _, e := range exs {
		row := map[string]string{
			"system":      sys,
			"instruction": e.Instruction,
			"input":       e.Input,
			"output":      strings.TrimSpace(e.Output),
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

// WriteShareGPTJSONL: {conversations:[{from,value},...]}
func WriteShareGPTJSONL(w io.Writer, exs []Example) error {
	enc := json.NewEncoder(w)
	sys := SystemCard()
	for _, e := range exs {
		user := e.Instruction
		if e.Input != "" {
			user += "\n\n" + e.Input
		}
		row := map[string]any{
			"conversations": []map[string]string{
				{"from": "system", "value": sys},
				{"from": "human", "value": user},
				{"from": "gpt", "value": strings.TrimSpace(e.Output)},
			},
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

// WriteCompletionsJSONL: prompt/completion for older APIs
func WriteCompletionsJSONL(w io.Writer, exs []Example) error {
	enc := json.NewEncoder(w)
	sys := SystemCard()
	for _, e := range exs {
		user := e.Instruction
		if e.Input != "" {
			user += "\n\n" + e.Input
		}
		prompt := sys + "\n\n### User\n" + user + "\n\n### Weft\n"
		row := map[string]string{
			"prompt":     prompt,
			"completion": strings.TrimSpace(e.Output) + "\n",
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

// DatasetCard markdown for Hugging Face / internal docs.
func DatasetCard(st Stats) string {
	var tags []string
	for t, n := range st.Tags {
		tags = append(tags, fmt.Sprintf("- `%s`: %d", t, n))
	}
	return fmt.Sprintf(`---
license: apache-2.0
language:
  - en
  - weft
task_categories:
  - text-generation
  - code-generation
pretty_name: Weft SFT
---

# Weft supervised fine-tuning dataset

Teach models to write **Weft** (LLM-first scripting), not Python.

## Stats

- Examples: **%d**
- Avg output size: **%.0f** chars
- Tags:
%s

## Formats in this folder

| File | Use with |
|------|----------|
| `+"`sft.jsonl`"+` | generic instruction tuning |
| `+"`chat.jsonl`"+` | OpenAI / chat template fine-tunes |
| `+"`alpaca.jsonl`"+` | Alpaca / many open trainers |
| `+"`sharegpt.jsonl`"+` | Axolotl ShareGPT |
| `+"`completions.jsonl`"+` | prompt/completion APIs |
| `+"`SYSTEM.md`"+` | system prompt (always use) |

## How to train (easy)

`+"```bash"+`
# 1. Produce this folder
weft train prepare -o weft-sft

# 2a. OpenAI-compatible fine-tune (chat.jsonl)
# 2b. Hugging Face TRL / Axolotl — point at sharegpt.jsonl or alpaca.jsonl

# Always keep SYSTEM.md as the system message.
`+"```"+`

## License

Apache-2.0 (same as Weft).
`, st.Count, st.AvgOut, strings.Join(tags, "\n"))
}
