package stdlib

import (
	"mime"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageMIME — Content-Type guesses for agents/HTTP.
func packageMIME() runtime.Value {
	p := pkg()

	// mime.by_ext(path_or_ext) -> str  e.g. ".json" or "a.json"
	set(p, "by_ext", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str("application/octet-stream"), nil
		}
		ext := args[0].String()
		if !strings.HasPrefix(ext, ".") {
			ext = filepath.Ext(ext)
		}
		if ext == "" {
			return runtime.Str("application/octet-stream"), nil
		}
		// ensure common types even if OS mime DB is thin
		switch strings.ToLower(ext) {
		case ".weft", ".loom":
			return runtime.Str("text/x-weft"), nil
		case ".json":
			return runtime.Str("application/json"), nil
		case ".jsonl":
			return runtime.Str("application/x-ndjson"), nil
		case ".md":
			return runtime.Str("text/markdown"), nil
		case ".toml":
			return runtime.Str("application/toml"), nil
		case ".yaml", ".yml":
			return runtime.Str("application/yaml"), nil
		case ".csv":
			return runtime.Str("text/csv"), nil
		case ".html", ".htm":
			return runtime.Str("text/html; charset=utf-8"), nil
		case ".svg":
			return runtime.Str("image/svg+xml"), nil
		}
		t := mime.TypeByExtension(ext)
		if t == "" {
			t = "application/octet-stream"
		}
		return runtime.Str(t), nil
	}, 1)

	// mime.ext(type) -> str  first extension or ""
	set(p, "ext", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		// strip params
		typ := strings.TrimSpace(strings.Split(args[0].String(), ";")[0])
		switch strings.ToLower(typ) {
		case "application/json":
			return runtime.Str(".json"), nil
		case "text/html":
			return runtime.Str(".html"), nil
		case "text/x-weft":
			return runtime.Str(".weft"), nil
		case "application/toml":
			return runtime.Str(".toml"), nil
		}
		exts, err := mime.ExtensionsByType(typ)
		if err != nil || len(exts) == 0 {
			return runtime.Str(""), nil
		}
		return runtime.Str(exts[0]), nil
	}, 1)

	return p
}
