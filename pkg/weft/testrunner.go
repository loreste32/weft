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
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/vm"
)

// TestCase is one fn test_* result.
type TestCase struct {
	File    string
	Name    string // function name
	OK      bool
	Skipped bool
	Reason  string
	Err     string
	Ms      int64
}

// TestReport aggregates weft test results.
type TestReport struct {
	Cases    []TestCase
	Pass     int
	Fail     int
	Skip     int
	Files    int
	Total    int
	Coverage *CoverageReport // nil unless --coverage
}

// CoverageReport tracks which functions were hit during test runs.
type CoverageReport struct {
	Hit   map[string]bool // "file:func" → true
	All   map[string]bool // all declared functions
	Pct   float64
}

// TestOptions configures discovery and execution.
type TestOptions struct {
	// Paths are files or directories (default: ".").
	Paths []string
	// Quiet suppresses per-test lines (summary only).
	Quiet bool
	// Filter substring: only run test names containing Filter.
	Filter string
	// Coverage enables function-level coverage tracking.
	Coverage bool
	// Options for the Weft runtime (LLM mock, env, …).
	Runtime Options
}

// RunTests discovers *_test.weft / test_*.weft and runs every fn test_*.
func RunTests(opts TestOptions) (*TestReport, error) {
	paths := opts.Paths
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, err := discoverTestFiles(paths)
	if err != nil {
		return nil, err
	}
	rep := &TestReport{Files: len(files)}
	if len(files) == 0 {
		return rep, nil
	}
	// Offline-friendly LLM for tests that touch llm.*
	if opts.Runtime.LLMDo == nil {
		opts.Runtime.LLMDo = defaultEvalLLMMock
	}

	// Shared coverage set across all test files
	var covHit map[string]bool
	var covAll map[string]bool
	if opts.Coverage {
		covHit = map[string]bool{}
		covAll = map[string]bool{}
	}

	for _, f := range files {
		cases, allFuncs, err := runTestFile(f, opts, covHit)
		if err != nil {
			rep.Cases = append(rep.Cases, TestCase{
				File: f, Name: "(file)", OK: false, Err: err.Error(),
			})
			rep.Fail++
			rep.Total++
			continue
		}
		if covAll != nil {
			for k := range allFuncs {
				covAll[k] = true
			}
		}
		for _, c := range cases {
			rep.Cases = append(rep.Cases, c)
			rep.Total++
			switch {
			case c.Skipped:
				rep.Skip++
			case c.OK:
				rep.Pass++
			default:
				rep.Fail++
			}
		}
	}

	if opts.Coverage && len(covAll) > 0 {
		rep.Coverage = &CoverageReport{
			Hit: covHit,
			All: covAll,
			Pct: float64(len(covHit)) / float64(len(covAll)) * 100,
		}
	}

	return rep, nil
}

func discoverTestFiles(paths []string) ([]string, error) {
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
			if isTestFile(root) {
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
				if name == "vendor" || name == ".git" || name == "node_modules" ||
					name == "weft-sft" || name == "editors" {
					return filepath.SkipDir
				}
				return nil
			}
			if isTestFile(path) {
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

func isTestFile(path string) bool {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".weft") && !strings.HasSuffix(base, ".loom") {
		return false
	}
	name := strings.TrimSuffix(strings.TrimSuffix(base, ".weft"), ".loom")
	return strings.HasSuffix(name, "_test") || strings.HasPrefix(name, "test_")
}

func runTestFile(path string, opts TestOptions, covHit map[string]bool) ([]TestCase, map[string]bool, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	file, perrs := parse.ParseFile(path, string(src))
	if perrs.HasErrors() {
		return nil, nil, fmt.Errorf("parse: %v", perrs)
	}

	// Fresh env per file so tests don't share mutable state.
	ctx := New(opts.Runtime)
	env := ctx.Env()

	// Enable coverage tracking
	if covHit != nil {
		env.Coverage = covHit
	}
	// Package-dir tests: load weft.json entry first so `use "./lib.weft"` resolves and
	// same-folder modules work without a full vendor install.
	if dir := filepath.Dir(path); dir != "" {
		env.ProjectDir = DetectProjectDir(dir)
		if env.ProjectDir == "" {
			env.ProjectDir = dir
		}
	}

	prog, cerrs := compile.CompileFileLib(file, env)
	if cerrs.HasErrors() {
		return nil, nil, fmt.Errorf("compile: %v", cerrs)
	}

	// Collect all declared functions for coverage denominator
	allFuncs := map[string]bool{}
	for name := range prog.Funcs {
		key := path + ":" + name
		allFuncs[key] = true
	}

	var names []string
	for name, fn := range prog.Funcs {
		if !strings.HasPrefix(name, "test_") {
			continue
		}
		if fn.Arity != 0 {
			continue // only zero-arg tests
		}
		if opts.Filter != "" && !strings.Contains(name, opts.Filter) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	var cases []TestCase
	if len(names) == 0 {
		cases = append(cases, TestCase{
			File: path, Name: "(no test_* fns)", Skipped: true, Reason: "no tests", OK: true,
		})
		return cases, allFuncs, nil
	}

	machine := vm.New(env)
	for _, name := range names {
		fn := prog.Funcs[name]
		c := TestCase{File: path, Name: name}
		start := time.Now()
		_, err := machine.RunFunc(fn, nil)
		c.Ms = time.Since(start).Milliseconds()
		if err != nil {
			if msg, ok := stdlib.IsTestSkip(err); ok {
				c.Skipped = true
				c.Reason = msg
				c.OK = true
			} else {
				c.OK = false
				c.Err = err.Error()
			}
		} else {
			c.OK = true
		}
		cases = append(cases, c)
	}
	return cases, allFuncs, nil
}

// PrintTestReport writes a human summary; returns process exit code.
func PrintTestReport(rep *TestReport, quiet bool) int {
	if rep == nil {
		fmt.Println("no tests")
		return 0
	}
	if !quiet {
		for _, c := range rep.Cases {
			rel := c.File
			if wd, err := os.Getwd(); err == nil {
				if r, err := filepath.Rel(wd, c.File); err == nil {
					rel = r
				}
			}
			switch {
			case c.Skipped:
				fmt.Printf("SKIP  %s::%s  (%s)\n", rel, c.Name, c.Reason)
			case c.OK:
				fmt.Printf("ok    %s::%s  (%dms)\n", rel, c.Name, c.Ms)
			default:
				fmt.Printf("FAIL  %s::%s  — %s\n", rel, c.Name, c.Err)
			}
		}
		if len(rep.Cases) > 0 {
			fmt.Println()
		}
	}
	fmt.Printf("weft test  %d passed, %d failed, %d skipped  (%d files, %d cases)\n",
		rep.Pass, rep.Fail, rep.Skip, rep.Files, rep.Total)

	if rep.Coverage != nil {
		fmt.Printf("\ncoverage:  %.1f%% of functions (%d/%d)\n", rep.Coverage.Pct, len(rep.Coverage.Hit), len(rep.Coverage.All))
		if !quiet {
			// Show uncovered functions
			var uncovered []string
			for fn := range rep.Coverage.All {
				if !rep.Coverage.Hit[fn] {
					uncovered = append(uncovered, fn)
				}
			}
			sort.Strings(uncovered)
			if len(uncovered) > 0 && len(uncovered) <= 30 {
				fmt.Println("uncovered:")
				for _, fn := range uncovered {
					fmt.Printf("  %s\n", fn)
				}
			} else if len(uncovered) > 30 {
				fmt.Printf("uncovered: %d functions (use -q for summary only)\n", len(uncovered))
			}
		}
	}

	if rep.Fail > 0 {
		return 1
	}
	if rep.Total == 0 {
		fmt.Println("(no *_test.weft found — add fn test_* in foo_test.weft)")
	}
	return 0
}

// CheckPaths type-checks one or more .weft files (no execution).
func CheckPaths(paths []string, showTypes bool) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths")
	}
	var files []string
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return err
		}
		if st.IsDir() {
			err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					if info.Name() == "vendor" || info.Name() == ".git" {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(path, ".weft") || strings.HasSuffix(path, ".loom") {
					files = append(files, path)
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else {
			files = append(files, p)
		}
	}
	var first error
	n := 0
	for _, f := range files {
		info, err := InferFile(f)
		if err != nil {
			if first == nil {
				first = fmt.Errorf("%s: %w", f, err)
			}
			fmt.Fprintf(os.Stderr, "FAIL  %s — %v\n", f, err)
			continue
		}
		n++
		if showTypes {
			fmt.Printf("// %s\n", f)
			for name, t := range info.Bindings {
				if t != nil {
					fmt.Printf("  %s : %s\n", name, t.String())
				}
			}
			for name, t := range info.FnRet {
				if t != nil {
					fmt.Printf("  fn %s -> %s\n", name, t.String())
				}
			}
		} else {
			fmt.Printf("ok    %s\n", f)
		}
	}
	if first != nil {
		return first
	}
	if n == 0 {
		return fmt.Errorf("no .weft files")
	}
	return nil
}
