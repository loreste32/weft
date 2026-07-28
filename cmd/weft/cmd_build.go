package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/loreste/weft/pkg/weft"
)

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
