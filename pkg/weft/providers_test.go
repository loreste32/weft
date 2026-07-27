package weft

import (
	"os"
	"strings"
	"testing"
)

func clearProviderEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"WEFT_PROVIDER", "LLM_PROVIDER",
		"OLLAMA_HOST", "OLLAMA_BASE_URL", "OLLAMA_MODEL", "OLLAMA_API_KEY",
		"VLLM_BASE_URL", "VLLM_HOST", "VLLM_MODEL", "VLLM_API_KEY",
		"OPENAI_BASE_URL", "WEFT_API_BASE", "LLM_BASE_URL",
		"OPENAI_API_KEY", "WEFT_API_KEY", "LLM_API_KEY",
		"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL",
		"WEFT_MODEL", "OPENAI_MODEL", "LLM_MODEL",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

func TestDetectProvider_Explicit(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("WEFT_PROVIDER", "ollama")
	if got := DetectProvider(); got != ProviderOllama {
		t.Fatalf("got %q", got)
	}
	t.Setenv("WEFT_PROVIDER", "vllm")
	if got := DetectProvider(); got != ProviderVLLM {
		t.Fatalf("got %q", got)
	}
	t.Setenv("WEFT_PROVIDER", "openai")
	if got := DetectProvider(); got != ProviderOpenAI {
		t.Fatalf("got %q", got)
	}
	t.Setenv("WEFT_PROVIDER", "anthropic")
	if got := DetectProvider(); got != ProviderAnthropic {
		t.Fatalf("got %q", got)
	}
}

func TestDetectProvider_AnthropicKey(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	if got := DetectProvider(); got != ProviderAnthropic {
		t.Fatalf("got %q want anthropic", got)
	}
	// OpenAI key wins when both set (explicit cloud default)
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	if got := DetectProvider(); got != ProviderOpenAI {
		t.Fatalf("got %q want openai when both keys set", got)
	}
}

func TestDetectProvider_InferOllamaHost(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:11434")
	if got := DetectProvider(); got != ProviderOllama {
		t.Fatalf("got %q", got)
	}
}

func TestDetectProvider_InferVLLM(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("VLLM_BASE_URL", "http://127.0.0.1:8000/v1")
	if got := DetectProvider(); got != ProviderVLLM {
		t.Fatalf("got %q", got)
	}
}

func TestDetectProvider_InferFromBaseURL(t *testing.T) {
	clearProviderEnv(t)
	// Full default host:port (port-only must NOT reclassify random :11434 hosts)
	t.Setenv("OPENAI_BASE_URL", DefaultOllamaHost+"/v1")
	if got := DetectProvider(); got != ProviderOllama {
		t.Fatalf("got %q want ollama", got)
	}
	clearProviderEnv(t)
	t.Setenv("OPENAI_BASE_URL", "http://localhost:11434/v1") // different host
	if got := DetectProvider(); got != ProviderOpenAI {
		t.Fatalf("port-only must not force ollama, got %q", got)
	}
	clearProviderEnv(t)
	t.Setenv("OPENAI_BASE_URL", "http://my-api:8000/v1")
	if got := DetectProvider(); got != ProviderOpenAI {
		t.Fatalf("port-only must not force vllm, got %q", got)
	}
}

func TestOllamaOpenAIBase(t *testing.T) {
	clearProviderEnv(t)
	if got := OllamaOpenAIBase(); got != "http://127.0.0.1:11434/v1" {
		t.Fatalf("default base %q", got)
	}
	t.Setenv("OLLAMA_HOST", "http://10.0.0.5:11434")
	if got := OllamaOpenAIBase(); got != "http://10.0.0.5:11434/v1" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("OLLAMA_HOST", "http://10.0.0.5:11434/v1")
	if got := OllamaOpenAIBase(); got != "http://10.0.0.5:11434/v1" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("OLLAMA_HOST", "http://10.0.0.5:11434/api")
	if got := OllamaOpenAIBase(); got != "http://10.0.0.5:11434/v1" {
		t.Fatalf("got %q", got)
	}
}

func TestOllamaNativeBase(t *testing.T) {
	clearProviderEnv(t)
	if got := OllamaNativeBase(); got != "http://127.0.0.1:11434" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("OLLAMA_HOST", "http://h:11434/v1")
	if got := OllamaNativeBase(); got != "http://h:11434" {
		t.Fatalf("got %q", got)
	}
}

func TestVLLMOpenAIBase(t *testing.T) {
	clearProviderEnv(t)
	if got := VLLMOpenAIBase(); got != "http://127.0.0.1:8000/v1" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("VLLM_BASE_URL", "http://gpu:8000")
	if got := VLLMOpenAIBase(); got != "http://gpu:8000/v1" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("VLLM_BASE_URL", "http://gpu:8000/v1")
	if got := VLLMOpenAIBase(); got != "http://gpu:8000/v1" {
		t.Fatalf("got %q", got)
	}
}

func TestNewLLMClientFromEnv_Ollama(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("WEFT_PROVIDER", "ollama")
	t.Setenv("OLLAMA_MODEL", "qwen2.5:7b")
	c, err := NewLLMClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if c.Provider != ProviderOllama {
		t.Fatalf("provider %q", c.Provider)
	}
	if c.Model != "qwen2.5:7b" {
		t.Fatalf("model %q", c.Model)
	}
	if !strings.HasSuffix(c.BaseURL, "/v1") {
		t.Fatalf("base %q", c.BaseURL)
	}
	if c.APIKey != "ollama" {
		t.Fatalf("key %q", c.APIKey)
	}
}

func TestNewLLMClientFromEnv_VLLM(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("WEFT_PROVIDER", "vllm")
	t.Setenv("VLLM_MODEL", "meta-llama/Meta-Llama-3-8B-Instruct")
	c, err := NewLLMClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if c.Provider != ProviderVLLM {
		t.Fatalf("provider %q", c.Provider)
	}
	if c.Model != "meta-llama/Meta-Llama-3-8B-Instruct" {
		t.Fatalf("model %q", c.Model)
	}
	if c.APIKey != "EMPTY" {
		t.Fatalf("key %q", c.APIKey)
	}
}

func TestNewLLMClientFromEnv_OpenAIRequiresKey(t *testing.T) {
	clearProviderEnv(t)
	_, err := NewLLMClientFromEnv()
	if err == nil {
		t.Fatal("expected error without API key")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("error should mention OPENAI_API_KEY: %v", err)
	}
	t.Setenv("OPENAI_API_KEY", "sk-test")
	c, err := NewLLMClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if c.Provider != ProviderOpenAI {
		t.Fatalf("provider %q", c.Provider)
	}
}
