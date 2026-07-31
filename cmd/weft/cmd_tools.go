package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/loreste/weft/pkg/weft"
)

func isCommand(s string) bool {
	switch s {
	case "run", "build", "lint", "doc", "info", "sysinfo", "version", "--version", "-v", "help", "-h", "--help",
		"repl", "check", "test", "stdlib", "fmt", "bench", "init", "new", "mod", "get", "install", "list", "deps",
		"packages", "pkgs", "catalog",
		"publish", "registry", "notebook", "nb", "debug", "profile",
		"prompt", "teach", "train", "eval", "gen", "doctor", "ollama", "vllm", "lsp",
		"update", "upgrade", "outdated", "mcp":
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
	return cmdCheckPaths([]string{path}, showTypes, false)
}

func cmdCheckPaths(paths []string, showTypes bool, strict bool) int {
	if err := weft.CheckPathsOpts(paths, showTypes, strict); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdStdlib(args []string) int {
	if len(args) >= 1 && !strings.HasPrefix(args[0], "-") {
		name := args[0]
		// handle "pkg.member" — show help AND run it if it takes no args
		if strings.Contains(name, ".") {
			parts := strings.SplitN(name, ".", 2)
			return stdlibMemberInfo(parts[0], parts[1])
		}
		// also handle "weft stdlib sysinfo cpu_count" (two args)
		if len(args) >= 2 && !strings.HasPrefix(args[1], "-") {
			return stdlibMemberInfo(name, args[1])
		}
		members := weft.StdlibMembers(name)
		if members == nil {
			fmt.Fprintf(os.Stderr, "unknown stdlib package %q\nrun: weft stdlib  # list all packages\n", name)
			return 1
		}
		fmt.Printf("%s (%d members)\n\n", name, len(members))
		for _, m := range members {
			sig, detail := weft.StdlibMemberHelp(name, m)
			if sig != "" {
				fmt.Printf("  %-24s %s\n", sig, detail)
			} else {
				fmt.Printf("  %s.%s\n", name, m)
			}
		}
		fmt.Printf("\nuse %s\n", name)
		return 0
	}
	names := weft.StdlibNames()
	fmt.Printf("weft stdlib — %d packages\n\n", len(names))
	// group by category
	categories := []struct {
		name string
		pkgs []string
	}{
		{"LLM / AI", []string{"llm", "ollama", "vllm", "mcp", "deepgram", "elevenlabs", "mlinfer"}},
		{"Web", []string{"http", "web", "ws", "webrtc", "graphql", "url"}},
		{"Data", []string{"db", "redis", "mongo", "nats", "amqp", "csv", "json", "jsonl", "yaml", "toml", "xml", "ini", "table"}},
		{"DevOps", []string{"sysinfo", "proc", "netutil", "sh", "fs", "cli", "env", "platform", "signal", "secrets", "log", "os"}},
		{"Network", []string{"pcap", "socket", "email", "ip", "dns", "tls"}},
		{"Text / Math", []string{"str", "re", "math", "decimal", "time", "random", "uuid", "base64", "encoding", "mime", "html", "crypto"}},
		{"Collections", []string{"iter", "collections", "heap", "bisect", "pipe", "functools", "copy", "traceback"}},
		{"Runtime", []string{"governor", "supervisor", "cluster", "ratelimit", "migrate", "metrics", "tokenizer", "dataset"}},
		{"Other", []string{"archive", "compress", "binstruct", "difflib", "shlex", "pickle", "viz", "io", "test"}},
	}
	listed := map[string]bool{}
	for _, cat := range categories {
		var inCat []string
		for _, p := range cat.pkgs {
			for _, n := range names {
				if n == p {
					inCat = append(inCat, n)
					listed[n] = true
				}
			}
		}
		if len(inCat) == 0 {
			continue
		}
		fmt.Printf("  %s:\n", cat.name)
		for _, n := range inCat {
			mem := weft.StdlibMembers(n)
			fmt.Printf("    %-14s %d members\n", n, len(mem))
		}
		fmt.Println()
	}
	// unlisted
	for _, n := range names {
		if !listed[n] {
			mem := weft.StdlibMembers(n)
			fmt.Printf("  %-14s %d members\n", n, len(mem))
		}
	}
	fmt.Println("weft stdlib <name>  # show members with signatures")
	return 0
}

func stdlibMemberInfo(pkg, member string) int {
	members := weft.StdlibMembers(pkg)
	if members == nil {
		fmt.Fprintf(os.Stderr, "unknown stdlib package %q\nrun: weft stdlib  # list all packages\n", pkg)
		return 1
	}
	known := false
	for _, candidate := range members {
		if candidate == member {
			known = true
			break
		}
	}
	if !known {
		fmt.Fprintf(os.Stderr, "unknown stdlib member %q\n", pkg+"."+member)
		fmt.Fprintf(os.Stderr, "available members: %s\n", strings.Join(members, ", "))
		return 1
	}

	sig, detail := weft.StdlibMemberHelp(pkg, member)
	if sig != "" {
		fmt.Printf("\n  %s\n  %s\n\n", sig, detail)
	} else {
		fmt.Printf("\n  %s.%s\n\n", pkg, member)
	}
	if sig == "" || !canProbeWithoutArgs(sig) {
		if sig != "" && !canProbeWithoutArgs(sig) {
			fmt.Println("  (not run: this member requires arguments)")
			fmt.Println()
		}
		return 0
	}

	// Run zero-argument functions and render their result with labels where
	// the shape has well-defined units or ordering. Capture the probe so a
	// failed lookup cannot look like a successful command.
	src := fmt.Sprintf("fn main -> Result { say(json.pretty(%s.%s()?)) }", pkg, member)
	var output bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &output, Stderr: os.Stderr})
	err := ctx.RunSource(context.Background(), "stdlib-probe.weft", src)
	if err != nil {
		if strings.Contains(sig, "-> Result") {
			fmt.Fprintf(os.Stderr, "unable to run %s.%s: %v\n", pkg, member, err)
			return 1
		}
		// try without ? (non-Result functions)
		src = fmt.Sprintf("fn main { say(json.pretty(%s.%s())) }", pkg, member)
		output.Reset()
		ctx = weft.New(weft.Options{Stdout: &output, Stderr: os.Stderr})
		if fallbackErr := ctx.RunSource(context.Background(), "stdlib-probe.weft", src); fallbackErr != nil {
			fmt.Fprintf(os.Stderr, "unable to run %s.%s: %v\n", pkg, member, fallbackErr)
			return 1
		}
	}
	printStdlibProbe(pkg, member, output.Bytes())
	fmt.Println()
	return 0
}

func canProbeWithoutArgs(sig string) bool {
	start := strings.IndexByte(sig, '(')
	if start < 0 {
		return false
	}
	end := strings.IndexByte(sig[start+1:], ')')
	if end < 0 {
		return false
	}
	params := strings.TrimSpace(sig[start+1 : start+1+end])
	if params == "" {
		return true
	}
	for _, param := range strings.Split(params, ",") {
		param = strings.TrimSpace(param)
		if !strings.HasSuffix(param, "?") {
			return false
		}
	}
	return true
}

func printStdlibProbe(pkg, member string, raw []byte) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		fmt.Print(string(raw))
		return
	}

	switch pkg + "." + member {
	case "sysinfo.memory":
		if m, ok := value.(map[string]any); ok {
			printByteSummary(m, "available")
			return
		}
	case "sysinfo.disk":
		if m, ok := value.(map[string]any); ok {
			printByteSummary(m, "free")
			if path, ok := m["path"].(string); ok && path != "" {
				fmt.Printf("  path: %s\n", path)
			}
			return
		}
	case "sysinfo.loadavg":
		if values, ok := value.([]any); ok && len(values) >= 3 {
			fmt.Println("  1m:", values[0])
			fmt.Println("  5m:", values[1])
			fmt.Println("  15m:", values[2])
			return
		}
	case "sysinfo.uptime":
		if m, ok := value.(map[string]any); ok {
			if seconds, ok := m["seconds"]; ok {
				fmt.Printf("  seconds: %v\n", seconds)
			}
			if human, ok := m["human"].(string); ok {
				fmt.Printf("  duration: %s\n", human)
			}
			return
		}
	case "sysinfo.cpu_count":
		fmt.Printf("  logical CPUs visible to this process: %v\n", value)
		return
	case "proc.self":
		if m, ok := value.(map[string]any); ok {
			printProcessSummary(m)
			return
		}
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err == nil {
		fmt.Print(pretty.String())
	} else {
		fmt.Print(string(raw))
	}
}

func printByteSummary(m map[string]any, availableKey string) {
	for _, key := range []string{"total", availableKey, "used"} {
		if value, ok := numberValue(m[key]); ok {
			fmt.Printf("  %-10s %s (%s bytes)\n", key+":", formatBytes(value), formatInteger(value))
		}
	}
	if percent, ok := numberValue(m["percent"]); ok {
		fmt.Printf("  %-10s %.2f%% used\n", "percent:", percent)
	}
	if availableKey == "free" {
		fmt.Println("  note: free is space available to the current user")
	}
	if unit, ok := m["unit"].(string); ok && unit != "" {
		fmt.Printf("  unit: %s\n", unit)
	}
}

func printProcessSummary(m map[string]any) {
	labels := []struct {
		key   string
		label string
	}{
		{"pid", "pid"},
		{"ppid", "parent pid"},
		{"uid", "user id"},
		{"gid", "group id"},
		{"user", "user"},
		{"group", "group"},
		{"executable", "executable"},
		{"cwd", "working directory"},
	}
	for _, item := range labels {
		if value, ok := m[item.key]; ok && value != nil {
			fmt.Printf("  %-19s %v\n", item.label+":", value)
		}
	}
}

func numberValue(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func formatInteger(value float64) string {
	return strconv.FormatInt(int64(math.Round(value)), 10)
}

func formatBytes(value float64) string {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return "unknown"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f %s", value, units[unit])
	}
	return fmt.Sprintf("%.2f %s", value, units[unit])
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
	var savePath string
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
		case a == "--compare":
			if i+1 < len(args) {
				i++
				opts.Compare = args[i]
			}
		case a == "--save":
			if i+1 < len(args) {
				i++
				savePath = args[i]
			}
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown flag %s\n", a)
			fmt.Fprintln(os.Stderr, "usage: weft bench [path…] [-n N] [-run filter] [--compare baseline.json] [--save out.json]")
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
	code := weft.PrintBenchReport(rep, opts.Quiet)
	if savePath != "" {
		if err := weft.SaveBenchJSON(rep, savePath); err != nil {
			fmt.Fprintf(os.Stderr, "save: %v\n", err)
		} else {
			fmt.Printf("saved to %s\n", savePath)
		}
	}
	if opts.Compare != "" {
		if err := weft.CompareBench(rep, opts.Compare); err != nil {
			fmt.Fprintf(os.Stderr, "compare: %v\n", err)
		}
	}
	return code
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
		case a == "--race":
			opts.Race = true
		case a == "--memprofile" || a == "--mem":
			opts.Memprofile = true
		case a == "--timeout" || a == "-t":
			if i+1 < len(args) {
				i++
				n, _ := fmt.Sscanf(args[i], "%d", &opts.Timeout)
				if n == 0 {
					opts.Timeout = 30
				}
			}
		case strings.HasPrefix(a, "--timeout="):
			fmt.Sscanf(strings.TrimPrefix(a, "--timeout="), "%d", &opts.Timeout)
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
			fmt.Fprintln(os.Stderr, "usage: weft test [path…] [-q] [-run filter] [--race] [--mem] [--timeout N]")
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
		showAll := false
		for _, a := range args[1:] {
			if a == "--all" || a == "-a" {
				showAll = true
			} else {
				if q != "" {
					q += " "
				}
				q += a
			}
		}
		if err := weft.RegistrySearch(q, showAll); err != nil {
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
	case "trust":
		// weft registry trust <namespace> <pubkey-hex>
		// weft registry trust <namespace> --key <local-key-name>
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: weft registry trust <namespace> <pubkey-hex>")
			fmt.Fprintln(os.Stderr, "       weft registry trust <namespace> --key <keyname>")
			return 2
		}
		ns := args[1]
		if args[2] == "--key" || args[2] == "-k" {
			if len(args) < 4 {
				fmt.Fprintln(os.Stderr, "usage: weft registry trust <namespace> --key <keyname>")
				return 2
			}
			if err := weft.RegistryTrustLocal(ns, args[3]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		}
		note := ""
		if len(args) > 3 && (args[3] == "--note" || args[3] == "-n") && len(args) > 4 {
			note = args[4]
		}
		if err := weft.RegistryTrust(ns, args[2], note); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "untrust":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: weft registry untrust <namespace> [pubkey-hex]")
			return 2
		}
		pub := ""
		if len(args) > 2 {
			pub = args[2]
		}
		if err := weft.RegistryUntrust(args[1], pub); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "trusts":
		if err := weft.RegistryListTrust(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "trust-rotate":
		// weft registry trust-rotate <ns> <new-pubkey> [--retire <old-pubkey>]
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: weft registry trust-rotate <namespace> <new-pubkey> [--retire <old-pubkey>]")
			return 2
		}
		ns, newPub := args[1], args[2]
		retire := ""
		for i := 3; i < len(args); i++ {
			if (args[i] == "--retire" || args[i] == "-r") && i+1 < len(args) {
				i++
				retire = args[i]
			}
		}
		if err := weft.RegistryRotateTrust(ns, newPub, retire); err != nil {
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
		fmt.Fprintln(os.Stderr, "usage: weft registry search|info|install|keygen|keys|trust|untrust|trusts|trust-rotate|serve")
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
