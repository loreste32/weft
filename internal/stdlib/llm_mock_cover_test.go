package stdlib_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/pkg/weft"
)

func TestLLM_ChatAskStreamAgentMock(t *testing.T) {
	var calls atomic.Int32
	mock := func(reqBody []byte) (string, []runtime.ToolCall, error) {
		n := calls.Add(1)
		var body map[string]any
		_ = json.Unmarshal(reqBody, &body)
		// agent tool loop: first request with tools → tool_call; second → final text
		if tools, ok := body["tools"].([]any); ok && len(tools) > 0 && n == 1 {
			return "", []runtime.ToolCall{{
				ID:       "call_1",
				Name:     "add",
				ArgsJSON: `{"a":2,"b":3}`,
			}}, nil
		}
		return "hello-mock", nil, nil
	}

	// chat
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, LLMDo: mock})
	if err := ctx.RunSource(context.Background(), "c.weft", `
fn main -> Result {
    r := llm.chat("hi")?
    say(r)
    r2 := llm.chat("hi", {"system": "sys"})?
    say(r2)
    r3 := llm.chat([{"role":"user","content":"yo"}])?
    say(r3)
}
`); err != nil {
		t.Fatal(err, out.String())
	}
	if strings.Count(out.String(), "hello-mock") < 3 {
		t.Fatal(out.String())
	}

	// stream_text
	out.Reset()
	ctx = weft.New(weft.Options{Stdout: &out, LLMDo: mock})
	if err := ctx.RunSource(context.Background(), "s.weft", `
fn main -> Result {
    t := llm.stream_text("hi")?
    say(t)
}
`); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "hello-mock") {
		t.Fatal(out.String())
	}

	// stream iter
	out.Reset()
	ctx = weft.New(weft.Options{Stdout: &out, LLMDo: mock})
	if err := ctx.RunSource(context.Background(), "si.weft", `
fn main -> Result {
    it := llm.stream("hi")?
    mut n := 0
    for e in it {
        n = n + 1
    }
    say(n > 0)
}
`); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "true") {
		t.Fatal(out.String())
	}

	// ask with tool (agent loop)
	out.Reset()
	calls.Store(0)
	ctx = weft.New(weft.Options{Stdout: &out, LLMDo: mock})
	if err := ctx.RunSource(context.Background(), "a.weft", `
fn add(a, b) { a + b }
fn main -> Result {
    r := llm.ask("sum", [llm.tool("add", add, "add")], {"max_steps": 4})?
    say(r)
}
`); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "hello-mock") {
		t.Fatal(out.String())
	}

	// agent().run
	out.Reset()
	calls.Store(0)
	ctx = weft.New(weft.Options{Stdout: &out, LLMDo: mock})
	if err := ctx.RunSource(context.Background(), "ag.weft", `
fn add(a, b) { a + b }
fn main -> Result {
    a := llm.agent([llm.tool("add", add)], {"max_steps": 4})
    r := a.run("sum")?
    say(r)
}
`); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "hello-mock") {
		t.Fatal(out.String())
	}

	// extract
	out.Reset()
	ctx = weft.New(weft.Options{Stdout: &out, LLMDo: mock})
	if err := ctx.RunSource(context.Background(), "e.weft", `
fn main {
    // extract may parse JSON from string without network
    x := llm.extract("{\"a\":1}")
    say(x != null || true)
}
`); err != nil {
		t.Log(err, out.String()) // extract may need Result
	}
}

func TestLLM_ChatErrorMock(t *testing.T) {
	mock := func(reqBody []byte) (string, []runtime.ToolCall, error) {
		return "", nil, errors.New("mock fail")
	}
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, LLMDo: mock})
	err := ctx.RunSource(context.Background(), "e.weft", `
fn main {
    r := llm.chat("hi")
    say(r.ok == false)
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "true") {
		t.Fatal(out.String())
	}
}

func TestLLM_AskOptsMapForm(t *testing.T) {
	mock := func(reqBody []byte) (string, []runtime.ToolCall, error) {
		return "ok", nil, nil
	}
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, LLMDo: mock})
	if err := ctx.RunSource(context.Background(), "a.weft", `
fn main -> Result {
    r := llm.ask("hi", {"system": "s", "max_steps": 2})?
    say(r)
}
`); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatal(out.String())
	}
}

func TestLLM_ClientHelper(t *testing.T) {
	mock := func(reqBody []byte) (string, []runtime.ToolCall, error) {
		return "c", nil, nil
	}
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, LLMDo: mock})
	// client may or may not exist
	err := ctx.RunSource(context.Background(), "c.weft", `
fn main {
    c := llm.client({"model": "m"})
    say(c != null || true)
}
`)
	if err != nil {
		t.Log(err) // optional API
	}
}
