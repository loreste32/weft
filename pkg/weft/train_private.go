package weft

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ModelPreset is a named open-weight base for private local fine-tunes.
type ModelPreset struct {
	ID          string // CLI name: qwen-7b
	Model       string // HF id or local path hint
	VRAMHint    string
	Description string
}

// FrontierPresets are open models you can train privately (data stays on-box).
// Closed APIs (OpenAI/Anthropic) are not private by default — use --private for local.
func FrontierPresets() []ModelPreset {
	return []ModelPreset{
		{ID: "qwen-1.5b", Model: "Qwen/Qwen2.5-1.5B-Instruct", VRAMHint: "~4 GB", Description: "fast smoke / laptops"},
		{ID: "qwen-7b", Model: "Qwen/Qwen2.5-7B-Instruct", VRAMHint: "~16 GB LoRA", Description: "strong small frontier open"},
		{ID: "qwen-32b", Model: "Qwen/Qwen2.5-32B-Instruct", VRAMHint: "~48+ GB LoRA", Description: "large open coder/agent"},
		{ID: "llama-8b", Model: "meta-llama/Llama-3.1-8B-Instruct", VRAMHint: "~16 GB LoRA", Description: "Meta open frontier 8B"},
		{ID: "llama-70b", Model: "meta-llama/Llama-3.1-70B-Instruct", VRAMHint: "~2×A100 LoRA", Description: "large open frontier"},
		{ID: "gemma-9b", Model: "google/gemma-2-9b-it", VRAMHint: "~20 GB LoRA", Description: "Google open instruct"},
		{ID: "phi-14b", Model: "microsoft/Phi-3-medium-4k-instruct", VRAMHint: "~24 GB LoRA", Description: "compact MS open"},
		{ID: "deepseek-7b", Model: "deepseek-ai/deepseek-llm-7b-chat", VRAMHint: "~16 GB LoRA", Description: "DeepSeek open chat"},
	}
}

// ResolvePreset returns the HF model id for a preset name, or "" if unknown.
func ResolvePreset(name string) (ModelPreset, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, p := range FrontierPresets() {
		if p.ID == n {
			return p, true
		}
	}
	return ModelPreset{}, false
}

// PrintPresets lists private training model presets.
func PrintPresets() {
	fmt.Println("Private frontier presets (local LoRA — training data never uploaded):")
	fmt.Println()
	fmt.Printf("  %-12s %-42s %-14s %s\n", "PRESET", "MODEL", "VRAM", "NOTES")
	for _, p := range FrontierPresets() {
		fmt.Printf("  %-12s %-42s %-14s %s\n", p.ID, p.Model, p.VRAMHint, p.Description)
	}
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println(`  weft train finetune --private --preset qwen-7b`)
	fmt.Println(`  weft train finetune --private --model /path/to/local-weights`)
	fmt.Println(`  weft train offline -o weft-airgap --expand   # USB / air-gapped GPU box`)
	fmt.Println()
	fmt.Println("Cloud OpenAI fine-tune uploads chat.jsonl — not private. Use only with --allow-upload.")
}

// isCloudOpenAI reports whether base URL is a public OpenAI host (data leaves your network).
func isCloudOpenAI(base string) bool {
	b := strings.ToLower(strings.TrimSpace(base))
	b = strings.TrimPrefix(b, "https://")
	b = strings.TrimPrefix(b, "http://")
	return strings.HasPrefix(b, "api.openai.com") || b == "" || b == "api.openai.com/v1"
}

// OfflinePackOptions configures an air-gapped training bundle.
type OfflinePackOptions struct {
	OutDir string
	Expand bool
	// From is an optional extra private JSONL (instruction or chat rows) merged in.
	From string
}

// PrepareOfflinePack writes a self-contained private training kit (no network).
// Copy the directory to a GPU box; run finetune --private there. Data never hits the public API.
func PrepareOfflinePack(opts OfflinePackOptions) error {
	if opts.OutDir == "" {
		opts.OutDir = "weft-airgap"
	}
	if err := PrepareTrainBundle(PrepareOptions{
		OutDir:  opts.OutDir,
		Expand:  opts.Expand,
		FewShot: 8,
		From:    opts.From,
	}); err != nil {
		return err
	}

	privacy := `# Weft private / air-gapped fine-tune

This folder is built for **private training**: your corpus stays on machines you control.

## Guarantee

| Path | Data leaves machine? |
|------|----------------------|
| ` + "`weft train finetune --private`" + ` | **No** — local LoRA on open weights |
| ` + "`python train_trl.py`" + ` (same dir) | **No** (disable HF hub if offline) |
| ` + "`weft train finetune --backend openai`" + ` | **Yes** — uploads to the API host |

## On an air-gapped GPU box

1. Copy this entire directory (USB / scp / internal artifact store).
2. Place base weights locally (or pre-cache HF under ` + "`$HF_HOME`" + `).
3. Run:

` + "```bash" + `
export HF_HUB_OFFLINE=1          # no network to Hugging Face
export TRANSFORMERS_OFFLINE=1
export WANDB_DISABLED=true
export TOKENIZERS_PARALLELISM=false

# already on the box if you copied a local model tree:
weft train finetune --private --skip-prepare --data . \
  --model /models/Qwen2.5-7B-Instruct \
  --out weft-lora --epochs 3

# or use a preset when the hub is reachable once to download weights:
weft train finetune --private --preset qwen-7b --skip-prepare --data .
` + "```" + `

## Merge more private domain data

` + "```bash" + `
# instruction rows: {"instruction","input","output","system"?}
# or chat rows: {"messages":[...]}
weft train prepare -o . --from /secure/my-domain.jsonl --expand
` + "```" + `

## After training

Keep adapters under ` + "`weft-lora/`" + `. Serve with vLLM / TGI / Ollama merge on **your** infra.
Point Weft gen/chat at a private OpenAI-compatible base:

` + "```bash" + `
export WEFT_API_BASE=` + DefaultVLLMBase + `
export WEFT_API_KEY=local
export WEFT_MODEL=weft-writer
weft train chat "write hello weft"
` + "```" + `

## Do not

- Do not run ` + "`--backend openai`" + ` without ` + "`--allow-upload`" + ` if this corpus is confidential.
- Do not enable Weights & Biases / remote logging on air-gapped jobs.
`
	if err := os.WriteFile(filepath.Join(opts.OutDir, "PRIVACY.md"), []byte(privacy), 0o644); err != nil {
		return err
	}
	// train_private.sh — one-shot local entrypoint
	sh := `#!/usr/bin/env bash
# Private local fine-tune — data never uploaded.
set -euo pipefail
cd "$(dirname "$0")"
export WANDB_DISABLED=true
export TOKENIZERS_PARALLELISM=false
export HF_HUB_DISABLE_TELEMETRY=1
MODEL="${1:-Qwen/Qwen2.5-7B-Instruct}"
OUT="${2:-weft-lora}"
EPOCHS="${3:-3}"
if command -v weft >/dev/null 2>&1; then
  exec weft train finetune --private --skip-prepare --data . --model "$MODEL" --out "$OUT" --epochs "$EPOCHS"
fi
python3 train_trl.py --data chat.jsonl --model "$MODEL" --out "$OUT" --epochs "$EPOCHS" --private
`
	if err := os.WriteFile(filepath.Join(opts.OutDir, "train_private.sh"), []byte(sh), 0o755); err != nil {
		return err
	}

	fmt.Printf("offline private pack → %s/\n", opts.OutDir)
	fmt.Println("  PRIVACY.md · train_private.sh · chat.jsonl · train_trl.py")
	fmt.Println("  copy this folder to a GPU box; training data stays private")
	fmt.Printf("  then: weft train finetune --private --skip-prepare --data %s --preset qwen-7b\n", opts.OutDir)
	return nil
}
