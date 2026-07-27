package weft

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/loreste/weft/internal/llmconfig"
)

// Provider names — re-export shared config (env always overrides endpoints/models).
const (
	ProviderOpenAI    = llmconfig.ProviderOpenAI
	ProviderOllama    = llmconfig.ProviderOllama
	ProviderVLLM      = llmconfig.ProviderVLLM
	ProviderAnthropic = llmconfig.ProviderAnthropic

	DefaultOllamaHost     = llmconfig.DefaultOllamaHost
	DefaultVLLMBase       = llmconfig.DefaultVLLMBase
	DefaultOpenAIBase     = llmconfig.DefaultOpenAIBase
	DefaultAnthropicBase  = llmconfig.DefaultAnthropicBase
	DefaultOllamaModel    = llmconfig.DefaultOllamaModel
	DefaultVLLMModel      = llmconfig.DefaultVLLMModel
	DefaultOpenAIModel    = llmconfig.DefaultOpenAIModel
	DefaultAnthropicModel = llmconfig.DefaultAnthropicModel
	DefaultOllamaAPIKey   = llmconfig.DefaultOllamaAPIKey
	DefaultVLLMAPIKey     = llmconfig.DefaultVLLMAPIKey
)

// DetectProvider returns openai|ollama|vllm|anthropic from env (WEFT_PROVIDER / LLM_PROVIDER)
// or by inferring from base URL / OLLAMA_HOST / VLLM_BASE_URL / ANTHROPIC_API_KEY.
func DetectProvider() string {
	if p := firstEnv("WEFT_PROVIDER", "LLM_PROVIDER"); p != "" {
		return strings.ToLower(strings.TrimSpace(p))
	}
	if firstEnv("ANTHROPIC_API_KEY") != "" && firstEnv("OPENAI_API_KEY", "WEFT_API_KEY") == "" {
		return ProviderAnthropic
	}
	if firstEnv("OLLAMA_HOST", "OLLAMA_BASE_URL") != "" {
		return ProviderOllama
	}
	if firstEnv("VLLM_BASE_URL", "VLLM_HOST") != "" {
		return ProviderVLLM
	}
	base := firstEnv("OPENAI_BASE_URL", "WEFT_API_BASE", "LLM_BASE_URL")
	b := strings.ToLower(base)
	if urlLooksLike(b, DefaultOllamaHost, "ollama") {
		return ProviderOllama
	}
	if urlLooksLike(b, DefaultVLLMBase, "vllm") {
		// heuristic only when no OpenAI key
		if firstEnv("OPENAI_API_KEY", "WEFT_API_KEY") == "" {
			return ProviderVLLM
		}
	}
	return ProviderOpenAI
}

// urlLooksLike matches provider name token or the full default host:port.
// Port-only matching is intentionally NOT used — :8000/:11434 are common and
// must not reclassify unrelated OpenAI-compatible servers as ollama/vllm.
func urlLooksLike(baseLower, defaultURL, nameToken string) bool {
	if nameToken != "" && strings.Contains(baseLower, nameToken) {
		return true
	}
	hp := strings.ToLower(hostPortOf(defaultURL))
	if hp != "" && strings.Contains(baseLower, hp) {
		return true
	}
	return false
}

func hostPortOf(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}

// OllamaOpenAIBase returns OpenAI-compatible base for Ollama (…/v1).
func OllamaOpenAIBase() string {
	host := firstEnv("OLLAMA_HOST", "OLLAMA_BASE_URL", "WEFT_API_BASE", "LLM_BASE_URL")
	if host == "" {
		host = DefaultOllamaHost
	}
	host = strings.TrimRight(host, "/")
	// allow full …/v1 already
	if strings.HasSuffix(host, "/v1") {
		return host
	}
	// strip /api if user set OLLAMA native root
	host = strings.TrimSuffix(host, "/api")
	return host + "/v1"
}

// OllamaNativeBase returns host root for /api/* (no /v1).
func OllamaNativeBase() string {
	host := firstEnv("OLLAMA_HOST", "OLLAMA_BASE_URL")
	if host == "" {
		host = DefaultOllamaHost
	}
	host = strings.TrimRight(host, "/")
	host = strings.TrimSuffix(host, "/v1")
	host = strings.TrimSuffix(host, "/api")
	return host
}

// VLLMOpenAIBase returns OpenAI-compatible base for vLLM.
func VLLMOpenAIBase() string {
	base := firstEnv("VLLM_BASE_URL", "VLLM_HOST", "WEFT_API_BASE", "LLM_BASE_URL")
	if base == "" {
		base = DefaultVLLMBase
	}
	base = strings.TrimRight(base, "/")
	if !strings.HasSuffix(base, "/v1") {
		base = base + "/v1"
	}
	return base
}

// NewLLMClientFromEnv builds a client for OpenAI, Ollama, vLLM, or Anthropic.
// Local providers do not require a real API key (uses "ollama" / "EMPTY" placeholders).
func NewLLMClientFromEnv() (*LLMClient, error) {
	provider := DetectProvider()
	var base, key, model string

	switch provider {
	case ProviderOllama:
		base = OllamaOpenAIBase()
		key = firstEnv("OLLAMA_API_KEY", "OPENAI_API_KEY", "WEFT_API_KEY", "LLM_API_KEY")
		if key == "" {
			key = DefaultOllamaAPIKey
		}
		model = firstEnv("OLLAMA_MODEL", "WEFT_MODEL", "LLM_MODEL", "OPENAI_MODEL")
		if model == "" {
			model = DefaultOllamaModel
		}
	case ProviderVLLM:
		base = VLLMOpenAIBase()
		key = firstEnv("VLLM_API_KEY", "OPENAI_API_KEY", "WEFT_API_KEY", "LLM_API_KEY")
		if key == "" {
			key = DefaultVLLMAPIKey
		}
		model = firstEnv("VLLM_MODEL", "WEFT_MODEL", "LLM_MODEL", "OPENAI_MODEL")
		if model == "" {
			model = DefaultVLLMModel
		}
	case ProviderAnthropic:
		key = firstEnv("ANTHROPIC_API_KEY", "WEFT_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("set ANTHROPIC_API_KEY (or WEFT_PROVIDER=openai|ollama|vllm)")
		}
		base = firstEnv("ANTHROPIC_BASE_URL", "WEFT_API_BASE")
		if base == "" {
			base = DefaultAnthropicBase
		}
		model = firstEnv("ANTHROPIC_MODEL", "WEFT_MODEL", "LLM_MODEL")
		if model == "" {
			model = DefaultAnthropicModel
		}
	default:
		key = firstEnv("OPENAI_API_KEY", "WEFT_API_KEY", "LLM_API_KEY")
		if key == "" {
			return nil, fmt.Errorf(`set OPENAI_API_KEY or ANTHROPIC_API_KEY, or use a local provider:

  # Ollama
  export WEFT_PROVIDER=ollama
  export OLLAMA_HOST=%s
  export OLLAMA_MODEL=%s

  # vLLM
  export WEFT_PROVIDER=vllm
  export VLLM_BASE_URL=%s
  export VLLM_MODEL=<served-model-id>

  # Anthropic
  export WEFT_PROVIDER=anthropic
  export ANTHROPIC_API_KEY=sk-ant-…
`, DefaultOllamaHost, DefaultOllamaModel, DefaultVLLMBase)
		}
		base = firstEnv("OPENAI_BASE_URL", "WEFT_API_BASE", "LLM_BASE_URL")
		if base == "" {
			base = DefaultOpenAIBase
		}
		model = firstEnv("WEFT_MODEL", "OPENAI_MODEL", "LLM_MODEL")
		if model == "" {
			model = DefaultOpenAIModel
		}
	}

	return &LLMClient{
		BaseURL:  strings.TrimRight(base, "/"),
		APIKey:   key,
		Model:    model,
		Provider: provider,
		HTTP:     &http.Client{Timeout: 300 * time.Second}, // local gen can be slow
	}, nil
}

// ListOpenAIModels GETs /models (works for Ollama /v1 and vLLM).
func ListOpenAIModels(baseURL, apiKey string) ([]string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	req, err := http.NewRequest("GET", baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("models HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Data))
	for _, d := range out.Data {
		if d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}

// OllamaListTags uses native GET /api/tags.
func OllamaListTags(host string) ([]string, error) {
	if host == "" {
		host = OllamaNativeBase()
	}
	host = strings.TrimRight(host, "/")
	host = strings.TrimSuffix(host, "/v1")
	raw, err := ollamaGET(host + "/api/tags")
	if err != nil {
		return nil, err
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Models))
	for _, m := range out.Models {
		if m.Name != "" {
			ids = append(ids, m.Name)
		}
	}
	return ids, nil
}

// OllamaRunningModel is one entry from GET /api/ps.
type OllamaRunningModel struct {
	Name  string
	Size  int64
	VRAMb int64
}

// OllamaPS lists models currently loaded in memory (native /api/ps).
func OllamaPS(host string) ([]OllamaRunningModel, error) {
	if host == "" {
		host = OllamaNativeBase()
	}
	host = strings.TrimRight(host, "/")
	host = strings.TrimSuffix(host, "/v1")
	raw, err := ollamaGET(host + "/api/ps")
	if err != nil {
		return nil, err
	}
	var out struct {
		Models []struct {
			Name  string `json:"name"`
			Size  int64  `json:"size"`
			SizeV int64  `json:"size_vram"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	list := make([]OllamaRunningModel, 0, len(out.Models))
	for _, m := range out.Models {
		list = append(list, OllamaRunningModel{Name: m.Name, Size: m.Size, VRAMb: m.SizeV})
	}
	return list, nil
}

func ollamaGET(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return raw, nil
}

// PingURL returns nil if GET url responds < 500.
func PingURL(url string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// ProviderStatus is one line for doctor.
func ProviderStatus() (name, detail string, ok bool) {
	p := DetectProvider()
	switch p {
	case ProviderOllama:
		host := OllamaNativeBase()
		err := PingURL(host)
		if err != nil {
			return "ollama", host + " — unreachable (" + err.Error() + ")", false
		}
		models, err := OllamaListTags(host)
		if err != nil {
			return "ollama", host + " — up, tags: " + err.Error(), true
		}
		return "ollama", fmt.Sprintf("%s — %d model(s)", host, len(models)), true
	case ProviderVLLM:
		base := VLLMOpenAIBase()
		err := PingURL(base + "/models")
		if err != nil {
			// try health
			root := strings.TrimSuffix(base, "/v1")
			if err2 := PingURL(root + "/health"); err2 != nil {
				return "vllm", base + " — unreachable", false
			}
			return "vllm", base + " — health ok", true
		}
		return "vllm", base + " — reachable", true
	case ProviderAnthropic:
		if firstEnv("ANTHROPIC_API_KEY", "WEFT_API_KEY") != "" {
			return "anthropic", "API key set", true
		}
		return "anthropic", "no ANTHROPIC_API_KEY", false
	default:
		if os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("WEFT_API_KEY") != "" {
			return "openai", "API key set", true
		}
		return "openai", "no API key (set WEFT_PROVIDER=ollama|vllm|anthropic)", false
	}
}
