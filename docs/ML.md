# ML in Weft — module, not core

**Balance:** close the *scripting* ML gap (embeddings, RAG, metrics, private train glue) without turning Weft into a full numeric / deep-learning stack.

| Layer | Where | Role |
|-------|--------|------|
| **Core binary** | `llm` · `ollama` · `vllm` · `weft train` | chat, local providers, fine-tune orchestration |
| **`packages/ml`** | installable Weft module | light vectors / embeddings / metrics for RAG |
| **External** | GPU train (TRL), heavy science | optional training toolchains, notebooks, remote services |

## Why a separate module?

1. **Core stays small** — agent/HTTP/ops scripts do not pay for ML surface area.  
2. **Same extension path as everyone else** — `weft get` / `vendor/` / lock / capabilities.  
3. **Evolve independently** — version `packages/ml` without shipping a new `weft` binary.  
4. **Honest scope** — pure Weft is fine for RAG-scale vectors; it is *not* a BLAS.

## Install

```bash
weft get ml ./packages/ml          # monorepo
# weft get ml github.com/you/weft-ml@v0.1.0
weft install
```

```weft
use ml

fn main -> Result {
    hits := ml.topk(q, docs, 5)
    // v := ml.embed("query")?   // needs Ollama embed model or API key
}
```

Demo (offline):

```bash
cd examples/ml_demo && weft install && weft run main.weft
```

## What ships in `packages/ml`

- **Vectors:** `cosine` `dot` `norm` `topk`  
- **Embeddings:** `embed` / `embed_with` / `embed_many` (HTTP → Ollama or OpenAI-compat)  
- **Index:** in-memory + `index_save` / `index_load` JSON  
- **Metrics:** `accuracy` `mse` `split`  

Training **weights** remains:

```bash
weft train finetune --private --preset qwen-7b
```

See [`docs/FINETUNE.md`](FINETUNE.md) · [`docs/LLM_LOCAL.md`](LLM_LOCAL.md).

## What we will not put in core

- Dense linear algebra / GPU tensors  
- DataFrame engine  
- Full AutoML / classic ML library zoo  
- Native `.so` plugins for deep-learning runtimes  

Those stay **external services** or remote APIs. Weft orchestrates.

## ONNX / native models → sidecar (not core)

Weft never loads `.onnx` in-process. Pattern:

1. Run **ONNX Runtime / a model-serving stack / Triton** (or any HTTP model server) as a sibling process.  
2. Call it with `http.post` + retries / circuit breaker.  
3. Keep GPU drivers and weights outside the `weft` binary.

Offline demo (mock JSON contract, no ONNX install):

```bash
weft run examples/onnx_sidecar/mock_server.weft   # terminal 1
weft run examples/onnx_sidecar/main.weft          # terminal 2
```

Details: [`examples/onnx_sidecar/README.md`](../examples/onnx_sidecar/README.md).

## Roadmap (ML slice)

| Horizon | Deliverable |
|---------|-------------|
| **Now** | `packages/ml` + offline demo + this doc |
| **Now** | ONNX *sidecar* example + HTTP contract (separate process) |
| **Now** | Embed hardened: OpenAI-compat shapes, `/v1` URL normalize, batch `embed_many`, Azure `api-key` |
| **Next** | optional HNSW-ish index in pure Weft or Go *only if* justified |
| **Never (core)** | In-process deep learning training · CGo ONNX inside `weft` |

Broader product roadmap (ecosystem, IDE, registry): [`docs/ROADMAP.md`](ROADMAP.md).
