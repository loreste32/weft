# Where we are, and where we hope to go

Weft is for agent scripts, HTTP glue, and ops tooling. It is not trying to replace CPython or the scientific Python stack.

## Where we are now (0.3.29)

We are in the middle of a **0.3.x** line (branch `0.3.1`, patches through **0.3.35** — see [VERSIONING.md](VERSIONING.md)). You can build the binary, write real scripts, and run them without a Python runtime on the critical path.

**Solid enough to use today**

- Language loop: lex → parse → compile → stack VM  
- Own syntax (`:=`, `mut`, `use`, `say`, `?`, match, defer, enum)  
- **Closures capture outer locals by value** (deep-copied at creation — safe under concurrency)  
- **Enums** as string-tagged maps (`enum Status { Ok, Err }` → `Status.Ok == "Ok"`)  
- **Match** on literals, consts, and field patterns (`Status.Ok`) plus `_`  
- Errors via `Result` + `?` (no try/catch)  
- Concurrency without `async`/`await` (map/filter fan-out, spawn, channels)  
- Packages: path/git, vendor, lock; monorepo catalog (`ml`, `tokensave`); optional `WEFT_CATALOG_URL`  
- Catalog CLI: `packages list|search|info|get[@constraint]` with suggestions  
- `weft doctor` surfaces catalog + project deps/vendor health (CI smoke)  
- `say` works as statement and expression (`|> say`)  
- Module author DX: `mod check` + **`mod check --tests`**; CI runs it on `packages/*`  
- Scientific floats: `1e-6`, `2.5E+3`  
- Hex/bin/oct ints + digit separators: `0xff`, `0b1010`, `0o755`, `1_000`  
- Editor number highlighting + `fmt` preserve advanced literal forms  

- Lite version constraints: `^`, `~`, `>=`, exact (checked against package `weft.json` version)  
- LLM: OpenAI-compatible, Anthropic tools, Ollama, vLLM; private fine-tune is optional and GPU-side  
- Stdlib for I/O, HTTP, web, text/math, config (yaml/toml/ini), some “lite” cousins of common Python modules  
- Day-to-day tools: `weft check`, `test`, `fmt`, `bench`, `stdlib`, LSP (incl. format)  
- Sysops surface: `sh`/`fs`/`cli`/`env`/`platform`/`secrets` + host-check example ([SYSOPS.md](SYSOPS.md))  
- Stdlib Tier A/B complete (ops/agent lite): `shlex`, `signal`, `binstruct`, `difflib`, `copy`, `functools`, `traceback`, path/bytes, secrets tokens, stats, IP network ([STDLIB_GAPS.md](STDLIB_GAPS.md))  
- Docs: tutorial, language reference, cookbook + `examples/cookbook/` (CI smoke)  
- Gold corpus includes closures, enums, and match (train eval 100%)  
- Agent/LLM ergonomics: multi-turn `llm.chat`, `ask`+opts, `stream_text` (in gold corpus)  
- Glue ergonomics: `json.get`/`env.get` defaults, `http.get_json`, `str.starts_with` (in gold)  

**Still rough or incomplete**

- Type checking is gradual, not a full sound system  
- `weft fmt` covers the common style (enums, match arms, closures); still not every edge case  
- LSP is usable daily (completion, hover, signatures, definition, symbols, diagnostics, format); not IDE-grade refactoring  
- Stdlib is broad-and-shallow: good for glue, not a CPython replacement  
- No public package registry / signed packages yet  
- Streaming works for common SSE paths; it is not a full product surface  
- Scientific compute and heavy training stay outside (on purpose)  

In one line: **usable for agents and ops scripts; still early as a language ecosystem.**

## Where we hope to go

We are not racing to 1.0. The near goal is a boring, dependable **0.3.x** through **0.3.35**: fix sharp edges, deepen what people already touch, keep the binary small.

**On this line (0.3.x), we hope to**

- Harden error messages, check/test/fmt/bench until they feel ordinary  
- Grow stdlib only where agent/ops scripts actually hurt  
- Make modules and the monorepo catalog easier to live with  
- Improve LSP enough that daily editing is not painful  
- Keep gold/train eval honest so models learn real Weft  
- Document limits as carefully as features  

**Maybe later (only if they earn their keep)**

- Public package discovery if path/git becomes a tax  
- Stronger package trust (signing, richer version ranges)  
- Richer editor packaging (marketplace polish)  
- More LLM providers or stream polish — without swallowing every vendor beta  
- Sum types with payloads (enums today are string tags only)  

**Probably never in core**

- NumPy / SciPy / pandas  
- In-process PyTorch training  
- Jupyter as the main loop  
- Full enterprise cloud SDKs  
- `async`/`await` keywords (would undo concurrent-by-default)  

## How we decide what goes in core

Before adding to the **core binary**, we ask:

1. Do most agent/ops scripts need this?  
2. Could it be a `packages/*` module instead?  
3. Does it force GPU, huge native deps, or a second language on the hot path?  
4. Does it fight small-language principles?  

Rule of thumb: **HTTP + agents + local LLM → core. Embeddings/RAG → module. GPU train → orchestrate outside.**

## Related

- [README.md](README.md) — documentation index  
- [TUTORIAL.md](TUTORIAL.md) · [LANGUAGE.md](LANGUAGE.md) · [COOKBOOK.md](COOKBOOK.md) · [STDLIB.md](STDLIB.md)  
- Runnable recipes: [examples/cookbook/](../examples/cookbook/)  
- [PRINCIPLES.md](PRINCIPLES.md) — product rules  
- [PRODUCTION.md](PRODUCTION.md) — timeouts, secrets, deploy sketch  
- [TOOLING.md](TOOLING.md) · [TESTING.md](TESTING.md) · [ERRORS.md](ERRORS.md)  
- [ML.md](ML.md) · [modules.md](modules.md) · [FINETUNE.md](FINETUNE.md)  

### Stdlib vs Python (reference)

We added lite cousins where scripts need them (`math`, `fs`, `yaml`, `iter`, `collections`, `bisect`, `heap`, …). Run `weft stdlib` for the live list. Full CPython parity is not a goal.
