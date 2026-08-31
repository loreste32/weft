# Weft documentation

Weft is a small scripting language for **agents, HTTP glue, and ops tools**.  
One Go binary; optional modules vendored into `vendor/`.

## How the pieces fit

**Start with the map:** **[ECOSYSTEM.md](ECOSYSTEM.md)** — language · stdlib · modules · agents · telecom · web · trust.

```text
Your app
   ↑
24 registry modules    telecom · auth · router · config · …   (packages/ → vendor/)
   ↑
83 stdlib packages     llm · mcp · dns · tls · os · http · …  (in the weft binary)
   ↑
Language + VM          Result/? · concurrency · auto-fetch
```

| I want to… | Go here |
|------------|---------|
| Learn the language in an hour | [TUTORIAL.md](TUTORIAL.md) |
| Paste a recipe | [COOKBOOK.md](COOKBOOK.md) · [`examples/cookbook/`](../examples/cookbook/) |
| Build an agent | [ECOSYSTEM.md](ECOSYSTEM.md#agent-path-cohesive-recipe) → [LLM_PROVIDERS.md](LLM_PROVIDERS.md) → [MOLD.md](MOLD.md) |
| Structure LLM JSON / tool params | [MOLD.md](MOLD.md) · `weft get mold` |
| Embeddings / RAG | [ML.md](ML.md) |
| HTTP + HTMX UI | [web.md](web.md) |
| Install / publish modules | [packages.md](packages.md) · [modules.md](modules.md) |
| Run safely in production | [SECURITY.md](../SECURITY.md) · [PRODUCTION.md](PRODUCTION.md) |

---

## Start here (language)

| Doc | What it is |
|-----|------------|
| **[TUTORIAL.md](TUTORIAL.md)** | Guided first hour |
| **[LANGUAGE.md](LANGUAGE.md)** | Language reference |
| **[COOKBOOK.md](COOKBOOK.md)** | Paste-ready recipes |
| **[SYNTAX.md](SYNTAX.md)** | Cheatsheet |
| **[STDLIB.md](STDLIB.md)** | Stdlib map (`weft stdlib`) |
| **[STDLIB_GAPS.md](STDLIB_GAPS.md)** | Tiers A/B complete; C non-goals |
| **[ROADMAP.md](ROADMAP.md)** | Now / next / never |
| **[PRINCIPLES.md](PRINCIPLES.md)** | Product rules |

Runnable offline recipes: **[`examples/cookbook/`](../examples/cookbook/)** (`01`…`14_mold`).

---

## Agents, telecom & optional modules

24 modules in the [public registry](https://registry.weftproject.dev). Install with `weft get <name>`.

| Module | Role | Doc |
|--------|------|-----|
| **telecom** | IVA voice agents, FreeSWITCH ESL, Asterisk ARI, STT/TTS, DTMF, routing, queues, CDR | [TELECOM.md](TELECOM.md) |
| **mold** | Validate / coerce structured JSON; JSON Schema & tool params | [MOLD.md](MOLD.md) |
| **ml** | Embeddings, vectors, RAG index, metrics | [ML.md](ML.md) |
| **tokensave** | Context thrift, memory, teach → train | [`packages/tokensave`](../packages/tokensave/) |
| **auth** | HMAC, password hashing, tokens, OAuth helpers | [registry](https://registry.weftproject.dev) |
| **config** | Unified config loader (.env/JSON/YAML/TOML) | [registry](https://registry.weftproject.dev) |
| **logger** | Structured logging: levels, JSON/text, child loggers | [registry](https://registry.weftproject.dev) |
| **router** | HTTP routing, path params, middleware, CORS | [registry](https://registry.weftproject.dev) |
| **queue** | Job queue with retries and dead-letter | [registry](https://registry.weftproject.dev) |
| **retry** · **semver** · **cache** · **color** · **jwt** · **warp** | Ops utilities | [registry](https://registry.weftproject.dev) |
| **http_router** · **template** · **validate** · **cron** | Web & app utilities | [registry](https://registry.weftproject.dev) |
| **infra** | Infrastructure automation: health checks, service management, deploy patterns, alerting | [registry](https://registry.weftproject.dev) |

| Stdlib (binary) | Role | Doc |
|-----------------|------|-----|
| `llm` · `ollama` · `vllm` | Chat, tools, stream, local hosts | [LLM_PROVIDERS.md](LLM_PROVIDERS.md) · [LLM_LOCAL.md](LLM_LOCAL.md) |
| `mcp` | MCP client + server for AI assistants | [MCP.md](MCP.md) |
| `deepgram` · `elevenlabs` | Streaming STT/TTS (WebSocket) | [DEEPGRAM.md](DEEPGRAM.md) · [ELEVENLABS.md](ELEVENLABS.md) |
| `mlinfer` | ONNX Runtime / Triton / HuggingFace inference | [MLINFER.md](MLINFER.md) |
| train CLI | Private fine-tune orchestration | [FINETUNE.md](FINETUNE.md) |

```bash
weft get telecom
weft get mold
```

---

## Numerical, data, and ML workflows

| Topic | Doc | Scope |
|-------|-----|-------|
| NumPy-style arrays | [WARP.md](WARP.md) · [ML.md](ML.md) · [`packages/warp/`](../packages/warp/) | Broadcasting, reductions, linear algebra, FFTs, dtypes, and explicit accelerator dispatch |
| DataFrames | [DATAFRAME.md](DATAFRAME.md) · [`packages/dataframe/`](../packages/dataframe/) | Null-aware tabular operations, joins, windows, CSV/JSON/SQL, and documented pandas boundaries |
| Classical ML and autodiff | [ML.md](ML.md) · [`packages/ml/`](../packages/ml/) | Linear/logistic models, minibatches, optimizers, forward/reverse autodiff, and selected higher-order tests |
| Native CUDA, ROCm, and MLX | [ACCELERATORS.md](ACCELERATORS.md) · [native ABI](../native/accelerator/README.md) | Capability-gated plugins with explicit device/fallback reporting; hardware claims require provider conformance |
| Compatibility boundaries | [COMPATIBILITY.md](COMPATIBILITY.md) | What is implemented, partial, unsupported, or still release-gated |

These modules cover supported Weft workflows; they are not a claim of drop-in compatibility with every NumPy, pandas, or deep-learning API. Use the compatibility and accelerator documents when making production or hardware-specific claims.

## Web, data, ops

| Topic | Doc |
|-------|-----|
| HTTP servers, HTMX, cookies, `before` | [web.md](web.md) |
| CLI tools | [cli.md](cli.md) |
| Data / SQL / CSV | [data.md](data.md) |
| Packet captures (pcap) | [STDLIB.md](STDLIB.md#pcap) |
| Charts | [viz.md](viz.md) |
| Sysops / runbooks | [SYSOPS.md](SYSOPS.md) |
| Production checklist | [PRODUCTION.md](PRODUCTION.md) |

---

## Packages, tooling, safety

| Topic | Doc |
|-------|-----|
| Install packages (consumer) | [packages.md](packages.md) |
| Author modules | [modules.md](modules.md) |
| Monorepo catalog | [`packages/README.md`](../packages/README.md) |
| Threat model / capabilities | **[SECURITY.md](../SECURITY.md)** · [security/](security/) |
| Tooling (`check`, `fmt`, `bench`, LSP) | [TOOLING.md](TOOLING.md) |
| Testing | [TESTING.md](TESTING.md) |
| Errors (`Result`, `?`) | [ERRORS.md](ERRORS.md) |
| Concurrency | [CONCURRENCY.md](CONCURRENCY.md) |
| Versioning | [VERSIONING.md](VERSIONING.md) |

---

## Install and first run

```bash
go build -o weft ./cmd/weft
# or: make install

./weft doctor
./weft run examples/hello.weft
./weft run examples/cookbook/01_hello.weft
./weft check examples/fib.weft --types
```

```weft
fn main {
    say("hello, weft")
}
```

---

## Examples in the repo

| Path | Theme |
|------|--------|
| **`examples/cookbook/`** | Language + agent recipes (offline) |
| `examples/hello.weft` | Minimal |
| `examples/htmx.weft` · `webapp.weft` | HTTP / HTMX |
| **`examples/agent_stack/`** | mold + tokensave + ml (offline) |
| `examples/mold_ai.weft` · `cookbook/14_mold.weft` | mold alone |
| `examples/ml_demo/` · `tokensave_demo/` | single-module demos |
| `examples/cli_tool.weft` · `sysops_host.weft` | CLI / ops |
| `examples/realworld/` | Agents, pipelines |
| `packages/{mold,ml,tokensave}` | Optional modules |

---

## Additional topics

| Topic | Doc |
|-------|-----|
| Building example applications | [BUILDING.md](BUILDING.md) |
| Cluster, governor & supervisor | [CLUSTER.md](CLUSTER.md) |
| ETL pipelines (`map`/`filter`/`reduce`, jsonl) | [PIPELINES.md](PIPELINES.md) |
| Contributing | [CONTRIBUTING_GUIDE.md](CONTRIBUTING_GUIDE.md) |

---

## Design history

| Doc | Audience |
|------|----------|
| [weft-design.md](weft-design.md) | Full design notes (phases, acceptance) |
| [DESIGN_0_4.md](DESIGN_0_4.md) | 0.4.x–0.6.x design: type system, Wasm, DAP |
| [BRAND.md](BRAND.md) | Brand / mascot |

## What Weft is not

Not a general-purpose OS scripting replacement for every stack. Not a heavy scientific array runtime. Not in-process deep-learning training. Not notebook-first. See [ROADMAP.md](ROADMAP.md) and [ECOSYSTEM.md](ECOSYSTEM.md).
