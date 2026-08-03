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
	{"shlex", func(env *runtime.Env) runtime.Value { return packageShlex() }},
	{"signal", packageSignal},
	{"binstruct", func(env *runtime.Env) runtime.Value { return packageBinstruct() }},
	{"difflib", func(env *runtime.Env) runtime.Value { return packageDifflib() }},
	{"copy", func(env *runtime.Env) runtime.Value { return packageCopy() }},
	{"functools", packageFunctools},
	{"traceback", func(env *runtime.Env) runtime.Value { return packageTraceback() }},
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
	{"pcap", func(env *runtime.Env) runtime.Value { return packagePcap() }},
	{"tokenizer", func(env *runtime.Env) runtime.Value { return packageTokenizer() }},
	{"metrics", func(env *runtime.Env) runtime.Value { return packageMetrics() }},
	{"dataset", func(env *runtime.Env) runtime.Value { return packageDataset() }},
	{"ratelimit", func(env *runtime.Env) runtime.Value { return packageRatelimit() }},
	{"migrate", packageMigrate},
	{"log", packageLog},
	{"crypto", func(env *runtime.Env) runtime.Value { return packageCrypto() }},
	{"re", func(env *runtime.Env) runtime.Value { return packageRe() }},
	{"test", func(env *runtime.Env) runtime.Value { return packageTest() }},
	{"secrets", packageSecrets},
	{"llm", packageLLM},
	{"ollama", packageOllama},
	{"vllm", packageVLLM},
	{"governor", packageGovernor},
	{"cluster", packageCluster},
	{"supervisor", packageSupervisor},
	{"mcp", packageMCP},
	{"deepgram", packageDeepgram},
	{"mlinfer", packageMLInfer},
	{"accelerator", func(env *runtime.Env) runtime.Value { return packageAccelerator() }},
	{"elevenlabs", packageElevenLabs},
	{"sysinfo", func(env *runtime.Env) runtime.Value { return packageSysinfo() }},
	{"proc", func(env *runtime.Env) runtime.Value { return packageProc() }},
	{"netutil", packageNetutil},
	{"encoding", func(env *runtime.Env) runtime.Value { return packageEncoding() }},
	{"compress", func(env *runtime.Env) runtime.Value { return packageCompress() }},
	{"dns", func(env *runtime.Env) runtime.Value { return packageDNS() }},
	{"tls", func(env *runtime.Env) runtime.Value { return packageTLS() }},
	{"os", func(env *runtime.Env) runtime.Value { return packageOS() }},
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

// StdlibMaturity returns "stable", "beta", or "experimental" for a stdlib package.
// Unknown packages default to "experimental" — explicit registration is required for "stable".
func StdlibMaturity(name string) string {
	if m, ok := stdlibMaturity[name]; ok {
		return m
	}
	return "experimental"
}

// stablePackages are packages that have proven stable and well-tested.
// Everything NOT in stdlibMaturity defaults to experimental.
func init() {
	stableDefaults := []string{
		"json", "jsonl", "str", "csv", "time", "math", "random", "uuid",
		"base64", "url", "html", "xml", "yaml", "toml", "ini", "mime",
		"http", "web", "fs", "io", "sh", "cli", "env", "log",
		"db", "redis", "crypto", "re", "test", "table", "pipe",
		"email", "socket", "ip", "archive", "secrets", "signal",
		"llm", "ollama", "vllm",
		"iter", "collections", "heap", "bisect", "functools", "copy",
		"traceback", "platform", "shlex", "difflib", "binstruct",
		"decimal", "pickle",
	}
	for _, name := range stableDefaults {
		if _, exists := stdlibMaturity[name]; !exists {
			stdlibMaturity[name] = "stable"
		}
	}
}

var stdlibMaturity = map[string]string{
	// experimental: new, API may change
	"deepgram":   "experimental",
	"elevenlabs": "experimental",
	"mlinfer":    "experimental",
	"governor":   "experimental",
	"supervisor": "experimental",
	"cluster":    "experimental",
	"mcp":        "experimental",
	"encoding":   "experimental",
	"compress":   "experimental",
	"dns":        "experimental",
	"tls":        "experimental",
	"os":         "experimental",
	"webrtc":     "experimental",
	"pcap":       "experimental",
	// beta: functional but limited test coverage
	"sysinfo": "beta",
	"proc":    "beta",
	"netutil": "beta",
	"migrate": "beta",
	"viz":     "beta",
	// everything else: stable
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
