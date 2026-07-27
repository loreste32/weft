# ml — Weft ML module (not core)

**Balanced ML for agent/RAG scripts** — pure `.weft`, installable, not baked into the `weft` binary.

| In scope | Out of scope (stay external) |
|----------|------------------------------|
| Embeddings via OpenAI-compat / Ollama | Heavy tensor / array frameworks |
| Cosine search, JSON vector index | Full training loops / GPU kernels |
| Accuracy / MSE / train-test split | Full dataframe / ML toolkits |
| Glue to `llm` + `weft train` | In-process deep-learning training |

## Install

```bash
# from this monorepo
weft get ml ./packages/ml
weft install

# or path relative to your app
weft get ml ../packages/ml
```

```weft
use ml

fn main -> Result {
    // offline vectors (no API)
    q := [1.0, 0.0, 0.0]
    docs := [
        {"id": "a", "vec": [1.0, 0.0, 0.0], "meta": {"t": "alpha"}},
        {"id": "b", "vec": [0.0, 1.0, 0.0], "meta": {"t": "beta"}},
    ]
    say(ml.topk(q, docs, 1))

    // live embeddings (optional — needs Ollama embed model or OpenAI key)
    // v := ml.embed("hello weft")?
    // say(len(v))
}
```

## API

| Call | Role |
|------|------|
| `ml.cosine(a,b)` `ml.dot` `ml.norm` | vector math |
| `ml.topk(q, items, k)` | similarity ranking |
| `ml.embed(text)?` | embedding (`WEFT_PROVIDER` / Ollama / OpenAI-compat) |
| `ml.embed_with(text, {model, base_url, api_key, headers})?` | override host |
| `ml.embed_many([…])?` | batch (one OpenAI-compat call when possible) |
| `ml.index()` · `index_add` · `index_search` · `index_save` · `index_load` | tiny RAG store |
| `ml.accuracy` `ml.mse` `ml.split` | eval helpers |
| `ml.provider()` | detected provider string |

Env (embeddings):

| Variable | Role |
|----------|------|
| `WEFT_PROVIDER` | `ollama` / `vllm` / `openai` (any OpenAI-compat host) |
| `WEFT_EMBED_MODEL` | e.g. `nomic-embed-text`, `text-embedding-3-small` |
| `OLLAMA_HOST` | default `http://127.0.0.1:11434` |
| `OPENAI_BASE_URL` / `WEFT_API_BASE` | host or `…/v1` — `/embeddings` path is normalized |
| `OPENAI_API_KEY` / `WEFT_API_KEY` | Bearer + Azure-style `api-key` header |
| `OPENAI_API_KEY` / `WEFT_API_KEY` | cloud embeddings |

## Training

Weights still use **`weft train finetune --private`** (optional TRL on your GPU). This module does not reimplement training — it closes the **inference/RAG/eval scripting** gap.

## Design

Core language stays small. Heavy or optional domains ship as **modules** under `packages/`. See [docs/ML.md](../../docs/ML.md) and [docs/ROADMAP.md](../../docs/ROADMAP.md).
