# Weft

A scripting language for agent tools, telecom, HTTP glue, and ops work. One binary, no runtime dependencies.

[![CI](https://github.com/loreste32/weft/actions/workflows/ci.yml/badge.svg)](https://github.com/loreste32/weft/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

![Wifty — Weft mascot](assets/brand/wifty.jpg)

## What it is

Weft is a small language with its own syntax, a stack VM, and 76 stdlib packages baked into the binary. You write `.weft` files, run them with `weft run`, and ship scripts without setting up environments or installing interpreters.

It handles errors with `Result` / `?` instead of exceptions, runs concurrent work without `async`/`await`, and talks to LLMs, databases, SIP servers, and HTTP services out of the box.

| | |
|--|--|
| Version | 0.4.4 (`main` branch) |
| Website | [weftproject.dev](https://weftproject.dev) |
| Install | `curl -fsSL https://weftproject.dev/install.sh \| sh` |
| Docs | [weftproject.dev/docs.html](https://weftproject.dev/docs.html) |
| Registry | [registry.weftproject.dev](https://registry.weftproject.dev) |
| Security | [SECURITY.md](SECURITY.md) |

## Install

```bash
# one-line (macOS / Linux)
curl -fsSL https://weftproject.dev/install.sh | sh

# Ubuntu / Debian
curl -fsSL https://weftproject.dev/weft-archive-keyring.gpg | sudo gpg --dearmor -o /usr/share/keyrings/weft.gpg
echo "deb [signed-by=/usr/share/keyrings/weft.gpg] https://weftproject.dev/apt stable main" | sudo tee /etc/apt/sources.list.d/weft.list
sudo apt update && sudo apt install weft

# Fedora / RHEL
sudo dnf config-manager --add-repo https://weftproject.dev/rpm
sudo dnf install weft

# macOS (Homebrew)
brew tap loreste32/tap && brew install weft

# from source
go build -o weft ./cmd/weft
```

## Quick start

```bash
weft doctor
weft run examples/hello.weft
weft run examples/todoapp/main.weft   # web app with SQLite
```

## The language

```weft
fn weather(city) { "clear in $city" }

fn main -> Result {
    reply := llm.ask("Weather in Paris?", [
        llm.tool("weather", weather),
    ])?
    say(reply)
}
```

| Syntax | What it does |
|--------|-------------|
| `x := 1` | Bind a value |
| `mut n := 0` | Mutable binding |
| `use pkg` | Import a package |
| `say("hi")` | Print |
| `"hello $name"` | String interpolation |
| `expr?` | Propagate errors |
| `x \|> f` | Pipeline |
| `match x { 1 { "one" } _ { "other" } }` | Pattern matching |

Sum types with payloads:

```weft
enum Shape {
    Circle(radius)
    Rect(w, h)
    Point
}

fn area(s) {
    match s {
        Shape.Circle(r) { 3.14 * r * r }
        Shape.Rect(w, h) { w * h }
        Shape.Point { 0 }
    }
}
```

Full syntax: [docs/SYNTAX.md](docs/SYNTAX.md) | Language reference: [docs/LANGUAGE.md](docs/LANGUAGE.md)

## What's in the box

**Language:** lex, parse, compile, stack VM. Closures (capture by value), sum types with payloads, `match` with destructuring, `defer`, `Result`/`?`. Concurrent `map`/`filter`, `spawn`, channels — no `async`/`await`.

**76 stdlib packages (in the binary):**

| Area | Packages |
|------|----------|
| LLM / AI | `llm` (OpenAI/Anthropic/Ollama/vLLM), `mcp` (Model Context Protocol), `deepgram` (streaming STT), `elevenlabs` (streaming TTS), `mlinfer` (ONNX/Triton/HF inference) |
| Web | `http`, `web` (HTMX/SSE), `ws`, `webrtc`, `graphql` |
| DevOps | `sysinfo`, `proc`, `netutil`, `sh`, `fs`, `cli`, `env`, `signal`, `secrets`, `log` |
| Data | `db` (SQLite/Postgres/MySQL), `csv`, `json`, `yaml`, `toml`, `xml`, `redis`, `mongo`, `nats`, `amqp` |
| Network | `pcap`, `socket`, `email`, `ip` |
| All | Run `weft stdlib` to see the full list |

**Tooling:**
- `weft check [--types] [--strict]` — type checking (`--strict` fails on type warnings; CI uses this)
- `weft test [--coverage]` — run `fn test_*` in `*_test.weft` files
- `weft fmt [--check]` — code formatter (CI-friendly with `--check`)
- `weft run [--watch]` — run scripts, auto-reload on file changes
- `weft debug [--dap]` / `profile` — debugger (CLI or DAP for VS Code) and profiler
- **Browser Wasm** — `make wasm` builds a client-side runtime (`wasm/playground.html`)
- **LSP types** — hover/completion use annotations + inference; type warnings as diagnostics
- **Reliability** — bytecode validation, fuzz/race/compat CI, `make release-smoke`; slim binary (`-tags slim`); see [docs/STABILITY.md](docs/STABILITY.md)
- `weft notebook` — run `.weft` as cells, output HTML
- `weft mcp serve <file>` — expose Weft functions as MCP tools
- `weft update` — self-update to latest version
- `weft upgrade` — upgrade installed packages
- `weft lsp` — Language Server (completion, hover, rename, diagnostics)

**14 registry modules** at [registry.weftproject.dev](https://registry.weftproject.dev):

| Module | What it does |
|--------|-------------|
| [telecom](packages/telecom/) | IVA voice agents, FreeSWITCH ESL, Asterisk ARI, STT/TTS, DTMF, routing, queues, CDR |
| [mold](packages/mold/) | Structured LLM JSON, validation, tool params |
| [ml](packages/ml/) | Embeddings, vectors, RAG index |
| [tokensave](packages/tokensave/) | Context thrift, memory, train data |
| [warp](packages/warp/) | N-dimensional array math |
| [retry](packages/retry/) | Exponential backoff for flaky operations |
| [semver](packages/semver/) | Semver parsing, comparison, constraints |
| [cache](packages/cache/) | In-memory key-value cache with TTL |
| [color](packages/color/) | ANSI terminal colors for CLI tools |
| [jwt](packages/jwt/) | JWT token decode and inspection |
| [http_router](packages/http_router/) | Routing with path params, middleware, groups, CORS |
| [template](packages/template/) | String templating with placeholders, loops, HTML escaping |
| [validate](packages/validate/) | Data validation for forms/APIs |
| [cron](packages/cron/) | Recurring task scheduler with intervals and daily times |

`packages/` also holds 4 local ML-stack packages — [dataframe](packages/dataframe/), [embed](packages/embed/), [experiment](packages/experiment/), [metrics](packages/metrics/) — installable via path/git (see `packages/index.json`).

**Packages:**
- Path and git imports into `vendor/`
- Public registry with ed25519 signing, version immutability, namespace key ownership
- Local package trust: `weft registry trust|untrust|trusts` (`WEFT_REQUIRE_TRUST=1`)
- Capability system for third-party package sandboxing
- `weft registry search|install|keygen|serve`

## CLI

```text
weft                              REPL
weft run <file.weft> [--watch]    run a script
weft build [dir] [-o out]        bundle into .weftapp archive
weft check <file|dir> [--types] [--strict]   type check
weft test [--race] [--mem] [--timeout N] [--coverage]
weft fmt [--check] <file|dir>     format
weft notebook <file> [-o out.html]
weft bench | stdlib | doctor | version | lsp
weft debug [--dap] [file.weft]    debugger (DAP for IDEs)
weft profile <file.weft>          profiler
weft mcp serve <file.weft>        MCP tool server
weft update                       self-update binary
weft upgrade                      upgrade packages

weft new module|app|cli <name>    scaffold
weft get <name> <path|git>        add dependency
weft install                      install from weft.json
weft publish [--key name]         sign and upload
weft registry search|install|keygen|serve
weft gen "task" [-o out.weft]     LLM generates Weft
weft train prepare|finetune|eval  fine-tuning pipeline
```

## Examples

```bash
weft run examples/hello.weft
weft run examples/todoapp/main.weft        # web app: SQLite + JSON API + HTML
weft run examples/cli_tool.weft -- --help  # CLI with flags
weft run examples/sysops_host.weft -- info # host checks
weft run examples/pipeline_etl.weft        # data pipeline
weft run examples/db_sqlite.weft           # database CRUD
weft run examples/channels.weft            # concurrency
```

More in [`examples/`](examples/) and [`examples/cookbook/`](examples/cookbook/).

## Documentation

| | |
|---|---|
| [weftproject.dev/docs.html](https://weftproject.dev/docs.html) | Full docs (on-site, searchable) |
| [weftproject.dev/cookbook.html](https://weftproject.dev/cookbook.html) | 22 searchable recipes |
| [weftproject.dev/download.html](https://weftproject.dev/download.html) | Install guides (apt, dnf, brew, Docker) |
| [docs/TUTORIAL.md](docs/TUTORIAL.md) | Guided first hour |
| [docs/STDLIB.md](docs/STDLIB.md) | Stdlib reference |
| [docs/TELECOM.md](docs/TELECOM.md) | Telecom / IVA / FreeSWITCH / Asterisk |
| [docs/MCP.md](docs/MCP.md) | MCP integration |
| [docs/ECOSYSTEM.md](docs/ECOSYSTEM.md) | How pieces fit together |
| [SECURITY.md](SECURITY.md) | Threat model and capabilities |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Where we are and what's next |

## Why this exists

We wanted a small language for agent tools, telecom, and ops glue — one binary, explicit error handling, simple packages, and LLM integration without a heavy runtime.

## Develop

```bash
make test       # go test ./...
make ci         # gofmt, vet, tests, example smoke
make build      # ./weft
make install    # ~/.local/bin/weft
```

Contributing: [CONTRIBUTING.md](CONTRIBUTING.md) | Security: [SECURITY.md](SECURITY.md)

## License

Apache-2.0
