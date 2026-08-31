package llmconfig

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestConstants(t *testing.T) {
	// Verify constants exist and are non-empty
	consts := []string{
		ProviderOpenAI, ProviderOllama, ProviderVLLM, ProviderAnthropic,
		DefaultOllamaHost, DefaultVLLMBase, DefaultOpenAIBase, DefaultAnthropicBase,
		DefaultOllamaModel, DefaultVLLMModel, DefaultOpenAIModel, DefaultAnthropicModel,
		DefaultOllamaAPIKey, DefaultVLLMAPIKey, AnthropicAPIVersion,
	}
	for _, c := range consts {
		if c == "" {
			t.Fatal("empty constant")
		}
	}
}

func TestProviderNamesDistinct(t *testing.T) {
	providers := []string{ProviderOpenAI, ProviderOllama, ProviderVLLM, ProviderAnthropic}
	seen := map[string]bool{}
	for _, p := range providers {
		if seen[p] {
			t.Fatalf("duplicate provider name %q", p)
		}
		seen[p] = true
	}
}

func TestDefaultBasesAreURLs(t *testing.T) {
	bases := map[string]string{
		"ollama":    DefaultOllamaHost,
		"vllm":      DefaultVLLMBase,
		"openai":    DefaultOpenAIBase,
		"anthropic": DefaultAnthropicBase,
	}
	for name, base := range bases {
		u, err := url.Parse(base)
		if err != nil {
			t.Errorf("%s base %q does not parse: %v", name, base, err)
			continue
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			t.Errorf("%s base %q has scheme %q", name, base, u.Scheme)
		}
		if u.Host == "" {
			t.Errorf("%s base %q has no host", name, base)
		}
	}
}

func TestLocalDefaultsAreLoopback(t *testing.T) {
	// Self-hosted providers must not default to a remote address.
	for _, base := range []string{DefaultOllamaHost, DefaultVLLMBase} {
		if !strings.Contains(base, "127.0.0.1") && !strings.Contains(base, "localhost") {
			t.Errorf("local provider base %q is not loopback", base)
		}
	}
}

func TestAnthropicAPIVersionFormat(t *testing.T) {
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(AnthropicAPIVersion) {
		t.Fatalf("AnthropicAPIVersion %q is not a YYYY-MM-DD date", AnthropicAPIVersion)
	}
}
