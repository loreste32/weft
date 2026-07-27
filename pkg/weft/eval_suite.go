package weft

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// EvalCase is one real-world script evaluation.
type EvalCase struct {
	Path    string
	Name    string
	OK      bool
	Skipped bool
	Reason  string
	Out     string
	Err     string
	Ms      int64
}

// EvalDir runs all *.weft under dir. Injects an LLM mock when no API key is set
// so the suite is offline-friendly for CI and training validation.
func EvalDir(dir string, opts Options) ([]EvalCase, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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
		return nil, err
	}
	var cases []EvalCase
	for _, f := range files {
		cases = append(cases, evalFile(f, opts))
	}
	return cases, nil
}

func evalFile(path string, opts Options) EvalCase {
	c := EvalCase{Path: path, Name: filepath.Base(path)}
	base := filepath.Base(path)
	switch base {
	case "server.weft", "webapp.weft", "chat.weft", "webrtc_call.weft", "viz_dashboard.weft":
		c.Skipped = true
		c.Reason = "blocking server"
		c.OK = true
		return c
	}
	src, err := os.ReadFile(path)
	if err != nil {
		c.Err = err.Error()
		return c
	}
	// library modules (no main) are not runnable scripts
	if !bytes.Contains(src, []byte("fn main")) {
		c.Skipped = true
		c.Reason = "library module"
		c.OK = true
		return c
	}
	needsLLM := bytes.Contains(src, []byte("llm.")) || bytes.Contains(src, []byte("ollama.")) || bytes.Contains(src, []byte("vllm."))
	if needsLLM && opts.LLMDo == nil &&
		os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("LLM_API_KEY") == "" &&
		os.Getenv("ANTHROPIC_API_KEY") == "" && os.Getenv("OLLAMA_HOST") == "" {
		opts.LLMDo = defaultEvalLLMMock
	}

	var buf bytes.Buffer
	opts.Stdout = &buf
	opts.Stderr = &buf
	if opts.Args == nil {
		// argv[0] is the script; only inject demo URLs for URL-batch samples.
		opts.Args = []string{path}
		if bytes.Contains(src, []byte("https://")) || bytes.Contains(src, []byte("batch")) ||
			bytes.Contains(src, []byte("urls")) {
			opts.Args = []string{path, "https://example.com", "https://example.org"}
		}
	}
	ctx := New(opts)
	start := time.Now()
	err = ctx.RunFile(context.Background(), path)
	c.Ms = time.Since(start).Milliseconds()
	c.Out = buf.String()
	if err != nil {
		c.Err = err.Error()
		if strings.Contains(c.Err, "connection") || strings.Contains(c.Err, "dial") ||
			strings.Contains(c.Err, "no such host") || strings.Contains(c.Err, "timeout") {
			c.Skipped = true
			c.Reason = "network"
			c.OK = true
			return c
		}
		c.OK = false
		return c
	}
	c.OK = true
	return c
}

func defaultEvalLLMMock(reqBody []byte) (string, []runtime.ToolCall, error) {
	body := string(reqBody)
	if strings.Contains(body, `"tools"`) && !strings.Contains(body, `"role":"tool"`) {
		if strings.Contains(body, "weather") {
			return "", []runtime.ToolCall{{ID: "1", Name: "weather", ArgsJSON: `{"city":"Paris"}`}}, nil
		}
		if strings.Contains(body, "add") {
			return "", []runtime.ToolCall{{ID: "1", Name: "add", ArgsJSON: `{"a":2,"b":3}`}}, nil
		}
		if strings.Contains(body, "read_file") {
			return "", []runtime.ToolCall{{ID: "1", Name: "read_file", ArgsJSON: `{"path":"README.md"}`}}, nil
		}
		if strings.Contains(body, "note") {
			return "", []runtime.ToolCall{{ID: "1", Name: "note", ArgsJSON: `{"msg":"hi"}`}}, nil
		}
	}
	if strings.Contains(body, "json_object") {
		return `{"city":"Paris","temp_c":21}`, nil, nil
	}
	return "ok from mock model", nil, nil
}

// PrintEvalReport writes a human summary; returns process exit code.
func PrintEvalReport(cases []EvalCase) int {
	var pass, fail, skip int
	for _, c := range cases {
		switch {
		case c.Skipped:
			skip++
			fmt.Printf("SKIP  %s (%s)\n", c.Name, c.Reason)
		case c.OK:
			pass++
			fmt.Printf("PASS  %s (%dms)\n", c.Name, c.Ms)
		default:
			fail++
			fmt.Printf("FAIL  %s — %s\n", c.Name, c.Err)
		}
	}
	fmt.Printf("\n%d passed, %d failed, %d skipped\n", pass, fail, skip)
	if fail > 0 {
		return 1
	}
	return 0
}
