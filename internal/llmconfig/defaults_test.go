package llmconfig

import "testing"

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
