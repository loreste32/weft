package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/loreste/weft/pkg/weft"
)

func cmdOllama(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: weft ollama list|chat|ps|pull [args]")
		return 2
	}
	switch args[0] {
	case "list", "ls", "tags":
		names, err := weft.OllamaListTags("")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "hint: start ollama (ollama serve) and pull a model")
			return 1
		}
		if len(names) == 0 {
			fmt.Println("(no models — ollama pull llama3.2)")
			return 0
		}
		for _, n := range names {
			fmt.Println(n)
		}
		return 0
	case "ps":
		host := weft.OllamaNativeBase()
		running, err := weft.OllamaPS(host)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ollama unreachable:", err)
			return 1
		}
		if len(running) == 0 {
			fmt.Println("(no models loaded — chat once to warm, or: ollama run llama3.2)")
			return 0
		}
		for _, m := range running {
			if m.VRAMb > 0 {
				fmt.Printf("%s  vram=%d\n", m.Name, m.VRAMb)
			} else {
				fmt.Println(m.Name)
			}
		}
		return 0
	case "chat":
		prompt := "Say hello in one short sentence."
		model := ""
		for i := 1; i < len(args); i++ {
			if (args[i] == "-m" || args[i] == "--model") && i+1 < len(args) {
				model = args[i+1]
				i++
			} else if !strings.HasPrefix(args[i], "-") {
				prompt = args[i]
			}
		}
		// force provider
		_ = os.Setenv("WEFT_PROVIDER", "ollama")
		if model != "" {
			_ = os.Setenv("OLLAMA_MODEL", model)
		}
		c, err := weft.NewLLMClientFromEnv()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if model != "" {
			c.Model = model
		}
		out, err := c.Chat([]weft.ChatMessage{{Role: "user", Content: prompt}})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(out)
		return 0
	case "pull":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft ollama pull <model>")
			return 2
		}
		fmt.Fprintln(os.Stderr, "use: ollama pull", args[1], "  (or: weft run with ollama.pull in script)")
		fmt.Fprintln(os.Stderr, "CLI pull shells out if ollama binary exists…")
		cmd := exec.Command("ollama", "pull", args[1])
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown ollama subcommand %q\n", args[0])
		return 2
	}
}

func cmdVLLM(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: weft vllm list|chat|health [args]")
		return 2
	}
	base := weft.VLLMOpenAIBase()
	key := os.Getenv("VLLM_API_KEY")
	if key == "" {
		key = "EMPTY"
	}
	switch args[0] {
	case "list", "ls", "models":
		names, err := weft.ListOpenAIModels(base, key)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "hint: start vLLM OpenAI server, set VLLM_BASE_URL")
			return 1
		}
		if len(names) == 0 {
			fmt.Println("(no models reported)")
			return 0
		}
		for _, n := range names {
			fmt.Println(n)
		}
		return 0
	case "health":
		root := strings.TrimSuffix(base, "/v1")
		if err := weft.PingURL(root + "/health"); err != nil {
			if err2 := weft.PingURL(base + "/models"); err2 != nil {
				fmt.Fprintln(os.Stderr, "vllm unreachable:", err)
				return 1
			}
		}
		fmt.Println("vllm ok:", base)
		return 0
	case "chat":
		prompt := "Say hello in one short sentence."
		model := ""
		for i := 1; i < len(args); i++ {
			if (args[i] == "-m" || args[i] == "--model") && i+1 < len(args) {
				model = args[i+1]
				i++
			} else if !strings.HasPrefix(args[i], "-") {
				prompt = args[i]
			}
		}
		_ = os.Setenv("WEFT_PROVIDER", "vllm")
		if model != "" {
			_ = os.Setenv("VLLM_MODEL", model)
		}
		c, err := weft.NewLLMClientFromEnv()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if model != "" {
			c.Model = model
		}
		out, err := c.Chat([]weft.ChatMessage{{Role: "user", Content: prompt}})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(out)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown vllm subcommand %q\n", args[0])
		return 2
	}
}

func cmdPackages(args []string) int {
	if len(args) == 0 {
		args = []string{"list"}
	}
	wd, _ := os.Getwd()
	switch args[0] {
	case "list", "ls":
		q := ""
		if len(args) > 1 {
			q = strings.Join(args[1:], " ")
		}
		if err := weft.CatalogList(wd, q); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "search", "find":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft packages search <query>")
			return 2
		}
		if err := weft.CatalogList(wd, strings.Join(args[1:], " ")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "info", "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft packages info <name>")
			return 2
		}
		if err := weft.CatalogInfo(wd, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft packages get <name[@constraint]>")
			fmt.Fprintln(os.Stderr, "  weft packages get ml")
			fmt.Fprintln(os.Stderr, "  weft packages get tokensave@^0.5.0")
			return 2
		}
		if err := weft.CatalogGet(wd, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("ok — dependency added and installed (vendor/ + weft.lock)")
		return 0
	default:
		fmt.Fprintln(os.Stderr, "usage: weft packages list [query] | search <q> | info <name> | get <name[@constraint]>")
		return 2
	}
}
