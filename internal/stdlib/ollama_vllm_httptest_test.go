package stdlib

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestOllama_HelpersAndHTTPMock(t *testing.T) {
	env := runtime.NewEnv()
	if normalizeOllamaHost("http://h/v1/") != "http://h" {
		t.Fatal(normalizeOllamaHost("http://h/v1/"))
	}
	if normalizeOllamaHost("http://h/api") != "http://h" {
		t.Fatal()
	}

	env.Environ = map[string]string{"OLLAMA_HOST": "http://custom:11434"}
	if ollamaNativeHost(env) != "http://custom:11434" {
		t.Fatal(ollamaNativeHost(env))
	}
	env.Environ = map[string]string{"OLLAMA_BASE_URL": "http://base:1/v1"}
	if !strings.Contains(ollamaNativeHost(env), "base") {
		t.Fatal(ollamaNativeHost(env))
	}
	env.Environ = map[string]string{"OLLAMA_MODEL": "mymodel"}
	if ollamaDefaultModel(env) != "mymodel" {
		t.Fatal(ollamaDefaultModel(env))
	}
	env.Environ = map[string]string{"WEFT_MODEL": "wmodel"}
	if ollamaDefaultModel(env) != "wmodel" {
		t.Fatal()
	}
	env.Environ = map[string]string{"LLM_MODEL": "lmodel"}
	if ollamaDefaultModel(env) != "lmodel" {
		t.Fatal()
	}
	env.Environ = nil

	// chat args
	_, _, err := ollamaChatArgs(env, nil)
	if err == nil {
		t.Fatal("arity")
	}
	opts, prompt, err := ollamaChatArgs(env, []runtime.Value{runtime.Str("hi")})
	if err != nil || prompt != "hi" {
		t.Fatal(err, prompt)
	}
	_ = opts
	om := runtime.NewMap()
	omo := om.Obj.(*runtime.MapObj)
	omo.Keys = []string{"prompt", "system", "model"}
	omo.Vals["prompt"] = runtime.Str("p")
	omo.Vals["system"] = runtime.Str("s")
	omo.Vals["model"] = runtime.Str("m")
	opts, prompt, err = ollamaChatArgs(env, []runtime.Value{om})
	if err != nil || prompt != "p" {
		t.Fatal(err, prompt)
	}
	// message alias
	om2 := runtime.NewMap()
	om2.Obj.(*runtime.MapObj).Keys = []string{"message"}
	om2.Obj.(*runtime.MapObj).Vals["message"] = runtime.Str("msg")
	_, prompt, err = ollamaChatArgs(env, []runtime.Value{om2})
	if err != nil || prompt != "msg" {
		t.Fatal(err, prompt)
	}
	// missing prompt
	empty := runtime.NewMap()
	_, _, err = ollamaChatArgs(env, []runtime.Value{empty})
	if err == nil {
		t.Fatal("need prompt")
	}
	// second arg merge
	extra := runtime.NewMap()
	extra.Obj.(*runtime.MapObj).Keys = []string{"model"}
	extra.Obj.(*runtime.MapObj).Vals["model"] = runtime.Str("x")
	_, _, err = ollamaChatArgs(env, []runtime.Value{runtime.Str("hi"), extra})
	if err != nil {
		t.Fatal(err)
	}

	setMapStr(opts, "k", "v")
	if mapGetStr(opts, "k", "") != "v" {
		t.Fatal("setMapStr")
	}
	// non-map no-op
	setMapStr(runtime.Str("x"), "k", "v")
	if orStr("a", "b") != "a" || orStr("", "b") != "b" {
		t.Fatal("orStr")
	}

	// httptest mock daemon
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "llama3.2"}, {"name": "nomic"}},
			})
		case r.URL.Path == "/api/ps":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		case r.URL.Path == "/api/generate":
			_ = json.NewEncoder(w).Encode(map[string]any{"response": "gen-ok"})
		case r.URL.Path == "/api/pull":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success"})
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]any{"content": "chat-ok"}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	env = runtime.NewEnv()
	env.HTTPClient = srv.Client()
	env.Environ = map[string]string{"OLLAMA_HOST": srv.URL}
	// LLMDo for chat path through chatCompletions
	env.LLMDo = func(reqBody []byte) (string, []runtime.ToolCall, error) {
		return "chat-ok", nil, nil
	}

	p := packageOllama(env)
	host := callPkg(t, p, "host")
	if host.S != srv.URL {
		t.Fatal(host)
	}
	ob := callPkg(t, p, "openai_base")
	if !strings.HasSuffix(ob.S, "/v1") {
		t.Fatal(ob)
	}

	names := mustOk(t, callPkg(t, p, "list"))
	if names.Kind != runtime.KindList || len(names.Obj.(*runtime.ListObj).Items) < 2 {
		t.Fatalf("list %v", names)
	}
	// list with explicit host
	mustOk(t, callPkg(t, p, "list", runtime.Str(srv.URL)))

	ps := mustOk(t, callPkg(t, p, "ps"))
	_ = ps

	// generate model+prompt
	g := mustOk(t, callPkg(t, p, "generate", runtime.Str("llama3.2"), runtime.Str("hi")))
	if g.S != "gen-ok" {
		t.Fatal(g)
	}
	// generate opts map
	gm := runtime.NewMap()
	gmo := gm.Obj.(*runtime.MapObj)
	gmo.Keys = []string{"model", "prompt", "host"}
	gmo.Vals["model"] = runtime.Str("m")
	gmo.Vals["prompt"] = runtime.Str("p")
	gmo.Vals["host"] = runtime.Str(srv.URL)
	mustOk(t, callPkg(t, p, "generate", gm))
	// generate prompt-only
	mustOk(t, callPkg(t, p, "generate", runtime.Str("just prompt")))
	// empty prompt error
	mustErr(t, callPkg(t, p, "generate", runtime.NewMap()))

	// pull
	pl := mustOk(t, callPkg(t, p, "pull", runtime.Str("llama3.2")))
	if pl.S == "" {
		t.Fatal(pl)
	}
	mustErr(t, callPkg(t, p, "pull"))

	// chat
	ch := mustOk(t, callPkg(t, p, "chat", runtime.Str("hello")))
	if ch.S != "chat-ok" {
		t.Fatal(ch)
	}
	chm := runtime.NewMap()
	chm.Obj.(*runtime.MapObj).Keys = []string{"prompt", "system"}
	chm.Obj.(*runtime.MapObj).Vals["prompt"] = runtime.Str("p")
	chm.Obj.(*runtime.MapObj).Vals["system"] = runtime.Str("sys")
	mustOk(t, callPkg(t, p, "chat", chm))
	mustErr(t, callPkg(t, p, "chat"))

	// connect → client
	cli := mustOk(t, callPkg(t, p, "connect", runtime.Str(srv.URL)))
	mustOk(t, callMap(t, cli, "list"))
	mustOk(t, callMap(t, cli, "chat", runtime.Str("yo")))

	// connect fail
	mustErr(t, callPkg(t, p, "connect", runtime.Str("http://127.0.0.1:1")))
}

func TestVLLM_HelpersAndHTTPMock(t *testing.T) {
	env := runtime.NewEnv()
	if normalizeVLLMBase("http://h") != "http://h/v1" {
		t.Fatal(normalizeVLLMBase("http://h"))
	}
	if normalizeVLLMBase("http://h/v1/") != "http://h/v1" {
		t.Fatal()
	}

	env.Environ = map[string]string{"VLLM_BASE_URL": "http://v:8000"}
	if !strings.HasSuffix(vllmOpenAIBase(env), "/v1") {
		t.Fatal(vllmOpenAIBase(env))
	}
	env.Environ = map[string]string{"VLLM_HOST": "http://v2"}
	if !strings.Contains(vllmOpenAIBase(env), "v2") {
		t.Fatal()
	}
	env.Environ = map[string]string{"WEFT_API_BASE": "http://w", "WEFT_PROVIDER": "vllm"}
	if !strings.Contains(vllmOpenAIBase(env), "w") {
		t.Fatal()
	}
	env.Environ = map[string]string{"VLLM_MODEL": "vm"}
	if vllmDefaultModel(env) != "vm" {
		t.Fatal()
	}
	env.Environ = map[string]string{"WEFT_MODEL": "wm"}
	if vllmDefaultModel(env) != "wm" {
		t.Fatal()
	}
	env.Environ = map[string]string{"LLM_MODEL": "lm"}
	if vllmDefaultModel(env) != "lm" {
		t.Fatal()
	}
	env.Environ = map[string]string{"VLLM_API_KEY": "vk"}
	if vllmAPIKey(env) != "vk" {
		t.Fatal()
	}
	env.Environ = map[string]string{"OPENAI_API_KEY": "ok"}
	if vllmAPIKey(env) != "ok" {
		t.Fatal()
	}
	env.Environ = nil

	_, _, err := vllmChatArgs(env, nil)
	if err == nil {
		t.Fatal()
	}
	_, pmt, err := vllmChatArgs(env, []runtime.Value{runtime.Str("hi")})
	if err != nil || pmt != "hi" {
		t.Fatal(err, pmt)
	}
	om := runtime.NewMap()
	om.Obj.(*runtime.MapObj).Keys = []string{"prompt"}
	om.Obj.(*runtime.MapObj).Vals["prompt"] = runtime.Str("p")
	_, pmt, err = vllmChatArgs(env, []runtime.Value{om})
	if err != nil || pmt != "p" {
		t.Fatal()
	}
	om2 := runtime.NewMap()
	om2.Obj.(*runtime.MapObj).Keys = []string{"message"}
	om2.Obj.(*runtime.MapObj).Vals["message"] = runtime.Str("m")
	_, pmt, err = vllmChatArgs(env, []runtime.Value{om2})
	if err != nil || pmt != "m" {
		t.Fatal()
	}
	_, _, err = vllmChatArgs(env, []runtime.Value{runtime.NewMap()})
	if err == nil {
		t.Fatal()
	}
	ex := runtime.NewMap()
	ex.Obj.(*runtime.MapObj).Keys = []string{"system"}
	ex.Obj.(*runtime.MapObj).Vals["system"] = runtime.Str("s")
	_, _, err = vllmChatArgs(env, []runtime.Value{runtime.Str("hi"), ex})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health":
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`ok`))
		case r.URL.Path == "/v1/models" || r.URL.Path == "/models":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "meta-llama"}, {"id": "other"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	env = runtime.NewEnv()
	env.HTTPClient = srv.Client()
	env.Environ = map[string]string{
		"VLLM_BASE_URL": srv.URL + "/v1",
		"VLLM_API_KEY":  "testkey",
	}
	env.LLMDo = func(reqBody []byte) (string, []runtime.ToolCall, error) {
		return "vllm-chat", nil, nil
	}

	p := packageVLLM(env)
	base := callPkg(t, p, "base")
	if !strings.Contains(base.S, "v1") {
		t.Fatal(base)
	}

	h := mustOk(t, callPkg(t, p, "health"))
	if !h.B {
		t.Fatal(h)
	}
	mustOk(t, callPkg(t, p, "health", runtime.Str(srv.URL)))

	lst := mustOk(t, callPkg(t, p, "list"))
	if lst.Kind != runtime.KindList || len(lst.Obj.(*runtime.ListObj).Items) < 1 {
		t.Fatal(lst)
	}
	mustOk(t, callPkg(t, p, "list", runtime.Str(srv.URL+"/v1")))

	ch := mustOk(t, callPkg(t, p, "chat", runtime.Str("hi")))
	if ch.S != "vllm-chat" {
		t.Fatal(ch)
	}
	cm := runtime.NewMap()
	cm.Obj.(*runtime.MapObj).Keys = []string{"prompt", "system"}
	cm.Obj.(*runtime.MapObj).Vals["prompt"] = runtime.Str("p")
	cm.Obj.(*runtime.MapObj).Vals["system"] = runtime.Str("sys")
	mustOk(t, callPkg(t, p, "chat", cm))
	mustErr(t, callPkg(t, p, "chat"))

	cli := mustOk(t, callPkg(t, p, "connect", runtime.Str(srv.URL)))
	mustOk(t, callMap(t, cli, "list"))
	mustOk(t, callMap(t, cli, "chat", runtime.Str("yo")))
	mustOk(t, callMap(t, cli, "health"))

	// connect fail
	mustErr(t, callPkg(t, p, "connect", runtime.Str("http://127.0.0.1:1")))

	// health models-only server (no /health)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "models") {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "m"}}})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv2.Close()
	env2 := runtime.NewEnv()
	env2.HTTPClient = srv2.Client()
	p2 := packageVLLM(env2)
	// health tries /health fail then /models
	mustOk(t, callPkg(t, p2, "health", runtime.Str(srv2.URL+"/v1")))
	// connect health-only path when list works
	mustOk(t, callPkg(t, p2, "connect", runtime.Str(srv2.URL)))
}
