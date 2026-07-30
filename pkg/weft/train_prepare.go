package weft

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/loreste/weft/internal/llmpack"
)

// PrepareOptions configures weft train prepare.
type PrepareOptions struct {
	OutDir  string
	Expand  bool // paraphrase instructions
	FewShot int  // also write fewshot.md
	// From is an optional private domain JSONL merged into the gold corpus.
	// Data is read only from disk — never uploaded by prepare itself.
	From string
}

// PrepareTrainBundle writes a ready-to-train directory.
//
//	weft-sft/
//	  SYSTEM.md
//	  sft.jsonl alpaca.jsonl chat.jsonl sharegpt.jsonl completions.jsonl
//	  fewshot.md
//	  README.md (dataset card)
//	  stats.json
//	  train.axolotl.yml  (starter config)
//	  train_trl.py       (minimal HF script)
func PrepareTrainBundle(opts PrepareOptions) error {
	if opts.OutDir == "" {
		opts.OutDir = "weft-sft"
	}
	if opts.FewShot <= 0 {
		opts.FewShot = 8
	}
	exs := llmpack.Examples()
	if len(exs) == 0 {
		return fmt.Errorf("empty training corpus")
	}
	// validate base first
	if errs := llmpack.ValidateAll(); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		return fmt.Errorf("corpus validation failed")
	}
	privateN := 0
	if opts.From != "" {
		extra, err := llmpack.LoadExamplesFile(opts.From)
		if err != nil {
			return fmt.Errorf("private data --from: %w", err)
		}
		for _, example := range extra {
			if err := llmpack.ValidateExample(example); err != nil {
				return fmt.Errorf("private data --from: %w", err)
			}
		}
		privateN = len(extra)
		exs = append(exs, extra...)
		fmt.Fprintf(os.Stderr, "merged %d private examples from %s\n", privateN, opts.From)
	}
	baseCount := len(exs)
	if opts.Expand {
		exs = llmpack.Expand(exs)
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return err
	}

	// SYSTEM.md
	if err := os.WriteFile(filepath.Join(opts.OutDir, "SYSTEM.md"), []byte(llmpack.SystemCard()), 0o644); err != nil {
		return err
	}
	// fewshot
	if err := os.WriteFile(filepath.Join(opts.OutDir, "fewshot.md"), []byte(llmpack.FewShot(opts.FewShot)), 0o644); err != nil {
		return err
	}

	writers := []struct {
		name string
		fn   func(io.Writer, []llmpack.Example) error
	}{
		{"sft.jsonl", llmpack.WriteInstructionJSONL},
		{"chat.jsonl", llmpack.WriteChatJSONL},
		{"alpaca.jsonl", llmpack.WriteAlpacaJSONL},
		{"sharegpt.jsonl", llmpack.WriteShareGPTJSONL},
		{"completions.jsonl", llmpack.WriteCompletionsJSONL},
	}
	for _, w := range writers {
		f, err := os.Create(filepath.Join(opts.OutDir, w.name))
		if err != nil {
			return err
		}
		if err := w.fn(f, exs); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	st := llmpack.ComputeStats(exs)
	st.Expanded = len(exs) - baseCount
	sb, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(filepath.Join(opts.OutDir, "stats.json"), append(sb, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "README.md"), []byte(llmpack.DatasetCard(st)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "train.axolotl.yml"), []byte(axolotlConfig()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "train_trl.py"), []byte(trlScriptV2()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "requirements-train.txt"), []byte(trainReqs()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "QUICKSTART.md"), []byte(quickstartMD(opts.OutDir, st)), 0o644); err != nil {
		return err
	}

	fmt.Printf("prepared %s/\n", opts.OutDir)
	fmt.Printf("  %d examples", st.Count)
	if st.Expanded > 0 {
		fmt.Printf(" (%d rows before expand + %d paraphrases)", baseCount, st.Expanded)
	}
	if privateN > 0 {
		fmt.Printf("  [+%d private domain]", privateN)
	}
	fmt.Println()
	fmt.Println("  formats: sft, chat, alpaca, sharegpt, completions")
	fmt.Println("  private (data stays on-box):")
	fmt.Printf("    weft train finetune --private --data %s --preset qwen-7b\n", opts.OutDir)
	fmt.Printf("    weft train offline -o weft-airgap --expand   # air-gapped GPU kit\n")
	fmt.Println("  cloud OpenAI (uploads chat.jsonl — needs --allow-upload):")
	fmt.Printf("    weft train finetune --backend openai --allow-upload --data %s --wait\n", opts.OutDir)
	fmt.Println("  see QUICKSTART.md · PRIVACY path: --private")
	return nil
}

// TrainStats prints corpus statistics.
func TrainStats(expand bool) error {
	exs := llmpack.Examples()
	base := len(exs)
	if expand {
		exs = llmpack.Expand(exs)
	}
	st := llmpack.ComputeStats(exs)
	fmt.Printf("examples: %d", st.Count)
	if expand {
		fmt.Printf(" (expanded from %d)", base)
	}
	fmt.Println()
	fmt.Printf("avg output chars: %.0f\n", st.AvgOut)
	fmt.Println("tags:")
	tags := make([]string, 0, len(st.Tags))
	for tag := range st.Tags {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		fmt.Printf("  %-12s %d\n", tag, st.Tags[tag])
	}
	return nil
}

func axolotlConfig() string {
	return `# Axolotl starter — Weft SFT (ShareGPT)
# https://github.com/OpenAccess-AI-Collective/axolotl
base_model: meta-llama/Llama-3.2-3B-Instruct
model_type: AutoModelForCausalLM
tokenizer_type: AutoTokenizer

load_in_8bit: true
adapter: lora
lora_r: 16
lora_alpha: 32
lora_target_modules:
  - q_proj
  - v_proj
  - k_proj
  - o_proj

datasets:
  - path: sharegpt.jsonl
    type: sharegpt
    conversation: chatml

dataset_prepared_path: last_run_prepared
val_set_size: 0.05
output_dir: ./lora-out

sequence_len: 4096
sample_packing: true
pad_to_sequence_len: true

gradient_accumulation_steps: 4
micro_batch_size: 2
num_epochs: 3
optimizer: adamw_torch
lr_scheduler: cosine
learning_rate: 0.0002

train_on_inputs: false
group_by_length: false
bf16: auto
gradient_checkpointing: true
logging_steps: 5
warmup_ratio: 0.05
saves_per_epoch: 1
`
}

func quickstartMD(dir string, st llmpack.Stats) string {
	return fmt.Sprintf(`# Fine-tune a Weft model

Dataset: **%d** examples (SYSTEM.md baked into chat rows).

## Private (recommended for confidential data)

Training data **never leaves your machines**. Uses open frontier weights + local LoRA.

`+"```bash"+`
weft train finetune --private --data %s --preset qwen-7b
# or air-gapped:
weft train offline -o weft-airgap --expand
# copy weft-airgap/ to GPU box, then:
weft train finetune --private --skip-prepare --data weft-airgap --model /models/Qwen2.5-7B-Instruct
`+"```"+`

| Preset | Model |
|--------|-------|
| qwen-7b | Qwen2.5-7B-Instruct |
| llama-8b | Llama-3.1-8B-Instruct |
| qwen-32b | Qwen2.5-32B-Instruct |

`+"```bash"+`
weft train presets
`+"```"+`

Merge **your** private domain JSONL (stays local):

`+"```bash"+`
weft train prepare -o %s --from /secure/domain.jsonl --expand
`+"```"+`

## Cloud OpenAI (uploads data)

Only when the corpus is OK to leave your network:

`+"```bash"+`
export OPENAI_API_KEY=sk-...
weft train finetune --backend openai --allow-upload --data %s --wait
`+"```"+`

Private **self-hosted** OpenAI-compatible fine-tune API (your VPC — not public OpenAI):

`+"```bash"+`
export WEFT_API_BASE=https://ft.internal.corp/v1
export WEFT_API_KEY=...
weft train finetune --backend openai --private-endpoint --wait
`+"```"+`

## After training

`+"```bash"+`
weft check model_output.weft
weft run model_output.weft
weft eval examples/realworld/
`+"```"+`
`, st.Count, dir, dir, dir)
}
