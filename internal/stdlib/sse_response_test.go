package stdlib

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestWriteWeftResponseSSEList(t *testing.T) {
	src := runtime.List(
		runtime.Str("hello"),
		runtime.Str("world"),
	)
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"status", "type", "stream", "sse"}
	mo.Vals["status"] = runtime.Int(200)
	mo.Vals["type"] = runtime.Str("text/event-stream")
	mo.Vals["stream"] = src
	mo.Vals["sse"] = runtime.Bool(true)

	rec := httptest.NewRecorder()
	writeWeftResponse(rec, m)
	body := rec.Body.String()
	if !strings.Contains(body, "data: hello\n\n") || !strings.Contains(body, "data: world\n\n") {
		t.Fatalf("body=%q", body)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("ctype=%q", ct)
	}
}

func TestWriteWeftResponseSSEllmEvents(t *testing.T) {
	ev1 := event("text", map[string]runtime.Value{"text": runtime.Str("hi")})
	ev2 := event("done", nil)
	src := runtime.List(ev1, ev2)
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"stream", "sse", "type", "status"}
	mo.Vals["stream"] = src
	mo.Vals["sse"] = runtime.Bool(true)
	mo.Vals["type"] = runtime.Str("text/event-stream")
	mo.Vals["status"] = runtime.Int(200)

	rec := httptest.NewRecorder()
	writeWeftResponse(rec, m)
	body := rec.Body.String()
	if body != "data: hi\n\ndata: [DONE]\n\n" {
		t.Fatalf("body=%q", body)
	}
}

func TestFormatSSEChunkMap(t *testing.T) {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"event", "data"}
	mo.Vals["event"] = runtime.Str("ping")
	mo.Vals["data"] = runtime.Str("{}")
	got := formatSSEChunk(m, true)
	if got != "event: ping\ndata: {}\n\n" {
		t.Fatalf("%q", got)
	}
}

func TestWebSSEHelper(t *testing.T) {
	p := packageWeb(runtime.NewEnv())
	fn, ok := mapGet(p, "sse")
	if !ok {
		t.Fatal("no web.sse")
	}
	ret, err := fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.List(runtime.Str("a"))})
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := mapGet(ret, "sse"); !ok || !v.IsTruthy() {
		t.Fatal("sse flag")
	}
	if v, ok := mapGet(ret, "stream"); !ok || v.Kind != runtime.KindList {
		t.Fatal("stream")
	}
}
