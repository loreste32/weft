# Weft documentation

Weft is a small scripting language for **agents, HTTP glue, and ops tools**. One Go binary, no Python on the critical path, packages vendored like `go mod`.

This folder is the human-facing docs set. Start with the language guide and the cookbook; dive into topic pages when you need detail.

## Start here

| Doc | What it is |
|-----|------------|
| **[TUTORIAL.md](TUTORIAL.md)** | Guided first hour — install → hello → errors → concurrency → tests |
| **[LANGUAGE.md](LANGUAGE.md)** | End-to-end language reference: values, control flow, functions, closures, enums, match, errors, modules, concurrency, types |
| **[COOKBOOK.md](COOKBOOK.md)** | Recipes you can paste and adapt — files, HTTP, JSON, CLI, agents, concurrency, packages, tests |
| **[SYNTAX.md](SYNTAX.md)** | Short cheatsheet and style preferences |
| **[STDLIB.md](STDLIB.md)** | Stdlib map (`weft stdlib` for the live list) |
| **[ROADMAP.md](ROADMAP.md)** | Where we are (0.3.x) and what we will / won’t do |

Runnable recipe files: **[examples/cookbook/](../examples/cookbook/)** (offline-friendly).

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
| Web / HTTP servers | [web.md](web.md) |
| CLI tools | [cli.md](cli.md) |
| Data / SQL / CSV | [data.md](data.md) |
| Charts | [viz.md](viz.md) |
| LLM providers | [LLM_PROVIDERS.md](LLM_PROVIDERS.md) |
| Local Ollama / vLLM | [LLM_LOCAL.md](LLM_LOCAL.md) |
| Fine-tune (private by default) | [FINETUNE.md](FINETUNE.md) |
| ML module (optional) | [ML.md](ML.md) |
| Production notes | [PRODUCTION.md](PRODUCTION.md) |
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
| **`examples/cookbook/`** | Tutorial + cookbook recipes (offline) |
| `examples/hello.weft` | Minimal |
| `examples/weft_style.weft` | Language feel |
| `examples/errors_demo.weft` | `Result` / `?` |
| `examples/json_http.weft` | JSON + env |
| `examples/channels.weft` · `parallel.weft` | Concurrency |
| `examples/cli_tool.weft` | Flags / subcommands |
| `examples/server.weft` · `webapp.weft` | HTTP servers |
| `examples/realworld/` | Agents, pipelines |
| `examples/pkg_demo/` · `modules/` | Packages |

## Mental model

```text
source .weft  →  lex  →  parse  →  compile  →  stack VM
                      ↘ optional gradual types (weft check)
```

- **Errors:** `Result` + `?` — no try/catch as the primary style  
- **Concurrency:** ordinary `fn` + `map` / `parallel` / `spawn` — no `async`/`await`  
- **Packages:** `vendor/` + lockfile, not a global site-packages  
- **Closures:** capture outer locals **by value** (deep-copied at creation)  

## What Weft is not

Not a CPython replacement. Not NumPy/SciPy. Not in-process PyTorch training. Not Jupyter-first. See [ROADMAP.md](ROADMAP.md) for the honest list.
