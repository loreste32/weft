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
	Cases []TestCase
	Pass  int
	Fail  int
	Skip  int
	Files int
	Total int
}

// TestOptions configures discovery and execution.
type TestOptions struct {
	// Paths are files or directories (default: ".").
	Paths []string
	// Quiet suppresses per-test lines (summary only).
	Quiet bool
	// Filter substring: only run test names containing Filter.
	Filter string
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

	for _, f := range files {
		cases, err := runTestFile(f, opts)
		if err != nil {
			// file-level failure (parse/compile)
			rep.Cases = append(rep.Cases, TestCase{
				File: f, Name: "(file)", OK: false, Err: err.Error(),
			})
			rep.Fail++
			rep.Total++
			continue
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

func runTestFile(path string, opts TestOptions) ([]TestCase, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file, perrs := parse.ParseFile(path, string(src))
	if perrs.HasErrors() {
		return nil, fmt.Errorf("parse: %v", perrs)
	}

	// Fresh env per file so tests don't share mutable state.
	ctx := New(opts.Runtime)
	env := ctx.Env()
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
		return nil, fmt.Errorf("compile: %v", cerrs)
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
		// File matched pattern but no test_* — still report
		cases = append(cases, TestCase{
			File: path, Name: "(no test_* fns)", Skipped: true, Reason: "no tests", OK: true,
		})
		return cases, nil
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
	return cases, nil
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
