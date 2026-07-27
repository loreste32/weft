package weft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/loreste/weft/internal/llmconfig"
)

// LLMClient is a pure-Go chat client (OpenAI-compat, Ollama, vLLM, Anthropic).
type LLMClient struct {
	BaseURL  string
	APIKey   string
	Model    string
	Provider string // openai | ollama | vllm | anthropic
	HTTP     *http.Client
}

// NewLLMClientFromEnv is defined in providers.go (OpenAI / Ollama / vLLM / Anthropic).

// ChatMessage is one chat turn.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chat sends a chat completion and returns assistant text.
func (c *LLMClient) Chat(messages []ChatMessage) (string, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 120 * time.Second}
	}
	if strings.EqualFold(c.Provider, ProviderAnthropic) || strings.Contains(strings.ToLower(c.BaseURL), "anthropic") {
		return c.chatAnthropic(messages)
	}
	return c.chatOpenAICompat(messages)
}

func (c *LLMClient) chatOpenAICompat(messages []ChatMessage) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":       c.Model,
		"messages":    messages,
		"temperature": 0.2,
	})
	req, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("chat HTTP %d: %s", resp.StatusCode, truncate(string(raw), 400))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("%s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("empty chat response")
	}
	return out.Choices[0].Message.Content, nil
}

// Anthropic Messages API (not OpenAI-compat).
func (c *LLMClient) chatAnthropic(messages []ChatMessage) (string, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	// base may be https://api.anthropic.com or …/v1
	url := base
	if strings.HasSuffix(url, "/v1") {
		url = url + "/messages"
	} else if strings.Contains(url, "/messages") {
		// already
	} else {
		url = url + "/v1/messages"
	}

	sys, turns := splitSystem(messages)
	body := map[string]any{
		"model":      c.Model,
		"max_tokens": 4096,
		"messages":   turns,
	}
	if sys != "" {
		body["system"] = sys
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", llmconfig.AnthropicAPIVersion)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rawResp, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, truncate(string(rawResp), 400))
	}
	return parseAnthropicMessages(rawResp)
}

func splitSystem(messages []ChatMessage) (system string, turns []map[string]string) {
	for _, m := range messages {
		role := strings.ToLower(m.Role)
		if role == "system" {
			if system != "" {
				system += "\n"
			}
			system += m.Content
			continue
		}
		// Anthropic only allows user|assistant in messages
		if role != "user" && role != "assistant" {
			role = "user"
		}
		turns = append(turns, map[string]string{"role": role, "content": m.Content})
	}
	if len(turns) == 0 {
		turns = []map[string]string{{"role": "user", "content": "hello"}}
	}
	return system, turns
}

func parseAnthropicMessages(raw []byte) (string, error) {
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("%s", out.Error.Message)
	}
	var b strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" || c.Type == "" {
			b.WriteString(c.Text)
		}
	}
	s := strings.TrimSpace(b.String())
	if s == "" {
		return "", fmt.Errorf("empty anthropic response")
	}
	return s, nil
}

// ExtractWeftCode pulls a Weft program out of a model reply (fenced or raw).
func ExtractWeftCode(reply string) string {
	s := strings.TrimSpace(reply)
	// ```weft ... ``` or ```loom ... ``` or ``` ... ```
	for _, lang := range []string{"weft", "loom", ""} {
		open := "```" + lang
		if i := strings.Index(s, open); i >= 0 {
			rest := s[i+len(open):]
			rest = strings.TrimPrefix(rest, "\n")
			if j := strings.Index(rest, "```"); j >= 0 {
				return strings.TrimSpace(rest[:j])
			}
		}
	}
	// bare script starting with use/fn/import
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "fn ") || strings.HasPrefix(t, "use ") || strings.HasPrefix(t, "import ") || strings.HasPrefix(t, "pub ") {
			return strings.TrimSpace(s)
		}
	}
	return s
}

// mustEnv for gen when testing without key — allow empty only for dry paths
func mustWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
