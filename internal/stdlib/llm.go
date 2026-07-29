//go:build !js

package stdlib

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/loreste/weft/internal/llmconfig"
	"github.com/loreste/weft/internal/runtime"
)

// packageLLM: concise LLM surface — chat, tool, agent, ask, stream.
//
//	llm.chat("hi")                                      // one-shot
//	llm.chat("hi", {system: "…"})                       // + system
//	llm.chat([{role:"user", content:"hi"}, …])          // multi-turn
//	llm.tool("name", fn)                                // or + "desc"
//	llm.agent([tools...]).run("prompt")                 // tool loop
//	llm.ask("prompt", [tools...])                       // one-liner agent
//	llm.ask("prompt", [tools...], {system, max_steps})  // agent + opts
//	llm.stream / llm.stream_text                        // SSE tokens
func packageLLM(env *runtime.Env) runtime.Value {
	p := pkg()

	// llm.chat(prompt | messages | opts) -> Result[str]
	set(p, "chat", func(args []runtime.Value) (runtime.Value, error) {
		opts, msgs, err := parseChatMessages(env, args)
		if err != nil {
			return errRes(err.Error(), "llm"), nil
		}
		text, _, e := chatCompletions(env, opts, msgs, nil)
		if e != nil {
			return errRes(e.Error(), "llm"), nil
		}
		return runtime.Ok(runtime.Str(text)), nil
	}, -1)

	// llm.tool(name, fn, desc?) -> ToolBinding map
	set(p, "tool", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("llm.tool(name, fn, desc?)", "llm"), nil
		}
		name := args[0].String()
		fn := args[1]
		if fn.Kind != runtime.KindFunc && fn.Kind != runtime.KindBuiltin {
			return errRes("llm.tool: second arg must be a function", "llm"), nil
		}
		desc := name
		if len(args) > 2 {
			desc = args[2].String()
		}
		schema := toolSchemaFromFn(name, desc, fn)
		binding := runtime.NewMap()
		mo := binding.Obj.(*runtime.MapObj)
		for k, v := range map[string]runtime.Value{
			"name":        runtime.Str(name),
			"description": runtime.Str(desc),
			"fn":          fn,
			"schema":      schema,
		} {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		return binding, nil
	}, -1)

	// llm.agent(tools | opts) -> Agent (map with .run)
	set(p, "agent", func(args []runtime.Value) (runtime.Value, error) {
		opts, tools, err := parseAgentArgs(env, args)
		if err != nil {
			return errRes(err.Error(), "llm"), nil
		}
		return makeAgent(env, opts, tools), nil
	}, -1)

	// llm.ask(prompt, tools?, opts?) -> Result[str]  — shortest path
	// opts: system, max_steps, model, base_url, … (merged into agent)
	set(p, "ask", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("llm.ask(prompt, tools?, opts?)", "llm"), nil
		}
		prompt := args[0].String()
		var tools []runtime.Value
		opts := defaultLLMOpts(env)
		if len(args) > 1 {
			switch args[1].Kind {
			case runtime.KindList:
				tools = args[1].Obj.(*runtime.ListObj).Items
				if len(args) > 2 && args[2].Kind == runtime.KindMap {
					opts = mergeOpts(opts, args[2])
				}
			case runtime.KindMap:
				// treat as opts with optional tools key
				opts = mergeOpts(opts, args[1])
				if t, ok := mapGet(args[1], "tools"); ok && t.Kind == runtime.KindList {
					tools = t.Obj.(*runtime.ListObj).Items
				}
			}
		}
		agent := makeAgent(env, opts, tools)
		return callAgentRun(env, agent, prompt)
	}, -1)

	// llm.stream(prompt) -> Result[Iter]  events: {kind, text?}  kinds: text|done|error
	set(p, "stream", func(args []runtime.Value) (runtime.Value, error) {
		opts, msgs, err := parseChatMessages(env, args)
		if err != nil {
			return errRes(err.Error(), "llm"), nil
		}
		it, e := chatStreamIter(env, opts, msgs)
		if e != nil {
			return errRes(e.Error(), "llm"), nil
		}
		return runtime.Ok(runtime.MakeIter(it)), nil
	}, -1)

	// llm.stream_text(prompt | opts) -> Result[str]
	// Collects stream "text" events into one string (common SSE glue).
	set(p, "stream_text", func(args []runtime.Value) (runtime.Value, error) {
		opts, msgs, err := parseChatMessages(env, args)
		if err != nil {
			return errRes(err.Error(), "llm"), nil
		}
		it, e := chatStreamIter(env, opts, msgs)
		if e != nil {
			return errRes(e.Error(), "llm"), nil
		}
		var b strings.Builder
		for {
			v, ok := it.Next()
			if !ok {
				break
			}
			kind := mapGetStr(v, "kind", "")
			switch kind {
			case "text":
				b.WriteString(mapGetStr(v, "text", ""))
			case "error":
				msg := mapGetStr(v, "error", mapGetStr(v, "text", mapGetStr(v, "message", "stream error")))
				return errRes(msg, "llm"), nil
			}
		}
		return runtime.Ok(runtime.Str(b.String())), nil
	}, -1)

	// llm.extract(prompt) -> Result[map]  — JSON object from model, no schema ceremony
	set(p, "extract", func(args []runtime.Value) (runtime.Value, error) {
		opts, msgs, err := parseChatMessages(env, args)
		if err != nil {
			return errRes(err.Error(), "llm"), nil
		}
		// ensure a JSON-nudge system message when caller didn't set one
		hasSys := false
		for _, m := range msgs {
			if m["role"] == "system" {
				hasSys = true
				break
			}
		}
		if !hasSys {
			sys := mapGetStr(opts, "system", "Reply with a single JSON object only. No markdown.")
			msgs = append([]map[string]any{{"role": "system", "content": sys}}, msgs...)
		}
		text, _, e := chatCompletionsJSON(env, opts, msgs)
		if e != nil {
			return errRes(e.Error(), "llm"), nil
		}
		text = strings.TrimSpace(text)
		// strip ```json fences if model ignores instructions
		text = stripFences(text)
		var raw any
		if err := json.Unmarshal([]byte(text), &raw); err != nil {
			return errRes("extract: not JSON: "+truncate(text, 120), "llm"), nil
		}
		return runtime.Ok(goToValue(raw)), nil
	}, -1)

	// llm.client(opts?) -> thin client map {chat, agent}
	set(p, "client", func(args []runtime.Value) (runtime.Value, error) {
		opts := defaultLLMOpts(env)
		if len(args) > 0 && args[0].Kind == runtime.KindMap {
			opts = mergeOpts(opts, args[0])
		}
		c := pkg()
		set(c, "chat", func(a []runtime.Value) (runtime.Value, error) {
			prompt := ""
			if len(a) > 0 {
				prompt = a[0].String()
			}
			msgs := []map[string]any{{"role": "user", "content": prompt}}
			text, _, e := chatCompletions(env, opts, msgs, nil)
			if e != nil {
				return errRes(e.Error(), "llm"), nil
			}
			return runtime.Ok(runtime.Str(text)), nil
		}, 1)
		set(c, "agent", func(a []runtime.Value) (runtime.Value, error) {
			var tools []runtime.Value
			if len(a) > 0 && a[0].Kind == runtime.KindList {
				tools = a[0].Obj.(*runtime.ListObj).Items
			}
			return makeAgent(env, opts, tools), nil
		}, 1)
		return c, nil
	}, -1)

	return p
}

func defaultLLMOpts(env *runtime.Env) runtime.Value {
	provider := detectLLMProvider(env)
	base := env.LLMBaseURL
	key := ""
	model := llmconfig.DefaultOpenAIModel

	switch provider {
	case llmconfig.ProviderOllama:
		base = ollamaOpenAIBase(env)
		key = llmconfig.DefaultOllamaAPIKey
		if v, ok := getenv(env, "OLLAMA_API_KEY"); ok && v != "" {
			key = v
		}
		model = ollamaDefaultModel(env)
	case llmconfig.ProviderVLLM:
		base = vllmOpenAIBase(env)
		key = vllmAPIKey(env)
		model = vllmDefaultModel(env)
	case llmconfig.ProviderAnthropic:
		base = llmconfig.DefaultAnthropicBase
		if v, ok := getenv(env, "ANTHROPIC_BASE_URL"); ok && v != "" {
			base = v
		} else if v, ok := getenv(env, "WEFT_API_BASE"); ok && v != "" {
			base = v
		}
		if v, ok := getenv(env, "ANTHROPIC_API_KEY"); ok {
			key = v
		}
		if v, ok := getenv(env, "WEFT_API_KEY"); ok && v != "" && key == "" {
			key = v
		}
		model = llmconfig.DefaultAnthropicModel
		if v, ok := getenv(env, "ANTHROPIC_MODEL"); ok && v != "" {
			model = v
		} else if v, ok := getenv(env, "WEFT_MODEL"); ok && v != "" {
			model = v
		} else if v, ok := getenv(env, "LLM_MODEL"); ok && v != "" {
			model = v
		}
	default:
		if base == "" {
			if v, ok := getenv(env, "LLM_BASE_URL"); ok && v != "" {
				base = v
			} else if v, ok := getenv(env, "OPENAI_BASE_URL"); ok && v != "" {
				base = v
			} else if v, ok := getenv(env, "WEFT_API_BASE"); ok && v != "" {
				base = v
			} else {
				base = llmconfig.DefaultOpenAIBase
			}
		}
		if v, ok := getenv(env, "OPENAI_API_KEY"); ok {
			key = v
		}
		if v, ok := getenv(env, "WEFT_API_KEY"); ok && v != "" {
			key = v
		}
		if v, ok := getenv(env, "LLM_API_KEY"); ok && v != "" {
			key = v
		}
		if v, ok := getenv(env, "WEFT_MODEL"); ok && v != "" {
			model = v
		} else if v, ok := getenv(env, "OPENAI_MODEL"); ok && v != "" {
			model = v
		} else if v, ok := getenv(env, "LLM_MODEL"); ok && v != "" {
			model = v
		}
	}
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	apiKeyVal := runtime.Str("")
	if key != "" {
		// Secret so accidental print/JSON redacts; unwrap only at HTTP layer.
		apiKeyVal = runtime.Struct("Secret", map[string]runtime.Value{
			"value": runtime.Str(key),
		}, []string{"value"})
	}
	for k, v := range map[string]runtime.Value{
		"base_url": runtime.Str(strings.TrimRight(base, "/")),
		"api_key":  apiKeyVal,
		"model":    runtime.Str(model),
		"provider": runtime.Str(provider),
	} {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	return m
}

// llmBaseTrustedForEnvKey reports whether base may receive process-env API keys.
// Untrusted/public attacker bases must not get OPENAI_API_KEY etc. (package supply-chain).
// Trust is hostname-only (exact or strict subdomain suffix) — never path/substring.
func llmBaseTrustedForEnvKey(base string) bool {
	host, ok := llmBaseHostname(base)
	if !ok {
		// empty base → default provider hosts; reject unparseable URLs
		return strings.TrimSpace(base) == ""
	}
	for _, h := range []string{
		"api.openai.com",
		"api.anthropic.com",
		"127.0.0.1",
		"localhost",
		"::1",
	} {
		if hostMatchesTrusted(host, h) {
			return true
		}
	}
	// optional allowlist: WEFT_LLM_TRUST_HOSTS=host1,host2 (hostname tokens only)
	if v := os.Getenv("WEFT_LLM_TRUST_HOSTS"); v != "" {
		for _, h := range strings.Split(v, ",") {
			h = strings.ToLower(strings.TrimSpace(h))
			if h != "" && hostMatchesTrusted(host, h) {
				return true
			}
		}
	}
	return false
}

// llmBaseHostname returns the lowercased hostname of base_url, or false if unsafe/unparseable.
func llmBaseHostname(base string) (string, bool) {
	b := strings.TrimSpace(base)
	if b == "" {
		return "", false
	}
	if !strings.Contains(b, "://") {
		b = "https://" + b
	}
	u, err := url.Parse(b)
	if err != nil || u.Host == "" {
		return "", false
	}
	// Reject userinfo (credential smuggling / odd parsers).
	if u.User != nil {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", false
	}
	return host, true
}

// hostMatchesTrusted: exact host match, or host is a subdomain of allowed
// (foo.api.openai.com matches api.openai.com; api.openai.com.evil.com does not).
func hostMatchesTrusted(host, allowed string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	allowed = strings.ToLower(strings.TrimSpace(allowed))
	if host == "" || allowed == "" {
		return false
	}
	if host == allowed {
		return true
	}
	return strings.HasSuffix(host, "."+allowed)
}

// guardLLMEnvKey refuses to send environment-sourced API keys to untrusted base_url.
func guardLLMEnvKey(env *runtime.Env, base, key string) error {
	if key == "" || llmBaseTrustedForEnvKey(base) {
		return nil
	}
	for _, ek := range []string{
		"OPENAI_API_KEY", "WEFT_API_KEY", "LLM_API_KEY",
		"ANTHROPIC_API_KEY", "VLLM_API_KEY", "OLLAMA_API_KEY",
	} {
		if v, ok := getenv(env, ek); ok && v != "" && v == key {
			return fmt.Errorf("refusing environment API key for untrusted base_url %q (set WEFT_LLM_TRUST_HOSTS or use a trusted host)", base)
		}
	}
	return nil
}

// detectLLMProvider: WEFT_PROVIDER / LLM_PROVIDER or infer from host env keys.
func detectLLMProvider(env *runtime.Env) string {
	if v, ok := getenv(env, "WEFT_PROVIDER"); ok && v != "" {
		return strings.ToLower(v)
	}
	if v, ok := getenv(env, "LLM_PROVIDER"); ok && v != "" {
		return strings.ToLower(v)
	}
	// Anthropic when only Anthropic key is set
	if v, ok := getenv(env, "ANTHROPIC_API_KEY"); ok && v != "" {
		if oai, _ := getenv(env, "OPENAI_API_KEY"); oai == "" {
			if wk, _ := getenv(env, "WEFT_API_KEY"); wk == "" {
				return llmconfig.ProviderAnthropic
			}
		}
	}
	if v, ok := getenv(env, "OLLAMA_HOST"); ok && v != "" {
		return llmconfig.ProviderOllama
	}
	if v, ok := getenv(env, "OLLAMA_BASE_URL"); ok && v != "" {
		return llmconfig.ProviderOllama
	}
	if v, ok := getenv(env, "VLLM_BASE_URL"); ok && v != "" {
		return llmconfig.ProviderVLLM
	}
	if v, ok := getenv(env, "VLLM_HOST"); ok && v != "" {
		return llmconfig.ProviderVLLM
	}
	base, _ := getenv(env, "LLM_BASE_URL")
	if base == "" {
		base, _ = getenv(env, "OPENAI_BASE_URL")
	}
	b := strings.ToLower(base)
	if urlLooksLikeProvider(b, llmconfig.DefaultOllamaHost, "ollama") {
		return llmconfig.ProviderOllama
	}
	if urlLooksLikeProvider(b, llmconfig.DefaultVLLMBase, "vllm") {
		return llmconfig.ProviderVLLM
	}
	if strings.Contains(b, "anthropic") {
		return llmconfig.ProviderAnthropic
	}
	return llmconfig.ProviderOpenAI
}

func urlLooksLikeProvider(baseLower, defaultURL, nameToken string) bool {
	if nameToken != "" && strings.Contains(baseLower, nameToken) {
		return true
	}
	hp := strings.ToLower(hostPort(defaultURL))
	if hp != "" && strings.Contains(baseLower, hp) {
		return true
	}
	return false
}

func hostPort(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}

func mergeOpts(base, over runtime.Value) runtime.Value {
	out := runtime.NewMap()
	omo := out.Obj.(*runtime.MapObj)
	copyMap := func(src runtime.Value) {
		if src.Kind != runtime.KindMap {
			return
		}
		mo := src.Obj.(*runtime.MapObj)
		for _, k := range mo.Keys {
			if _, exists := omo.Vals[k]; !exists {
				omo.Keys = append(omo.Keys, k)
			}
			omo.Vals[k] = mo.Vals[k]
		}
		for k, v := range mo.Vals {
			if _, exists := omo.Vals[k]; !exists {
				omo.Keys = append(omo.Keys, k)
			}
			omo.Vals[k] = v
		}
	}
	copyMap(base)
	copyMap(over)
	return out
}

func parseChatArgs(env *runtime.Env, args []runtime.Value) (opts runtime.Value, prompt string, err error) {
	opts, msgs, err := parseChatMessages(env, args)
	if err != nil {
		return opts, "", err
	}
	// last user content as prompt (compat for callers that only want a string)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i]["role"] == "user" {
			if s, ok := msgs[i]["content"].(string); ok {
				return opts, s, nil
			}
		}
	}
	return opts, "", fmt.Errorf("llm.chat needs a prompt")
}

// parseChatMessages builds OpenAI-style messages from:
//
//	llm.chat("hi")
//	llm.chat("hi", {system: "…", model: "…"})
//	llm.chat({prompt: "hi", system: "…"})
//	llm.chat([{role:"system", content:"…"}, {role:"user", content:"…"}])
//	llm.chat(messages, {model: "…"})
func parseChatMessages(env *runtime.Env, args []runtime.Value) (opts runtime.Value, msgs []map[string]any, err error) {
	opts = defaultLLMOpts(env)
	if len(args) == 0 {
		return opts, nil, fmt.Errorf("llm.chat needs a prompt or messages")
	}

	// Multi-turn: list of {role, content}
	if args[0].Kind == runtime.KindList {
		msgs, err = listToChatMessages(args[0])
		if err != nil {
			return opts, nil, err
		}
		if len(args) > 1 && args[1].Kind == runtime.KindMap {
			opts = mergeOpts(opts, args[1])
		}
		// optional system in opts when not already in messages
		if sys := mapGetStr(opts, "system", ""); sys != "" {
			hasSys := false
			for _, m := range msgs {
				if m["role"] == "system" {
					hasSys = true
					break
				}
			}
			if !hasSys {
				msgs = append([]map[string]any{{"role": "system", "content": sys}}, msgs...)
			}
		}
		return opts, msgs, nil
	}

	if args[0].Kind == runtime.KindMap {
		opts = mergeOpts(opts, args[0])
		// either prompt/message string or messages list inside the map
		if t, ok := mapGet(args[0], "messages"); ok && t.Kind == runtime.KindList {
			msgs, err = listToChatMessages(t)
			if err != nil {
				return opts, nil, err
			}
			return opts, msgs, nil
		}
		prompt := mapGetStr(args[0], "prompt", mapGetStr(args[0], "message", ""))
		if prompt == "" {
			return opts, nil, fmt.Errorf("llm.chat map needs prompt or messages")
		}
		msgs = []map[string]any{{"role": "user", "content": prompt}}
		if sys := mapGetStr(opts, "system", ""); sys != "" {
			msgs = append([]map[string]any{{"role": "system", "content": sys}}, msgs...)
		}
		return opts, msgs, nil
	}

	prompt := args[0].String()
	if len(args) > 1 && args[1].Kind == runtime.KindMap {
		opts = mergeOpts(opts, args[1])
	}
	msgs = []map[string]any{{"role": "user", "content": prompt}}
	if sys := mapGetStr(opts, "system", ""); sys != "" {
		msgs = append([]map[string]any{{"role": "system", "content": sys}}, msgs...)
	}
	return opts, msgs, nil
}

func listToChatMessages(list runtime.Value) ([]map[string]any, error) {
	if list.Kind != runtime.KindList {
		return nil, fmt.Errorf("messages must be a list")
	}
	items := list.Obj.(*runtime.ListObj).Items
	if len(items) == 0 {
		return nil, fmt.Errorf("messages list is empty")
	}
	out := make([]map[string]any, 0, len(items))
	for i, it := range items {
		if it.Kind != runtime.KindMap {
			// bare string → user turn
			out = append(out, map[string]any{"role": "user", "content": it.String()})
			continue
		}
		role := mapGetStr(it, "role", "user")
		content := mapGetStr(it, "content", mapGetStr(it, "text", ""))
		if content == "" && role != "assistant" {
			return nil, fmt.Errorf("messages[%d] missing content", i)
		}
		out = append(out, map[string]any{"role": role, "content": content})
	}
	return out, nil
}

func parseAgentArgs(env *runtime.Env, args []runtime.Value) (opts runtime.Value, tools []runtime.Value, err error) {
	opts = defaultLLMOpts(env)
	if len(args) == 0 {
		return opts, nil, nil
	}
	if args[0].Kind == runtime.KindList {
		tools = args[0].Obj.(*runtime.ListObj).Items
		if len(args) > 1 && args[1].Kind == runtime.KindMap {
			opts = mergeOpts(opts, args[1])
		}
		return opts, tools, nil
	}
	if args[0].Kind == runtime.KindMap {
		opts = mergeOpts(opts, args[0])
		if t, ok := mapGet(args[0], "tools"); ok && t.Kind == runtime.KindList {
			tools = t.Obj.(*runtime.ListObj).Items
		}
		return opts, tools, nil
	}
	return opts, nil, fmt.Errorf("llm.agent(tools) or llm.agent({tools, ...})")
}

func toolSchemaFromFn(name, desc string, fn runtime.Value) runtime.Value {
	props := runtime.NewMap()
	required := runtime.List()
	if fn.Kind == runtime.KindFunc {
		fo := fn.Obj.(*runtime.FuncObj)
		// Prefer TypeInfo field names (always set for named/anon fns with params).
		// Fallback: arity-only tools still expose generic arg0..N so models can call them.
		pmo := props.Obj.(*runtime.MapObj)
		var req []runtime.Value
		if fo.TypeInfo != nil && len(fo.TypeInfo.Fields) > 0 {
			for _, f := range fo.TypeInfo.Fields {
				pmo.Keys = append(pmo.Keys, f.Name)
				pmo.Vals[f.Name] = runtime.NewMap()
				tm := pmo.Vals[f.Name].Obj.(*runtime.MapObj)
				tm.Keys = []string{"type"}
				tm.Vals["type"] = runtime.Str(jsonTypeName(f.Type))
				req = append(req, runtime.Str(f.Name))
			}
		} else if fo.Arity > 0 {
			for i := 0; i < fo.Arity; i++ {
				key := fmt.Sprintf("arg%d", i)
				pmo.Keys = append(pmo.Keys, key)
				pmo.Vals[key] = runtime.NewMap()
				tm := pmo.Vals[key].Obj.(*runtime.MapObj)
				tm.Keys = []string{"type"}
				tm.Vals["type"] = runtime.Str("string")
				req = append(req, runtime.Str(key))
			}
		}
		required = runtime.List(req...)
	}
	// OpenAI function schema as map
	schema := runtime.NewMap()
	mo := schema.Obj.(*runtime.MapObj)
	params := runtime.NewMap()
	pmo := params.Obj.(*runtime.MapObj)
	for k, v := range map[string]runtime.Value{
		"type":       runtime.Str("object"),
		"properties": props,
		"required":   required,
	} {
		pmo.Keys = append(pmo.Keys, k)
		pmo.Vals[k] = v
	}
	for k, v := range map[string]runtime.Value{
		"name":        runtime.Str(name),
		"description": runtime.Str(desc),
		"parameters":  params,
	} {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	return schema
}

func jsonTypeName(t *runtime.TypeInfo) string {
	if t == nil {
		return "string"
	}
	switch t.Name {
	case "int", "float":
		return "number"
	case "bool":
		return "boolean"
	default:
		return "string"
	}
}

func makeAgent(env *runtime.Env, opts runtime.Value, tools []runtime.Value) runtime.Value {
	// Store agent state in a map; .run is a builtin closing over state.
	state := &agentState{env: env, opts: opts, tools: tools}
	a := runtime.NewMap()
	mo := a.Obj.(*runtime.MapObj)
	mo.Keys = []string{"run"}
	mo.Vals["run"] = runtime.MakeBuiltin("agent.run", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("agent.run needs a prompt", "llm"), nil
		}
		return state.run(args[0].String())
	})
	return a
}

func callAgentRun(env *runtime.Env, agent runtime.Value, prompt string) (runtime.Value, error) {
	run, ok := mapGet(agent, "run")
	if !ok {
		return errRes("not an agent", "llm"), nil
	}
	if run.Kind != runtime.KindBuiltin {
		return errRes("agent.run missing", "llm"), nil
	}
	return run.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str(prompt)})
}

type agentState struct {
	env   *runtime.Env
	opts  runtime.Value
	tools []runtime.Value
}

func (a *agentState) run(prompt string) (runtime.Value, error) {
	maxSteps := int(mapGetInt(a.opts, "max_steps", 20))
	system := mapGetStr(a.opts, "system", "You are a helpful assistant. Use tools when needed.")

	toolByName := map[string]runtime.Value{}
	var apiTools []map[string]any
	for _, t := range a.tools {
		name := mapGetStr(t, "name", "")
		if name == "" {
			continue
		}
		toolByName[name] = t
		schemaVal, _ := mapGet(t, "schema")
		apiTools = append(apiTools, map[string]any{
			"type":     "function",
			"function": valueToGo(schemaVal),
		})
	}

	msgs := []map[string]any{
		{"role": "system", "content": system},
		{"role": "user", "content": prompt},
	}

	for step := 0; step < maxSteps; step++ {
		text, toolCalls, err := chatCompletions(a.env, a.opts, msgs, apiTools)
		if err != nil {
			return errRes(err.Error(), "llm"), nil
		}
		if len(toolCalls) == 0 {
			return runtime.Ok(runtime.Str(text)), nil
		}
		// assistant message with tool_calls
		asst := map[string]any{"role": "assistant", "content": text}
		var tcWire []map[string]any
		for _, tc := range toolCalls {
			tcWire = append(tcWire, map[string]any{
				"id":   tc.ID,
				"type": "function",
				"function": map[string]any{
					"name":      tc.Name,
					"arguments": tc.ArgsJSON,
				},
			})
		}
		asst["tool_calls"] = tcWire
		msgs = append(msgs, asst)

		// Concurrent by default: fan out tool calls (no asyncio — Go tasks).
		// Results reattached in call order for a stable messages list.
		type toolOut struct {
			id   string
			body string
		}
		outs := make([]toolOut, len(toolCalls))
		var wg sync.WaitGroup
		for i, tc := range toolCalls {
			wg.Add(1)
			go func(i int, tc toolCall) {
				defer wg.Done()
				binding, ok := toolByName[tc.Name]
				if !ok {
					outs[i] = toolOut{id: tc.ID, body: "unknown tool " + tc.Name}
					return
				}
				result, err := a.callTool(binding, tc.ArgsJSON)
				if err != nil {
					result = "error: " + err.Error()
				}
				outs[i] = toolOut{id: tc.ID, body: result}
			}(i, tc)
		}
		wg.Wait()
		for _, o := range outs {
			msgs = append(msgs, map[string]any{
				"role":         "tool",
				"tool_call_id": o.id,
				"content":      o.body,
			})
		}
	}
	return errRes(fmt.Sprintf("max_steps exceeded (%d); raise max_steps in agent opts", maxSteps), "llm"), nil
}

func (a *agentState) callTool(binding runtime.Value, argsJSON string) (string, error) {
	fn, ok := mapGet(binding, "fn")
	if !ok {
		return "", fmt.Errorf("tool has no fn")
	}
	var argMap map[string]any
	if argsJSON == "" {
		argMap = map[string]any{}
	} else if err := json.Unmarshal([]byte(argsJSON), &argMap); err != nil {
		// model sometimes sends raw string
		argMap = map[string]any{}
	}

	// Build args in schema parameter order when TypeInfo present
	var callArgs []runtime.Value
	if fn.Kind == runtime.KindFunc {
		fo := fn.Obj.(*runtime.FuncObj)
		if fo.TypeInfo != nil && len(fo.TypeInfo.Fields) > 0 {
			for _, f := range fo.TypeInfo.Fields {
				if v, ok := argMap[f.Name]; ok {
					callArgs = append(callArgs, goToValue(v))
				} else {
					callArgs = append(callArgs, runtime.Null())
				}
			}
		} else {
			// single string arg: dump whole json or first value
			if len(argMap) == 1 {
				for _, v := range argMap {
					callArgs = append(callArgs, goToValue(v))
				}
			} else if len(argMap) == 0 {
				// ok
			} else {
				callArgs = append(callArgs, goToValue(argMap))
			}
		}
	} else {
		for _, v := range argMap {
			callArgs = append(callArgs, goToValue(v))
			break
		}
	}

	if a.env.Call == nil {
		return "", fmt.Errorf("runtime cannot call Weft functions (Call not set)")
	}
	ret, err := a.env.Call(fn, callArgs)
	if err != nil {
		return "", err
	}
	if ret.Kind == runtime.KindResult {
		ro := ret.Obj.(*runtime.ResultObj)
		if !ro.Ok {
			return "error: " + ro.Err.String(), nil
		}
		return ro.Val.String(), nil
	}
	return ret.String(), nil
}

type toolCall struct {
	ID       string
	Name     string
	ArgsJSON string
}

func chatCompletions(env *runtime.Env, opts runtime.Value, messages []map[string]any, tools []map[string]any) (text string, calls []toolCall, err error) {
	base := mapGetStr(opts, "base_url", "https://api.openai.com/v1")
	key := SecretString(mapGetStrVal(opts, "api_key"))
	if key == "" {
		if v, ok := mapGet(opts, "api_key"); ok {
			key = SecretString(v)
		}
	}
	if err := guardLLMEnvKey(env, base, key); err != nil {
		return "", nil, err
	}
	model := mapGetStr(opts, "model", llmconfig.DefaultOpenAIModel)
	provider := mapGetStr(opts, "provider", "")

	// Anthropic Messages API (chat + tool_use)
	if provider == llmconfig.ProviderAnthropic || strings.Contains(strings.ToLower(base), "anthropic") {
		return chatAnthropic(env, base, key, model, messages, tools)
	}

	body := map[string]any{
		"model":    model,
		"messages": messages,
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}

	if env.LLMDo != nil {
		text, tcs, e := env.LLMDo(raw)
		if e != nil {
			return "", nil, e
		}
		var calls []toolCall
		for _, tc := range tcs {
			calls = append(calls, toolCall{ID: tc.ID, Name: tc.Name, ArgsJSON: tc.ArgsJSON})
		}
		return text, calls, nil
	}

	url := strings.TrimRight(base, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(env.Context(), "POST", url, bytes.NewReader(raw))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := env.HTTPClient
	if client == nil {
		client = DefaultHTTPClient()
		client.Timeout = 120 * time.Second
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("llm http %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	return parseChatResponse(respBody)
}

func chatAnthropic(env *runtime.Env, base, key, model string, messages []map[string]any, tools []map[string]any) (string, []toolCall, error) {
	if err := guardLLMEnvKey(env, base, key); err != nil {
		return "", nil, err
	}
	sys, turns := messagesToAnthropic(messages)
	body := map[string]any{
		"model":      model,
		"max_tokens": 4096,
		"messages":   turns,
	}
	if sys != "" {
		body["system"] = sys
	}
	if len(tools) > 0 {
		body["tools"] = openaiToolsToAnthropic(tools)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}
	if env.LLMDo != nil {
		// mocks may return OpenAI-style tool calls; body is Anthropic-shaped
		text, tcs, e := env.LLMDo(raw)
		if e != nil {
			return "", nil, e
		}
		var calls []toolCall
		for _, tc := range tcs {
			calls = append(calls, toolCall{ID: tc.ID, Name: tc.Name, ArgsJSON: tc.ArgsJSON})
		}
		return text, calls, nil
	}
	url := strings.TrimRight(base, "/")
	if strings.HasSuffix(url, "/v1") {
		url += "/messages"
	} else if !strings.Contains(url, "/messages") {
		url += "/v1/messages"
	}
	req, err := http.NewRequestWithContext(env.Context(), "POST", url, bytes.NewReader(raw))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", llmconfig.AnthropicAPIVersion)
	client := env.HTTPClient
	if client == nil {
		client = DefaultHTTPClient()
		client.Timeout = 120 * time.Second
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("anthropic http %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	return parseAnthropicResponse(respBody)
}

// openaiToolsToAnthropic maps OpenAI function tools → Anthropic tools.
func openaiToolsToAnthropic(tools []map[string]any) []map[string]any {
	var out []map[string]any
	for _, t := range tools {
		if name, ok := t["name"].(string); ok && t["input_schema"] != nil {
			// already Anthropic-shaped
			out = append(out, t)
			_ = name
			continue
		}
		fn, _ := t["function"].(map[string]any)
		if fn == nil {
			continue
		}
		params := fn["parameters"]
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, map[string]any{
			"name":         fn["name"],
			"description":  fn["description"],
			"input_schema": params,
		})
	}
	return out
}

// messagesToAnthropic converts OpenAI-style history (incl. tool_calls / role:tool)
// into Anthropic system + turns with content blocks.
func messagesToAnthropic(messages []map[string]any) (system string, turns []map[string]any) {
	var toolBuf []map[string]any // pending tool_result blocks
	flushTools := func() {
		if len(toolBuf) == 0 {
			return
		}
		turns = append(turns, map[string]any{"role": "user", "content": toolBuf})
		toolBuf = nil
	}
	for _, m := range messages {
		role, _ := m["role"].(string)
		role = strings.ToLower(role)
		switch role {
		case "system":
			c := contentAsString(m["content"])
			if system != "" {
				system += "\n"
			}
			system += c
		case "tool":
			// OpenAI tool result → Anthropic tool_result (batched as user content)
			id, _ := m["tool_call_id"].(string)
			toolBuf = append(toolBuf, map[string]any{
				"type":        "tool_result",
				"tool_use_id": id,
				"content":     contentAsString(m["content"]),
			})
		case "assistant":
			flushTools()
			// tool_calls (OpenAI) → content blocks
			if tcs, ok := m["tool_calls"].([]map[string]any); ok && len(tcs) > 0 {
				var blocks []map[string]any
				if s := contentAsString(m["content"]); s != "" {
					blocks = append(blocks, map[string]any{"type": "text", "text": s})
				}
				for _, tc := range tcs {
					fn, _ := tc["function"].(map[string]any)
					name, _ := fn["name"].(string)
					args, _ := fn["arguments"].(string)
					id, _ := tc["id"].(string)
					var input any = map[string]any{}
					if args != "" {
						_ = json.Unmarshal([]byte(args), &input)
					}
					blocks = append(blocks, map[string]any{
						"type":  "tool_use",
						"id":    id,
						"name":  name,
						"input": input,
					})
				}
				turns = append(turns, map[string]any{"role": "assistant", "content": blocks})
				continue
			}
			// also handle []any from JSON round-trip
			if tcsAny, ok := m["tool_calls"].([]any); ok && len(tcsAny) > 0 {
				var blocks []map[string]any
				if s := contentAsString(m["content"]); s != "" {
					blocks = append(blocks, map[string]any{"type": "text", "text": s})
				}
				for _, raw := range tcsAny {
					tc, _ := raw.(map[string]any)
					fn, _ := tc["function"].(map[string]any)
					name, _ := fn["name"].(string)
					args, _ := fn["arguments"].(string)
					id, _ := tc["id"].(string)
					var input any = map[string]any{}
					if args != "" {
						_ = json.Unmarshal([]byte(args), &input)
					}
					blocks = append(blocks, map[string]any{
						"type":  "tool_use",
						"id":    id,
						"name":  name,
						"input": input,
					})
				}
				turns = append(turns, map[string]any{"role": "assistant", "content": blocks})
				continue
			}
			turns = append(turns, map[string]any{"role": "assistant", "content": contentAsString(m["content"])})
		case "user":
			flushTools()
			turns = append(turns, map[string]any{"role": "user", "content": contentAsString(m["content"])})
		default:
			flushTools()
			turns = append(turns, map[string]any{"role": "user", "content": contentAsString(m["content"])})
		}
	}
	flushTools()
	if len(turns) == 0 {
		turns = []map[string]any{{"role": "user", "content": "hello"}}
	}
	return system, turns
}

func contentAsString(c any) string {
	if c == nil {
		return ""
	}
	switch v := c.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func parseAnthropicResponse(body []byte) (string, []toolCall, error) {
	var parsed struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", nil, err
	}
	if parsed.Error != nil {
		return "", nil, fmt.Errorf("%s", parsed.Error.Message)
	}
	var text strings.Builder
	var calls []toolCall
	for _, c := range parsed.Content {
		switch c.Type {
		case "text", "":
			text.WriteString(c.Text)
		case "tool_use":
			args := "{}"
			if len(c.Input) > 0 {
				args = string(c.Input)
			}
			id := c.ID
			if id == "" {
				id = "tool_" + c.Name
			}
			calls = append(calls, toolCall{ID: id, Name: c.Name, ArgsJSON: args})
		}
	}
	s := strings.TrimSpace(text.String())
	if s == "" && len(calls) == 0 {
		return "", nil, fmt.Errorf("empty anthropic response")
	}
	return s, calls, nil
}

func mapGetStrVal(m runtime.Value, key string) runtime.Value {
	v, _ := mapGet(m, key)
	return v
}

func parseChatResponse(body []byte) (string, []toolCall, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", nil, err
	}
	if parsed.Error != nil {
		return "", nil, fmt.Errorf("%s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", nil, fmt.Errorf("empty llm response")
	}
	msg := parsed.Choices[0].Message
	var calls []toolCall
	for _, tc := range msg.ToolCalls {
		calls = append(calls, toolCall{
			ID: tc.ID, Name: tc.Function.Name, ArgsJSON: tc.Function.Arguments,
		})
	}
	return msg.Content, calls, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

func event(kind string, fields map[string]runtime.Value) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = append(mo.Keys, "kind")
	mo.Vals["kind"] = runtime.Str(kind)
	for k, v := range fields {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	return m
}

// chatCompletionsJSON requests json_object response format when provider supports it.
func chatCompletionsJSON(env *runtime.Env, opts runtime.Value, messages []map[string]any) (string, []toolCall, error) {
	base := mapGetStr(opts, "base_url", "https://api.openai.com/v1")
	key := SecretString(mapGetStrVal(opts, "api_key"))
	if key == "" {
		if v, ok := mapGet(opts, "api_key"); ok {
			key = SecretString(v)
		}
	}
	if err := guardLLMEnvKey(env, base, key); err != nil {
		return "", nil, err
	}
	model := mapGetStr(opts, "model", llmconfig.DefaultOpenAIModel)
	body := map[string]any{
		"model":           model,
		"messages":        messages,
		"response_format": map[string]any{"type": "json_object"},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}
	if env.LLMDo != nil {
		text, tcs, e := env.LLMDo(raw)
		if e != nil {
			return "", nil, e
		}
		var calls []toolCall
		for _, tc := range tcs {
			calls = append(calls, toolCall{ID: tc.ID, Name: tc.Name, ArgsJSON: tc.ArgsJSON})
		}
		return text, calls, nil
	}
	url := strings.TrimRight(base, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(env.Context(), "POST", url, bytes.NewReader(raw))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := env.HTTPClient
	if client == nil {
		client = DefaultHTTPClient()
		client.Timeout = 120 * time.Second
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode >= 300 {
		// retry without response_format for picky providers
		return chatCompletions(env, opts, messages, nil)
	}
	return parseChatResponse(respBody)
}

// chatStreamIter: pull iterator over stream events {kind, text?}.
// Mock: chunks via channel. Live: incremental SSE (OpenAI-compat or Anthropic) —
// tokens are pushed as lines arrive; the body is not buffered whole first.
func chatStreamIter(env *runtime.Env, opts runtime.Value, messages []map[string]any) (runtime.Iter, error) {
	ch := make(chan runtime.Value, 32)

	if env.LLMDo != nil {
		go func() {
			defer close(ch)
			body, _ := json.Marshal(map[string]any{"model": "mock", "messages": messages, "stream": true})
			text, _, err := env.LLMDo(body)
			if err != nil {
				ch <- event("error", map[string]runtime.Value{"error": runtime.Str(err.Error())})
				return
			}
			for _, part := range chunkString(text, 8) {
				ch <- event("text", map[string]runtime.Value{"text": runtime.Str(part)})
			}
			ch <- event("done", nil)
		}()
		return &runtime.ChanIter{Ch: ch}, nil
	}

	base := mapGetStr(opts, "base_url", "https://api.openai.com/v1")
	key := SecretString(mapGetStrVal(opts, "api_key"))
	if key == "" {
		if v, ok := mapGet(opts, "api_key"); ok {
			key = SecretString(v)
		}
	}
	if err := guardLLMEnvKey(env, base, key); err != nil {
		return nil, err
	}
	model := mapGetStr(opts, "model", llmconfig.DefaultOpenAIModel)
	provider := mapGetStr(opts, "provider", "")
	anthropic := provider == llmconfig.ProviderAnthropic || strings.Contains(strings.ToLower(base), "anthropic")

	var (
		payload []byte
		url     string
		err     error
	)
	if anthropic {
		sys, turns := messagesToAnthropic(messages)
		body := map[string]any{
			"model":      model,
			"max_tokens": 4096,
			"messages":   turns,
			"stream":     true,
		}
		if sys != "" {
			body["system"] = sys
		}
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
		url = strings.TrimRight(base, "/")
		if strings.HasSuffix(url, "/v1") {
			url += "/messages"
		} else if !strings.Contains(url, "/messages") {
			url += "/v1/messages"
		}
	} else {
		payload, err = json.Marshal(map[string]any{
			"model": model, "messages": messages, "stream": true,
		})
		if err != nil {
			return nil, err
		}
		url = strings.TrimRight(base, "/") + "/chat/completions"
	}

	req, err := http.NewRequestWithContext(env.Context(), "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if anthropic {
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", llmconfig.AnthropicAPIVersion)
	} else if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	// Own client so we do not mutate a shared env.HTTPClient Timeout.
	// Timeout=0: stream can outlive 30s; cancel via env.Context().
	baseClient := env.HTTPClient
	if baseClient == nil {
		baseClient = DefaultHTTPClient()
	}
	client := *baseClient
	client.Timeout = 0

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("llm stream http %d: %s", resp.StatusCode, truncate(string(b), 200))
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()
		// Cap total bytes read without buffering the whole response first.
		r := io.LimitReader(resp.Body, maxBodyBytes)
		if anthropic {
			streamAnthropicSSE(r, ch)
		} else {
			streamOpenAISSE(r, ch)
		}
	}()
	return &runtime.ChanIter{Ch: ch}, nil
}

func chunkString(s string, n int) []string {
	if s == "" {
		return nil
	}
	var out []string
	for len(s) > 0 {
		if len(s) < n {
			out = append(out, s)
			break
		}
		out = append(out, s[:n])
		s = s[n:]
	}
	return out
}

// streamOpenAISSE reads OpenAI-compatible SSE incrementally and pushes text/done/error events.
func streamOpenAISSE(r io.Reader, ch chan<- runtime.Value) {
	sc := bufio.NewScanner(r)
	// Allow larger SSE lines (models sometimes dump big JSON chunks).
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sawDone := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue // keep-alive / comments
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			ch <- event("done", nil)
			sawDone = true
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			ch <- event("text", map[string]runtime.Value{
				"text": runtime.Str(chunk.Choices[0].Delta.Content),
			})
		}
	}
	if err := sc.Err(); err != nil && !sawDone {
		ch <- event("error", map[string]runtime.Value{"error": runtime.Str(err.Error())})
		return
	}
	if !sawDone {
		ch <- event("done", nil)
	}
}

// streamAnthropicSSE reads Anthropic Messages SSE (content_block_delta / message_stop).
func streamAnthropicSSE(r io.Reader, ch chan<- runtime.Value) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sawDone := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(payload), &raw); err != nil {
			continue
		}
		typ, _ := raw["type"].(string)
		switch typ {
		case "content_block_delta":
			// {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi"}}
			delta, _ := raw["delta"].(map[string]any)
			if delta == nil {
				continue
			}
			if text, ok := delta["text"].(string); ok && text != "" {
				ch <- event("text", map[string]runtime.Value{"text": runtime.Str(text)})
			}
		case "message_stop", "error":
			if typ == "error" {
				msg := "anthropic stream error"
				if e, ok := raw["error"].(map[string]any); ok {
					if m, ok := e["message"].(string); ok && m != "" {
						msg = m
					}
				}
				ch <- event("error", map[string]runtime.Value{"error": runtime.Str(msg)})
			}
			ch <- event("done", nil)
			sawDone = true
			// keep reading? message_stop is terminal
			if typ == "message_stop" || typ == "error" {
				return
			}
		}
	}
	if err := sc.Err(); err != nil && !sawDone {
		ch <- event("error", map[string]runtime.Value{"error": runtime.Str(err.Error())})
		return
	}
	if !sawDone {
		ch <- event("done", nil)
	}
}

// parseSSEBody is retained for tests / offline dumps; prefers the incremental path at runtime.
func parseSSEBody(r io.Reader) []runtime.Value {
	ch := make(chan runtime.Value, 64)
	go func() {
		defer close(ch)
		streamOpenAISSE(r, ch)
	}()
	var events []runtime.Value
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}
