package weft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FinetuneOptions controls weft train finetune.
//
// Privacy model:
//   - --private / backend private|trl|local: data never uploaded (open frontier LoRA on-box)
//   - backend openai without --allow-upload: refused when targeting public OpenAI
//   - backend openai + --private-endpoint: your VPC OpenAI-compatible FT API only
//
// Weft runtime never requires Python. Local LoRA uses an optional Python/TRL helper.
type FinetuneOptions struct {
	// Backend: "private" (local, recommended) | "openai" (API) | "trl" (alias of private)
	Backend string
	// DataDir is the prepare output (contains chat.jsonl)
	DataDir string
	// Model base model id or local path
	Model string
	// Preset selects a named open frontier model (see FrontierPresets)
	Preset string
	// OutDir for local adapters/checkpoints
	OutDir string
	// Epochs for local training
	Epochs int
	// DryRun only prints the command / plan
	DryRun bool
	// SkipPrepare if data dir already exists
	SkipPrepare bool
	// Expand when auto-preparing
	Expand bool
	// InstallDeps allows trl path to pip install (off by default)
	InstallDeps bool
	// Private forces local-only training (never uploads)
	Private bool
	// AllowUpload permits sending chat.jsonl to a remote fine-tune API
	AllowUpload bool
	// PrivateEndpoint marks the API base as a self-hosted / VPC endpoint (not public OpenAI)
	PrivateEndpoint bool
	// From merges private domain JSONL when auto-preparing
	From string
	// OpenAIAPIKey overrides env OPENAI_API_KEY / WEFT_API_KEY
	OpenAIAPIKey string
	// OpenAIBaseURL default https://api.openai.com/v1 (any OpenAI-compatible FT API)
	OpenAIBaseURL string
	// Poll job until terminal (optional)
	Wait bool
}

// Finetune runs a fine-tune job via the chosen backend.
func Finetune(opts FinetuneOptions) error {
	if opts.OpenAIAPIKey == "" {
		opts.OpenAIAPIKey = firstEnv("OPENAI_API_KEY", "WEFT_API_KEY")
	}
	if opts.OpenAIBaseURL == "" {
		opts.OpenAIBaseURL = firstEnv("OPENAI_BASE_URL", "WEFT_API_BASE")
	}
	if opts.OpenAIBaseURL == "" {
		opts.OpenAIBaseURL = "https://api.openai.com/v1"
	}
	// Env WEFT_TRAIN_PRIVATE=1 forces private mode
	if os.Getenv("WEFT_TRAIN_PRIVATE") == "1" || strings.EqualFold(os.Getenv("WEFT_TRAIN_PRIVATE"), "true") {
		opts.Private = true
	}
	if opts.Private {
		if opts.Backend == "" || opts.Backend == "openai" || opts.Backend == "api" || opts.Backend == "http" {
			opts.Backend = "private"
		}
	}
	if opts.Backend == "" {
		// Prefer private when no upload consent and no explicit cloud intent
		if opts.AllowUpload || opts.PrivateEndpoint {
			opts.Backend = "openai"
		} else {
			opts.Backend = "private"
		}
	}
	if opts.DataDir == "" {
		opts.DataDir = "weft-sft"
	}
	if opts.OutDir == "" {
		opts.OutDir = "weft-finetune-out"
	}
	if opts.Epochs <= 0 {
		opts.Epochs = 3
	}
	if opts.Preset != "" {
		p, ok := ResolvePreset(opts.Preset)
		if !ok {
			return fmt.Errorf("unknown preset %q — run: weft train presets", opts.Preset)
		}
		if opts.Model == "" {
			opts.Model = p.Model
		}
		fmt.Fprintf(os.Stderr, "preset %s → %s (%s)\n", p.ID, p.Model, p.VRAMHint)
	}

	// Ensure training data exists
	chatPath := filepath.Join(opts.DataDir, "chat.jsonl")
	if _, err := os.Stat(chatPath); err != nil {
		if opts.SkipPrepare {
			return fmt.Errorf("missing %s — run: weft train prepare -o %s --expand", chatPath, opts.DataDir)
		}
		fmt.Fprintf(os.Stderr, "preparing dataset in %s …\n", opts.DataDir)
		if err := PrepareTrainBundle(PrepareOptions{
			OutDir:  opts.DataDir,
			Expand:  opts.Expand,
			FewShot: 8,
			From:    opts.From,
		}); err != nil {
			return err
		}
	}

	switch strings.ToLower(opts.Backend) {
	case "openai", "api", "http":
		if opts.Private {
			return fmt.Errorf(`--private refuses cloud upload.

Use a local open frontier model instead:
  weft train finetune --private --preset qwen-7b
  weft train finetune --private --model /path/to/weights

Or for a self-hosted FT API inside your VPC (not public OpenAI):
  export WEFT_API_BASE=https://ft.internal/v1
  weft train finetune --backend openai --private-endpoint --wait`)
		}
		if isCloudOpenAI(opts.OpenAIBaseURL) && !opts.AllowUpload && !opts.PrivateEndpoint {
			return fmt.Errorf(`refusing to upload training data to public OpenAI (privacy).

Your chat.jsonl would leave this machine. Pick one:

  # Private — open frontier LoRA, data stays local (recommended)
  weft train finetune --private --preset qwen-7b

  # Air-gapped pack for an offline GPU box
  weft train offline -o weft-airgap --expand

  # Explicit cloud upload (you accept data leaving the network)
  weft train finetune --backend openai --allow-upload --wait

  # Self-hosted OpenAI-compatible FT API in your VPC
  export WEFT_API_BASE=https://ft.internal.corp/v1
  weft train finetune --backend openai --private-endpoint --wait`)
		}
		if opts.Model == "" {
			opts.Model = DefaultOpenAIModel
		}
		if opts.OpenAIAPIKey == "" && !opts.DryRun {
			return fmt.Errorf(`fine-tune API needs a key:

  export OPENAI_API_KEY=sk-...
  weft train finetune --backend openai --allow-upload --wait

Private local path (no upload):
  weft train finetune --private --preset qwen-7b`)
		}
		if isCloudOpenAI(opts.OpenAIBaseURL) {
			fmt.Fprintln(os.Stderr, "privacy: uploading chat.jsonl to public OpenAI (--allow-upload)")
		} else {
			fmt.Fprintf(os.Stderr, "privacy: fine-tune API at %s (your endpoint)\n", opts.OpenAIBaseURL)
		}
		return finetuneOpenAI(opts)
	case "private", "trl", "local", "hf", "lora":
		fmt.Fprintln(os.Stderr, "privacy: local training — corpus is not uploaded")
		if opts.Model == "" {
			opts.Model = "Qwen/Qwen2.5-7B-Instruct"
		}
		return finetuneTRL(opts)
	default:
		return fmt.Errorf("unknown backend %q (want private, openai, or trl)", opts.Backend)
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func finetuneTRL(opts FinetuneOptions) error {
	script := filepath.Join(opts.DataDir, "train_trl.py")
	// Always refresh script to latest privacy-aware trainer
	if err := os.WriteFile(script, []byte(trlScriptV2()), 0o755); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(opts.DataDir, "requirements-train.txt"), []byte(trainReqs()), 0o644)
	_ = os.WriteFile(filepath.Join(opts.DataDir, "PRIVACY.md"), []byte(localPrivacyNote()), 0o644)

	args := []string{
		"--data", "chat.jsonl",
		"--model", opts.Model,
		"--out", opts.OutDir,
		"--epochs", fmt.Sprintf("%d", opts.Epochs),
		"--private",
	}

	fmt.Printf("backend: private (local LoRA)\nmodel:   %s\ndata:    %s/chat.jsonl\nout:     %s/%s\nprivacy: corpus stays on this machine\n\n",
		opts.Model, opts.DataDir, opts.DataDir, opts.OutDir)

	py := findPython()
	if opts.DryRun {
		if py == "" {
			py = "python3"
		}
		fmt.Printf("dry-run: cd %s && %s train_trl.py %s\n", opts.DataDir, py, strings.Join(args, " "))
		fmt.Println("install deps: pip install -r requirements-train.txt")
		fmt.Println("privacy: no network upload will be performed")
		return nil
	}
	if py == "" {
		return fmt.Errorf(`python3 not found — private local fine-tune needs a Python env for TRL/PyTorch only.

  # Install Python 3.10+, then:
  weft train finetune --private --preset qwen-7b --install-deps

  # Or air-gap pack for a machine that already has the stack:
  weft train offline -o weft-airgap --expand`)
	}

	cmd := exec.Command(py, append([]string{script}, args...)...)
	cmd.Dir = opts.DataDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"TOKENIZERS_PARALLELISM=false",
		"WANDB_DISABLED=true",
		"HF_HUB_DISABLE_TELEMETRY=1",
	)
	if err := exec.Command(py, "-c", "import trl, transformers, datasets, peft").Run(); err != nil {
		if !opts.InstallDeps {
			return fmt.Errorf(`private local backend needs Python packages (Weft runtime still pure Go):

  cd %s
  pip install -r requirements-train.txt
  weft train finetune --private --skip-prepare --data %s --model %s

Auto-install: weft train finetune --private --install-deps --preset qwen-7b`, opts.DataDir, opts.DataDir, opts.Model)
		}
		fmt.Fprintln(os.Stderr, "installing private-train deps (pip) …")
		inst := exec.Command(py, "-m", "pip", "install", "-r", "requirements-train.txt")
		inst.Dir = opts.DataDir
		inst.Stdout = os.Stdout
		inst.Stderr = os.Stderr
		if err := inst.Run(); err != nil {
			return fmt.Errorf("pip install failed: %w", err)
		}
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("private local training failed: %w", err)
	}
	fmt.Printf("\nprivate fine-tune complete → %s/%s\n", opts.DataDir, opts.OutDir)
	fmt.Println("adapters stayed local. Serve on your infra; point WEFT_API_BASE at your endpoint.")
	return nil
}

func localPrivacyNote() string {
	return `# Privacy — local Weft fine-tune

This training path does **not** upload chat.jsonl.

- Weights: load from Hugging Face hub or a **local directory** (` + "`--model /path/to/weights`" + `).
- Telemetry: WANDB disabled; HF telemetry disabled by Weft.
- Air-gap: set ` + "`HF_HUB_OFFLINE=1`" + ` and use pre-downloaded weights.

Cloud OpenAI fine-tune is a separate path and requires ` + "`--allow-upload`" + `.
`
}

func findPython() string {
	for _, c := range []string{"python3", "python"} {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

func finetuneOpenAI(opts FinetuneOptions) error {
	chatPath := filepath.Join(opts.DataDir, "chat.jsonl")
	fmt.Printf("backend: openai-api\nmodel:   %s\ndata:    %s\nbase:    %s\n\n", opts.Model, chatPath, opts.OpenAIBaseURL)
	if opts.DryRun {
		fmt.Println("dry-run: would upload chat.jsonl and create a fine-tuning job")
		return nil
	}

	fileID, err := openaiUploadFile(opts.OpenAIBaseURL, opts.OpenAIAPIKey, chatPath)
	if err != nil {
		return err
	}
	fmt.Println("uploaded file:", fileID)

	jobID, err := openaiCreateJob(opts.OpenAIBaseURL, opts.OpenAIAPIKey, opts.Model, fileID)
	if err != nil {
		return err
	}
	fmt.Println("fine-tuning job:", jobID)
	if isCloudOpenAI(opts.OpenAIBaseURL) {
		fmt.Println("track: https://platform.openai.com/finetune")
	}

	// persist job id
	_ = os.WriteFile(filepath.Join(opts.DataDir, "openai_job.json"), []byte(fmt.Sprintf(
		`{"job_id":%q,"model":%q,"file_id":%q,"base":%q,"created":%q,"privacy":"uploaded"}`+"\n",
		jobID, opts.Model, fileID, opts.OpenAIBaseURL, time.Now().UTC().Format(time.RFC3339),
	)), 0o644)

	if !opts.Wait {
		fmt.Printf("status: weft train status --job %s\n", jobID)
		return nil
	}
	return openaiWaitJob(opts.OpenAIBaseURL, opts.OpenAIAPIKey, jobID)
}

// FinetuneStatus polls an OpenAI fine-tune job.
func FinetuneStatus(baseURL, apiKey, jobID string) error {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY required")
	}
	st, model, err := openaiGetJob(baseURL, apiKey, jobID)
	if err != nil {
		return err
	}
	fmt.Printf("job:    %s\nstatus: %s\n", jobID, st)
	if model != "" {
		fmt.Println("model: ", model)
	}
	return nil
}

func openaiUploadFile(base, key, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("purpose", "fine-tune")
	part, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	_ = w.Close()

	req, err := http.NewRequest("POST", strings.TrimRight(base, "/")+"/files", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("upload failed %d: %s", resp.StatusCode, truncate(string(b), 400))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("no file id in response: %s", truncate(string(b), 200))
	}
	return out.ID, nil
}

func openaiCreateJob(base, key, model, fileID string) (string, error) {
	payload := map[string]any{
		"training_file": fileID,
		"model":         model,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", strings.TrimRight(base, "/")+"/fine_tuning/jobs", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("create job failed %d: %s", resp.StatusCode, truncate(string(b), 400))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func openaiGetJob(base, key, jobID string) (status, fineTunedModel string, err error) {
	req, err := http.NewRequest("GET", strings.TrimRight(base, "/")+"/fine_tuning/jobs/"+jobID, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("job status %d: %s", resp.StatusCode, truncate(string(b), 300))
	}
	var out struct {
		Status         string `json:"status"`
		FineTunedModel string `json:"fine_tuned_model"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", "", err
	}
	return out.Status, out.FineTunedModel, nil
}

func openaiWaitJob(base, key, jobID string) error {
	for {
		st, model, err := openaiGetJob(base, key, jobID)
		if err != nil {
			return err
		}
		fmt.Println("status:", st)
		switch st {
		case "succeeded":
			fmt.Println("fine-tuned model:", model)
			return nil
		case "failed", "cancelled":
			return fmt.Errorf("job %s", st)
		}
		time.Sleep(15 * time.Second)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func trainReqs() string {
	return `torch
transformers>=4.45.0
datasets>=2.20.0
trl>=0.12.0
peft>=0.13.0
accelerate>=0.34.0
`
}

// trlScriptV2 is a LoRA SFT script with privacy defaults (no remote logging).
func trlScriptV2() string {
	return `#!/usr/bin/env python3
"""Weft private LoRA fine-tune (TRL). Data is read only from local files.

Usage:
  python train_trl.py --data chat.jsonl --model Qwen/Qwen2.5-7B-Instruct --private
  python train_trl.py --data chat.jsonl --model /models/local-weights --private
"""
from __future__ import annotations

import argparse
import json
import os
from datasets import Dataset
from peft import LoraConfig
from transformers import AutoModelForCausalLM, AutoTokenizer
from trl import SFTTrainer, SFTConfig
import torch


def load_chat(path: str) -> Dataset:
    rows = []
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rows.append(json.loads(line))
    if not rows:
        raise SystemExit(f"no rows in {path}")
    return Dataset.from_list(rows)


def main() -> None:
    ap = argparse.ArgumentParser(description="Private fine-tune: teach a model to write Weft")
    ap.add_argument("--data", default="chat.jsonl")
    ap.add_argument("--model", default="Qwen/Qwen2.5-7B-Instruct")
    ap.add_argument("--out", default="weft-finetune-out")
    ap.add_argument("--epochs", type=float, default=3)
    ap.add_argument("--lr", type=float, default=2e-4)
    ap.add_argument("--batch", type=int, default=2)
    ap.add_argument("--grad-accum", type=int, default=4)
    ap.add_argument("--max-seq", type=int, default=2048)
    ap.add_argument("--no-lora", action="store_true", help="full fine-tune (needs lots of VRAM)")
    ap.add_argument("--private", action="store_true", help="disable telemetry / remote report_to")
    ap.add_argument("--offline", action="store_true", help="HF offline mode (local weights only)")
    args = ap.parse_args()

    if args.private or args.offline:
        os.environ.setdefault("WANDB_DISABLED", "true")
        os.environ.setdefault("HF_HUB_DISABLE_TELEMETRY", "1")
        os.environ.setdefault("DISABLE_MLFLOW", "true")
    if args.offline:
        os.environ["HF_HUB_OFFLINE"] = "1"
        os.environ["TRANSFORMERS_OFFLINE"] = "1"

    local_files = os.path.isdir(args.model)
    print(f"privacy: local training (upload=never) model={args.model} local_dir={local_files}")
    print(f"loading tokenizer/model: {args.model}")
    tok = AutoTokenizer.from_pretrained(
        args.model, trust_remote_code=True, local_files_only=args.offline or local_files
    )
    if tok.pad_token is None:
        tok.pad_token = tok.eos_token

    ds = load_chat(args.data)
    print(f"rows: {len(ds)} from {args.data}")

    def to_text(row):
        text = tok.apply_chat_template(
            row["messages"], tokenize=False, add_generation_prompt=False
        )
        return {"text": text}

    ds = ds.map(to_text, remove_columns=ds.column_names)

    dtype = torch.bfloat16 if torch.cuda.is_available() and torch.cuda.is_bf16_supported() else torch.float16
    if not torch.cuda.is_available() and torch.backends.mps.is_available():
        dtype = torch.float32  # MPS-friendly
    model_kwargs = {
        "trust_remote_code": True,
        "torch_dtype": dtype,
        "local_files_only": args.offline or local_files,
    }
    if torch.cuda.is_available():
        model_kwargs["device_map"] = "auto"

    model = AutoModelForCausalLM.from_pretrained(args.model, **model_kwargs)
    peft_cfg = None
    if not args.no_lora:
        peft_cfg = LoraConfig(
            r=16,
            lora_alpha=32,
            lora_dropout=0.05,
            bias="none",
            task_type="CAUSAL_LM",
            target_modules=["q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"],
        )
        print("using LoRA adapters")

    sft = SFTConfig(
        output_dir=args.out,
        num_train_epochs=args.epochs,
        per_device_train_batch_size=args.batch,
        gradient_accumulation_steps=args.grad_accum,
        learning_rate=args.lr,
        logging_steps=5,
        save_strategy="epoch",
        max_length=args.max_seq,
        dataset_text_field="text",
        bf16=dtype == torch.bfloat16,
        fp16=dtype == torch.float16,
        report_to=[],  # never push metrics to cloud
        packing=False,
    )

    trainer = SFTTrainer(
        model=model,
        args=sft,
        train_dataset=ds,
        processing_class=tok,
        peft_config=peft_cfg,
    )
    trainer.train()
    trainer.save_model(args.out)
    tok.save_pretrained(args.out)
    print(f"done → {args.out}/  (private: nothing uploaded)")
    print("Serve adapters on your infra; set WEFT_API_BASE to your private endpoint.")


if __name__ == "__main__":
    main()
`
}
