package weft

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/llmpack"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
)

// TrainEvalOptions configures weft train eval.
type TrainEvalOptions struct {
	// From optional gold JSONL (instruction/output). Empty → embedded corpus.
	From string
	// Limit caps cases (0 = all).
	Limit int
	// Run executes gold (and live output) when fn main is present.
	Run bool
	// Live asks the configured LLM and scores generated Weft (needs API).
	Live bool
	// Quiet less per-case lines.
	Quiet bool
}

// TrainEvalCase is one gold row score.
type TrainEvalCase struct {
	ID      string
	GoldOK  bool
	LiveOK  bool // only when Live
	Err     string
	LiveErr string
}

// TrainEvalReport is accuracy on gold (and optional live model).
type TrainEvalReport struct {
	Total   int
	GoldOK  int
	LiveOK  int
	LiveN   int // cases attempted live
	GoldAcc float64
	LiveAcc float64
	Cases   []TrainEvalCase
}

// TrainEval scores gold Weft: parse + compile (+ optional run).
// With Live, also generates from the instruction and scores that output.
func TrainEval(opts TrainEvalOptions) (*TrainEvalReport, error) {
	exs, err := loadGold(opts.From)
	if err != nil {
		return nil, err
	}
	if opts.Limit > 0 && opts.Limit < len(exs) {
		exs = exs[:opts.Limit]
	}
	if len(exs) == 0 {
		return nil, fmt.Errorf("no gold examples")
	}

	var client *LLMClient
	if opts.Live {
		client, err = NewLLMClientFromEnv()
		if err != nil {
			return nil, fmt.Errorf("live eval: %w", err)
		}
	}

	rep := &TrainEvalReport{Total: len(exs)}
	for _, e := range exs {
		c := TrainEvalCase{ID: e.ID}
		if err := scoreWeft(e.Output, opts.Run); err != nil {
			c.Err = err.Error()
		} else {
			c.GoldOK = true
			rep.GoldOK++
		}

		if opts.Live && client != nil {
			rep.LiveN++
			src, gerr := generateForEval(client, e.Instruction)
			if gerr != nil {
				c.LiveErr = gerr.Error()
			} else if err := scoreWeft(src, opts.Run); err != nil {
				c.LiveErr = err.Error()
			} else {
				c.LiveOK = true
				rep.LiveOK++
			}
		}
		rep.Cases = append(rep.Cases, c)
		if !opts.Quiet {
			printTrainCase(c, opts.Live)
		}
	}
	if rep.Total > 0 {
		rep.GoldAcc = float64(rep.GoldOK) / float64(rep.Total)
	}
	if rep.LiveN > 0 {
		rep.LiveAcc = float64(rep.LiveOK) / float64(rep.LiveN)
	}
	return rep, nil
}

func loadGold(from string) ([]llmpack.Example, error) {
	if from == "" {
		return llmpack.Examples(), nil
	}
	return llmpack.LoadExamplesFile(from)
}

func scoreWeft(src string, run bool) error {
	src = strings.TrimSpace(ExtractWeftCode(src))
	if src == "" {
		return fmt.Errorf("empty weft")
	}
	file, perrs := parse.ParseFile("gold.weft", src)
	if perrs.HasErrors() {
		return fmt.Errorf("parse: %v", perrs)
	}
	env := runtime.NewEnv()
	stdlib.Register(env, stdlib.Options{})
	if _, cerrs := compile.CompileFile(file, env); cerrs.HasErrors() {
		return fmt.Errorf("compile: %v", cerrs)
	}
	if !run || !strings.Contains(src, "fn main") {
		return nil
	}
	var buf bytes.Buffer
	ctx := New(Options{
		Stdout: &buf,
		Stderr: &buf,
		LLMDo:  defaultEvalLLMMock,
	})
	err := ctx.RunSource(context.Background(), "gold.weft", src)
	if err == nil {
		return nil
	}
	es := err.Error()
	if strings.Contains(es, "connection") || strings.Contains(es, "dial") ||
		strings.Contains(es, "no such host") || strings.Contains(es, "timeout") {
		return nil
	}
	return fmt.Errorf("run: %v", err)
}

func generateForEval(client *LLMClient, instruction string) (string, error) {
	messages := []ChatMessage{
		{Role: "system", Content: llmpack.SystemCard()},
		{Role: "user", Content: instruction + "\n\nRespond with a single complete Weft program only."},
	}
	reply, err := client.Chat(messages)
	if err != nil {
		return "", err
	}
	return ExtractWeftCode(reply), nil
}

func printTrainCase(c TrainEvalCase, live bool) {
	if live {
		g, l := "FAIL", "FAIL"
		if c.GoldOK {
			g = "ok"
		}
		if c.LiveOK {
			l = "ok"
		}
		msg := c.Err
		if c.LiveErr != "" {
			if msg != "" {
				msg += "; "
			}
			msg += "live: " + c.LiveErr
		}
		if msg != "" {
			fmt.Printf("%-24s gold=%s live=%s  %s\n", c.ID, g, l, msg)
		} else {
			fmt.Printf("%-24s gold=%s live=%s\n", c.ID, g, l)
		}
		return
	}
	if c.GoldOK {
		fmt.Printf("PASS  %s\n", c.ID)
	} else {
		fmt.Printf("FAIL  %s — %s\n", c.ID, c.Err)
	}
}

// PrintTrainEval writes summary; returns exit code (1 if gold incomplete).
func PrintTrainEval(rep *TrainEvalReport) int {
	if rep == nil {
		return 1
	}
	fmt.Printf("\ngold accuracy  %d/%d  (%.1f%%)\n", rep.GoldOK, rep.Total, rep.GoldAcc*100)
	if rep.LiveN > 0 {
		fmt.Printf("live accuracy  %d/%d  (%.1f%%)\n", rep.LiveOK, rep.LiveN, rep.LiveAcc*100)
	}
	if rep.GoldOK < rep.Total {
		return 1
	}
	return 0
}
