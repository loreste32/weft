# How Weft fits together

One picture for language, stdlib, optional modules, agents, web, and trust.
Detail pages stay short and point back here when you get lost.

## Layers

```text
┌─────────────────────────────────────────────────────────────┐
│  Your app  (.weft + weft.json + vendor/)                     │
├─────────────────────────────────────────────────────────────┤
│  Optional modules  (packages/* — NOT in the binary)          │
│    mold · ml · tokensave                                     │
├─────────────────────────────────────────────────────────────┤
│  Stdlib  (IN the binary — weft stdlib)                       │
│    llm · web · http · fs · json · secrets · …                │
├─────────────────────────────────────────────────────────────┤
│  Language + VM  (weft binary)                                │
│    Result/? · concurrency · modules system                   │
└─────────────────────────────────────────────────────────────┘
```

| Layer | Lives where | You get it by |
|-------|-------------|----------------|
| Language / VM | `weft` binary | build / install `weft` |
| Stdlib | same binary | `use http` / `use llm` / … |
| Optional modules | `packages/` → `vendor/` | `weft get` / `weft packages get` |
| Your code | project dir | `weft run` |

**Rule:** if it is domain-specific (embeddings, structured tool args, context thrift), it is a **module**. If most agent/ops scripts need it (HTTP, LLM chat, files), it is **stdlib**.

---

## Agent path (cohesive recipe)

Typical agent work uses **core `llm`** plus optional modules as needed:

```text
  user ask
     │
     ▼
 tokensave          (optional) clarify + pick relevant context
     │
     ▼
 llm.ask / chat     (stdlib) model call, tools, stream
     │
     ├── tool JSON ──► mold.parse / extract   (optional) shape & validate
     │
     └── embeddings ─► ml.embed / topk        (optional) RAG vectors
```

| Piece | Kind | Job | Doc |
|-------|------|-----|-----|
| `llm` · `ollama` · `vllm` | **stdlib** | chat, tools, stream, local hosts | [LLM_PROVIDERS.md](LLM_PROVIDERS.md) · [LLM_LOCAL.md](LLM_LOCAL.md) |
| **mold** | **module** | structured models, validate JSON, JSON Schema / tool params | [MOLD.md](MOLD.md) |
| **tokensave** | **module** | thrift context, memory, teach → train gold | [`packages/tokensave`](../packages/tokensave/) |
| **ml** | **module** | embeddings, vectors, RAG index, metrics | [ML.md](ML.md) |
| fine-tune | **CLI + external GPU** | private train orchestration | [FINETUNE.md](FINETUNE.md) |

### Install the agent modules you need

```bash
# monorepo
weft packages list
weft packages get mold
weft packages get ml
weft packages get tokensave
weft install

# or path form
weft get mold ./packages/mold
```

```weft
use mold
use ml          // only if you installed it
use tokensave   // only if you installed it

fn main -> Result {
    // shape tool args / model JSON
    Args := mold.model({"city": "str!"})?
    a := mold.parse(Args, "{\"city\":\"Paris\"}")?

    // chat / tools stay in stdlib
    say(llm.chat("hi")?)
}
```

Offline samples:

| Example | Shows |
|---------|--------|
| **[`examples/agent_stack/`](../examples/agent_stack/)** | **mold + tokensave + ml together** (no network) |
| [`examples/cookbook/13_agent.weft`](../examples/cookbook/13_agent.weft) | `llm` tools (stdlib, mock-friendly) |
| [`examples/cookbook/14_mold.weft`](../examples/cookbook/14_mold.weft) | mold parse / extract / tool_params |
| [`examples/mold_ai.weft`](../examples/mold_ai.weft) | mold + tool_spec wire formats |
| [`examples/ml_demo/`](../examples/ml_demo/) | ml vectors (after install) |
| [`examples/tokensave_demo/`](../examples/tokensave_demo/) | tokensave brain + memory |

---

## Web path

```text
  browser / HTMX
       │
       ▼
  web.listen + app.routes     (stdlib)
       │
       ├── app.before         auth for routes + static + WebSocket
       ├── req.form / files   forms + multipart
       └── web.htmx*          partials, OOB, cookies, triggers
```

| Piece | Doc |
|-------|-----|
| Routes, static, SSE | [web.md](web.md) |
| HTMX helpers, cookies, `before` | [web.md](web.md#htmx) |
| Demo | `weft run examples/htmx.weft` |

---

## Packages path (consumer vs author)

| You are… | Doc |
|----------|-----|
| Installing modules into an app | [packages.md](packages.md) |
| Writing a module for others | [modules.md](modules.md) |
| Monorepo catalog (`ml` / `mold` / `tokensave`) | [`packages/README.md`](../packages/README.md) · `weft packages list` |

Same install model for all modules:

```text
weft get <name> <path|git@tag>   →  weft.json deps
weft install                     →  vendor/ + weft.lock
use <name>                      →  import
```

Capabilities: third-party code in `vendor/` is **restricted by default**. Profiles (`@llm`, `@agent`, `@data`, …) and explicit grants live in the module’s `weft.json`. Details: [modules.md](modules.md#capabilities-host-access) · [SECURITY.md](../SECURITY.md).

| Module | Typical caps | Notes |
|--------|----------------|-------|
| `mold` | none | pure validation — safe default |
| `ml` | `@agent` + `fs` + `env` | HTTP embeddings / keys |
| `tokensave` | `@agent` + `fs` + `env` | memory on disk + model calls |

---

## Trust path

Weft is **host-power** for your scripts (like a shell + HTTP toolkit), not a multi-tenant sandbox.

| Boundary | Meaning |
|----------|---------|
| Your app scripts | Full host (`fs`, `sh`, `http`, `llm`, …) |
| `vendor/` modules | Only granted stdlib (capabilities) |
| Outbound HTTP | SSRF guards (private/metadata blocked by default) |
| Env API keys → `llm` | Hostname allowlist only (`WEFT_LLM_TRUST_HOSTS` to extend) |
| `Secret` | Prints as `***`; use `secrets.unwrap` at the edge |

Operator checklist and hardening list: **[SECURITY.md](../SECURITY.md)**.  
Production habits: [PRODUCTION.md](PRODUCTION.md).

---

## Doc map (where to go)

### Learn the language
[TUTORIAL](TUTORIAL.md) → [LANGUAGE](LANGUAGE.md) → [SYNTAX](SYNTAX.md) → [COOKBOOK](COOKBOOK.md)

### Build agents
This page → [LLM_PROVIDERS](LLM_PROVIDERS.md) → [MOLD](MOLD.md) → [ML](ML.md) → [FINETUNE](FINETUNE.md)

### Build HTTP / HTMX
[web.md](web.md) → cookbook server examples → [PRODUCTION](PRODUCTION.md)

### Extend Weft
[packages.md](packages.md) (consume) · [modules.md](modules.md) (author) · [`packages/`](../packages/)

### Operate safely
[SECURITY.md](../SECURITY.md) · [PRODUCTION.md](PRODUCTION.md) · [SYSOPS.md](SYSOPS.md)

### What ships / what never will
[STDLIB.md](STDLIB.md) · [STDLIB_GAPS.md](STDLIB_GAPS.md) · [ROADMAP.md](ROADMAP.md) · [PRINCIPLES.md](PRINCIPLES.md)
