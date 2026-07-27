package weft

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/loreste/weft/internal/llmpack"
	"github.com/loreste/weft/internal/pkgman"
)

// Doctor reports environment readiness for Weft (no Python required).
func Doctor() error {
	fmt.Printf("weft %s\n", Version)
	fmt.Printf("go:   %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	check := func(name, detail string, good bool) {
		status := "ok"
		if !good {
			status = "—"
		}
		fmt.Printf("  %-14s %-8s %s\n", name, status, detail)
	}

	prov, provDetail, provOK := ProviderStatus()
	check("provider", prov+": "+provDetail, provOK)

	key := firstEnv("OPENAI_API_KEY", "WEFT_API_KEY", "LLM_API_KEY")
	check("api_key", maskKey(key), key != "" || prov == ProviderOllama || prov == ProviderVLLM)
	base := firstEnv("OPENAI_BASE_URL", "WEFT_API_BASE", "LLM_BASE_URL")
	if base == "" {
		switch DetectProvider() {
		case ProviderOllama:
			base = OllamaOpenAIBase()
		case ProviderVLLM:
			base = VLLMOpenAIBase()
		default:
			base = DefaultOpenAIBase + " (default)"
		}
	}
	check("api_base", base, true)
	model := firstEnv("WEFT_MODEL", "OPENAI_MODEL", "LLM_MODEL", "OLLAMA_MODEL", "VLLM_MODEL")
	if model == "" {
		switch DetectProvider() {
		case ProviderOllama:
			model = DefaultOllamaModel + " (ollama default)"
		case ProviderVLLM:
			model = DefaultVLLMModel + " (set VLLM_MODEL)"
		default:
			model = DefaultOpenAIModel + " (default)"
		}
	}
	check("model", model, true)

	_, gitErr := exec.LookPath("git")
	check("git", "optional — weft get from github", gitErr == nil)

	py := ""
	for _, c := range []string{"python3", "python"} {
		if p, err := exec.LookPath(c); err == nil {
			py = p
			break
		}
	}
	check("python", "optional — only --backend trl", py != "")

	wd, _ := os.Getwd()
	root := DetectProjectDir(wd)
	hasProject := false
	if _, err := os.Stat(filepath.Join(root, "weft.json")); err == nil {
		check("project", root, true)
		hasProject = true
	} else if _, err := os.Stat(filepath.Join(root, "loom.json")); err == nil {
		check("project", root+" (legacy loom.json)", true)
		hasProject = true
	} else {
		check("project", "no weft.json in tree (ok)", true)
	}

	// Dependencies + vendor integrity when this is a Weft project.
	if hasProject {
		if m, err := pkgman.LoadManifest(root); err == nil {
			n := len(m.Deps)
			if n == 0 {
				check("deps", "none declared", true)
			} else {
				names := make([]string, 0, n)
				for name := range m.Deps {
					names = append(names, name)
				}
				// stable-ish short list
				detail := fmt.Sprintf("%d: %s", n, joinMax(names, 4))
				check("deps", detail, true)
			}
		}
		if _, err := os.Stat(filepath.Join(root, "weft.lock")); err == nil {
			if err := pkgman.VerifyLock(root); err != nil {
				check("vendor", err.Error(), false)
			} else {
				check("vendor", "weft.lock sums match", true)
			}
		} else if m, err := pkgman.LoadManifest(root); err == nil && len(m.Deps) > 0 {
			check("vendor", "no weft.lock — run weft install", false)
		}
	}

	// Monorepo / remote catalog discovery (same as weft packages list).
	if catPath, cat, err := pkgman.FindCatalog(wd); err != nil {
		detail := "none (set WEFT_PACKAGES or WEFT_CATALOG_URL, or use monorepo packages/)"
		if os.Getenv("WEFT_CATALOG_URL") != "" {
			detail = "WEFT_CATALOG_URL set but unreachable; " + err.Error()
			check("catalog", detail, false)
		} else {
			check("catalog", detail, true) // optional
		}
	} else {
		src := catPath
		if len(src) > 56 {
			src = "…" + src[len(src)-52:]
		}
		check("catalog", fmt.Sprintf("%d pkg(s) · %s", len(cat.Packages), src), true)
		// brief names for humans
		var names []string
		for _, p := range cat.Packages {
			names = append(names, p.Name)
		}
		if len(names) > 0 {
			check("catalog_pkgs", joinMax(names, 6), true)
		}
	}
	if u := os.Getenv("WEFT_CATALOG_URL"); u != "" {
		check("catalog_url", u, true)
	}
	if p := os.Getenv("WEFT_PACKAGES"); p != "" {
		check("packages_env", p, true)
	}

	check("train_corpus", fmt.Sprintf("%d gold examples embedded", len(llmpack.Examples())), true)
	check("system_card", fmt.Sprintf("%d bytes", len(llmpack.SystemCard())), true)

	priv := firstEnv("WEFT_TRAIN_PRIVATE")
	check("train_private", "WEFT_TRAIN_PRIVATE="+orDefault(priv, "unset (finetune defaults private)"), true)

	workers := os.Getenv("WEFT_WORKERS")
	if workers == "" {
		workers = "auto (GOMAXPROCS)"
	}
	check("workers", "WEFT_WORKERS="+workers+" — map/filter concurrency", true)

	fmt.Println()
	fmt.Println("concurrent: map/filter fan-out by default · gather · race · timeout · spawn.await")
	fmt.Println("local LLM:  export WEFT_PROVIDER=ollama   # or vllm")
	fmt.Println("            weft ollama list · weft vllm list · weft gen \"…\"")
	fmt.Println("packages:   weft packages list|info|get · weft install")
	fmt.Println("private train: weft train finetune --private --preset qwen-7b")
	if key == "" && DetectProvider() == ProviderOpenAI {
		fmt.Println("hint: OPENAI_API_KEY, or WEFT_PROVIDER=ollama|vllm for local models")
	} else {
		fmt.Println("ready: weft gen · train chat · ollama/vllm/openai via WEFT_PROVIDER")
	}
	fmt.Println("always:  weft run · check · test · eval · train prepare")
	return nil
}

// joinMax joins names with commas, truncating after max with "…".
func joinMax(names []string, max int) string {
	if len(names) == 0 {
		return ""
	}
	// insertion-sort style for small n (no import sort cycle concerns)
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	if len(names) <= max {
		out := names[0]
		for i := 1; i < len(names); i++ {
			out += ", " + names[i]
		}
		return out
	}
	out := names[0]
	for i := 1; i < max; i++ {
		out += ", " + names[i]
	}
	return fmt.Sprintf("%s …(+%d)", out, len(names)-max)
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func maskKey(k string) string {
	if k == "" {
		return "(not set)"
	}
	if len(k) < 8 {
		return "***"
	}
	return k[:3] + "…" + k[len(k)-4:]
}
