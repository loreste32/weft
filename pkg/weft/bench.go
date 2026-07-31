package weft

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/vm"
)

// BenchOptions configures weft bench.
type BenchOptions struct {
	// Paths: files or dirs. Default: discover *_bench.weft / bench_*.weft under .
	Paths []string
	// N iterations per bench (default 1000)
	N int
	// Quiet: one line per bench
	Quiet bool
	// Filter: substring match on function name
	Filter string
	// Compare: path to a previous bench run JSON to compare against
	Compare string
}

// BenchResult is one fn bench_* measurement.
type BenchResult struct {
	File string
	Name string
	N    int
	NsOp int64 // nanoseconds per iteration
	OK   bool
	Err  string
}

// BenchReport aggregates results.
type BenchReport struct {
	Results []BenchResult
}

// RunBench discovers and runs bench_* functions (zero-arg) in bench files.
// Convention: files `*_bench.weft` or `bench_*.weft`, functions `fn bench_*`.
func RunBench(opts BenchOptions) (*BenchReport, error) {
	if opts.N <= 0 {
		opts.N = 1000
	}
	paths := opts.Paths
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, err := discoverBenchFiles(paths)
	if err != nil {
		return nil, err
	}
	rep := &BenchReport{}
	for _, f := range files {
		rs, err := runBenchFile(f, opts)
		if err != nil {
			rep.Results = append(rep.Results, BenchResult{
				File: f, Name: "(file)", OK: false, Err: err.Error(),
			})
			continue
		}
		rep.Results = append(rep.Results, rs...)
	}
	return rep, nil
}

func discoverBenchFiles(paths []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		out = append(out, abs)
	}
	for _, root := range paths {
		st, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			if isBenchFile(root) {
				add(root)
			}
			continue
		}
		err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				name := info.Name()
				if name == "vendor" || name == ".git" || name == "node_modules" || name == "editors" {
					return filepath.SkipDir
				}
				return nil
			}
			if isBenchFile(path) {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

func isBenchFile(path string) bool {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".weft") && !strings.HasSuffix(base, ".loom") {
		return false
	}
	name := strings.TrimSuffix(strings.TrimSuffix(base, ".weft"), ".loom")
	return strings.HasSuffix(name, "_bench") || strings.HasPrefix(name, "bench_")
}

func runBenchFile(path string, opts BenchOptions) ([]BenchResult, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file, perrs := parse.ParseFile(path, string(src))
	if perrs.HasErrors() {
		return nil, fmt.Errorf("parse: %v", perrs)
	}
	ctx := New(Options{LLMDo: defaultEvalLLMMock})
	env := ctx.Env()
	prog, cerrs := compile.CompileFileLib(file, env)
	if cerrs.HasErrors() {
		return nil, fmt.Errorf("compile: %v", cerrs)
	}
	var names []string
	for name, fn := range prog.Funcs {
		if !strings.HasPrefix(name, "bench_") || fn.Arity != 0 {
			continue
		}
		if opts.Filter != "" && !strings.Contains(name, opts.Filter) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	machine := vm.New(env)
	var results []BenchResult
	for _, name := range names {
		fn := prog.Funcs[name]
		r := BenchResult{File: path, Name: name, N: opts.N}
		// warmup
		if _, err := machine.RunFunc(fn, nil); err != nil {
			r.OK = false
			r.Err = err.Error()
			results = append(results, r)
			continue
		}
		start := time.Now()
		failed := false
		for i := 0; i < opts.N; i++ {
			if _, err := machine.RunFunc(fn, nil); err != nil {
				r.OK = false
				r.Err = err.Error()
				results = append(results, r)
				failed = true
				break
			}
		}
		if failed {
			continue
		}
		elapsed := time.Since(start)
		r.NsOp = elapsed.Nanoseconds() / int64(opts.N)
		r.OK = true
		results = append(results, r)
	}
	return results, nil
}

// PrintBenchReport prints human-readable bench results; returns exit code.
func PrintBenchReport(rep *BenchReport, quiet bool) int {
	if rep == nil || len(rep.Results) == 0 {
		fmt.Println("no benches (files: *_bench.weft · fn bench_*)")
		return 0
	}
	fail := 0
	for _, r := range rep.Results {
		rel := r.File
		if wd, err := os.Getwd(); err == nil {
			if x, err := filepath.Rel(wd, r.File); err == nil {
				rel = x
			}
		}
		if !r.OK {
			fail++
			fmt.Printf("FAIL  %s::%s  — %s\n", rel, r.Name, r.Err)
			continue
		}
		// human units
		ns := float64(r.NsOp)
		unit := "ns/op"
		if ns >= 1e6 {
			ns /= 1e6
			unit = "ms/op"
		} else if ns >= 1e3 {
			ns /= 1e3
			unit = "µs/op"
		}
		fmt.Printf("ok    %s::%s  %d  %.2f %s\n", rel, r.Name, r.N, ns, unit)
	}
	if fail > 0 {
		return 1
	}
	return 0
}

// SaveBenchJSON saves bench results as JSON for later --compare.
func SaveBenchJSON(rep *BenchReport, path string) error {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i, r := range rep.Results {
		if i > 0 {
			sb.WriteString(",\n")
		}
		fmt.Fprintf(&sb, `  {"name":%q,"ns_op":%d,"ok":%v}`, r.Name, r.NsOp, r.OK)
	}
	sb.WriteString("\n]\n")
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// CompareBench compares current results against a baseline JSON file.
func CompareBench(rep *BenchReport, baselinePath string) error {
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		return fmt.Errorf("reading baseline: %w", err)
	}
	// parse simple JSON array
	baseline := map[string]int64{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, ",")
		if !strings.HasPrefix(line, `{"name":`) {
			continue
		}
		// extract name and ns_op
		var name string
		var nsOp int64
		fmt.Sscanf(extractJSONField(line, "name"), "%q", &name)
		fmt.Sscanf(extractJSONField(line, "ns_op"), "%d", &nsOp)
		if name != "" {
			baseline[name] = nsOp
		}
	}

	fmt.Println("\n--- comparison vs baseline ---")
	fmt.Printf("%-30s %12s %12s %8s\n", "BENCH", "BASELINE", "CURRENT", "CHANGE")
	for _, r := range rep.Results {
		if !r.OK {
			continue
		}
		base, ok := baseline[r.Name]
		if !ok {
			fmt.Printf("%-30s %12s %12s %8s\n", r.Name, "(new)", fmtNs(r.NsOp), "—")
			continue
		}
		pct := 0.0
		if base > 0 {
			pct = float64(r.NsOp-base) / float64(base) * 100
		}
		sign := ""
		if pct > 0 {
			sign = "+"
		}
		fmt.Printf("%-30s %12s %12s %s%.1f%%\n", r.Name, fmtNs(base), fmtNs(r.NsOp), sign, pct)
	}
	return nil
}

func fmtNs(ns int64) string {
	f := float64(ns)
	switch {
	case f >= 1e6:
		return fmt.Sprintf("%.2f ms", f/1e6)
	case f >= 1e3:
		return fmt.Sprintf("%.2f µs", f/1e3)
	default:
		return fmt.Sprintf("%d ns", ns)
	}
}

func extractJSONField(line, field string) string {
	key := `"` + field + `":`
	idx := strings.Index(line, key)
	if idx < 0 {
		return ""
	}
	rest := line[idx+len(key):]
	// find end: next comma or }
	end := strings.IndexAny(rest, ",}")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}
