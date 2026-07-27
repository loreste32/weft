package stdlib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/loreste/weft/internal/llmconfig"
	"github.com/loreste/weft/internal/runtime"
)

// packageVLLM — first-class vLLM OpenAI-compatible server integration.
//
//	vllm.chat("Explain transformers")
//	vllm.chat({"model": "meta-llama/…", "prompt": "hi"})
//	vllm.list()?
//	vllm.health()?
//	c := vllm.connect("http://127.0.0.1:8000/v1")?
func packageVLLM(env *runtime.Env) runtime.Value {
	p := pkg()

	set(p, "base", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str(vllmOpenAIBase(env)), nil
	}, 0)

	// vllm.health() -> Result[bool]
	set(p, "health", func(args []runtime.Value) (runtime.Value, error) {
		base := vllmOpenAIBase(env)
		if len(args) >= 1 && args[0].String() != "" {
			base = normalizeVLLMBase(args[0].String())
		}
		root := strings.TrimSuffix(base, "/v1")
		// try /health then /v1/models
		if err := vllmPing(env, root+"/health"); err == nil {
			return runtime.Ok(runtime.Bool(true)), nil
		}
		if err := vllmPing(env, base+"/models"); err != nil {
			return errRes(err.Error(), "vllm"), nil
		}
		return runtime.Ok(runtime.Bool(true)), nil
	}, 1)

	// vllm.list() -> Result[[str]]
	set(p, "list", func(args []runtime.Value) (runtime.Value, error) {
		base := vllmOpenAIBase(env)
		if len(args) >= 1 && args[0].String() != "" {
			base = normalizeVLLMBase(args[0].String())
		}
		names, err := vllmListModels(env, base)
		if err != nil {
			return errRes(err.Error(), "vllm"), nil
		}
		items := make([]runtime.Value, len(names))
		for i, n := range names {
			items[i] = runtime.Str(n)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// vllm.chat(prompt|opts) -> Result[str]
	set(p, "chat", func(args []runtime.Value) (runtime.Value, error) {
		opts, prompt, err := vllmChatArgs(env, args)
		if err != nil {
			return errRes(err.Error(), "vllm"), nil
		}
		msgs := []map[string]any{{"role": "user", "content": prompt}}
		if sys := mapGetStr(opts, "system", ""); sys != "" {
			msgs = append([]map[string]any{{"role": "system", "content": sys}}, msgs...)
		}
		text, _, e := chatCompletions(env, opts, msgs, nil)
		if e != nil {
			return errRes(e.Error(), "vllm"), nil
		}
		return runtime.Ok(runtime.Str(text)), nil
	}, -1)

	// vllm.connect(base?) -> client
	set(p, "connect", func(args []runtime.Value) (runtime.Value, error) {
		base := vllmOpenAIBase(env)
		if len(args) >= 1 && args[0].String() != "" {
			base = normalizeVLLMBase(args[0].String())
		}
		if _, err := vllmListModels(env, base); err != nil {
			// health-only servers
			root := strings.TrimSuffix(base, "/v1")
			if err2 := vllmPing(env, root+"/health"); err2 != nil {
				return errRes(err.Error(), "vllm"), nil
			}
		}
		return runtime.Ok(wrapVLLMClient(env, base)), nil
	}, 1)

	return p
}

func wrapVLLMClient(env *runtime.Env, base string) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("vllm."+name, arity, fn)
	}
	mo.Keys = append(mo.Keys, "base_url", "model")
	mo.Vals["base_url"] = runtime.Str(base)
	mo.Vals["model"] = runtime.Str(vllmDefaultModel(env))

	put("list", 0, func(args []runtime.Value) (runtime.Value, error) {
		names, err := vllmListModels(env, base)
		if err != nil {
			return errRes(err.Error(), "vllm"), nil
		}
		items := make([]runtime.Value, len(names))
		for i, n := range names {
			items[i] = runtime.Str(n)
		}
		return runtime.Ok(runtime.List(items...)), nil
	})
	put("chat", -1, func(args []runtime.Value) (runtime.Value, error) {
		opts, prompt, err := vllmChatArgs(env, args)
		if err != nil {
			return errRes(err.Error(), "vllm"), nil
		}
		setMapStr(opts, "base_url", base)
		msgs := []map[string]any{{"role": "user", "content": prompt}}
		if sys := mapGetStr(opts, "system", ""); sys != "" {
			msgs = append([]map[string]any{{"role": "system", "content": sys}}, msgs...)
		}
		text, _, e := chatCompletions(env, opts, msgs, nil)
		if e != nil {
			return errRes(e.Error(), "vllm"), nil
		}
		return runtime.Ok(runtime.Str(text)), nil
	})
	put("health", 0, func(args []runtime.Value) (runtime.Value, error) {
		root := strings.TrimSuffix(base, "/v1")
		if err := vllmPing(env, root+"/health"); err != nil {
			if err2 := vllmPing(env, base+"/models"); err2 != nil {
				return errRes(err.Error(), "vllm"), nil
			}
		}
		return runtime.Ok(runtime.Bool(true)), nil
	})
	return m
}

func vllmOpenAIBase(env *runtime.Env) string {
	if v, ok := getenv(env, "VLLM_BASE_URL"); ok && v != "" {
		return normalizeVLLMBase(v)
	}
	if v, ok := getenv(env, "VLLM_HOST"); ok && v != "" {
		return normalizeVLLMBase(v)
	}
	if v, ok := getenv(env, "WEFT_API_BASE"); ok && v != "" {
		// only if provider is vllm
		if p, ok := getenv(env, "WEFT_PROVIDER"); ok && strings.EqualFold(p, "vllm") {
			return normalizeVLLMBase(v)
		}
	}
	return llmconfig.DefaultVLLMBase
}

func normalizeVLLMBase(base string) string {
	base = strings.TrimRight(base, "/")
	if !strings.HasSuffix(base, "/v1") {
		base = base + "/v1"
	}
	return base
}

func vllmDefaultModel(env *runtime.Env) string {
	if v, ok := getenv(env, "VLLM_MODEL"); ok && v != "" {
		return v
	}
	if v, ok := getenv(env, "WEFT_MODEL"); ok && v != "" {
		return v
	}
	if v, ok := getenv(env, "LLM_MODEL"); ok && v != "" {
		return v
	}
	return llmconfig.DefaultVLLMModel
}

func vllmAPIKey(env *runtime.Env) string {
	if v, ok := getenv(env, "VLLM_API_KEY"); ok && v != "" {
		return v
	}
	if v, ok := getenv(env, "OPENAI_API_KEY"); ok && v != "" {
		return v
	}
	return llmconfig.DefaultVLLMAPIKey
}

func defaultVLLMOpts(env *runtime.Env) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	for k, v := range map[string]runtime.Value{
		"base_url": runtime.Str(vllmOpenAIBase(env)),
		"api_key":  runtime.Str(vllmAPIKey(env)),
		"model":    runtime.Str(vllmDefaultModel(env)),
	} {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	return m
}

func vllmChatArgs(env *runtime.Env, args []runtime.Value) (opts runtime.Value, prompt string, err error) {
	opts = defaultVLLMOpts(env)
	if len(args) < 1 {
		return opts, "", fmt.Errorf("vllm.chat(prompt|opts)")
	}
	if args[0].Kind == runtime.KindMap {
		opts = mergeOpts(opts, args[0])
		prompt = mapGetStr(args[0], "prompt", mapGetStr(args[0], "message", ""))
		if prompt == "" {
			return opts, "", fmt.Errorf("opts need prompt")
		}
		return opts, prompt, nil
	}
	prompt = args[0].String()
	if len(args) >= 2 && args[1].Kind == runtime.KindMap {
		opts = mergeOpts(opts, args[1])
	}
	return opts, prompt, nil
}

func vllmListModels(env *runtime.Env, base string) ([]string, error) {
	raw, err := vllmGET(env, strings.TrimRight(base, "/")+"/models")
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	var ids []string
	for _, d := range out.Data {
		if d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}

func vllmPing(env *runtime.Env, url string) error {
	client := env.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	req, err := http.NewRequestWithContext(env.Context(), "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func vllmGET(env *runtime.Env, url string) ([]byte, error) {
	client := env.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(env.Context(), "GET", url, nil)
	if err != nil {
		return nil, err
	}
	key := vllmAPIKey(env)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncStr(string(raw), 200))
	}
	return raw, nil
}
