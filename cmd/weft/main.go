// Command weft is the Weft language CLI.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/loreste/weft/internal/lsp"
	"github.com/loreste/weft/pkg/weft"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// check if this binary has an embedded weft app
	if dir, entry, ok := weft.CheckEmbeddedApp(); ok {
		defer os.RemoveAll(dir)
		entryPath := dir + "/" + entry
		return cmdRun(append([]string{entryPath}, args...))
	}

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
	case "build":
		entry := "main.weft"
		output := ""
		dir := "."
		for i := 1; i < len(args); i++ {
			switch {
			case args[i] == "-o" && i+1 < len(args):
				i++
				output = args[i]
			case !strings.HasPrefix(args[i], "-"):
				if strings.HasSuffix(args[i], ".weft") {
					entry = args[i]
				} else {
					dir = args[i]
				}
			}
		}
		if err := weft.Build(dir, entry, output); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
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
	case "update":
		if err := weft.SelfUpdate(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "upgrade":
		dir := "."
		if len(args) > 1 {
			dir = args[1]
		}
		if err := weft.UpgradePackages(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "mcp":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft mcp serve <file.weft>")
			return 2
		}
		if args[1] == "serve" {
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "usage: weft mcp serve <file.weft>")
				return 2
			}
			if err := weft.MCPServe(args[2]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		}
		fmt.Fprintln(os.Stderr, "usage: weft mcp serve <file.weft>")
		return 2
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
		fmt.Print(`weft — weave agents into code

Language:
  weft                      REPL
  weft run <file.weft> [--watch]    run a script
  weft build [dir] [entry] [-o out] bundle into .weftapp archive
  weft check <file|dir>… [--types]
  weft test [path…] [-q] [-run filter]   # run fn test_* in *_test.weft
  weft stdlib [pkg]           # list stdlib packages (or members of pkg)
  weft fmt [--check] <file.weft|dir>…  # format (--check for CI)
  weft bench [path…] [-n N]  # microbench fn bench_* in *_bench.weft
  weft gen "task" [-o out.weft] [--run]   # LLM writes Weft (pure Go API)
  weft update                update weft to the latest version
  weft upgrade               upgrade installed packages to latest
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

Registry (public packages with ed25519 signing):
  weft registry search [q]    # browse registry
  weft registry info <name>   # package details
  weft registry install <name[@constraint]>
  weft registry keygen [name] # generate signing key
  weft registry keys          # list signing keys
  weft publish [--key name]   # validate, sign, upload
`)
		return 0
	case "notebook", "nb":
		return cmdNotebook(args[1:])
	case "debug":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft debug <file.weft>")
			return 2
		}
		if err := weft.StartDebug(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "profile":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft profile <file.weft>")
			return 2
		}
		if err := weft.RunProfile(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "publish":
		return cmdPublish(args[1:])
	case "registry":
		return cmdRegistry(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		return 2
	}
}
