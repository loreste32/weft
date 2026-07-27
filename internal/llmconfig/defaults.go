// Package llmconfig holds shared LLM provider defaults (no hardcoded scatter).
// Env vars always win; these are only fallbacks when unset.
package llmconfig

const (
	ProviderOpenAI    = "openai"
	ProviderOllama    = "ollama"
	ProviderVLLM      = "vllm"
	ProviderAnthropic = "anthropic"

	DefaultOllamaHost     = "http://127.0.0.1:11434"
	DefaultVLLMBase       = "http://127.0.0.1:8000/v1"
	DefaultOpenAIBase     = "https://api.openai.com/v1"
	DefaultAnthropicBase  = "https://api.anthropic.com"
	DefaultOllamaModel    = "llama3.2"
	DefaultVLLMModel      = "default"
	DefaultOpenAIModel    = "gpt-4o-mini"
	DefaultAnthropicModel = "claude-sonnet-4-20250514"
	DefaultOllamaAPIKey   = "ollama"
	DefaultVLLMAPIKey     = "EMPTY"
	AnthropicAPIVersion   = "2023-06-01"
)
