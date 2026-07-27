// Command weft is the Weft language CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/lsp"
	"github.com/loreste/weft/pkg/weft"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		if err := weft.StartREPL(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if !isCommand(args[0]) && (strings.HasSuffix(args[0], ".weft") || strings.HasSuffix(args[0], ".loom")) {
		return cmdRun(args)
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("weft", weft.Version)
		return 0
	case "run":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft run <file.weft>")
			return 2
		}
		return cmdRun(args[1:])
	case "check":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft check <file.weft|dir>… [--types]")
			return 2
		}
		showTypes := false
		var paths []string
		for _, a := range args[1:] {
			if a == "--types" || a == "-t" {
				showTypes = true
			} else if !strings.HasPrefix(a, "-") {
				paths = append(paths, a)
			}
		}
		if len(paths) == 0 {
			fmt.Fprintln(os.Stderr, "usage: weft check <file.weft|dir>… [--types]")
			return 2
		}
		return cmdCheckPaths(paths, showTypes)
	case "test":
		return cmdTest(args[1:])
	case "stdlib":
		return cmdStdlib(args[1:])
	case "fmt":
		return cmdFmt(args[1:])
	case "bench":
		return cmdBench(args[1:])
	case "repl":
		if err := weft.StartREPL(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "init":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		wd, _ := os.Getwd()
		if err := weft.PkgInit(wd, name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("wrote weft.json")
		return 0
	case "new":
		return cmdNew(args[1:])
	case "mod":
		return cmdMod(args[1:])
	case "get":
		// weft get <name> [spec]  or  weft get <spec>
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft get <name> [path|git|url]")
			fmt.Fprintln(os.Stderr, "  weft get greeter ./packages/greeter")
			fmt.Fprintln(os.Stderr, "  weft get util github.com/org/repo@v0.1.0")
			return 2
		}
		name, spec := args[1], ""
		if len(args) >= 3 {
			spec = args[2]
		}
		wd, _ := os.Getwd()
		if err := weft.PkgGet(wd, name, spec); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("ok")
		return 0
	case "install":
		wd, _ := os.Getwd()
		if err := weft.PkgInstall(wd); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "list", "deps":
		// weft list              — project deps
		// weft list packages     — monorepo catalog (packages/index.json)
		if len(args) > 1 && (args[1] == "packages" || args[1] == "pkgs" || args[1] == "catalog") {
			wd, _ := os.Getwd()
			if err := weft.CatalogList(wd); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		}
		wd, _ := os.Getwd()
		if err := weft.PkgList(wd); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "packages", "pkgs", "catalog":
		// weft packages [list|get <name>]
		return cmdPackages(args[1:])
	case "prompt", "teach":
		// weft prompt [--few N]
		few := 0
		for i := 1; i < len(args); i++ {
			if (args[i] == "--few" || args[i] == "-f") && i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &few)
				i++
			} else if strings.HasPrefix(args[i], "--few=") {
				fmt.Sscanf(strings.TrimPrefix(args[i], "--few="), "%d", &few)
			}
		}
		if err := weft.WritePrompt(os.Stdout, few); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "train":
		return cmdTrain(args[1:])
	case "eval":
		dir := "examples"
		if len(args) > 1 {
			dir = args[1]
		}
		cases, err := weft.EvalDir(dir, weft.Options{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return weft.PrintEvalReport(cases)
	case "gen":
		return cmdGen(args[1:])
	case "doctor":
		if err := weft.Doctor(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "ollama":
		return cmdOllama(args[1:])
	case "vllm":
		return cmdVLLM(args[1:])
	case "lsp":
		if err := lsp.StartStdio(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "help", "-h", "--help":
		fmt.Print(`weft — weave agents into code (no Python required)

Language:
  weft                      REPL
  weft run <file.weft>
  weft check <file|dir>… [--types]
  weft test [path…] [-q] [-run filter]   # run fn test_* in *_test.weft
  weft stdlib [pkg]           # list stdlib packages (or members of pkg)
  weft fmt <file.weft|dir>…   # AST pretty-print (weft style)
  weft bench [path…] [-n N]  # microbench fn bench_* in *_bench.weft
  weft gen "task" [-o out.weft] [--run]   # LLM writes Weft (pure Go API)
  weft doctor                environment readiness
  weft lsp                   Language Server (stdio) for editors
  weft ollama list|chat|ps   # local Ollama
  weft vllm list|chat|health # local vLLM
  weft version

Concurrency (default — not asyncio):
  map/filter fan-out · gather/parallel · race · timeout · spawn.await · channels
  WEFT_WORKERS=N  # default pool size for map/filter

Fine-tune (private by default — data stays local):
  weft train prepare -o weft-sft --expand [--from private.jsonl]
  weft train eval [--run] [--limit N] [--from gold.jsonl] [--live]
  weft train finetune --private --preset qwen-7b
  weft train offline -o weft-airgap --expand     # air-gapped GPU kit
  weft train presets
  weft train finetune --backend openai --allow-upload --wait   # explicit cloud
  weft train chat "write hello" [--model ft:...]
  weft train status --job ftjob-...
  weft prompt [--few N]
  weft eval [dir]              # run scripts (CI smoke)
  weft test [path…]            # unit tests (*_test.weft · fn test_*)
  weft train eval              # gold accuracy (parse/compile)

Packages & modules (for other developers):
  weft new module <name>     # scaffold a publishable library
  weft new app <name>        # scaffold an application
  weft new cli <name>        # scaffold a devops/data CLI tool
  weft mod check [dir] [--tests]  # validate (+ run tests with --tests)
  weft mod pack [dir] [-o x.zip]
  weft init | get | install | list
  weft packages list [q]     # monorepo catalog (packages/index.json)
  weft packages search <q>   # filter catalog by name/summary
  weft packages info <name>  # one catalog entry
  weft packages get <name[@constraint]>  # add path dep (+ pin version)
`)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func cmdNew(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, `usage: weft new module <name>
       weft new app <name>
       weft new cli <name>`)
		return 2
	}
	kind := args[0]
	name := ""
	force := false
	for i := 1; i < len(args); i++ {
		if args[i] == "--force" || args[i] == "-f" {
			force = true
		} else if !strings.HasPrefix(args[i], "-") && name == "" {
			name = args[i]
		}
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: weft new module|app|cli <name>")
		return 2
	}
	wd, _ := os.Getwd()
	var (
		root string
		err  error
	)
	switch kind {
	case "module", "mod", "pkg", "package", "lib":
		root, err = weft.NewModule(wd, name, force)
	case "app", "project":
		root, err = weft.NewApp(wd, name, force)
	case "cli", "tool", "ctl":
		root, err = weft.NewCLI(wd, name, force)
	default:
		fmt.Fprintf(os.Stderr, "unknown kind %q (want module, app, or cli)\n", kind)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("created", root)
	switch kind {
	case "module", "mod", "pkg", "package", "lib":
		fmt.Println("  next: edit lib.weft · weft mod check · share via path or git")
	case "cli", "tool", "ctl":
		fmt.Println("  next: weft run main.weft -- --help")
	default:
		fmt.Println("  next: weft run main.weft")
	}
	return 0
}

func cmdMod(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: weft mod check [dir] [--tests] | pack [dir] [-o out.zip]")
		return 2
	}
	switch args[0] {
	case "check", "validate":
		dir := ""
		runTests := false
		quietTests := false
		for i := 1; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--tests" || a == "-t":
				runTests = true
			case a == "--quiet" || a == "-q":
				quietTests = true
			case strings.HasPrefix(a, "-"):
				fmt.Fprintf(os.Stderr, "unknown flag %q\n", a)
				return 2
			case dir == "":
				dir = a
			}
		}
		if err := weft.ModCheckWith(weft.ModCheckOptions{
			Dir: dir, RunTests: runTests, QuietTests: quietTests,
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "pack", "zip":
		dir, out := "", ""
		for i := 1; i < len(args); i++ {
			a := args[i]
			if (a == "-o" || a == "--out") && i+1 < len(args) {
				out = args[i+1]
				i++
			} else if strings.HasPrefix(a, "--out=") {
				out = strings.TrimPrefix(a, "--out=")
			} else if !strings.HasPrefix(a, "-") && dir == "" {
				dir = a
			}
		}
		if err := weft.ModPack(dir, out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "new":
		// alias: weft mod new name → weft new module name
		return cmdNew(append([]string{"module"}, args[1:]...))
	default:
		fmt.Fprintf(os.Stderr, "unknown mod subcommand %q\n", args[0])
		return 2
	}
}

func cmdTrain(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: weft train prepare|eval|finetune|offline|presets|status|validate|export|stats")
		fmt.Fprintln(os.Stderr, "  weft train finetune --private --preset qwen-7b   # data stays local")
		fmt.Fprintln(os.Stderr, "  weft train eval [--run] [--limit N] [--from gold.jsonl]")
		fmt.Fprintln(os.Stderr, "  weft train offline -o weft-airgap --expand       # air-gapped pack")
		fmt.Fprintln(os.Stderr, "  weft train finetune --backend openai --allow-upload --wait")
		return 2
	}
	switch args[0] {
	case "eval", "score":
		opts := weft.TrainEvalOptions{}
		for i := 1; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--run":
				opts.Run = true
			case a == "--live":
				opts.Live = true
			case a == "--quiet" || a == "-q":
				opts.Quiet = true
			case (a == "--from" || a == "-f") && i+1 < len(args):
				opts.From = args[i+1]
				i++
			case strings.HasPrefix(a, "--from="):
				opts.From = strings.TrimPrefix(a, "--from=")
			case (a == "--limit" || a == "-n") && i+1 < len(args):
				fmt.Sscanf(args[i+1], "%d", &opts.Limit)
				i++
			case strings.HasPrefix(a, "--limit="):
				fmt.Sscanf(strings.TrimPrefix(a, "--limit="), "%d", &opts.Limit)
			}
		}
		rep, err := weft.TrainEval(opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return weft.PrintTrainEval(rep)
	case "prepare":
		opts := weft.PrepareOptions{OutDir: "weft-sft", Expand: false, FewShot: 8}
		for i := 1; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--expand" || a == "-e":
				opts.Expand = true
			case (a == "-o" || a == "--out") && i+1 < len(args):
				opts.OutDir = args[i+1]
				i++
			case strings.HasPrefix(a, "--out="):
				opts.OutDir = strings.TrimPrefix(a, "--out=")
			case (a == "--few" || a == "-f") && i+1 < len(args):
				fmt.Sscanf(args[i+1], "%d", &opts.FewShot)
				i++
			case (a == "--from" || a == "--private-data") && i+1 < len(args):
				opts.From = args[i+1]
				i++
			case strings.HasPrefix(a, "--from="):
				opts.From = strings.TrimPrefix(a, "--from=")
			}
		}
		if err := weft.PrepareTrainBundle(opts); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "offline", "airgap", "pack":
		opts := weft.OfflinePackOptions{OutDir: "weft-airgap", Expand: false}
		for i := 1; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--expand" || a == "-e":
				opts.Expand = true
			case (a == "-o" || a == "--out") && i+1 < len(args):
				opts.OutDir = args[i+1]
				i++
			case strings.HasPrefix(a, "--out="):
				opts.OutDir = strings.TrimPrefix(a, "--out=")
			case (a == "--from" || a == "--private-data") && i+1 < len(args):
				opts.From = args[i+1]
				i++
			case strings.HasPrefix(a, "--from="):
				opts.From = strings.TrimPrefix(a, "--from=")
			}
		}
		if err := weft.PrepareOfflinePack(opts); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "presets", "models":
		weft.PrintPresets()
		return 0
	case "finetune", "ft":
		opts := weft.FinetuneOptions{
			DataDir: "weft-sft",
			Expand:  true,
		}
		for i := 1; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--backend" || a == "-b":
				if i+1 < len(args) {
					opts.Backend = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--backend="):
				opts.Backend = strings.TrimPrefix(a, "--backend=")
			case a == "--data" || a == "-d":
				if i+1 < len(args) {
					opts.DataDir = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--data="):
				opts.DataDir = strings.TrimPrefix(a, "--data=")
			case a == "--model" || a == "-m":
				if i+1 < len(args) {
					opts.Model = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--model="):
				opts.Model = strings.TrimPrefix(a, "--model=")
			case a == "--preset" || a == "-p":
				if i+1 < len(args) {
					opts.Preset = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--preset="):
				opts.Preset = strings.TrimPrefix(a, "--preset=")
			case a == "--out" || a == "-o":
				if i+1 < len(args) {
					opts.OutDir = args[i+1]
					i++
				}
			case a == "--epochs":
				if i+1 < len(args) {
					fmt.Sscanf(args[i+1], "%d", &opts.Epochs)
					i++
				}
			case a == "--from" || a == "--private-data":
				if i+1 < len(args) {
					opts.From = args[i+1]
					i++
				}
			case a == "--dry-run":
				opts.DryRun = true
			case a == "--wait":
				opts.Wait = true
			case a == "--skip-prepare":
				opts.SkipPrepare = true
			case a == "--expand" || a == "-e":
				opts.Expand = true
			case a == "--install-deps":
				opts.InstallDeps = true
			case a == "--private":
				opts.Private = true
			case a == "--allow-upload":
				opts.AllowUpload = true
			case a == "--private-endpoint":
				opts.PrivateEndpoint = true
			}
		}
		if err := weft.Finetune(opts); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "status":
		job := ""
		for i := 1; i < len(args); i++ {
			if (args[i] == "--job" || args[i] == "-j") && i+1 < len(args) {
				job = args[i+1]
				i++
			} else if strings.HasPrefix(args[i], "--job=") {
				job = strings.TrimPrefix(args[i], "--job=")
			} else if !strings.HasPrefix(args[i], "-") {
				job = args[i]
			}
		}
		if job == "" {
			fmt.Fprintln(os.Stderr, "usage: weft train status --job ftjob-...")
			return 2
		}
		if err := weft.FinetuneStatus("", "", job); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "chat", "try":
		// weft train chat "prompt" [--model ft:...]
		prompt := "Write a Weft script that prints hello, weft"
		model := ""
		for i := 1; i < len(args); i++ {
			a := args[i]
			if (a == "--model" || a == "-m") && i+1 < len(args) {
				model = args[i+1]
				i++
			} else if strings.HasPrefix(a, "--model=") {
				model = strings.TrimPrefix(a, "--model=")
			} else if !strings.HasPrefix(a, "-") {
				prompt = a
			}
		}
		if err := weft.TrainChat(prompt, model); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "validate":
		if err := weft.ValidateTrainCorpus(); err != nil {
			return 1
		}
		return 0
	case "stats":
		expand := false
		for _, a := range args[1:] {
			if a == "--expand" || a == "-e" {
				expand = true
			}
		}
		if err := weft.TrainStats(expand); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "export":
		chat := false
		for _, a := range args[1:] {
			if a == "--chat" {
				chat = true
			}
		}
		var err error
		if chat {
			err = weft.WriteTrainChatJSONL(os.Stdout)
		} else {
			err = weft.WriteTrainJSONL(os.Stdout)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown train subcommand %q\n", args[0])
		return 2
	}
}

func cmdGen(args []string) int {
	opts := weft.GenOptions{Out: "generated.weft", MaxRetries: 2}
	var taskParts []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--out":
			if i+1 < len(args) {
				opts.Out = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--out="):
			opts.Out = strings.TrimPrefix(a, "--out=")
		case a == "--model" || a == "-m":
			if i+1 < len(args) {
				opts.Model = args[i+1]
				i++
			}
		case a == "--run":
			opts.RunAfter = true
		case a == "--dry-run":
			opts.DryRun = true
		case a == "-q" || a == "--quiet":
			opts.Quiet = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown flag %q\n", a)
			return 2
		default:
			taskParts = append(taskParts, a)
		}
	}
	opts.Task = strings.Join(taskParts, " ")
	if opts.Task == "" {
		fmt.Fprintln(os.Stderr, `usage: weft gen "describe the script" [-o out.weft] [--run] [--model id]`)
		return 2
	}
	if err := weft.Gen(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

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

func isCommand(s string) bool {
	switch s {
	case "run", "version", "--version", "-v", "help", "-h", "--help",
		"repl", "check", "test", "stdlib", "fmt", "bench", "init", "new", "mod", "get", "install", "list", "deps",
		"packages", "pkgs", "catalog",
		"prompt", "teach", "train", "eval", "gen", "doctor", "ollama", "vllm", "lsp":
		return true
	}
	return false
}

func cmdRun(args []string) int {
	path := args[0]
	scriptArgs := []string{path}
	if len(args) > 1 {
		rest := args[1:]
		if rest[0] == "--" {
			rest = rest[1:]
		}
		scriptArgs = append(scriptArgs, rest...)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := weft.New(weft.Options{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Args:   scriptArgs,
	})
	if err := ctx.RunFile(context.Background(), abs); err != nil {
		if ee, ok := err.(weft.ExitError); ok {
			if !ee.Silent() && ee.Error() != "" {
				fmt.Fprintln(os.Stderr, ee.Error())
			}
			return ee.Code
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdCheck(path string, showTypes bool) int {
	return cmdCheckPaths([]string{path}, showTypes)
}

func cmdCheckPaths(paths []string, showTypes bool) int {
	if err := weft.CheckPaths(paths, showTypes); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdStdlib(args []string) int {
	if len(args) >= 1 && !strings.HasPrefix(args[0], "-") {
		name := args[0]
		members := weft.StdlibMembers(name)
		if members == nil {
			fmt.Fprintf(os.Stderr, "unknown stdlib package %q\n", name)
			return 1
		}
		fmt.Printf("%s (%d)\n", name, len(members))
		for _, m := range members {
			fmt.Printf("  %s\n", m)
		}
		return 0
	}
	names := weft.StdlibNames()
	fmt.Printf("weft stdlib — %d packages\n", len(names))
	for _, n := range names {
		mem := weft.StdlibMembers(n)
		fmt.Printf("  %-14s %d members\n", n, len(mem))
	}
	fmt.Println("\nweft stdlib <name>  # list members")
	return 0
}

func cmdFmt(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: weft fmt <file.weft|dir>…")
		return 2
	}
	n, err := weft.FmtFiles(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if n == 0 {
		fmt.Println("already formatted")
	} else {
		fmt.Printf("formatted %d file(s)\n", n)
	}
	return 0
}

func cmdBench(args []string) int {
	opts := weft.BenchOptions{N: 1000}
	var paths []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-n" || a == "--count":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &opts.N)
			}
		case strings.HasPrefix(a, "-n="):
			fmt.Sscanf(strings.TrimPrefix(a, "-n="), "%d", &opts.N)
		case a == "-q" || a == "--quiet":
			opts.Quiet = true
		case a == "-run" || a == "--run":
			if i+1 < len(args) {
				i++
				opts.Filter = args[i]
			}
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown flag %s\n", a)
			fmt.Fprintln(os.Stderr, "usage: weft bench [path…] [-n N] [-run filter]")
			return 2
		default:
			paths = append(paths, a)
		}
	}
	opts.Paths = paths
	rep, err := weft.RunBench(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return weft.PrintBenchReport(rep, opts.Quiet)
}

func cmdTest(args []string) int {
	opts := weft.TestOptions{}
	var paths []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-q" || a == "--quiet":
			opts.Quiet = true
		case a == "-run" || a == "--run":
			if i+1 < len(args) {
				i++
				opts.Filter = args[i]
			}
		case strings.HasPrefix(a, "-run="):
			opts.Filter = strings.TrimPrefix(a, "-run=")
		case strings.HasPrefix(a, "--run="):
			opts.Filter = strings.TrimPrefix(a, "--run=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown flag %s\n", a)
			fmt.Fprintln(os.Stderr, "usage: weft test [path…] [-q] [-run filter]")
			return 2
		default:
			paths = append(paths, a)
		}
	}
	opts.Paths = paths
	rep, err := weft.RunTests(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return weft.PrintTestReport(rep, opts.Quiet)
}
