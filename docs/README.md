# Weft documentation

Weft is a small scripting language for **agents, HTTP glue, and ops tools**.  
One Go binary; optional modules vendored into `vendor/`.

## How the pieces fit

**Start with the map:** **[ECOSYSTEM.md](ECOSYSTEM.md)** — language · stdlib · optional modules (`mold` / `ml` / `tokensave`) · agents · web · trust.

```text
Your app
   ↑
Optional modules   mold · ml · tokensave     (packages/ → vendor/)
   ↑
Stdlib             llm · web · http · fs …   (in the weft binary)
   ↑
Language + VM      Result/? · concurrency
```

| I want to… | Go here |
|------------|---------|
| Learn the language in an hour | [TUTORIAL.md](TUTORIAL.md) |
| Paste a recipe | [COOKBOOK.md](COOKBOOK.md) · [`examples/cookbook/`](../examples/cookbook/) |
| Build an agent | [ECOSYSTEM.md](ECOSYSTEM.md#agent-path-cohesive-recipe) → [LLM_PROVIDERS.md](LLM_PROVIDERS.md) → [MOLD.md](MOLD.md) |
| Structure LLM JSON / tool params | [MOLD.md](MOLD.md) · `weft packages get mold` |
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

## Agents & optional modules

All three modules are **optional** — same install path, never built-ins. Catalog: [`packages/`](../packages/).

| Module | Role | Doc |
|--------|------|-----|
| **mold** | Validate / coerce structured JSON; JSON Schema & tool params | [MOLD.md](MOLD.md) |
| **ml** | Embeddings, vectors, RAG index, metrics | [ML.md](ML.md) |
| **tokensave** | Context thrift, memory, teach → train | [`packages/tokensave`](../packages/tokensave/) |

| Stdlib (binary) | Role | Doc |
|-----------------|------|-----|
| `llm` · `ollama` · `vllm` | Chat, tools, stream, local hosts | [LLM_PROVIDERS.md](LLM_PROVIDERS.md) · [LLM_LOCAL.md](LLM_LOCAL.md) |
| train CLI | Private fine-tune orchestration | [FINETUNE.md](FINETUNE.md) |

```bash
weft packages get mold && weft install
weft run examples/cookbook/14_mold.weft
```

---

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

## Design history

| Doc | Audience |
|------|----------|
| [weft-design.md](weft-design.md) | Full design notes (phases, acceptance) |
| [BRAND.md](BRAND.md) | Brand / mascot |

## What Weft is not

Not a general-purpose OS scripting replacement for every stack. Not a heavy scientific array runtime. Not in-process deep-learning training. Not notebook-first. See [ROADMAP.md](ROADMAP.md) and [ECOSYSTEM.md](ECOSYSTEM.md).
