package pkgman

import (
	"sort"
	"strings"
)

// Named capability profiles — shortcuts for common third-party module grants.
// Use as tokens in capabilities: ["@data"] or set "capability_profile": "data".
//
// Profiles are balanced: each is the least privilege for a common module job.
var Profiles = map[string][]string{
	// none / empty: pure helpers only (json, math, str, …) — no FS/net/LLM
	"none": {},
	// filesystem + archives
	"io": {"fs", "archive"},
	// outbound HTTP only
	"http": {"http"},
	// model providers (needs grant for modules that call llm.*)
	"llm": {"llm", "ollama", "vllm"},
	// agent-style modules: models + outbound HTTP
	"agent": {"llm", "ollama", "vllm", "http"},
	// data-plane connectors (SQL / KV / messaging)
	"data": {"db", "redis", "mongo", "nats", "amqp"},
	// raw network + SMTP (not process exec)
	"net": {"socket", "email", "http"},
	// host-level: shell, secrets, env, CLI, FS, plus net
	"host": {"sh", "secrets", "cli", "env", "fs", "http", "socket", "email", "pickle", "archive"},
	// everything restricted
	"full": {CapsAll},
}

// IsProfile reports whether name is a known profile (with or without @).
func IsProfile(name string) bool {
	_, ok := Profiles[strings.TrimPrefix(name, "@")]
	return ok
}

// ExpandCapabilities expands capability_profile and @profile tokens into a flat list.
// Unknown bare tokens are kept as-is (validated separately). De-duplicated, stable order.
func ExpandCapabilities(profile string, caps []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(c string) {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			return
		}
		seen[c] = true
		out = append(out, c)
	}
	addProfile := func(name string) {
		name = strings.TrimPrefix(strings.TrimSpace(name), "@")
		if toks, ok := Profiles[name]; ok {
			for _, t := range toks {
				add(t)
			}
			return
		}
		// unknown — keep as @name for validation
		if name != "" {
			add("@" + name)
		}
	}
	if profile != "" {
		addProfile(profile)
	}
	for _, c := range caps {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if strings.HasPrefix(c, "@") {
			addProfile(c)
			continue
		}
		add(c)
	}
	return out
}

// ProfileNames returns sorted known profile names for docs/CLI.
func ProfileNames() []string {
	names := make([]string, 0, len(Profiles))
	for k := range Profiles {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
