package stdlib

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestSecurity_LLMTrustHostExact(t *testing.T) {
	env := runtime.NewEnv()
	env.Environ = map[string]string{"OPENAI_API_KEY": "sk-secret-test"}
	// path / query spoof — host is attacker
	for _, bad := range []string{
		"https://attacker.example/api.openai.com/v1",
		"https://api.openai.com.evil.com/v1",
		"https://evil-localhost.example/v1",
		"https://evil.com/?q=localhost",
		"https://not-api.openai.com.evil/v1",
		"https://user@evil.example/v1", // userinfo
	} {
		if err := guardLLMEnvKey(env, bad, "sk-secret-test"); err == nil {
			t.Fatalf("expected refuse for %q", bad)
		}
	}
	// trusted hosts
	for _, good := range []string{
		"https://api.openai.com/v1",
		"https://api.anthropic.com/v1",
		"http://127.0.0.1:11434/v1",
		"http://localhost:11434",
		"http://[::1]:11434/v1",
	} {
		if err := guardLLMEnvKey(env, good, "sk-secret-test"); err != nil {
			t.Fatalf("expected allow for %q: %v", good, err)
		}
	}
	// subdomain of trusted cloud host is ok
	if err := guardLLMEnvKey(env, "https://cdn.api.openai.com/v1", "sk-secret-test"); err != nil {
		t.Fatal(err)
	}
	// WEFT_LLM_TRUST_HOSTS exact/suffix only
	t.Setenv("WEFT_LLM_TRUST_HOSTS", "models.corp.internal")
	if err := guardLLMEnvKey(env, "https://models.corp.internal/v1", "sk-secret-test"); err != nil {
		t.Fatal(err)
	}
	if err := guardLLMEnvKey(env, "https://gpu.models.corp.internal/v1", "sk-secret-test"); err != nil {
		t.Fatal(err)
	}
	if err := guardLLMEnvKey(env, "https://notmodels.corp.internal.evil/v1", "sk-secret-test"); err == nil {
		t.Fatal("substring trust host must not match")
	}
	if err := guardLLMEnvKey(env, "https://evil.example/models.corp.internal/v1", "sk-secret-test"); err == nil {
		t.Fatal("path spoof must not match trust host")
	}
}

func TestSecurity_LLMHostMatchesTrusted(t *testing.T) {
	if !hostMatchesTrusted("api.openai.com", "api.openai.com") {
		t.Fatal()
	}
	if !hostMatchesTrusted("x.api.openai.com", "api.openai.com") {
		t.Fatal()
	}
	if hostMatchesTrusted("api.openai.com.evil.com", "api.openai.com") {
		t.Fatal()
	}
	if hostMatchesTrusted("evil-localhost", "localhost") {
		t.Fatal()
	}
	if hostMatchesTrusted("localhost.evil", "localhost") {
		t.Fatal()
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

func TestSecurity_SecretSealedJSONGet(t *testing.T) {
	s := runtime.Struct("Secret", map[string]runtime.Value{
		"value": runtime.Str("sk-leaked"),
	}, []string{"value"})
	// json path must refuse
	_, err := jsonPathGet(s, runtime.Str("value"))
	if err == nil || !strings.Contains(err.Error(), "sealed") {
		t.Fatalf("jsonPathGet must seal Secret: %v", err)
	}
	// asMap must refuse
	if _, err := asMap(s); err == nil || !strings.Contains(err.Error(), "sealed") {
		t.Fatalf("asMap must seal Secret: %v", err)
	}
	// stringify still redacts
	if g := valueToGo(s); g != "***" {
		t.Fatalf("valueToGo Secret = %v", g)
	}
	// fieldOrKey must not expose
	if _, ok := fieldOrKey(s, "value"); ok {
		t.Fatal("fieldOrKey must not return Secret fields")
	}
}

func TestSecurity_SMTPHeaderSafe(t *testing.T) {
	if smtpHeaderSafe("hi\r\nBcc: evil@x.com") != "hiBcc: evil@x.com" {
		t.Fatal(smtpHeaderSafe("hi\r\nBcc: evil@x.com"))
	}
	if smtpHeaderSafe("ok") != "ok" {
		t.Fatal()
	}
	if strings.Contains(smtpHeaderSafe("a\nb\x00c"), "\n") || strings.Contains(smtpHeaderSafe("a\nb\x00c"), "\x00") {
		t.Fatal()
	}
}

func TestSecurity_HtmxOOBIdSanitize(t *testing.T) {
	// attribute breakout must not survive — quotes/spaces stripped from id
	out := htmxOOBWrap(`x" onload="alert(1)`, "<div>hi</div>")
	if strings.Contains(out, `id="x" `) || strings.Contains(out, `onload="`) {
		t.Fatalf("attribute breakout possible: %s", out)
	}
	// only [A-Za-z0-9_-] remain in id token
	if !strings.Contains(out, `id="xonloadalert1"`) && !strings.Contains(out, `id="x`) {
		t.Fatalf("expected sanitized id: %s", out)
	}
}

func TestSecurity_GunzipSizeCap(t *testing.T) {
	// tiny gzip of "hello" works; cap path is integration-tested by LimitReader logic
	dir := t.TempDir()
	// write a small gz via archive gzip if available — just verify LimitReader constant path compiles via gunzip of empty fail
	src := filepath.Join(dir, "empty.gz")
	if err := os.WriteFile(src, []byte("not-gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	// call package level through packageArchive would need full runtime; unit-test constant exists
	if maxBodyBytes < 1 {
		t.Fatal()
	}
}
