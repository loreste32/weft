# Fine-tune models to write Weft

The language, runtime, packages, agents, and default fine-tune path are pure Go / HTTP.

## Recommended: API fine-tune

```bash
# 1) Build dataset
weft train prepare -o weft-sft --expand

# 2) Fine-tune via OpenAI-compatible API
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

## Optional: local GPU LoRA

Only if you want open-weight adapters on *your* machine:

```bash
weft train finetune --backend trl --install-deps
# or: pip install -r weft-sft/requirements-train.txt yourself first
```

This shells out to a small training script because open-weight LLM training today is dominated by PyTorch. Running Weft scripts never uses that stack.

## No fine-tune at all

```bash
weft prompt --few 8    # system card for ChatGPT / Cursor / etc.
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

## Grow data

Edit `llm-pack/train.jsonl` → copy to `internal/llmpack/train.jsonl` →  
`weft train validate && weft train prepare -o weft-sft --expand`
