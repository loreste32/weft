# Where we are, and where we hope to go

Weft is for agent scripts, telecom, HTTP glue, and ops tooling. It stays small on purpose.

## Where we are now (0.4.6)

Weft is on the **0.4.x** line (0.3.x complete — see [VERSIONING.md](VERSIONING.md)). Positioning and maturity: [STABILITY.md](STABILITY.md). You can build the binary, write real scripts, and run them on a single Go runtime.

**Language**

- Lex → parse → compile → stack VM  
- Own syntax (`:=`, `mut`, `use`, `say`, `?`, match, defer, enum)  
- **Closures** capture outer locals by value (deep-copied — safe under concurrency)  
- **Sum types with payloads**: `enum Shape { Circle(r), Rect(w,h) }` + destructuring in `match`  
- Errors via `Result` + `?` (no try/catch)  
- Concurrency without `async`/`await` (map/filter fan-out, spawn, channels, race, timeout)  
- Scientific floats (`1e-6`), hex/bin/oct ints, digit separators  

**81 stdlib packages** (in the binary)

- LLM: `llm` (OpenAI/Anthropic/Ollama/vLLM), `ollama`, `vllm` — chat, tools, streaming, agents  
- AI integration: `mcp` (Model Context Protocol client + server), `deepgram` (streaming STT), `elevenlabs` (streaming TTS), `mlinfer` (ONNX/Triton/HuggingFace inference)  
- Web: `http`, `web` (HTMX, SSE, cookies, `app.before`), `ws`, `webrtc`  
- DevOps: `sysinfo` (CPU/memory/disk/uptime), `proc` (process list/kill), `netutil` (port scan/DNS/TCP ping), `sh`, `fs`, `signal`, `secrets`, `log`  
- Data: `db` (SQLite/Postgres/MySQL), `csv`, `json`, `jsonl`, `yaml`, `toml`, `xml`, `ini`, `redis`, `mongo`, `nats`, `amqp`, `graphql`  
- CLI/ops: `cli` (flags, subcommands), `env`, `platform`, `shlex`, `crypto`, `pcap`, `email`, `socket`  
- Collections: `str`, `math`, `time`, `re`, `iter`, `collections`, `heap`, `bisect`, `pipe`, `functools`, `copy`, `traceback`  
- Full list: `weft stdlib`  

**23 registry modules** at [registry.weftproject.dev](https://registry.weftproject.dev)

| Module | What |
|--------|------|
| `telecom` | IVA voice agents, FreeSWITCH ESL, Asterisk ARI, STT/TTS, DTMF, routing, queues, CDR |
| `mold` | Structured LLM JSON, validation, JSON Schema, tool params |
| `ml` | Embeddings, vectors, RAG index, metrics |
| `tokensave` | Context thrift, memory, teach → train export |
| `warp` | N-dimensional array math |
| `retry` | Exponential backoff for flaky operations |
| `semver` | Semver parsing, comparison, constraints |
| `cache` | In-memory key-value cache with TTL |
| `color` | ANSI terminal colors for CLI tools |
| `jwt` | JWT token decode and inspection |
| `http_router` | Routing with path params, middleware, groups, CORS |
| `template` | String templating with placeholders, loops, HTML escaping |
| `validate` | Data validation for forms/APIs |
| `cron` | Recurring task scheduler |
| `auth` | HMAC, password hashing, tokens, OAuth helpers |
| `queue` | In-process job queue with retries, dead-letter |
| `config` | Unified config: .env/JSON/YAML/TOML with validation |
| `logger` | Structured logging: levels, JSON/text, child loggers |
| `router` | HTTP routing, path params, middleware, CORS |

Plus 4 local ML-stack packages in `packages/` (`dataframe`, `embed`, `experiment`, `metrics`) — install via path/git.

**Registry and packages**

- Public registry hosted at **registry.weftproject.dev** with web UI  
- Mandatory ed25519 signing on all publishes  
- Version immutability (no overwrites)  
- Capability sandboxing for third-party packages  
- `weft registry search|info|install|keygen|keys|serve`  
- `weft publish --key <name>` with signature verification  

**Tooling**

- `weft check [--types]`, `test [--coverage]`, `fmt [--check]`, `bench`  
- `weft debug <file>` — debugger · `weft profile <file>` — profiler  
- `weft notebook <file> [-o out.html]` — cells to HTML  
- `weft mcp serve <file>` — expose functions as MCP tools  
- `weft update` — self-update binary · `weft upgrade` — upgrade packages  
- `weft gen "task" -o out.weft` — LLM generates Weft from English  
- `weft train prepare|finetune|eval` — private fine-tuning pipeline  
- LSP: completion, hover, signatures, definition, references, rename, extract-function, auto-import, diagnostics, format  
- VS Code and JetBrains editor plugins  

**Distribution**

- Website: [weftproject.dev](https://weftproject.dev) with docs, cookbook, download, registry  
- One-line install: `curl -fsSL https://weftproject.dev/install.sh | sh`  
- APT repo (Ubuntu/Debian): `apt install weft`  
- DNF repo (Fedora/RHEL): `dnf install weft`  
- Homebrew formula  
- Dockerfile for containers  
- GitHub Release with binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 — automated on `v*` tags (`.github/workflows/release.yml`)  
- macOS binaries ad-hoc signed for Gatekeeper  

**Still rough or incomplete**

- Type checking is gradual, not a full sound system  
- `weft fmt` covers the common style; still not every edge case  
- LSP is usable daily; not IDE-grade refactoring  
- Stdlib is broad-and-shallow: good for glue, not a full OS platform  
- Binary is convenience-first (~40MB with drivers); not a minimal embed  
- Concurrent-by-default `map`/`filter` need discipline (`seq_map` for stateful work)  
- Package signatures prove key identity, not human ownership (trust store helps)  
- DAP `evaluate` resolves identifiers only — no expression evaluation, `setVariable`, or restart yet  
- Windows `sysinfo` memory/disk return "not implemented"; Unix is full  
- Browser Wasm playground stubs 29 network/db/LLM packages — it is a pure-language sandbox  
- Slim build stubs the db/broker packages (clear runtime error, but no build-time warning)  
- CI executes on Linux only; macOS/Windows are cross-compiled, never run  

In one line: **useful for agents, telecom, and ops scripts when versions are pinned and tested; not a finished ecosystem.**

## Where we hope to go

The **0.3.x line is complete** (0.3.31–0.4.6). Everything shipped.

**Completed in 0.3.x:** changelog page, `weft doc`, `weft lint`, `weft build`, `weft test --race/--mem/--timeout`, `cluster`/`governor`/`supervisor` stdlib, `deepgram`/`elevenlabs`/`mlinfer`, MCP, telecom with FreeSWITCH/Asterisk, website with 36 doc pages. (0.4.0 then added the `http_router`, `template`, `validate`, `cron` registry modules — 14 total.)

## 0.4.x — make it solid

**Shipped in 0.4.x so far (0.4.0–0.4.6):** optional type annotations + `--strict`, DAP debugging, browser Wasm playground, registry namespace trust, telecom SIP REFER / WebRTC bridge, VS Code 0.4.6 (LSP types + DAP), bytecode validation, fuzz/race/bench smoke targets, grouped imports, registry auto-fetch, third-party git imports, LSP references/rename/extract/auto-import, REPL tab completion + multi-line polish, compat corpus expansion, glue benchmarks vs Python, reference apps (`ref_agent_ops`, `ref_http_glue`, `ref_ops`), tag-triggered release workflow.

**Reliability (priority now — prove the core):**
- Language/VM fuzzing and malformed-input testing (`make fuzz-smoke`) — done (smoke + weekly deep)  
- Race detector + concurrency stress (`make race-smoke`) — done  
- Cross-platform reproducible releases — done (`make release-smoke` + tag-triggered `.github/workflows/release.yml` publishing binaries, checksums, and the VSIX)  
- Compatibility / gold corpus discipline — done (`testdata/compat`, still expand)  
- Benchmarks vs Python for glue scripts — done (`make bench` + `make bench-glue`)  
- Optional stdlib build tags / binary size — done (`make build-slim`)  
- Formatter + LSP edge cases — partial (format corpus green; locals + multi-file rename + extract)  
- Error-message hardening — partial (arity, Illegal lit, use::, else-brace, empty match)

**Language maturity:**
- Harden error messages further (more edge cases)
- REPL: multi-line, history, tab completion / ↑↓ — done (TTY); pipes still Scanner-based

**IDE & tooling:**
- LSP: locals, multi-file rename, extract-function — done  
- VS Code extension 0.4.6 VSIX packaged; Marketplace publish needs `VSCE_PAT`

**Release & platform gaps (next):**
- macOS + Windows CI runners — today those targets are cross-compiled, never executed  
- Dockerfile built and smoke-tested in CI (currently unverified)  
- Windows `sysinfo` memory/disk implementation  
- DAP: real expression evaluation, `setVariable`, exception breakpoints  
- Live-broker test coverage for `amqp`/`mongo`/`nats` ([STDLIB_GAPS.md](STDLIB_GAPS.md))  
- LSP tests for rename / references / extract / auto-import  
- Bring `install.sh` / APT / DNF / Homebrew packaging automation in-repo (maintained out-of-tree today; this repo alone cannot reproduce those channels)  
- Audit registry.weftproject.dev contents against `packages/` — publish any of the 23 modules that are missing  

**Scale & adoption:**
- Key rotation policy for namespaces  
- More telecom (SIP REFER already partially in-module)  
- More production-quality reference apps (initial set shipped in 0.4.2; polish and expand)  

**Probably never in core**

- Heavy scientific array / dataframe stacks  
- In-process deep-learning training  
- Notebook as the main loop  
- Full enterprise cloud SDKs  
- `async`/`await` keywords (would undo concurrent-by-default)  

## How we decide what goes in core

Before adding to the **core binary**, we ask:

1. Do most agent/ops scripts need this?  
2. Could it be a `packages/*` module instead?  
3. Does it force GPU, huge native deps, or a second language on the hot path?  
4. Does it fight small-language principles?  

Rule of thumb: **HTTP + agents + local LLM → core. Embeddings/RAG → module. GPU train → orchestrate outside. SIP → module. ML inference → stdlib (HTTP client only).**

## Related

- [README.md](README.md) — documentation index  
- [TUTORIAL.md](TUTORIAL.md) · [LANGUAGE.md](LANGUAGE.md) · [COOKBOOK.md](COOKBOOK.md) · [STDLIB.md](STDLIB.md)  
- [TELECOM.md](TELECOM.md) · [MCP.md](MCP.md) · [ECOSYSTEM.md](ECOSYSTEM.md)  
- Runnable recipes: [examples/cookbook/](../examples/cookbook/)  
- [PRINCIPLES.md](PRINCIPLES.md) — product rules  
- [PRODUCTION.md](PRODUCTION.md) — timeouts, secrets, deploy sketch  
- [TOOLING.md](TOOLING.md) · [TESTING.md](TESTING.md) · [ERRORS.md](ERRORS.md)  
- [ML.md](ML.md) · [FINETUNE.md](FINETUNE.md) · [modules.md](modules.md)  
