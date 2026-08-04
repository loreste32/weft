# How Weft fits together

One picture for language, stdlib, optional modules, agents, telecom, web, and trust.
Detail pages stay short and point back here when you get lost.

## Layers

```text
┌─────────────────────────────────────────────────────────────┐
│  Your app  (.weft + weft.json + vendor/)                     │
├─────────────────────────────────────────────────────────────┤
│  Optional modules  (registry → vendor/ — NOT in the binary)  │
│    telecom · auth · router · config · logger · queue ·       │
│    mold · ml · tokensave · retry · cache · jwt · …  (23)     │
├─────────────────────────────────────────────────────────────┤
│  Stdlib  (IN the binary — 83 packages)                       │
│    llm · mcp · deepgram · elevenlabs · mlinfer · http ·     │
│    dns · tls · os · compress · encoding · governor ·         │
│    supervisor · cluster · sysinfo · proc · netutil · …       │
├─────────────────────────────────────────────────────────────┤
│  Language + VM  (weft binary)                                │
│    Result/? · concurrency · modules system · auto-fetch      │
└─────────────────────────────────────────────────────────────┘
```

| Layer | Lives where | You get it by |
|-------|-------------|----------------|
| Language / VM | `weft` binary | `curl -fsSL https://weftproject.dev/install.sh \| sh` |
| Stdlib (81 pkgs) | same binary | `use http` / `use llm` / `use dns` / … |
| Registry modules (23) | `packages/` → `vendor/` | `weft get name` or auto-fetch on `use` |
| Your code | project dir | `weft run` |

**Rule:** if it is domain-specific (telecom, embeddings, structured tool args), it is a **module**. If most agent/ops scripts need it (HTTP, LLM chat, files, DNS), it is **stdlib**.

---

## Agent path

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
| `mcp` | **stdlib** | Model Context Protocol client + server | [MCP.md](MCP.md) |
| `deepgram` | **stdlib** | streaming STT via WebSocket (Nova-2) | [DEEPGRAM.md](DEEPGRAM.md) |
| `elevenlabs` | **stdlib** | streaming TTS via WebSocket (Turbo v2.5) | [ELEVENLABS.md](ELEVENLABS.md) |
| `mlinfer` | **stdlib** | ML inference (ONNX, Triton, HuggingFace) | [MLINFER.md](MLINFER.md) |
| `governor` | **stdlib** | token/cost/time budgets for LLM calls | [STDLIB.md](STDLIB.md) |
| `supervisor` | **stdlib** | Erlang-style process supervision | [STDLIB.md](STDLIB.md) |
| `cluster` | **stdlib** | distributed state via Redis (locks, counters, pub/sub) | [CLUSTER.md](CLUSTER.md) |
| **mold** | **module** | structured models, validate JSON, JSON Schema / tool params | [MOLD.md](MOLD.md) |
| **tokensave** | **module** | thrift context, memory, teach → train gold | [`packages/tokensave`](../packages/tokensave/) |
| **ml** | **module** | embeddings, vectors, RAG index, metrics, classical minibatch training | [ML.md](ML.md) |
| fine-tune | **CLI + external GPU** | private train orchestration | [FINETUNE.md](FINETUNE.md) |

---

## Telecom / IVA path

Build voice applications with FreeSWITCH ESL or Asterisk ARI:

| Piece | Kind | Job |
|-------|------|-----|
| **telecom** | **module** | IVA agents, FreeSWITCH ESL, Asterisk ARI, SIP, routing, queues, CDR, dial plans, SSML |
| `deepgram` | **stdlib** | real-time speech-to-text |
| `elevenlabs` | **stdlib** | real-time text-to-speech |
| `llm` | **stdlib** | conversational AI for IVA |

```weft
use telecom

fn main -> Result {
    conn := telecom.esl_connect("127.0.0.1", 8021, "ClueCon")?
    telecom.esl_answer(conn)?
    telecom.esl_play(conn, "ivr/welcome.wav")?
    digits := telecom.dtmf_collect(conn, {"max_digits": 4, "timeout": 10})?
    say("caller pressed: " + digits)
}
```

Doc: [TELECOM.md](TELECOM.md)

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
| **router** module (path params, middleware, CORS) | `use router` |
| **template** module (layouts, partials, loops, escaping) | `use template` |
| **auth** module (HMAC, passwords, tokens, OAuth) | `use auth` |

---

## DevOps / sysops path

```text
  weft run runbook.weft
       │
       ├── sysinfo.memory / disk / uptime / loadavg
       ├── proc.list / find / kill
       ├── netutil.port_open / tcp_ping / scan_ports
       ├── dns.lookup / srv / mx / txt / reverse
       ├── tls.cert_info / verify / expiry_check
       ├── os.hostname / pid / user / stat / mkdir
       ├── compress.gzip / gunzip
       ├── encoding.hex_encode / base32_encode / url_encode
       └── sh.run / capture / lines
```

| Piece | Kind | Job |
|-------|------|-----|
| `sysinfo` | **stdlib** | CPU, memory, disk, uptime, load, interfaces |
| `proc` | **stdlib** | process list, find, kill, exists |
| `netutil` | **stdlib** | port check, TCP ping, DNS, port scan |
| `dns` | **stdlib** | full DNS client (A/AAAA, SRV, CNAME, NS, MX, TXT, PTR) |
| `tls` | **stdlib** | TLS certificate inspection, chain, expiry monitoring |
| `os` | **stdlib** | env, paths, user info, filesystem, platform |
| `compress` | **stdlib** | gzip/gunzip, deflate/inflate |
| `encoding` | **stdlib** | hex, base32, URL encoding |
| **config** module | config loader (.env/JSON/YAML/TOML, validation) | `use config` |
| **logger** module | structured logging (levels, JSON, child loggers) | `use logger` |

Doc: [SYSOPS.md](SYSOPS.md)

---

## Registry modules (23 packages)

All available at [registry.weftproject.dev](https://registry.weftproject.dev). Install with `weft get <name>` or they auto-fetch on first `use` (disable with `WEFT_NO_AUTO_FETCH=1`).

| Module | What it does |
|--------|-------------|
| **telecom** | IVA, FreeSWITCH ESL, Asterisk ARI, SIP, STT/TTS, routing, queues |
| **auth** | HMAC, Argon2id password hashing (Go native), PBKDF2, tokens, OAuth helpers |
| **router** | HTTP routing, path params, middleware chains, CORS |
| **config** | Unified config: .env/JSON/YAML/TOML, dot-path access, validation |
| **logger** | Structured logging: levels, JSON/text, context fields, child loggers |
| **queue** | Synchronous in-memory job list with retries and dead-letter |
| **template** | HTML templating: layouts, partials, loops, conditionals, auto-escaping |
| **validate** | Data validation for forms and APIs |
| **http_router** | HTTP routing with path params, middleware, groups, CORS |
| **jwt** | JWT decode, inspect claims, expiry check |
| **cache** | In-process LRU cache with TTL |
| **retry** | Retry with backoff, jitter, and circuit breaker |
| **cron** | Recurring task scheduler |
| **semver** | Semantic versioning: parse, compare, bump, ranges |
| **color** | Terminal color output (ANSI 256 / truecolor) |
| **mold** | Structured models, JSON Schema, tool params |
| **tokensave** | Thrift context, memory, teach → train gold |
| **ml** | Embeddings, vectors, RAG index, classical training |
| **warp** | Validated NumPy-style arrays and native CUDA/ROCm/MLX dispatch |
| **dataframe** | Validated tabular data: null-aware stats, joins, rolling, CSV/JSON |
| **embed** | Embeddings client + vector store |
| **experiment** | Experiment tracking: runs, params, metrics |
| **metrics** | ML metrics: accuracy, F1, precision, recall |

### Install

```bash
# auto-fetch: just use it
use auth
use config

# explicit install
weft get auth
weft get config
weft install

# grouped imports
use { "auth" "config" "logger" }
```

---

## Packages path (consumer vs author)

| You are… | Doc |
|----------|-----|
| Installing modules into an app | [packages.md](packages.md) |
| Writing a module for others | [modules.md](modules.md) |
| Browsing the registry | [registry.weftproject.dev](https://registry.weftproject.dev) |

Install model:

```text
weft get <name> [path|git@tag]   →  weft.json deps
weft install                     →  vendor/ + weft.lock
use <name>                      →  import

# or auto-fetch (no weft get needed):
use auth      # downloads from registry if not in vendor/
```

Capabilities: third-party code in `vendor/` is **restricted by default**. Profiles (`@llm`, `@agent`, `@data`, …) and explicit grants live in the module's `weft.json`. Details: [modules.md](modules.md#capabilities-host-access).

---

## CLI tools

| Command | What |
|---------|------|
| `weft run` | Run a script |
| `weft build` | Standalone executable (no weft needed on target) |
| `weft check [--types]` | Type check |
| `weft test [--race] [--mem] [--timeout N]` | Run tests |
| `weft bench [--save f.json] [--compare base.json]` | Benchmarks with regression tracking |
| `weft lint` | Static analysis |
| `weft doc` | Generate API docs |
| `weft fmt` | Format code |
| `weft info` | System report |
| `weft debug` | Interactive debugger |
| `weft profile` | Execution profiler |
| `weft mcp serve` | MCP tool server |
| `weft lsp` | Language server |
| `weft update` | Self-update weft |
| `weft upgrade` | Upgrade packages |
| `weft outdated` | Check for newer versions |
| `weft stdlib [pkg]` | Browse stdlib (live probe on zero-arg functions) |
| `weft doctor` | Check environment |

Doc: [TOOLING.md](TOOLING.md) · [cli.md](cli.md)

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
| `os.remove` | Single file only; `os.remove_tree` guards against `/` and `.` |
| `os.mkdir` | Permissions capped at 0755 |
| `weft update` | SHA-256 checksum verified before binary replacement |
| `install.sh` | SHA-256 verified before installing |
| Update/registry HTTP | Uses `netsafe.SafeHTTPClient` (SSRF protection) |
| WASM filesystem | Path traversal blocked; 16MB/file, 64MB total, 10k file cap |

Operator checklist: **[SECURITY.md](../SECURITY.md)** · Production habits: [PRODUCTION.md](PRODUCTION.md).

---

## Doc map (where to go)

### Learn the language
[TUTORIAL](TUTORIAL.md) → [LANGUAGE](LANGUAGE.md) → [SYNTAX](SYNTAX.md) → [COOKBOOK](COOKBOOK.md)

### Build agents
This page → [LLM_PROVIDERS](LLM_PROVIDERS.md) → [MCP](MCP.md) → [MOLD](MOLD.md) → [ML](ML.md) → [FINETUNE](FINETUNE.md)

### Build voice / telecom
[TELECOM](TELECOM.md) → [DEEPGRAM](DEEPGRAM.md) → [ELEVENLABS](ELEVENLABS.md)

### Build HTTP / HTMX
[web.md](web.md) → cookbook server examples → [PRODUCTION](PRODUCTION.md)

### DevOps / sysops
[SYSOPS](SYSOPS.md) → [STDLIB](STDLIB.md) → [TOOLING](TOOLING.md)

### Extend Weft
[packages.md](packages.md) (consume) · [modules.md](modules.md) (author) · [registry](https://registry.weftproject.dev)

### Operate safely
[SECURITY.md](../SECURITY.md) · [PRODUCTION.md](PRODUCTION.md) · [SYSOPS.md](SYSOPS.md)

### What ships / what never will
[STDLIB.md](STDLIB.md) · [STDLIB_GAPS.md](STDLIB_GAPS.md) · [ROADMAP.md](ROADMAP.md) · [PRINCIPLES.md](PRINCIPLES.md)
