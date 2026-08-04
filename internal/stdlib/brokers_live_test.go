//go:build !js && !slim

package stdlib

import (
	"os"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// Live broker tests. Pattern (roadmap N6): when the broker is unreachable the
// test reports itself unavailable via t.Skip — UNLESS WEFT_LIVE_REQUIRED=1 is
// set (CI live-services job), in which case an unreachable broker is a FAILURE.
// Once connected, every subsequent failure is a real failure, never a skip.

func liveRequired() bool { return os.Getenv("WEFT_LIVE_REQUIRED") == "1" }

// liveConnectResult extracts the Ok value or reports unavailable/failure.
func liveConnectResult(t *testing.T, broker string, r runtime.Value) runtime.Value {
	t.Helper()
	ro, ok := r.Obj.(*runtime.ResultObj)
	if !ok || !ro.Ok {
		if liveRequired() {
			t.Fatalf("%s broker required but unreachable: %v", broker, r)
		}
		t.Skipf("no local %s broker", broker)
	}
	return ro.Val
}

func liveEnv() *runtime.Env {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		if fn.Kind == runtime.KindBuiltin {
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		}
		return runtime.Unit(), nil
	}
	return env
}

func TestAMQP_Live(t *testing.T) {
	env := liveEnv()
	p := packageAMQP(env)
	ch := liveConnectResult(t, "amqp", callPkg(t, p, "connect", runtime.Str("")))
	defer callMap(t, ch, "close")

	qname := "weft.live.test"
	q := mustOk(t, callMap(t, ch, "queue_declare", runtime.Str(qname)))
	if name := q.Obj.(*runtime.MapObj).Vals["name"].S; name != qname {
		t.Fatalf("queue name = %q, want %q", name, qname)
	}

	got := make(chan string, 1)
	handler := runtime.MakeBuiltin("h", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) == 1 {
			if m, ok := args[0].Obj.(*runtime.MapObj); ok {
				got <- m.Vals["data"].S
			}
		}
		return runtime.Unit(), nil
	})
	mustOk(t, callMap(t, ch, "consume", runtime.Str(qname), handler))
	mustOk(t, callMap(t, ch, "publish", runtime.Str(""), runtime.Str(qname), runtime.Str("live-payload")))

	select {
	case body := <-got:
		if body != "live-payload" {
			t.Fatalf("consumed body = %q, want live-payload", body)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for consumed message")
	}
}

func TestMongo_Live(t *testing.T) {
	p := packageMongo(liveEnv())
	m := liveConnectResult(t, "mongo", callPkg(t, p, "connect", runtime.Str("")))
	defer callMap(t, m, "close")

	mustOk(t, callMap(t, m, "ping"))
	col := callMap(t, m, "collection", runtime.Str("weft_live"), runtime.Str("items"))

	doc := runtime.NewMap()
	dmo := doc.Obj.(*runtime.MapObj)
	dmo.Keys = []string{"name", "n"}
	dmo.Vals["name"] = runtime.Str("live-widget")
	dmo.Vals["n"] = runtime.Int(7)

	ins := mustOk(t, callMap(t, col, "insert", doc))
	if id := ins.Obj.(*runtime.MapObj).Vals["id"].S; id == "" {
		t.Fatal("insert returned empty id")
	}

	cnt := mustOk(t, callMap(t, col, "count", doc))
	if cnt.Kind != runtime.KindInt || cnt.I < 1 {
		t.Fatalf("count after insert = %v, want >= 1", cnt)
	}

	found := mustOk(t, callMap(t, col, "find", doc))
	items, ok := found.Obj.(*runtime.ListObj)
	if !ok || len(items.Items) < 1 {
		t.Fatalf("find returned %v, want >= 1 doc", found)
	}
	first := items.Items[0].Obj.(*runtime.MapObj)
	if name := first.Vals["name"].S; name != "live-widget" {
		t.Fatalf("found doc name = %q, want live-widget", name)
	}

	mustOk(t, callMap(t, col, "delete", doc, runtime.Bool(true)))
	after := mustOk(t, callMap(t, col, "count", doc))
	if after.Kind != runtime.KindInt || after.I != 0 {
		t.Fatalf("count after delete = %v, want 0", after)
	}
}
