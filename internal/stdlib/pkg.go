// Package stdlib registers Weft standard library packages (Go builtins).
package stdlib

import (
	"net/http"
	"sort"

	"github.com/loreste/weft/internal/runtime"
)

// Options tune stdlib (tests inject HTTP/env).
type Options struct {
	HTTPClient *http.Client
	// Environ overrides process env when non-nil (key -> value).
	Environ map[string]string
	// LLMBaseURL overrides default OpenAI-compat base URL.
	LLMBaseURL string
}

// packageFactory builds one bare-importable stdlib package map.
type packageFactory func(env *runtime.Env) runtime.Value

// packages is the single registry of stdlib package names → constructors.
// Add a package here only — Names(), IsPackage(), Register, and module host-share follow.
var packages = []struct {
	name  string
	build packageFactory
}{
	{"env", packageEnv},
	{"json", func(env *runtime.Env) runtime.Value { return packageJSON() }},
	{"jsonl", packageJSONL},
	{"table", packageTable},
	{"pipe", packagePipe},
	{"fs", func(env *runtime.Env) runtime.Value { return packageFS() }},
	{"http", packageHTTP},
	{"web", packageWeb},
	{"ws", packageWS},
	{"webrtc", packageWebRTC},
	{"viz", packageViz},
	{"cli", packageCLI},
	{"sh", packageSH},
	{"io", packageIO},
	{"str", func(env *runtime.Env) runtime.Value { return packageStr() }},
	{"csv", func(env *runtime.Env) runtime.Value { return packageCSV() }},
	{"time", func(env *runtime.Env) runtime.Value { return packageTime() }},
	{"math", func(env *runtime.Env) runtime.Value { return packageMath() }},
	{"random", func(env *runtime.Env) runtime.Value { return packageRandom() }},
	{"uuid", func(env *runtime.Env) runtime.Value { return packageUUID() }},
	{"base64", func(env *runtime.Env) runtime.Value { return packageBase64() }},
	{"url", func(env *runtime.Env) runtime.Value { return packageURL() }},
	{"archive", func(env *runtime.Env) runtime.Value { return packageArchive() }},
	{"decimal", func(env *runtime.Env) runtime.Value { return packageDecimal() }},
	{"xml", func(env *runtime.Env) runtime.Value { return packageXML() }},
	{"html", func(env *runtime.Env) runtime.Value { return packageHTML() }},
	{"ini", func(env *runtime.Env) runtime.Value { return packageINI() }},
	{"toml", func(env *runtime.Env) runtime.Value { return packageTOML() }},
	{"yaml", func(env *runtime.Env) runtime.Value { return packageYAML() }},
	{"mime", func(env *runtime.Env) runtime.Value { return packageMIME() }},
	{"email", packageEmail},
	{"socket", packageSocket},
	{"pickle", func(env *runtime.Env) runtime.Value { return packagePickle() }},
	{"iter", func(env *runtime.Env) runtime.Value { return packageIter() }},
	{"collections", func(env *runtime.Env) runtime.Value { return packageCollections() }},
	{"platform", func(env *runtime.Env) runtime.Value { return packagePlatform() }},
	{"ip", func(env *runtime.Env) runtime.Value { return packageIP() }},
	{"bisect", func(env *runtime.Env) runtime.Value { return packageBisect() }},
	{"heap", func(env *runtime.Env) runtime.Value { return packageHeap() }},
	{"db", packageDB},
	{"redis", packageRedis},
	{"nats", packageNATS},
	{"amqp", packageAMQP},
	{"mongo", packageMongo},
	{"graphql", packageGraphQL},
	{"log", packageLog},
	{"crypto", func(env *runtime.Env) runtime.Value { return packageCrypto() }},
	{"re", func(env *runtime.Env) runtime.Value { return packageRe() }},
	{"test", func(env *runtime.Env) runtime.Value { return packageTest() }},
	{"secrets", packageSecrets},
	{"llm", packageLLM},
	{"ollama", packageOllama},
	{"vllm", packageVLLM},
}

// packageIndex is built once from packages for O(1) IsPackage.
var packageIndex map[string]bool

func init() {
	packageIndex = make(map[string]bool, len(packages))
	for _, p := range packages {
		packageIndex[p.name] = true
	}
}

// Register installs all stdlib packages into env (host-shared with modules).
func Register(env *runtime.Env, opts Options) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = DefaultHTTPClient()
	}
	env.HTTPClient = opts.HTTPClient
	env.Environ = opts.Environ
	if opts.LLMBaseURL != "" {
		env.LLMBaseURL = opts.LLMBaseURL
	}

	// list pipeline ops as host-shared globals (map/filter/reduce/…)
	installListOps(env)

	for _, p := range packages {
		env.SetShared(p.name, p.build(env))
	}
}

// Names lists bare importable stdlib package names (sorted).
func Names() []string {
	out := make([]string, len(packages))
	for i, p := range packages {
		out[i] = p.name
	}
	sort.Strings(out)
	return out
}

// IsPackage reports whether name is a registered stdlib package (not a path dep).
func IsPackage(name string) bool {
	return packageIndex[name]
}

// PackageMembers returns sorted member names of a stdlib package (for LSP/completion).
// Empty if name is unknown. Builds a temporary env; safe for tooling.
func PackageMembers(name string) []string {
	if !IsPackage(name) {
		return nil
	}
	env := runtime.NewEnv()
	var built runtime.Value
	for _, p := range packages {
		if p.name == name {
			built = p.build(env)
			break
		}
	}
	if built.Kind != runtime.KindMap {
		return nil
	}
	mo := built.Obj.(*runtime.MapObj)
	out := append([]string(nil), mo.Keys...)
	sort.Strings(out)
	return out
}
