package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loreste/weft/pkg/weft"
)

func isCommand(s string) bool {
	switch s {
	case "run", "version", "--version", "-v", "help", "-h", "--help",
		"repl", "check", "test", "stdlib", "fmt", "bench", "init", "new", "mod", "get", "install", "list", "deps",
		"packages", "pkgs", "catalog",
		"publish", "registry", "notebook", "nb", "debug", "profile",
		"prompt", "teach", "train", "eval", "gen", "doctor", "ollama", "vllm", "lsp",
		"update", "upgrade", "mcp":
		return true
	}
	return false
}

func cmdRun(args []string) int {
	watch := false
	var filtered []string
	for _, a := range args {
		if a == "--watch" || a == "-w" {
			watch = true
		} else {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) == 0 {
		fmt.Fprintln(os.Stderr, "usage: weft run <file.weft> [--watch]")
		return 2
	}
	if watch {
		return cmdRunWatch(filtered)
	}
	return cmdRunOnce(filtered)
}

func cmdRunOnce(args []string) int {
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

func cmdRunWatch(args []string) int {
	path := args[0]
	abs, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir := filepath.Dir(abs)
	fmt.Fprintf(os.Stderr, "watching %s (press Ctrl-C to stop)\n", dir)

	lastRun := time.Time{}
	modTimes := map[string]time.Time{}

	for {
		changed := false
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				if name == "vendor" || name == ".git" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".weft") {
				return nil
			}
			mt := info.ModTime()
			if prev, ok := modTimes[p]; !ok || mt.After(prev) {
				modTimes[p] = mt
				if mt.After(lastRun) {
					changed = true
				}
			}
			return nil
		})

		if changed || lastRun.IsZero() {
			lastRun = time.Now()
			fmt.Fprintf(os.Stderr, "\n--- run %s ---\n", time.Now().Format("15:04:05"))
			cmdRunOnce(args)
		}
		time.Sleep(500 * time.Millisecond)
	}
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
	check := false
	var paths []string
	for _, a := range args {
		if a == "--check" || a == "-c" {
			check = true
		} else {
			paths = append(paths, a)
		}
	}
	if len(paths) < 1 {
		fmt.Fprintln(os.Stderr, "usage: weft fmt [--check] <file.weft|dir>…")
		return 2
	}
	if check {
		dirty, err := weft.FmtCheck(paths)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(dirty) == 0 {
			fmt.Println("all files formatted")
			return 0
		}
		for _, f := range dirty {
			fmt.Println(f)
		}
		fmt.Fprintf(os.Stderr, "%d file(s) need formatting\n", len(dirty))
		return 1
	}
	n, err := weft.FmtFiles(paths)
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
		case a == "--coverage" || a == "--cover" || a == "-cover":
			opts.Coverage = true
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

func cmdPublish(args []string) int {
	wd, _ := os.Getwd()
	dir := wd
	keyName := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--key" || args[i] == "-k":
			if i+1 < len(args) {
				keyName = args[i+1]
				i++
			}
		case !strings.HasPrefix(args[i], "-"):
			dir = args[i]
		}
	}
	if err := weft.Publish(dir, keyName); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdRegistry(args []string) int {
	if len(args) == 0 {
		args = []string{"search"}
	}
	switch args[0] {
	case "search", "list":
		q := ""
		if len(args) > 1 {
			q = strings.Join(args[1:], " ")
		}
		if err := weft.RegistrySearch(q); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "info":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft registry info <name>")
			return 2
		}
		if err := weft.RegistryInfo(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "install", "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft registry install <name[@constraint]>")
			return 2
		}
		wd, _ := os.Getwd()
		if err := weft.RegistryInstall(wd, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("ok")
		return 0
	case "keygen":
		name := "default"
		if len(args) > 1 {
			name = args[1]
		}
		if err := weft.RegistryKeygen(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "keys":
		if err := weft.RegistryListKeys(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "serve":
		addr := ":8089"
		dataDir := "./registry-data"
		token := os.Getenv("WEFT_REGISTRY_TOKEN")
		for i := 1; i < len(args); i++ {
			switch {
			case args[i] == "--addr" || args[i] == "-a":
				if i+1 < len(args) {
					i++
					addr = args[i]
				}
			case args[i] == "--data" || args[i] == "-d":
				if i+1 < len(args) {
					i++
					dataDir = args[i]
				}
			}
		}
		if err := weft.RegistryServe(addr, dataDir, token); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(os.Stderr, "usage: weft registry search|info|install|keygen|keys|serve")
		return 2
	}
}

func cmdNotebook(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: weft notebook <file.weft> [-o output.html]")
		return 2
	}
	path := ""
	outPath := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-o" || args[i] == "--output":
			if i+1 < len(args) {
				i++
				outPath = args[i]
			}
		case !strings.HasPrefix(args[i], "-"):
			path = args[i]
		}
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "usage: weft notebook <file.weft> [-o output.html]")
		return 2
	}
	if err := weft.RunNotebookToHTML(path, outPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	out := outPath
	if out == "" {
		out = strings.TrimSuffix(path, filepath.Ext(path)) + ".html"
	}
	fmt.Printf("wrote %s\n", out)
	return 0
}
