package stdlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/loreste/weft/internal/llmconfig"
	"github.com/loreste/weft/internal/runtime"
)

// packageOllama — first-class Ollama integration (local models).
//
//	ollama.chat("why is the sky blue?")
//	ollama.chat({"model": "llama3.2", "prompt": "hi", "system": "…"})
//	ollama.list()?
//	ollama.generate("llama3.2", "Write a haiku")?
//	c := ollama.connect()?  // client handle with .chat / .list
func packageOllama(env *runtime.Env) runtime.Value {
	p := pkg()

	set(p, "host", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str(ollamaNativeHost(env)), nil
	}, 0)

	// ollama.list() -> Result[[str]] model names
	set(p, "list", func(args []runtime.Value) (runtime.Value, error) {
		host := ollamaNativeHost(env)
		if len(args) >= 1 && args[0].String() != "" {
			host = strings.TrimRight(args[0].String(), "/")
		}
		names, err := ollamaTags(env, host)
		if err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		items := make([]runtime.Value, len(names))
		for i, n := range names {
			items[i] = runtime.Str(n)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// ollama.ps() -> Result running models (native /api/ps)
	set(p, "ps", func(args []runtime.Value) (runtime.Value, error) {
		host := ollamaNativeHost(env)
		raw, err := ollamaGET(env, host+"/api/ps")
		if err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		return runtime.Ok(goToValue(out)), nil
	}, 0)

	// ollama.chat(prompt|opts) -> Result[str]
	set(p, "chat", func(args []runtime.Value) (runtime.Value, error) {
		opts, prompt, err := ollamaChatArgs(env, args)
		if err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		text, e := ollamaChatCompletions(env, opts, prompt)
		if e != nil {
			return errRes(e.Error(), "ollama"), nil
		}
		return runtime.Ok(runtime.Str(text)), nil
	}, -1)

	// ollama.generate(model, prompt) or opts — native /api/generate
	set(p, "generate", func(args []runtime.Value) (runtime.Value, error) {
		host := ollamaNativeHost(env)
		model := ollamaDefaultModel(env)
		prompt := ""
		stream := false
		if len(args) >= 1 && args[0].Kind == runtime.KindMap {
			model = mapGetStr(args[0], "model", model)
			prompt = mapGetStr(args[0], "prompt", "")
			host = mapGetStr(args[0], "host", host)
		} else if len(args) >= 2 {
			model = args[0].String()
			prompt = args[1].String()
		} else if len(args) == 1 {
			prompt = args[0].String()
		}
		if prompt == "" {
			return errRes("ollama.generate(model, prompt) or {model, prompt}", "ollama"), nil
		}
		body, _ := json.Marshal(map[string]any{
			"model":  model,
			"prompt": prompt,
			"stream": stream,
		})
		raw, err := ollamaPOST(env, host+"/api/generate", body)
		if err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		var out struct {
			Response string `json:"response"`
			Error    string `json:"error"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		if out.Error != "" {
			return errRes(out.Error, "ollama"), nil
		}
		return runtime.Ok(runtime.Str(out.Response)), nil
	}, 2)

	// ollama.pull(model) -> Result  (may take long)
	set(p, "pull", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("ollama.pull(model)", "ollama"), nil
		}
		host := ollamaNativeHost(env)
		body, _ := json.Marshal(map[string]any{"name": args[0].String(), "stream": false})
		// long timeout client
		raw, err := ollamaPOSTTimeout(env, host+"/api/pull", body, 30*time.Minute)
		if err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		var out struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		_ = json.Unmarshal(raw, &out)
		if out.Error != "" {
			return errRes(out.Error, "ollama"), nil
		}
		return runtime.Ok(runtime.Str(orStr(out.Status, "ok"))), nil
	}, 1)

	// ollama.connect(host?) -> client map {chat, list, host, model}
	set(p, "connect", func(args []runtime.Value) (runtime.Value, error) {
		host := ollamaNativeHost(env)
		if len(args) >= 1 && args[0].String() != "" {
			host = strings.TrimRight(args[0].String(), "/")
		}
		// ping tags
		if _, err := ollamaTags(env, host); err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		return runtime.Ok(wrapOllamaClient(env, host)), nil
	}, 1)

	// ollama.openai_base() -> str  for llm.chat({base_url: …})
	set(p, "openai_base", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str(ollamaOpenAIBase(env)), nil
	}, 0)

	return p
}

func wrapOllamaClient(env *runtime.Env, host string) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("ollama."+name, arity, fn)
	}
	mo.Keys = append(mo.Keys, "host", "model")
	mo.Vals["host"] = runtime.Str(host)
	mo.Vals["model"] = runtime.Str(ollamaDefaultModel(env))

	put("list", 0, func(args []runtime.Value) (runtime.Value, error) {
		names, err := ollamaTags(env, host)
		if err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		items := make([]runtime.Value, len(names))
		for i, n := range names {
			items[i] = runtime.Str(n)
		}
		return runtime.Ok(runtime.List(items...)), nil
	})
	put("chat", -1, func(args []runtime.Value) (runtime.Value, error) {
		opts, prompt, err := ollamaChatArgs(env, args)
		if err != nil {
			return errRes(err.Error(), "ollama"), nil
		}
		// force host
		setMapStr(opts, "base_url", strings.TrimRight(host, "/")+"/v1")
		text, e := ollamaChatCompletions(env, opts, prompt)
		if e != nil {
			return errRes(e.Error(), "ollama"), nil
		}
		return runtime.Ok(runtime.Str(text)), nil
	})
	return m
}

func ollamaNativeHost(env *runtime.Env) string {
	if v, ok := getenv(env, "OLLAMA_HOST"); ok && v != "" {
		return normalizeOllamaHost(v)
	}
	if v, ok := getenv(env, "OLLAMA_BASE_URL"); ok && v != "" {
		return normalizeOllamaHost(v)
	}
	return llmconfig.DefaultOllamaHost
}

func normalizeOllamaHost(host string) string {
	host = strings.TrimRight(host, "/")
	host = strings.TrimSuffix(host, "/v1")
	host = strings.TrimSuffix(host, "/api")
	return host
}

func ollamaOpenAIBase(env *runtime.Env) string {
	return ollamaNativeHost(env) + "/v1"
}

func ollamaDefaultModel(env *runtime.Env) string {
	if v, ok := getenv(env, "OLLAMA_MODEL"); ok && v != "" {
		return v
	}
	if v, ok := getenv(env, "WEFT_MODEL"); ok && v != "" {
		return v
	}
	if v, ok := getenv(env, "LLM_MODEL"); ok && v != "" {
		return v
	}
	return llmconfig.DefaultOllamaModel
}

func ollamaChatArgs(env *runtime.Env, args []runtime.Value) (opts runtime.Value, prompt string, err error) {
	opts = defaultOllamaOpts(env)
	if len(args) < 1 {
		return opts, "", fmt.Errorf("ollama.chat(prompt|opts)")
	}
	if args[0].Kind == runtime.KindMap {
		opts = mergeOpts(opts, args[0])
		prompt = mapGetStr(args[0], "prompt", "")
		if prompt == "" {
			prompt = mapGetStr(args[0], "message", "")
		}
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

func defaultOllamaOpts(env *runtime.Env) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	for k, v := range map[string]runtime.Value{
		"base_url": runtime.Str(ollamaOpenAIBase(env)),
		"api_key":  runtime.Str(llmconfig.DefaultOllamaAPIKey),
		"model":    runtime.Str(ollamaDefaultModel(env)),
	} {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	return m
}

func ollamaChatCompletions(env *runtime.Env, opts runtime.Value, prompt string) (string, error) {
	msgs := []map[string]any{{"role": "user", "content": prompt}}
	if sys := mapGetStr(opts, "system", ""); sys != "" {
		msgs = append([]map[string]any{{"role": "system", "content": sys}}, msgs...)
	}
	text, _, err := chatCompletions(env, opts, msgs, nil)
	return text, err
}

func ollamaTags(env *runtime.Env, host string) ([]string, error) {
	raw, err := ollamaGET(env, strings.TrimRight(host, "/")+"/api/tags")
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
	var names []string
	for _, m := range out.Models {
		if m.Name != "" {
			names = append(names, m.Name)
		}
	}
	return names, nil
}

func ollamaGET(env *runtime.Env, url string) ([]byte, error) {
	client := env.HTTPClient
	if client == nil {
		client = DefaultHTTPClient()
	}
	req, err := http.NewRequestWithContext(env.Context(), "GET", url, nil)
	if err != nil {
		return nil, err
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

func ollamaPOST(env *runtime.Env, url string, body []byte) ([]byte, error) {
	return ollamaPOSTTimeout(env, url, body, 300*time.Second)
}

func ollamaPOSTTimeout(env *runtime.Env, url string, body []byte, timeout time.Duration) ([]byte, error) {
	// Prefer host-injected client (tests / custom transport); otherwise default + timeout.
	client := env.HTTPClient
	if client == nil {
		c := DefaultHTTPClient()
		c.Timeout = timeout
		client = c
	}
	req, err := http.NewRequestWithContext(env.Context(), "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
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
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncStr(string(raw), 300))
	}
	return raw, nil
}

func setMapStr(m runtime.Value, k, v string) {
	if m.Kind != runtime.KindMap {
		return
	}
	mo := m.Obj.(*runtime.MapObj)
	if _, ok := mo.Vals[k]; !ok {
		mo.Keys = append(mo.Keys, k)
	}
	mo.Vals[k] = runtime.Str(v)
}

func orStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
