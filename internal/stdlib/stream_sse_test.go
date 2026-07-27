package stdlib

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// Prove OpenAI SSE yields the first text event before the response body is fully written.
func TestStreamOpenAISSEIncremental(t *testing.T) {
	pr, pw := io.Pipe()
	ch := make(chan runtime.Value, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamOpenAISSE(pr, ch)
		close(ch)
	}()

	// First event only — if we buffered whole body, consumer would block until Close.
	fmt.Fprint(pw, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")

	select {
	case ev := <-ch:
		mo := ev.Obj.(*runtime.MapObj)
		if mo.Vals["kind"].S != "text" || mo.Vals["text"].S != "hi" {
			t.Fatalf("first event: %v", mo.Vals)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first SSE event — body still open (buffering?)")
	}

	fmt.Fprint(pw, "data: {\"choices\":[{\"delta\":{\"content\":\"!\"}}]}\n\n")
	fmt.Fprint(pw, "data: [DONE]\n\n")
	_ = pw.Close()

	var parts []string
	for ev := range ch {
		mo := ev.Obj.(*runtime.MapObj)
		if mo.Vals["kind"].S == "text" {
			parts = append(parts, mo.Vals["text"].S)
		}
	}
	<-done
	if strings.Join(parts, "") != "!" {
		// "hi" already consumed; remaining should be "!"
		t.Fatalf("remaining text %q", parts)
	}
}

func TestStreamAnthropicSSE(t *testing.T) {
	body := strings.Join([]string{
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Yo"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"!"}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")
	ch := make(chan runtime.Value, 8)
	go func() {
		streamAnthropicSSE(strings.NewReader(body), ch)
		close(ch)
	}()
	var text strings.Builder
	var kinds []string
	for ev := range ch {
		mo := ev.Obj.(*runtime.MapObj)
		k := mo.Vals["kind"].S
		kinds = append(kinds, k)
		if k == "text" {
			text.WriteString(mo.Vals["text"].S)
		}
	}
	if text.String() != "Yo!" {
		t.Fatalf("text=%q kinds=%v", text.String(), kinds)
	}
	if kinds[len(kinds)-1] != "done" {
		t.Fatalf("want trailing done, got %v", kinds)
	}
}

// Live http: first chunk flushed, second delayed — consumer must see "A" while server still open.
func TestLLMStreamHTTPLiveChunks(t *testing.T) {
	var second atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("no flusher")
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n")
		flusher.Flush()
		// Hold open until test sets flag or timeout
		deadline := time.Now().Add(3 * time.Second)
		for !second.Load() && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"B\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	env := runtime.NewEnv()
	env.HTTPClient = srv.Client()

	opts := runtime.NewMap()
	mo := opts.Obj.(*runtime.MapObj)
	mo.Keys = []string{"base_url", "api_key", "model"}
	mo.Vals["base_url"] = runtime.Str(srv.URL + "/v1")
	mo.Vals["api_key"] = runtime.Str("test")
	mo.Vals["model"] = runtime.Str("mock")

	msgs := []map[string]any{{"role": "user", "content": "hi"}}
	it, err := chatStreamIter(env, opts, msgs)
	if err != nil {
		t.Fatal(err)
	}

	// First Next should return "A" while server still waiting
	v, ok := it.Next()
	if !ok {
		t.Fatal("empty stream")
	}
	em := v.Obj.(*runtime.MapObj)
	if em.Vals["kind"].S != "text" || em.Vals["text"].S != "A" {
		t.Fatalf("first event %+v", em.Vals)
	}

	// Release second chunk
	second.Store(true)

	var rest strings.Builder
	for {
		v, ok := it.Next()
		if !ok {
			break
		}
		em := v.Obj.(*runtime.MapObj)
		if em.Vals["kind"].S == "text" {
			rest.WriteString(em.Vals["text"].S)
		}
	}
	if rest.String() != "B" {
		t.Fatalf("rest=%q", rest.String())
	}
}
