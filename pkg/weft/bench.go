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
