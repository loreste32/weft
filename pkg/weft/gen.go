package weft

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/llmpack"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/types"
)

// GenOptions configures weft gen (pure-Go LLM → Weft code).
type GenOptions struct {
	// Task is the natural-language request.
	Task string
	// Out path for .weft file (default: generated.weft)
	Out string
	// Model override
	Model string
	// MaxRetries if parse/check fails, re-ask model to fix
	MaxRetries int
	// DryRun prints the prompt only
	DryRun bool
	// RunAfter if true, execute the script after successful check
	RunAfter bool
	// Quiet less chatter
	Quiet bool
}

// Gen asks an LLM (HTTP API) to write Weft, validates it, optionally runs it.
func Gen(opts GenOptions) error {
	if strings.TrimSpace(opts.Task) == "" {
		return fmt.Errorf("usage: weft gen \"describe what you want\"")
	}
	if opts.Out == "" {
		opts.Out = "generated.weft"
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 2
	}
	if opts.DryRun {
		fmt.Println("--- system ---")
		fmt.Print(llmpack.SystemCard())
		fmt.Println("--- user ---")
		fmt.Println(opts.Task)
		return nil
	}

	client, err := NewLLMClientFromEnv()
	if err != nil {
		return err
	}
	if opts.Model != "" {
		client.Model = opts.Model
	}

	sys := llmpack.SystemCard()
	user := opts.Task + "\n\nRespond with a single complete Weft program only (fn main …). Prefer ```weft fences."

	var lastErr error
	var code string
	messages := []ChatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}

	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if !opts.Quiet {
			if attempt == 0 {
				fmt.Fprintf(os.Stderr, "generating Weft with %s …\n", client.Model)
			} else {
				fmt.Fprintf(os.Stderr, "fix attempt %d/%d …\n", attempt, opts.MaxRetries)
			}
		}
		reply, err := client.Chat(messages)
		if err != nil {
			return err
		}
		code = ExtractWeftCode(reply)
		if err := validateWeftSource(code); err != nil {
			lastErr = err
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: reply},
				ChatMessage{Role: "user", Content: "That Weft failed validation:\n" + err.Error() + "\n\nReturn a fixed complete Weft program only."},
			)
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		// still write for inspection
		_ = mustWriteFile(opts.Out, code+"\n")
		return fmt.Errorf("model output invalid after retries: %w\n(wrote %s for inspection)", lastErr, opts.Out)
	}

	if err := mustWriteFile(opts.Out, code+"\n"); err != nil {
		return err
	}
	abs, _ := filepath.Abs(opts.Out)
	if !opts.Quiet {
		fmt.Println("wrote", abs)
	}

	if opts.RunAfter {
		if !opts.Quiet {
			fmt.Fprintln(os.Stderr, "running …")
		}
		ctx := New(Options{Stdout: os.Stdout, Stderr: os.Stderr, Args: []string{abs}})
		return ctx.RunFile(context.Background(), abs)
	}
	return nil
}

func validateWeftSource(src string) error {
	src = strings.TrimSpace(src)
	if src == "" {
		return fmt.Errorf("empty program")
	}
	if !strings.Contains(src, "fn main") {
		return fmt.Errorf("missing fn main")
	}
	file, errs := parse.ParseFile("gen.weft", src)
	if errs.HasErrors() {
		return errs
	}
	if cerrs := types.Check(file); cerrs.HasErrors() {
		// soft: only fail hard on parse for gen? Prefer both for quality
		return cerrs
	}
	return nil
}

// TrainChat sends a prompt using SYSTEM.md to verify a fine-tuned (or any) model.
func TrainChat(prompt string, model string) error {
	if strings.TrimSpace(prompt) == "" {
		prompt = "Write a Weft script that prints hello, weft"
	}
	client, err := NewLLMClientFromEnv()
	if err != nil {
		return err
	}
	if model != "" {
		client.Model = model
	}
	reply, err := client.Chat([]ChatMessage{
		{Role: "system", Content: llmpack.SystemCard()},
		{Role: "user", Content: prompt + "\n\nComplete Weft program only."},
	})
	if err != nil {
		return err
	}
	code := ExtractWeftCode(reply)
	fmt.Println(code)
	fmt.Fprintln(os.Stderr, "---")
	if err := validateWeftSource(code); err != nil {
		fmt.Fprintln(os.Stderr, "validation:", err)
		return err
	}
	fmt.Fprintln(os.Stderr, "validation: ok")
	return nil
}
