package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestLLM_DetectProviderAndHelpers(t *testing.T) {
	env := runtime.NewEnv()
	env.Environ = map[string]string{}
	if detectLLMProvider(env) == "" {
		t.Fatal("default provider empty")
	}
	env.Environ["WEFT_PROVIDER"] = "Ollama"
	if detectLLMProvider(env) != "ollama" {
		t.Fatal(detectLLMProvider(env))
	}
	env.Environ = map[string]string{"LLM_PROVIDER": "vllm"}
	if detectLLMProvider(env) != "vllm" {
		t.Fatal(detectLLMProvider(env))
	}
	env.Environ = map[string]string{"ANTHROPIC_API_KEY": "k"}
	if detectLLMProvider(env) != "anthropic" && detectLLMProvider(env) == "" {
		t.Log(detectLLMProvider(env))
	}
	env.Environ = map[string]string{"OLLAMA_HOST": "http://127.0.0.1:11434"}
	if detectLLMProvider(env) != "ollama" {
		t.Fatal(detectLLMProvider(env))
	}
	env.Environ = map[string]string{"OLLAMA_BASE_URL": "http://x"}
	_ = detectLLMProvider(env)
	env.Environ = map[string]string{"VLLM_BASE_URL": "http://x"}
	_ = detectLLMProvider(env)
	env.Environ = map[string]string{"VLLM_HOST": "http://x"}
	_ = detectLLMProvider(env)
	env.Environ = map[string]string{"LLM_BASE_URL": "http://127.0.0.1:11434"}
	_ = detectLLMProvider(env)
	env.Environ = map[string]string{"OPENAI_BASE_URL": "https://api.anthropic.com"}
	_ = detectLLMProvider(env)
	env.Environ = map[string]string{"OPENAI_BASE_URL": "http://localhost:8000/v1"}
	_ = detectLLMProvider(env)

	if !urlLooksLikeProvider("http://ollama.local", "", "ollama") {
		t.Fatal()
	}
	if !urlLooksLikeProvider("http://127.0.0.1:11434", "http://127.0.0.1:11434", "") {
		t.Fatal()
	}
	if hostPort("https://ex.com:443/v1") != "ex.com:443" {
		t.Fatal(hostPort("https://ex.com:443/v1"))
	}
	if hostPort("http://ex.com/path") != "ex.com" {
		t.Fatal(hostPort("http://ex.com/path"))
	}

	base := runtime.NewMap()
	bo := base.Obj.(*runtime.MapObj)
	bo.Keys = []string{"a"}
	bo.Vals["a"] = runtime.Int(1)
	over := runtime.NewMap()
	oo := over.Obj.(*runtime.MapObj)
	oo.Keys = []string{"b"}
	oo.Vals["b"] = runtime.Int(2)
	oo.Vals["a"] = runtime.Int(3)
	m := mergeOpts(base, over)
	if m.Kind != runtime.KindMap {
		t.Fatal(m)
	}
	// only over
	m = mergeOpts(runtime.Str("x"), over)
	_ = m
	// package builds
	p := packageLLM(env)
	if p.Kind != runtime.KindMap {
		t.Fatal()
	}
	// tool helper
	if _, ok := p.Obj.(*runtime.MapObj).Vals["tool"]; ok {
		fn := runtime.MakeBuiltin("add", 2, func(args []runtime.Value) (runtime.Value, error) {
			return runtime.Int(1), nil
		})
		tool := p.Obj.(*runtime.MapObj).Vals["tool"]
		_, _ = tool.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("add"), fn, runtime.Str("add nums")})
		_, _ = tool.Obj.(*runtime.BuiltinObj).Fn(nil)
	}
	// extract if present
	if extract, ok := p.Obj.(*runtime.MapObj).Vals["extract"]; ok {
		_, _ = extract.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str(`{"a":1}`)})
		_, _ = extract.Obj.(*runtime.BuiltinObj).Fn(nil)
	}

	// parseChatMessages shapes
	_, _, err := parseChatMessages(env, nil)
	if err == nil {
		t.Fatal("empty chat args")
	}
	_, prompt, err := parseChatArgs(env, []runtime.Value{runtime.Str("hi")})
	if err != nil || prompt != "hi" {
		t.Fatal(prompt, err)
	}
	opts := runtime.NewMap()
	opto := opts.Obj.(*runtime.MapObj)
	opto.Keys = []string{"system"}
	opto.Vals["system"] = runtime.Str("sys")
	_, prompt, err = parseChatArgs(env, []runtime.Value{runtime.Str("hi"), opts})
	if err != nil || prompt != "hi" {
		t.Fatal(err)
	}
	// messages list
	msg := runtime.NewMap()
	mo := msg.Obj.(*runtime.MapObj)
	mo.Keys = []string{"role", "content"}
	mo.Vals["role"] = runtime.Str("user")
	mo.Vals["content"] = runtime.Str("hey")
	msgs := runtime.List(msg)
	_, ms, err := parseChatMessages(env, []runtime.Value{msgs})
	if err != nil || len(ms) != 1 {
		t.Fatal(err, ms)
	}
	_, ms, err = parseChatMessages(env, []runtime.Value{msgs, opts})
	if err != nil {
		t.Fatal(err)
	}
	// map form
	cm := runtime.NewMap()
	cmo := cm.Obj.(*runtime.MapObj)
	cmo.Keys = []string{"prompt", "system"}
	cmo.Vals["prompt"] = runtime.Str("p")
	cmo.Vals["system"] = runtime.Str("s")
	_, ms, err = parseChatMessages(env, []runtime.Value{cm})
	if err != nil || len(ms) < 1 {
		t.Fatal(err)
	}
	// map with messages
	cm2 := runtime.NewMap()
	cm2o := cm2.Obj.(*runtime.MapObj)
	cm2o.Keys = []string{"messages"}
	cm2o.Vals["messages"] = msgs
	_, ms, err = parseChatMessages(env, []runtime.Value{cm2})
	if err != nil {
		t.Fatal(err)
	}
	// listToChatMessages edges
	_, err = listToChatMessages(runtime.Str("x"))
	if err == nil {
		t.Fatal()
	}
	_, err = listToChatMessages(runtime.List())
	if err == nil {
		t.Fatal()
	}
	_, err = listToChatMessages(runtime.List(runtime.Str("bare")))
	if err != nil {
		t.Fatal(err)
	}
	bad := runtime.NewMap()
	_, err = listToChatMessages(runtime.List(bad))
	if err == nil {
		t.Fatal("missing content")
	}
	// agent args
	_, tools, err := parseAgentArgs(env, nil)
	if err != nil || tools != nil {
		t.Log(err, tools)
	}
	_, _, _ = parseAgentArgs(env, []runtime.Value{runtime.List()})
	_, _, _ = parseAgentArgs(env, []runtime.Value{runtime.List(), opts})
	_ = defaultLLMOpts(env)
}
