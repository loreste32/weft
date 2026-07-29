//go:build js && wasm

package stdlib

import (
	"fmt"
	"net/http"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// DefaultHTTPClient is a no-network client for browser builds.
func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

// stubPkg returns a package map whose methods error with a browser limitation message.
func stubPkg(name string) runtime.Value {
	m := runtime.NewMap()
	// Common method names so call sites fail with a clear message.
	for _, method := range []string{
		"get", "post", "put", "delete", "request", "listen", "serve",
		"connect", "query", "exec", "open", "close", "run", "publish", "subscribe",
		"chat", "complete", "embed", "transcribe", "speak",
	} {
		n := name + "." + method
		set(m, method, func(args []runtime.Value) (runtime.Value, error) {
			return runtime.Null(), fmt.Errorf("%s is not available in the browser Wasm build", n)
		}, -1)
	}
	return m
}

func packageAMQP(env *runtime.Env) runtime.Value       { return stubPkg("amqp") }
func packageCluster(env *runtime.Env) runtime.Value    { return stubPkg("cluster") }
func packageDB(env *runtime.Env) runtime.Value         { return stubPkg("db") }
func packageDeepgram(env *runtime.Env) runtime.Value   { return stubPkg("deepgram") }
func packageElevenLabs(env *runtime.Env) runtime.Value { return stubPkg("elevenlabs") }
func packageEmail(env *runtime.Env) runtime.Value      { return stubPkg("email") }
func packageGovernor(env *runtime.Env) runtime.Value   { return stubPkg("governor") }
func packageGraphQL(env *runtime.Env) runtime.Value    { return stubPkg("graphql") }
func packageHTTP(env *runtime.Env) runtime.Value       { return stubPkg("http") }
func packageLLM(env *runtime.Env) runtime.Value        { return stubPkg("llm") }
func packageMCP(env *runtime.Env) runtime.Value        { return stubPkg("mcp") }
func packageMigrate(env *runtime.Env) runtime.Value    { return stubPkg("migrate") }
func packageMLInfer(env *runtime.Env) runtime.Value    { return stubPkg("mlinfer") }
func packageMongo(env *runtime.Env) runtime.Value      { return stubPkg("mongo") }
func packageNATS(env *runtime.Env) runtime.Value       { return stubPkg("nats") }
func packageNetutil(env *runtime.Env) runtime.Value    { return stubPkg("netutil") }
func packageOllama(env *runtime.Env) runtime.Value     { return stubPkg("ollama") }
func packagePcap() runtime.Value                       { return stubPkg("pcap") }
func packageProc() runtime.Value                       { return stubPkg("proc") }
func packageRedis(env *runtime.Env) runtime.Value      { return stubPkg("redis") }
func packageSH(env *runtime.Env) runtime.Value         { return stubPkg("sh") }
func packageSignal(env *runtime.Env) runtime.Value     { return stubPkg("signal") }
func packageSocket(env *runtime.Env) runtime.Value     { return stubPkg("socket") }
func packageSupervisor(env *runtime.Env) runtime.Value { return stubPkg("supervisor") }
func packageSysinfo() runtime.Value                    { return stubPkg("sysinfo") }
func packageVLLM(env *runtime.Env) runtime.Value       { return stubPkg("vllm") }
func packageWeb(env *runtime.Env) runtime.Value        { return stubPkg("web") }
func packageWebRTC(env *runtime.Env) runtime.Value     { return stubPkg("webrtc") }
func packageWS(env *runtime.Env) runtime.Value         { return stubPkg("ws") }
