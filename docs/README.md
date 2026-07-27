# Weft documentation

Weft is a small scripting language for **agents, HTTP glue, and ops tools**. One Go binary; packages vendored into `vendor/`.

This folder is the human-facing docs set. Start with the language guide and the cookbook; dive into topic pages when you need detail.

## Start here

| Doc | What it is |
|-----|------------|
| **[TUTORIAL.md](TUTORIAL.md)** | Guided first hour — install → hello → errors → concurrency → tests |
| **[LANGUAGE.md](LANGUAGE.md)** | End-to-end language reference: values, control flow, functions, closures, enums, match, errors, modules, concurrency, types |
| **[COOKBOOK.md](COOKBOOK.md)** | Recipes you can paste and adapt — files, HTTP, JSON, CLI, agents, concurrency, packages, tests |
| **[SYNTAX.md](SYNTAX.md)** | Short cheatsheet and style preferences |
| **[STDLIB.md](STDLIB.md)** | Stdlib map (`weft stdlib` for the live list) |
| **[STDLIB_GAPS.md](STDLIB_GAPS.md)** | Coverage tiers A/B and permanent non-goals (C) |
| **[ROADMAP.md](ROADMAP.md)** | Where we are (0.3.x) and what we will / won’t do |
| **[../SECURITY.md](../SECURITY.md)** | Threat model, capabilities, operator checklist |

Runnable recipe files: **[examples/cookbook/](../examples/cookbook/)** (`01`…`14_mold`, offline-friendly).

## Topic guides

| Topic | Doc |
|-------|-----|
| Errors (`Result`, `?`) | [ERRORS.md](ERRORS.md) |
| Concurrency (no `async`/`await`) | [CONCURRENCY.md](CONCURRENCY.md) |
| Testing | [TESTING.md](TESTING.md) |
| Tooling (`check`, `fmt`, `bench`, LSP) | [TOOLING.md](TOOLING.md) |
| Packages (consumer) | [packages.md](packages.md) |
| Modules (author) | [modules.md](modules.md) |
| Pipelines / map-filter | [PIPELINES.md](PIPELINES.md) |
| Web / HTTP servers / HTMX | [web.md](web.md) |
| CLI tools | [cli.md](cli.md) |
| **Sysops / runbooks** | **[SYSOPS.md](SYSOPS.md)** |
| Data / SQL / CSV | [data.md](data.md) |
| Charts | [viz.md](viz.md) |
| LLM providers | [LLM_PROVIDERS.md](LLM_PROVIDERS.md) |
| Local Ollama / vLLM | [LLM_LOCAL.md](LLM_LOCAL.md) |
| Fine-tune (private by default) | [FINETUNE.md](FINETUNE.md) |
| ML module (optional) | [ML.md](ML.md) |
| **mold** (structured models, optional) | **[MOLD.md](MOLD.md)** |
| Production notes | [PRODUCTION.md](PRODUCTION.md) |
| Security audits / notes | [security/](security/) |
| Versioning (0.3.x) | [VERSIONING.md](VERSIONING.md) |
| Product principles | [PRINCIPLES.md](PRINCIPLES.md) |
| Brand / mascot | [BRAND.md](BRAND.md) |

## Design & architecture

| Doc | Audience |
|-----|----------|
| [weft-design.md](weft-design.md) | Full design notes (history, phases, acceptance) |

## Install and first run

```bash
go build -o weft ./cmd/weft
# or: make install

./weft doctor
./weft run examples/hello.weft
./weft run examples/cookbook/01_hello.weft
./weft check examples/fib.weft --types
```

Follow **[TUTORIAL.md](TUTORIAL.md)** for a structured first hour.

```weft
fn main {
    say("hello, weft")
}
```

## Examples in the repo

| Path | Theme |
|------|--------|
| **`examples/cookbook/`** | Tutorial + cookbook recipes (offline; includes `14_mold`) |
| `examples/hello.weft` | Minimal |
| `examples/weft_style.weft` | Language feel |
| `examples/errors_demo.weft` | `Result` / `?` |
| `examples/json_http.weft` | JSON + env |
| `examples/channels.weft` · `parallel.weft` | Concurrency |
| `examples/cli_tool.weft` | Flags / subcommands |
| `examples/server.weft` · `webapp.weft` · `htmx.weft` | HTTP / HTMX |
| `examples/mold_ai.weft` | mold module end-to-end |
| `examples/realworld/` | Agents, pipelines |
| `examples/pkg_demo/` · `modules/` | Packages |
| `packages/{ml,mold,tokensave}` | Optional modules (catalog) |

## Mental model

```text
source .weft  →  lex  →  parse  →  compile  →  stack VM
                      ↘ optional gradual types (weft check)
```

- **Errors:** `Result` + `?` — no try/catch as the primary style  
- **Concurrency:** ordinary `fn` + `map` / `parallel` / `spawn` — no `async`/`await`  
- **Packages:** `vendor/` + lockfile, not a global site-packages  
- **Optional modules:** `ml` · `mold` · `tokensave` under `packages/` (not in the binary)  
- **Closures:** capture outer locals **by value** (deep-copied at creation)  

## What Weft is not

Not a general-purpose OS scripting replacement for every stack. Not a heavy scientific array runtime. Not in-process deep-learning training. Not notebook-first. See [ROADMAP.md](ROADMAP.md) for the honest list.
