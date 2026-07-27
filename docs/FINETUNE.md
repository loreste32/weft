# Fine-tuning Weft writers (private by default)

## The point

Teach a model to emit **Weft**, not another language. Training data can be **your private domain corpus** — it does not need to leave your machines.

| Path | Data leaves machine? | Models |
|------|----------------------|--------|
| `weft train finetune --private` | **No** | Open frontier (Qwen, Llama, …) + LoRA on your GPU |
| `weft train offline` | **No** | Same, portable air-gapped kit |
| VPC OpenAI-compatible FT (`--private-endpoint`) | Only to **your** API | Whatever that host fine-tunes |
| `weft train finetune --backend openai --allow-upload` | **Yes** (public OpenAI) | OpenAI fine-tune models |

Closed vendor “frontier” APIs that only train in their cloud **cannot** keep raw SFT rows fully private unless you have a contractual private deployment. Weft’s private path uses **open frontier weights you control**.

## Private end-to-end (recommended)

```bash
# 0) Optional: collect gold while using the tokensave brain (external module)
#    use tokensave · brain(..., {memory_path}) · teach(mem, gold.jsonl, ask, answer, kind)
#    see packages/tokensave — clarifies messy asks + stores relevant context for train

# 1) Build dataset (optional: merge confidential domain rows / tokensave gold)
weft train prepare -o weft-sft --expand \
  --from /secure/my-domain.jsonl
# --from examples/tokensave_demo/domain-gold.jsonl

# 1b) Score gold — parse/compile accuracy (offline; add --run to execute)
weft train eval
weft train eval --run --limit 20
weft train eval --from /secure/my-domain.jsonl
# after serving a model: --live (calls WEFT_API_BASE / OpenAI-compat)

# 2) Local LoRA — chat.jsonl is never uploaded
weft train finetune --private --preset qwen-7b --install-deps

# 3) Serve adapters on your infra (vLLM / TGI / …), then:
export WEFT_API_BASE=http://127.0.0.1:8000/v1
export WEFT_API_KEY=local
export WEFT_MODEL=weft-writer
weft train chat "Write a Weft script that prints hello, weft"
weft train eval --live --limit 10   # model accuracy vs gold prompts
weft gen "agent with a weather tool" -o weather.weft --run
```

### Brain → gold → fine-tune (local models)

Use [`packages/tokensave`](../packages/tokensave) so training data is **clarified + relevant**, not chat noise:

```weft
use tokensave
// after a good answer:
tokensave.teach(".weft/memory.json", "domain-gold.jsonl", user_ask, good_weft, "weft")?
```

```bash
weft train prepare -o weft-sft --expand --from domain-gold.jsonl
weft train finetune --private --preset qwen-1.5b
weft train eval --from domain-gold.jsonl
```

### Gold vs script smoke

| Command | What it measures |
|---------|------------------|
| `weft train eval` | **Gold accuracy** — each training `output` parses/compiles (optional `--run`) |
| `weft train eval --live` | **Model accuracy** — generate from each `instruction`, score Weft |
| `weft eval [dir]` | **Script smoke** — run example `.weft` files (CI) |

### Presets (open frontier)

```bash
weft train presets
```

| Preset | Base | VRAM (LoRA, approx.) |
|--------|------|----------------------|
| `qwen-1.5b` | Qwen2.5-1.5B-Instruct | ~4 GB |
| `qwen-7b` | Qwen2.5-7B-Instruct | ~16 GB |
| `qwen-32b` | Qwen2.5-32B-Instruct | ~48+ GB |
| `llama-8b` | Llama-3.1-8B-Instruct | ~16 GB |
| `llama-70b` | Llama-3.1-70B-Instruct | multi-GPU |
| `gemma-9b` | Gemma-2-9B-IT | ~20 GB |

Local directory of weights works too (air-gap friendly):

```bash
weft train finetune --private --model /models/Qwen2.5-7B-Instruct
```

### Air-gapped GPU box

```bash
weft train offline -o weft-airgap --expand --from /secure/domain.jsonl
# copy weft-airgap/ via USB / internal store — no public upload step

# on the box:
export HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 WANDB_DISABLED=true
weft train finetune --private --skip-prepare --data weft-airgap \
  --model /models/Qwen2.5-7B-Instruct --out weft-lora
```

See `weft-airgap/PRIVACY.md`.

### Force private always

```bash
export WEFT_TRAIN_PRIVATE=1
# any accidental --backend openai without --allow-upload is rejected
```

## Private domain data format

`--from path.jsonl` accepts either:

```json
{"instruction":"print hello","input":"","output":"fn main { say(\"hello, weft\") }","tags":["domain"]}
```

or chat rows:

```json
{"messages":[{"role":"user","content":"print hello"},{"role":"assistant","content":"fn main { say(\"hello, weft\") }"}]}
```

Files are read from disk only. Prepare never phones home.

## Cloud OpenAI (explicit upload)

Public OpenAI fine-tune **uploads** `chat.jsonl`. Weft requires intent:

```bash
export OPENAI_API_KEY=sk-...
weft train finetune --backend openai --allow-upload --wait
```

Without `--allow-upload`, public OpenAI is refused.

## Self-hosted / VPC fine-tune API

If you run an OpenAI-compatible fine-tune service inside your network:

```bash
export WEFT_API_BASE=https://ft.internal.corp/v1
export WEFT_API_KEY=...
weft train finetune --backend openai --private-endpoint --wait
```

`--private-endpoint` skips the public-OpenAI upload gate (you own the host).

## Generate Weft from English

```bash
export OPENAI_API_KEY=sk-...   # or WEFT_API_BASE to a private server
weft gen "Read config.json and print the city field" -o job.weft
weft check job.weft
weft run job.weft
```

## Env vars

| Variable | Role |
|----------|------|
| `WEFT_TRAIN_PRIVATE` | `1` forces private training mode |
| `OPENAI_API_KEY` / `WEFT_API_KEY` | Auth for gen/chat/cloud FT |
| `OPENAI_BASE_URL` / `WEFT_API_BASE` | Compatible hosts (incl. private) |
| `WEFT_MODEL` / `OPENAI_MODEL` | Chat/gen model (incl. `ft:…`) |
| `HF_HUB_OFFLINE` | Air-gapped local weights only |

## Privacy checklist

- [ ] Prefer `--private` or `weft train offline` for confidential corpora  
- [ ] Merge secrets only via `--from` on machines you trust  
- [ ] Never pass `--allow-upload` for customer or regulated data  
- [ ] Disable W&B / remote logging on GPU boxes (`WANDB_DISABLED=true`)  
- [ ] Serve fine-tunes behind your own `WEFT_API_BASE`  
