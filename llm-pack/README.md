# Fine-tune models to write Weft

**Weft does not need Python.** The language, runtime, packages, agents, and default fine-tune path are pure Go / HTTP.

Python appears **only** if you opt into local open-weight LoRA via Hugging Face (`--backend trl`). That is the ML ecosystem’s baggage, not Weft’s.

## Recommended: zero-Python fine-tune (API)

```bash
# 1) Build dataset (Go only)
weft train prepare -o weft-sft --expand

# 2) Fine-tune via OpenAI-compatible API (Go HTTP — no pip)
export OPENAI_API_KEY=sk-...
weft train finetune --backend openai --wait

# 3) Check job
weft train status --job ftjob-...
```

Works with any **OpenAI-compatible** fine-tune endpoint:

```bash
export OPENAI_API_KEY=...
export OPENAI_BASE_URL=https://api.your-provider.com/v1
weft train finetune --backend openai --model <their-base-model> --wait
```

## Optional: local GPU LoRA (Python/PyTorch)

Only if you want open-weight adapters on *your* machine:

```bash
weft train finetune --backend trl --install-deps
# or: pip install -r weft-sft/requirements-train.txt yourself first
```

This shells out to a small `train_trl.py` because **training** open LLMs today is dominated by PyTorch. Running Weft scripts never uses that stack.

## No fine-tune at all

```bash
weft prompt --few 8    # system card for ChatGPT / Claude / Cursor
```

## Dataset only

```bash
weft train prepare -o weft-sft --expand
weft train validate
weft train stats --expand
```

Outputs: `chat.jsonl`, `alpaca.jsonl`, `sharegpt.jsonl`, `SYSTEM.md`, etc.

## After training

```bash
weft check generated.weft
weft run generated.weft
weft eval examples/realworld/
```

## Grow data (still no Python)

Edit `llm-pack/train.jsonl` → copy to `internal/llmpack/train.jsonl` →  
`weft train validate && weft train prepare -o weft-sft --expand`
