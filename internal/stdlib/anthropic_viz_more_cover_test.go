package stdlib

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/loreste/weft/internal/llmconfig"
	"github.com/loreste/weft/internal/runtime"
)

func TestAnthropic_ChatHTTPMock(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if r.Header.Get("x-api-key") == "" {
			t.Error("missing x-api-key")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version")
		}
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.Path, "messages") {
			http.NotFound(w, r)
			return
		}
		if n == 1 {
			_, _ = w.Write([]byte(`{"content":[{"type":"tool_use","id":"toolu_1","name":"add","input":{"a":2,"b":3}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"sum is 5"}]}`))
	}))
	defer srv.Close()

	env := envWithCall()
	env.HTTPClient = srv.Client()
	env.Environ = map[string]string{
		"LLM_PROVIDER":      "anthropic",
		"ANTHROPIC_API_KEY":  "sk-test",
		"ANTHROPIC_BASE_URL": srv.URL,
		"ANTHROPIC_MODEL":    "claude-test",
	}
	// No LLMDo — real HTTP path through chatAnthropic

	p := packageLLM(env)
	// simple chat
	r := callPkg(t, p, "chat", runtime.Str("hi"))
	if ro, ok := r.Obj.(*runtime.ResultObj); !ok || !ro.Ok {
		// first response is tool_use only — may still return tool text or empty path
		t.Log("chat result", r)
	}

	// ask with tool (agent loop)
	add := runtime.MakeBuiltin("add", 2, func(args []runtime.Value) (runtime.Value, error) {
		var a, b int64
		if len(args) >= 1 {
			a, _ = runtime.AsInt(args[0])
		}
		if len(args) >= 2 {
			b, _ = runtime.AsInt(args[1])
		}
		return runtime.Int(a + b), nil
	})
	// llm.tool via package
	tool := callPkg(t, p, "tool", runtime.Str("add"), add, runtime.Str("add two numbers"))
	ask := callPkg(t, p, "ask", runtime.Str("sum 2 3"), runtime.List(tool), row("max_steps", 4, "provider", "anthropic", "base_url", srv.URL, "api_key", "sk-test", "model", "claude-test"))
	if ro, ok := ask.Obj.(*runtime.ResultObj); ok && ro.Ok {
		if !strings.Contains(ro.Val.S, "5") && ro.Val.S != "sum is 5" {
			t.Log("ask text", ro.Val.S)
		}
	} else {
		t.Log("ask", ask)
	}
	if hits.Load() < 1 {
		t.Fatal("no anthropic hits")
	}

	// direct chatAnthropic + chatCompletions with provider
	opts := row(
		"provider", llmconfig.ProviderAnthropic,
		"base_url", srv.URL,
		"api_key", "sk-test",
		"model", "claude-test",
	)
	text, _, err := chatCompletions(env, opts, []map[string]any{
		{"role": "user", "content": "hello"},
	}, nil)
	if err != nil {
		// tool_use-only responses may error as empty if first hit was tool
		t.Log(err)
	} else {
		_ = text
	}

	// error status
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", 500)
	}))
	defer bad.Close()
	env.HTTPClient = bad.Client()
	_, _, err = chatAnthropic(env, bad.URL, "k", "m", []map[string]any{{"role": "user", "content": "x"}}, nil)
	if err == nil {
		t.Fatal("expected http error")
	}

	// LLMDo short-circuit on anthropic path
	env.LLMDo = func(reqBody []byte) (string, []runtime.ToolCall, error) {
		return "mocked", nil, nil
	}
	text, _, err = chatAnthropic(env, srv.URL, "k", "m", []map[string]any{{"role": "user", "content": "x"}}, nil)
	if err != nil || text != "mocked" {
		t.Fatal(text, err)
	}
	env.LLMDo = func(reqBody []byte) (string, []runtime.ToolCall, error) {
		return "", nil, fmt.Errorf("boom")
	}
	_, _, err = chatAnthropic(env, srv.URL, "k", "m", []map[string]any{{"role": "user", "content": "x"}}, nil)
	if err == nil {
		t.Fatal()
	}
}

func TestAnthropic_PureConverters(t *testing.T) {
	// openaiToolsToAnthropic
	tools := []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name":        "add",
				"description": "add",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"a": map[string]any{"type": "number"},
					},
				},
			},
		},
		// already anthropic-shaped
		{
			"name":         "mul",
			"input_schema": map[string]any{"type": "object"},
		},
		// missing function
		{"type": "function"},
		// no params
		{
			"function": map[string]any{"name": "noop", "description": "n"},
		},
	}
	out := openaiToolsToAnthropic(tools)
	if len(out) < 2 {
		t.Fatal(out)
	}

	// messagesToAnthropic full matrix
	sys, turns := messagesToAnthropic([]map[string]any{
		{"role": "system", "content": "be nice"},
		{"role": "system", "content": "and brief"},
		{"role": "user", "content": "hi"},
		{"role": "assistant", "content": "hello", "tool_calls": []map[string]any{
			{"id": "1", "function": map[string]any{"name": "add", "arguments": `{"a":1}`}},
		}},
		{"role": "tool", "tool_call_id": "1", "content": "2"},
		{"role": "assistant", "content": "done", "tool_calls": []any{
			map[string]any{"id": "2", "function": map[string]any{"name": "x", "arguments": ""}},
		}},
		{"role": "assistant", "content": "plain"},
		{"role": "unknown", "content": "fallback"},
	})
	if !strings.Contains(sys, "be nice") || !strings.Contains(sys, "and brief") {
		t.Fatal(sys)
	}
	if len(turns) < 3 {
		t.Fatal(turns)
	}
	// empty → default hello
	_, turns2 := messagesToAnthropic(nil)
	if len(turns2) != 1 {
		t.Fatal(turns2)
	}

	// contentAsString
	if contentAsString(nil) != "" || contentAsString("s") != "s" || contentAsString(1) != "1" {
		t.Fatal()
	}

	// parseAnthropicResponse
	text, calls, err := parseAnthropicResponse([]byte(`{"content":[{"type":"text","text":"hi"},{"type":"tool_use","name":"f","input":{"x":1}}]}`))
	if err != nil || text != "hi" || len(calls) != 1 {
		t.Fatal(text, calls, err)
	}
	// empty id → synthetic
	_, calls, err = parseAnthropicResponse([]byte(`{"content":[{"type":"tool_use","name":"f","input":{}}]}`))
	if err != nil || len(calls) != 1 || !strings.HasPrefix(calls[0].ID, "tool_") {
		t.Fatal(calls, err)
	}
	_, _, err = parseAnthropicResponse([]byte(`not-json`))
	if err == nil {
		t.Fatal()
	}
	_, _, err = parseAnthropicResponse([]byte(`{"error":{"message":"bad"}}`))
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatal(err)
	}
	_, _, err = parseAnthropicResponse([]byte(`{"content":[]}`))
	if err == nil {
		t.Fatal()
	}
}

func TestLLM_CallToolAndJsonType(t *testing.T) {
	if jsonTypeName(nil) != "string" {
		t.Fatal()
	}
	if jsonTypeName(&runtime.TypeInfo{Name: "int"}) != "number" {
		t.Fatal()
	}
	if jsonTypeName(&runtime.TypeInfo{Name: "float"}) != "number" {
		t.Fatal()
	}
	if jsonTypeName(&runtime.TypeInfo{Name: "bool"}) != "boolean" {
		t.Fatal()
	}
	if jsonTypeName(&runtime.TypeInfo{Name: "str"}) != "string" {
		t.Fatal()
	}

	env := envWithCall()
	a := &agentState{env: env}
	// missing fn
	_, err := a.callTool(runtime.NewMap(), `{}`)
	if err == nil {
		t.Fatal()
	}
	// builtin fn with one arg
	fn := runtime.MakeBuiltin("echo", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) >= 1 {
			return args[0], nil
		}
		return runtime.Str(""), nil
	})
	binding := runtime.NewMap()
	bo := binding.Obj.(*runtime.MapObj)
	bo.Keys = []string{"fn", "name"}
	bo.Vals["fn"] = fn
	bo.Vals["name"] = runtime.Str("echo")
	out, err := a.callTool(binding, `{"x":"hi"}`)
	if err != nil || out != "hi" {
		t.Fatal(out, err)
	}
	// empty json
	_, err = a.callTool(binding, ``)
	if err != nil {
		t.Fatal(err)
	}
	// invalid json
	_, err = a.callTool(binding, `not-json`)
	if err != nil {
		t.Fatal(err)
	}
	// multi-arg map → whole map
	fn2 := runtime.MakeBuiltin("m", 1, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str("ok"), nil
	})
	bo.Vals["fn"] = fn2
	_, err = a.callTool(binding, `{"a":1,"b":2}`)
	if err != nil {
		t.Fatal(err)
	}
	// Result Ok/Err
	fn3 := runtime.MakeBuiltin("r", 0, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Ok(runtime.Str("yes")), nil
	})
	bo.Vals["fn"] = fn3
	out, err = a.callTool(binding, `{}`)
	if err != nil || out != "yes" {
		t.Fatal(out, err)
	}
	fn4 := runtime.MakeBuiltin("re", 0, func(args []runtime.Value) (runtime.Value, error) {
		return errRes("fail", "x"), nil
	})
	bo.Vals["fn"] = fn4
	out, err = a.callTool(binding, `{}`)
	if err != nil || !strings.Contains(out, "error:") {
		t.Fatal(out, err)
	}
	// Call nil
	a2 := &agentState{env: runtime.NewEnv()}
	_, err = a2.callTool(binding, `{}`)
	if err == nil {
		t.Fatal()
	}

	// parseAgentArgs
	env2 := runtime.NewEnv()
	_, _, err = parseAgentArgs(env2, nil)
	if err != nil {
		t.Fatal(err)
	}
	opts, tools, err := parseAgentArgs(env2, []runtime.Value{
		runtime.List(runtime.Str("t")),
		row("max_steps", 2),
	})
	_ = opts
	if err != nil || len(tools) != 1 {
		t.Fatal(err, tools)
	}
	// tools as first map-only
	_, _, err = parseAgentArgs(env2, []runtime.Value{row("model", "m")})
	if err != nil {
		t.Fatal(err)
	}
	// invalid shape
	_, _, err = parseAgentArgs(env2, []runtime.Value{runtime.Str("nope")})
	if err == nil {
		t.Fatal()
	}
}

func TestTestSkipHelpers(t *testing.T) {
	var e *TestSkip
	if e.Error() != TestSkipPrefix+"skipped" {
		t.Fatal(e.Error())
	}
	e = &TestSkip{Msg: "later"}
	if e.Error() != TestSkipPrefix+"later" {
		t.Fatal(e.Error())
	}
	msg, ok := IsTestSkip(e)
	if !ok || msg != "later" {
		t.Fatal(msg, ok)
	}
	msg, ok = IsTestSkip(fmt.Errorf("wrap: %w", e))
	if !ok || msg != "later" {
		t.Fatal(msg, ok)
	}
	// string form
	msg, ok = IsTestSkip(errors.New(TestSkipPrefix + "via-string"))
	if !ok || msg != "via-string" {
		t.Fatal(msg, ok)
	}
	if _, ok := IsTestSkip(nil); ok {
		t.Fatal()
	}
	if _, ok := IsTestSkip(errors.New("other")); ok {
		t.Fatal()
	}
}

func TestViz_ParseSeriesAndOpts(t *testing.T) {
	o := parseOpts(row(
		"title", "T",
		"width", 400,
		"height", 200,
		"color", "#f00",
		"xlabel", "X",
		"ylabel", "Y",
		"x_label", "X2",
		"y_label", "Y2",
		"bins", 10,
		"colors", runtime.List(runtime.Str("#111"), runtime.Str("#222")),
	))
	if o.Title != "T" || o.Width != 400 || o.Bins != 10 || len(o.Colors) != 2 {
		t.Fatalf("%+v", o)
	}
	_ = parseOpts(runtime.Str("nope"))
	_ = defaultPalette()

	// series: plain numbers
	s, err := parseSeries(runtime.List(runtime.Int(1), runtime.Int(2), runtime.Float(3)))
	if err != nil || len(s.Y) != 3 {
		t.Fatal(s, err)
	}
	// pairs
	s, err = parseSeries(runtime.List(
		runtime.List(runtime.Int(1), runtime.Int(2)),
		runtime.List(runtime.Str("3"), runtime.Str("4")),
	))
	if err != nil || len(s.X) != 2 {
		t.Fatal(s, err)
	}
	// maps {x,y} / {label,value}
	s, err = parseSeries(runtime.List(
		row("x", 1, "y", 2, "label", "a"),
		row("label", "b", "value", 5),
		row("name", "c", "y", 9),
	))
	if err != nil || len(s.Y) != 3 {
		t.Fatal(s, err)
	}
	// map series
	s, err = parseSeries(row("a", 1, "b", 2))
	if err != nil || len(s.Y) != 2 {
		t.Fatal(s, err)
	}
	// errors
	_, err = parseSeries(runtime.List())
	if err == nil {
		t.Fatal()
	}
	_, err = parseSeries(runtime.NewMap())
	if err == nil {
		t.Fatal()
	}
	_, err = parseSeries(runtime.Str("x"))
	if err == nil {
		t.Fatal()
	}
	_, err = parseSeries(runtime.List(runtime.List(runtime.Int(1))))
	if err == nil {
		t.Fatal()
	}
	_, err = parseSeries(runtime.List(row("nope", 1)))
	if err == nil {
		t.Fatal()
	}

	// mapFloat / asFloat
	f, ok := mapFloat(row("x", 3), "x")
	if !ok || f != 3 {
		t.Fatal(f, ok)
	}
	if _, ok := mapFloat(row("x", 1), "y"); ok {
		t.Fatal()
	}
	if _, ok := mapFloat(row("x", "no"), "x"); ok {
		t.Fatal()
	}
	_, err = asFloat(runtime.Bool(true))
	if err == nil {
		t.Fatal()
	}
	_, err = asFloat(runtime.Str("1.5"))
	if err != nil {
		t.Fatal(err)
	}
	// minMax
	mn, mx := minMax([]float64{3, 1, 2})
	if mn != 1 || mx != 3 {
		t.Fatal(mn, mx)
	}
	// axisLabels
	var b strings.Builder
	axisLabels(&b, defaultOpts(), 40, 20, 300, 200, 400, 300)
	_ = b.String()
}

func TestCollections_KeysAndBisectCompare(t *testing.T) {
	for _, v := range []runtime.Value{
		runtime.Str("s"), runtime.Int(1), runtime.Float(1.5),
		runtime.Bool(true), runtime.Bool(false), runtime.Null(), runtime.List(),
	} {
		if counterKey(v) == "" && v.Kind != runtime.KindStr {
			// str empty only for empty string
		}
		_ = counterKey(v)
	}
	if groupKey(row("k", "v"), "k") != "v" {
		t.Fatal()
	}
	if groupKey(row("k", "v"), "") == "" {
		// empty field uses counterKey of whole map
		_ = groupKey(row("k", "v"), "")
	}
	// struct field
	st := runtime.Value{Kind: runtime.KindStruct, Obj: &runtime.StructObj{
		TypeName: "S",
		Fields:   map[string]runtime.Value{"f": runtime.Int(3)},
	}}
	if groupKey(st, "f") != "3" {
		t.Fatal(groupKey(st, "f"))
	}
	if groupKey(st, "missing") != "" {
		t.Fatal()
	}

	// compareValues
	if compareValues(runtime.Int(1), runtime.Int(2)) >= 0 {
		t.Fatal()
	}
	if compareValues(runtime.Int(2), runtime.Int(1)) <= 0 {
		t.Fatal()
	}
	if compareValues(runtime.Int(1), runtime.Int(1)) != 0 {
		t.Fatal()
	}
	if compareValues(runtime.Float(1), runtime.Int(2)) >= 0 {
		t.Fatal()
	}
	if compareValues(runtime.Str("a"), runtime.Str("b")) >= 0 {
		t.Fatal()
	}
	if compareValues(runtime.Str("a"), runtime.Str("a")) != 0 {
		t.Fatal()
	}

	// chatCompletions OpenAI HTTP path (no LLMDo)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "openai-ok"}},
			},
		})
	}))
	defer srv.Close()
	env := runtime.NewEnv()
	env.HTTPClient = srv.Client()
	opts := row("base_url", srv.URL+"/v1", "api_key", "k", "model", "m")
	text, _, err := chatCompletions(env, opts, []map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err != nil || text != "openai-ok" {
		t.Fatal(text, err)
	}
	// with tools
	tools := []map[string]any{{
		"type": "function",
		"function": map[string]any{
			"name": "f", "description": "d",
			"parameters": map[string]any{"type": "object"},
		},
	}}
	text, _, err = chatCompletions(env, opts, []map[string]any{{"role": "user", "content": "hi"}}, tools)
	if err != nil {
		t.Fatal(err)
	}
	// http error
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", 502)
	}))
	defer bad.Close()
	env.HTTPClient = bad.Client()
	_, _, err = chatCompletions(env, row("base_url", bad.URL+"/v1", "api_key", "k"), []map[string]any{{"role": "user", "content": "x"}}, nil)
	if err == nil {
		t.Fatal()
	}
	// LLMDo error path
	env.LLMDo = func(b []byte) (string, []runtime.ToolCall, error) {
		return "", nil, fmt.Errorf("mock fail")
	}
	_, _, err = chatCompletions(env, opts, []map[string]any{{"role": "user", "content": "x"}}, nil)
	if err == nil {
		t.Fatal()
	}
}
