package pkgman

import (
	"strings"
)

// RestrictedByDefault are stdlib packages third-party modules cannot use unless
// explicitly listed in weft.json "capabilities" (or capabilities includes "*").
// Apps and path-local scripts keep full host access.
//
// Includes shell/credentials, outbound I/O, LLM keys, and data-plane connectors
// that can exfiltrate or mutate infrastructure if a malicious module is installed.
var RestrictedByDefault = []string{
	"sh",          // process execution
	"secrets",     // credential material
	"cli",         // process exit / argv takeover
	"env",         // process environment (defeats secrets if open)
	"fs",          // full filesystem
	"http",        // outbound HTTP (exfil / SSRF surface)
	"llm",         // model calls + env API keys
	"ollama",      // local model HTTP
	"vllm",        // local model HTTP
	"web",         // HTTP server bind / static
	"archive",     // extract to disk
	"graphql",     // outbound GraphQL
	"db",          // SQL databases
	"redis",       // key-value / pubsub
	"mongo",       // document store
	"nats",        // messaging
	"amqp",        // messaging
	"socket",      // raw network
	"email",       // outbound SMTP
	"pickle",      // arbitrary deserialize
	"sysinfo",     // system metrics (memory, disk, interfaces)
	"proc",        // process list / kill
	"netutil",     // network diagnostics (port scan, DNS)
	"mcp",         // MCP client/server (process spawn, network)
	"deepgram",    // Deepgram STT (API keys, network)
	"elevenlabs",  // ElevenLabs TTS (API keys, network)
	"mlinfer",     // ML inference (network, API keys)
	"accelerator", // explicitly loaded native CUDA/ROCm/MLX providers
	"cluster",     // distributed state (Redis, locks)
	"dns",         // DNS lookups (network)
	"tls",         // TLS connections (network)
	"os",          // OS operations (env, filesystem, processes)
	"compress",    // compression (file I/O)
}

// CapsAll is the grant-everything token.
const CapsAll = "*"

// LoadCapabilities reads capabilities from package weft.json (empty = defaults).
// Expands capability_profile and @profile tokens.
func LoadCapabilities(pkgDir string) []string {
	m, err := LoadManifest(pkgDir)
	if err != nil || m == nil {
		return nil
	}
	return ExpandCapabilities(m.CapabilityProfile, m.Capabilities)
}

// Allows reports whether a package capability is granted.
// empty caps → only non-restricted packages; "*" → all; else explicit list.
func Allows(caps []string, pkg string) bool {
	for _, c := range caps {
		if c == CapsAll {
			return true
		}
		if c == pkg {
			return true
		}
	}
	// default-deny only the restricted set
	for _, r := range RestrictedByDefault {
		if r == pkg {
			return false
		}
	}
	return true
}

// IsKnownCapability reports whether name is "*" or a known restricted package.
func IsKnownCapability(name string) bool {
	if name == CapsAll {
		return true
	}
	for _, r := range RestrictedByDefault {
		if r == name {
			return true
		}
	}
	return false
}

// ValidateCapabilities returns errors for empty tokens and warnings for unknown names.
// Accepts raw manifest tokens (including @profiles) or already-expanded lists.
func ValidateCapabilities(caps []string) (errs, warns []string) {
	known := strings.Join(append(append([]string{}, RestrictedByDefault...), CapsAll), ",")
	profiles := strings.Join(ProfileNames(), ",")
	for _, c := range caps {
		if c == "" {
			errs = append(errs, "empty capability entry")
			continue
		}
		if strings.HasPrefix(c, "@") {
			name := strings.TrimPrefix(c, "@")
			if !IsProfile(name) {
				warns = append(warns, "unknown capability profile @"+name+" (known: "+profiles+")")
			}
			continue
		}
		if IsKnownCapability(c) {
			continue
		}
		// bare profile name without @ is a warning (prefer @data or capability_profile)
		if IsProfile(c) {
			warns = append(warns, "capability "+c+" looks like a profile — use @"+c+` or "capability_profile": "`+c+`"`)
			continue
		}
		warns = append(warns, "unknown capability "+c+" (known restricted: "+known+"; profiles: @"+strings.ReplaceAll(profiles, ",", ",@")+")")
	}
	return errs, warns
}

// ValidateCapabilityProfile warns on unknown profile field.
func ValidateCapabilityProfile(profile string) (errs, warns []string) {
	if profile == "" {
		return nil, nil
	}
	if !IsProfile(profile) {
		warns = append(warns, "unknown capability_profile "+profile+" (known: "+strings.Join(ProfileNames(), ",")+")")
	}
	return nil, warns
}
