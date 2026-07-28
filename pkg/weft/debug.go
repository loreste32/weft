package weft

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/vm"
)

// RunDebug starts an interactive debug session on a .weft file.
func RunDebug(path string, in io.Reader, out io.Writer) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	file, perrs := parse.ParseFile(abs, string(src))
	if perrs.HasErrors() {
		return fmt.Errorf("parse: %v", perrs)
	}
	env := runtime.NewEnv()
	stdlib.Register(env, stdlib.Options{})
	env.Set("__args", runtime.List(runtime.Str(abs)))
	prog, cerrs := compile.CompileFile(file, env)
	if cerrs.HasErrors() {
		return fmt.Errorf("compile: %v", cerrs)
	}
	if prog.Main == nil {
		return fmt.Errorf("no main function")
	}

	// Set up debug state
	ds := vm.NewDebugState()
	ds.StepMode = true // start in step mode

	scanner := bufio.NewScanner(in)
	lines := strings.Split(string(src), "\n")

	ds.OnPause = func(loc vm.FrameLoc, locals map[string]runtime.Value) {
		// Show current location
		if loc.File != "" && loc.Line > 0 {
			rel := loc.File
			if r, err := filepath.Rel(".", loc.File); err == nil {
				rel = r
			}
			fmt.Fprintf(out, "\n%s:%d", rel, loc.Line)
			if loc.Func != "" {
				fmt.Fprintf(out, " in %s", loc.Func)
			}
			fmt.Fprintln(out)
			// Show source line
			if loc.Line-1 < len(lines) {
				fmt.Fprintf(out, "  > %s\n", lines[loc.Line-1])
			}
		}

		// Debug REPL
		for {
			fmt.Fprint(out, "(debug) ")
			if !scanner.Scan() {
				ds.StepMode = false
				return
			}
			cmd := strings.TrimSpace(scanner.Text())
			switch {
			case cmd == "" || cmd == "n" || cmd == "next" || cmd == "s" || cmd == "step":
				ds.StepMode = true
				return
			case cmd == "c" || cmd == "continue":
				ds.StepMode = false
				return
			case cmd == "q" || cmd == "quit":
				fmt.Fprintln(out, "quit")
				os.Exit(0)
			case cmd == "l" || cmd == "locals":
				for name, val := range locals {
					if !strings.HasPrefix(name, "__") {
						fmt.Fprintf(out, "  %s = %s\n", name, val.String())
					}
				}
			case cmd == "s" || cmd == "stack":
				fmt.Fprintln(out, "  (use 'locals' to inspect variables)")
			case strings.HasPrefix(cmd, "b ") || strings.HasPrefix(cmd, "break "):
				bp := strings.TrimPrefix(strings.TrimPrefix(cmd, "break "), "b ")
				bp = strings.TrimSpace(bp)
				if !strings.Contains(bp, ":") {
					bp = loc.File + ":" + bp
				}
				ds.Breakpoints[bp] = true
				fmt.Fprintf(out, "breakpoint: %s\n", bp)
			case cmd == "bp" || cmd == "breakpoints":
				for bp := range ds.Breakpoints {
					fmt.Fprintf(out, "  %s\n", bp)
				}
			case strings.HasPrefix(cmd, "p ") || strings.HasPrefix(cmd, "print "):
				name := strings.TrimPrefix(strings.TrimPrefix(cmd, "print "), "p ")
				name = strings.TrimSpace(name)
				if v, ok := locals[name]; ok {
					fmt.Fprintf(out, "  %s = %s\n", name, v.String())
				} else {
					fmt.Fprintf(out, "  %s not found\n", name)
				}
			case cmd == "h" || cmd == "help":
				fmt.Fprintln(out, `  n/next/step  step to next line
  c/continue   run until breakpoint
  l/locals     show local variables
  p <name>     print variable
  b <line>     set breakpoint (e.g. b 10)
  bp           list breakpoints
  q/quit       exit debugger
  h/help       this text`)
			default:
				fmt.Fprintf(out, "  unknown command %q (type 'help')\n", cmd)
			}
		}
	}

	machine := vm.New(env)
	machine.Debug = ds
	fmt.Fprintf(out, "weft debugger — %s\n", path)
	fmt.Fprintln(out, "type 'help' for commands, 'n' to step, 'c' to continue")
	_, err = machine.RunFunc(prog.Main, nil)
	if err != nil {
		fmt.Fprintf(out, "error: %v\n", err)
	}
	fmt.Fprintln(out, "program finished")
	return nil
}

// StartDebug is the CLI entry point.
func StartDebug(path string) error {
	return RunDebug(path, os.Stdin, os.Stdout)
}

// Profile runs a script and reports function timing.
func RunProfile(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Enable coverage to track which functions are called
	env := runtime.NewEnv()
	stdlib.Register(env, stdlib.Options{})
	env.Set("__args", runtime.List(runtime.Str(abs)))
	env.Coverage = map[string]bool{}

	ctx := &Context{env: env}
	err = ctx.RunFile(context.Background(), abs)

	// Print profile (functions hit)
	fmt.Println("\n--- profile ---")
	fmt.Printf("functions called: %d\n", len(env.Coverage))
	for fn := range env.Coverage {
		fmt.Printf("  %s\n", fn)
	}
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
	return err
}
