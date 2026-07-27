package stdlib

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/loreste/weft/internal/netsafe"
	"github.com/loreste/weft/internal/runtime"
)

func pkg() runtime.Value {
	return runtime.NewMap()
}

func set(m runtime.Value, name string, fn runtime.Builtin, arity int) {
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = append(mo.Keys, name)
	mo.Vals[name] = runtime.MakeBuiltin(name, arity, fn)
}

func getenv(env *runtime.Env, key string) (string, bool) {
	if env.Environ != nil {
		v, ok := env.Environ[key]
		return v, ok
	}
	v, ok := os.LookupEnv(key)
	return v, ok
}

func mapGet(m runtime.Value, key string) (runtime.Value, bool) {
	if m.Kind != runtime.KindMap {
		return runtime.Null(), false
	}
	mo := m.Obj.(*runtime.MapObj)
	v, ok := mo.Vals[key]
	return v, ok
}

func mapGetStr(m runtime.Value, key, def string) string {
	v, ok := mapGet(m, key)
	if !ok || v.Kind == runtime.KindNull {
		return def
	}
	return v.String()
}

func mapGetInt(m runtime.Value, key string, def int64) int64 {
	v, ok := mapGet(m, key)
	if !ok {
		return def
	}
	n, err := runtime.AsInt(v)
	if err != nil {
		return def
	}
	return n
}

func asMap(v runtime.Value) (map[string]any, error) {
	switch v.Kind {
	case runtime.KindMap:
		mo := v.Obj.(*runtime.MapObj)
		out := make(map[string]any, len(mo.Vals))
		for k, val := range mo.Vals {
			out[k] = valueToGo(val)
		}
		return out, nil
	case runtime.KindStruct:
		so := v.Obj.(*runtime.StructObj)
		// Never expand Secret fields — valueToGo redacts nested Secrets but
		// asMap was walking Fields and leaking the raw "value" string.
		if so.TypeName == "Secret" {
			return nil, fmt.Errorf("Secret fields are sealed; use secrets.unwrap")
		}
		out := make(map[string]any, len(so.Fields))
		for k, val := range so.Fields {
			out[k] = valueToGo(val)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected map, got %s", v.KindName())
	}
}

func valueToGo(v runtime.Value) any {
	switch v.Kind {
	case runtime.KindNull:
		return nil
	case runtime.KindBool:
		return v.B
	case runtime.KindInt:
		return v.I
	case runtime.KindFloat:
		return v.F
	case runtime.KindStr:
		return v.S
	case runtime.KindList:
		lo := v.Obj.(*runtime.ListObj)
		out := make([]any, len(lo.Items))
		for i, it := range lo.Items {
			out[i] = valueToGo(it)
		}
		return out
	case runtime.KindMap:
		mo := v.Obj.(*runtime.MapObj)
		out := make(map[string]any, len(mo.Vals))
		for k, val := range mo.Vals {
			out[k] = valueToGo(val)
		}
		return out
	case runtime.KindStruct:
		so := v.Obj.(*runtime.StructObj)
		// Never serialize secret material into JSON/HTTP bodies.
		if so.TypeName == "Secret" {
			return "***"
		}
		out := make(map[string]any, len(so.Fields))
		for k, val := range so.Fields {
			out[k] = valueToGo(val)
		}
		return out
	case runtime.KindResult:
		ro := v.Obj.(*runtime.ResultObj)
		if ro.Ok {
			return valueToGo(ro.Val)
		}
		return map[string]any{"error": ro.Err.String()}
	default:
		return v.String()
	}
}

func goToValue(x any) runtime.Value {
	switch x := x.(type) {
	case nil:
		return runtime.Null()
	case bool:
		return runtime.Bool(x)
	case int:
		return runtime.Int(int64(x))
	case int8:
		return runtime.Int(int64(x))
	case int16:
		return runtime.Int(int64(x))
	case int32:
		return runtime.Int(int64(x))
	case int64:
		return runtime.Int(x)
	case uint:
		return runtime.Int(int64(x))
	case uint8:
		return runtime.Int(int64(x))
	case uint16:
		return runtime.Int(int64(x))
	case uint32:
		return runtime.Int(int64(x))
	case uint64:
		return runtime.Int(int64(x))
	case float32:
		return runtime.Float(float64(x))
	case float64:
		if x == float64(int64(x)) {
			return runtime.Int(int64(x))
		}
		return runtime.Float(x)
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return runtime.Int(i)
		}
		f, _ := x.Float64()
		return runtime.Float(f)
	case string:
		return runtime.Str(x)
	case []any:
		items := make([]runtime.Value, len(x))
		for i, it := range x {
			items[i] = goToValue(it)
		}
		return runtime.List(items...)
	case map[string]any:
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		for k, v := range x {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = goToValue(v)
		}
		return m
	// yaml.v2-style maps (defensive)
	case map[any]any:
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		for k, v := range x {
			ks := fmt.Sprint(k)
			mo.Keys = append(mo.Keys, ks)
			mo.Vals[ks] = goToValue(v)
		}
		return m
	default:
		return runtime.Str(fmt.Sprint(x))
	}
}

func errRes(msg, kind string) runtime.Value {
	return runtime.Err(runtime.NewError(msg, kind))
}

// DefaultHTTPClient is used when host does not inject one (bounded timeout + SSRF guard).
// Private/link-local/metadata destinations are blocked unless WEFT_HTTP_ALLOW_PRIVATE=1.
func DefaultHTTPClient() *http.Client {
	return netsafe.SafeHTTPClient(30 * time.Second)
}
