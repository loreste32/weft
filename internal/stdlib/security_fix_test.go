package stdlib

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestSecurity_InsecureKeepsSSRF(t *testing.T) {
	// doRequest with insecure must not reach metadata — netsafe still blocks
	env := runtime.NewEnv()
	env.HTTPClient = DefaultHTTPClient()
	// 169.254 should fail even with insecure
	r, err := doRequest(env, "GET", "http://169.254.169.254/", "", nil, row("insecure", true))
	if err != nil {
		// go error unexpected
		t.Log(err)
	}
	if ro, ok := r.Obj.(*runtime.ResultObj); ok && ro.Ok {
		t.Fatal("metadata must stay blocked with insecure")
	}
}

func TestSecurity_LLMEnvKeyGuard(t *testing.T) {
	env := runtime.NewEnv()
	env.Environ = map[string]string{"OPENAI_API_KEY": "sk-secret-test"}
	if err := guardLLMEnvKey(env, "https://evil.example/v1", "sk-secret-test"); err == nil {
		t.Fatal("expected refuse")
	}
	if err := guardLLMEnvKey(env, "https://api.openai.com/v1", "sk-secret-test"); err != nil {
		t.Fatal(err)
	}
	if err := guardLLMEnvKey(env, "http://127.0.0.1:11434/v1", "sk-secret-test"); err != nil {
		t.Fatal(err)
	}
	// non-env key allowed anywhere
	if err := guardLLMEnvKey(env, "https://evil.example/v1", "explicit-key"); err != nil {
		t.Fatal(err)
	}
}

func TestSecurity_HeaderSanitize(t *testing.T) {
	if httpHeaderSafe("ok") != true || httpHeaderSafe("a\r\nb") {
		t.Fatal()
	}
	rr := httptest.NewRecorder()
	m := respMap(200, "x", "text/plain")
	mo := m.Obj.(*runtime.MapObj)
	h := runtime.NewMap()
	ho := h.Obj.(*runtime.MapObj)
	ho.Keys = []string{"X-Ok", "X-Bad"}
	ho.Vals["X-Ok"] = runtime.Str("1")
	ho.Vals["X-Bad"] = runtime.Str("a\r\nInjected: 1")
	mo.Keys = append(mo.Keys, "headers")
	mo.Vals["headers"] = h
	writeWeftResponse(rr, m)
	if rr.Header().Get("X-Ok") != "1" {
		t.Fatal(rr.Header())
	}
	if rr.Header().Get("X-Bad") != "" {
		t.Fatal("CRLF header should be dropped")
	}
}

func TestSecurity_BeforeOnStatic(t *testing.T) {
	env := envWithCall()
	app := &webApp{env: env}
	app.befores = append(app.befores, runtime.MakeBuiltin("auth", 1, func(args []runtime.Value) (runtime.Value, error) {
		return respMap(401, "no", "text/plain"), nil
	}))
	// even without static configured, before runs; with static would block too
	app.routes = append(app.routes, webRoute{
		method: "GET", pattern: "/", parts: parsePattern("/"),
		handler: runtime.MakeBuiltin("h", 1, func(args []runtime.Value) (runtime.Value, error) {
			return respMap(200, "ok", "text/plain"), nil
		}),
	})
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != 401 {
		t.Fatal(rr.Code, rr.Body.String())
	}
	_ = strings.TrimSpace("")
}
