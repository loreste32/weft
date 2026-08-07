# Weft

A scripting language for agent tools, telecom, HTTP glue, and ops work. One binary, no runtime dependencies.

[![CI](https://github.com/loreste32/weft/actions/workflows/ci.yml/badge.svg)](https://github.com/loreste32/weft/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

![Wifty — Weft mascot](assets/brand/wifty.jpg)

## What it is

Weft is a small language with its own syntax, a stack VM, and 83 stdlib packages baked into the binary. You write `.weft` files, run them with `weft run`, and ship scripts without setting up environments or installing interpreters.

It handles errors with `Result` / `?` instead of exceptions, runs concurrent work without `async`/`await`, and talks to LLMs, databases, SIP servers, and HTTP services out of the box.

| | |
|--|--|
| Version | 0.6.0 (`main` branch) |
| Website | [weftproject.dev](https://weftproject.dev) |
| Install | `curl -fsSL https://weftproject.dev/install.sh \| sh` |
| Docs | [weftproject.dev/docs.html](https://weftproject.dev/docs.html) |
| Playground | [weftproject.dev/playground.html](https://weftproject.dev/playground.html) |
| Registry | [registry.weftproject.dev](https://registry.weftproject.dev) (23 modules) |
| VS Code | `editors/vscode/` — syntax, LSP, DAP debugger |

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
weft doctor                                # check environment
weft run examples/hello.weft               # hello world
weft run examples/todoapp/main.weft        # web app with SQLite
weft run examples/sysops_host.weft -- info # devops host checks
```

## Numerical, data, and ML stack

Weft now includes a practical, tested alternative to Python for supported numerical and tabular workflows:

- [`warp`](packages/warp/): validated NumPy-style arrays with broadcasting, reductions, linear algebra, FFTs, dtype handling, and explicit CPU/native-provider dispatch.
- [`dataframe`](packages/dataframe/): pandas-inspired tabular operations with null-aware statistics, joins, rolling/expanding windows, CSV/JSON/JSONL, and a SQL bridge.
- [`ml`](packages/ml/): classical linear/logistic training, minibatches, optimizers, forward- and reverse-mode autodiff, Jacobian-vector products, and selected higher-order derivatives.
- [`accelerator`](native/accelerator/): a capability-gated plugin ABI for CUDA, ROCm/HIP, and Apple MLX providers.

The provider ABI is deliberately explicit: a plugin must report whether an operation ran on the requested device or fell back. Weft does not claim complete NumPy, pandas, or deep-learning ecosystem compatibility, and vendor GPU claims require hardware-specific builds and conformance runs. See [`docs/ML.md`](docs/ML.md), [`docs/DATAFRAME.md`](docs/DATAFRAME.md), [`docs/ACCELERATORS.md`](docs/ACCELERATORS.md), and [`docs/COMPATIBILITY.md`](docs/COMPATIBILITY.md) for the supported surface and current boundaries.

```weft
use dataframe as df
use ml

fn main -> Result {
    frame := df.from_rows([
        {"hours": 1.0, "sales": 3.0},
        {"hours": 2.0, "sales": 5.0},
    ])?
    model := ml.fit("linear", [[1.0], [2.0]], [3.0, 5.0], {
        "epochs": 100,
        "learning_rate": 0.05,
    })?
    say(df.describe(frame, "sales"))
    say(ml.predict(model, [[3.0]])?)
}
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
| `let mut n = 0` | Mutable binding |
| `use pkg` | Import a stdlib or registry package |
| `use { "auth" "config" }` | Grouped imports |
| `say("hello $name")` | Print with string interpolation |
| `expr?` | Propagate errors (`Result` type) |
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

**Language:** lex → parse → compile → stack VM. Closures (capture by value), sum types with payloads, `match` with destructuring, `defer`, `Result`/`?`. Concurrent `map`/`filter`, `spawn`, channels — no `async`/`await`.

### 83 stdlib packages (in the binary)

| Area | Packages |
|------|----------|
| LLM / AI | `llm` (OpenAI/Anthropic/Ollama/vLLM), `mcp` (Model Context Protocol), `deepgram` (streaming STT), `elevenlabs` (streaming TTS), `mlinfer` (ONNX/Triton/HF inference) |
| Infrastructure | `governor` (token/cost budgets), `supervisor` (Erlang-style), `cluster` (distributed state via Redis), `ratelimit`, `migrate` |
| Web | `http`, `web` (HTMX/SSE/cookies), `ws`, `webrtc`, `graphql` |
| DevOps | `sysinfo`, `proc`, `netutil`, `os`, `sh`, `fs`, `cli`, `env`, `signal`, `secrets`, `log` |
| Network | `dns` (A/SRV/CNAME/NS/MX/TXT/PTR), `tls` (cert inspect/verify/expiry), `pcap`, `socket`, `email`, `ip` |
| Data | `db` (SQLite/Postgres/MySQL), `csv`, `json`, `yaml`, `toml`, `xml`, `redis`, `mongo`, `nats`, `amqp` |
| Encoding | `encoding` (hex/base32/URL), `compress` (gzip/zlib), `base64`, `crypto` |
| Text / math | `str`, `re`, `math`, `decimal`, `time`, `random`, `uuid`, `html`, `mime` |
| Collections | `iter`, `collections`, `heap`, `bisect`, `pipe`, `functools`, `copy` |
| ML | `tokenizer`, `dataset`, `metrics` |
| Other | `table`, `viz`, `archive`, `binstruct`, `difflib`, `shlex`, `platform`, `traceback`, `pickle`, `io`, `test` |

Full list: `weft stdlib`

### 23 registry modules

Install with `weft get <name>` — or just `use auth` and it auto-fetches from the registry.

| Module | What it does |
|--------|-------------|
| [telecom](packages/telecom/) | IVA voice agents, FreeSWITCH ESL, Asterisk ARI, STT/TTS, DTMF, routing, queues, CDR |
| [auth](packages/auth/) | HMAC, Argon2id password hashing, token generation, OAuth helpers |
| [router](packages/router/) | HTTP routing with path params, middleware chains, CORS |
| [config](packages/config/) | Unified config loader (.env/JSON/YAML/TOML) with validation |
| [logger](packages/logger/) | Structured logging: levels, JSON/text output, child loggers |
| [queue](packages/queue/) | In-process job queue with retries and dead-letter |
| [template](packages/template/) | HTML templating: layouts, partials, loops, conditionals, auto-escaping |
| [jwt](packages/jwt/) | JWT token decode, claims inspection, expiry check |
| [http_router](packages/http_router/) | Routing with path params, middleware, groups, CORS |
| [validate](packages/validate/) | Data validation for forms/APIs |
| [mold](packages/mold/) | Structured LLM JSON, validation, tool params |
| [ml](packages/ml/) | Embeddings, vectors, RAG index, classical minibatch training |
| [tokensave](packages/tokensave/) | Context thrift, memory, train data |
| [retry](packages/retry/) | Exponential backoff with jitter and circuit breaker |
| [cache](packages/cache/) | In-memory LRU cache with TTL |
| [cron](packages/cron/) | Recurring task scheduler |
| [semver](packages/semver/) | Semver parsing, comparison, constraints |
| [color](packages/color/) | ANSI terminal colors for CLI tools |
| [warp](packages/warp/) | Validated NumPy-style arrays with native accelerator dispatch |
| [dataframe](packages/dataframe/) | Validated tabular data: null-aware stats, joins, rolling, CSV/JSON |
| [embed](packages/embed/) | Embeddings client + vector store |
| [experiment](packages/experiment/) | Experiment tracking: runs, params, metrics |
| [metrics](packages/metrics/) | ML metrics: accuracy, F1, precision, recall |

Browse all: [registry.weftproject.dev](https://registry.weftproject.dev)

### Packages

- **Auto-fetch:** `use auth` downloads from registry if not in vendor/ (disable with `WEFT_NO_AUTO_FETCH=1`)
- **Grouped imports:** `use { "auth" "config" "logger" }`
- **Git imports:** `use "github.com/user/repo"` auto-clones into vendor/
- Public registry with ed25519 signing, version immutability, namespace key ownership
- Capability sandboxing for third-party code in `vendor/`

### Tooling

| Command | What it does |
|---------|-------------|
| `weft run [--watch]` | Run scripts, auto-reload on change |
| `weft build [-o out]` | Standalone executable (no weft needed on target) |
| `weft check [--types] [--strict]` | Type-check (CI uses `--strict`) |
| `weft test [--race] [--mem] [--timeout N] [--coverage]` | Unit tests |
| `weft bench [--save f.json] [--compare base.json]` | Benchmarks with regression tracking |
| `weft lint` | Static analysis (unused imports, TODOs, line length) |
| `weft fmt [--check]` | Code formatter |
| `weft doc` | Generate API docs from `pub fn` |
| `weft info` | System report (memory, disk, uptime, network) |
| `weft debug [--dap]` | Debugger (CLI or DAP for VS Code) |
| `weft profile` | Execution profiler |
| `weft notebook [-o out.html]` | Run `.weft` as cells, output HTML |
| `weft mcp serve <file>` | Expose functions as MCP tools |
| `weft lsp` | Language server (completion, hover, rename, diagnostics) |
| `weft update` | Self-update with SHA-256 verification |
| `weft upgrade` | Upgrade installed packages |
| `weft outdated` | Check for newer package versions |
| `weft stdlib [pkg[.member]]` | Browse stdlib (live probe on zero-arg functions) |
| `weft doctor` | Check environment |
| `weft gen "task"` | LLM generates Weft code |

Plus: `weft new module|app|cli <name>`, `weft get`, `weft install`, `weft publish`, `weft train prepare|finetune|eval`.

### Reliability

- Bytecode validation, lex/parse/compile fuzz, VM concurrency stress
- Race detector + compat corpus in CI
- Glue benchmarks vs Python (output parity)
- Browser WASM: async execution, Fetch HTTP, bounded virtual filesystem (path traversal blocked, 16MB/file, 64MB total, 10k file cap)
- Slim build (`-tags slim`) for smaller binaries
- Go and package-level tests run in CI; 23 registry modules are cataloged and checked for freshness
- See [docs/STABILITY.md](docs/STABILITY.md)

### Security

- `weft update` verifies SHA-256 checksums before replacing binaries
- Install script (`install.sh`) verifies SHA-256 before installing
- All update/registry HTTP uses `netsafe.SafeHTTPClient` (SSRF protection)
- Package registry: mandatory ed25519 signatures, version immutability
- Capability sandboxing for third-party vendor packages
- Argon2id/PBKDF2 with parameter bounds enforced at the Go level
- WASM filesystem: path traversal returns errors, entry limits enforced
- See [SECURITY.md](SECURITY.md)

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
| [weftproject.dev/playground.html](https://weftproject.dev/playground.html) | Try Weft in the browser |
| [weftproject.dev/download.html](https://weftproject.dev/download.html) | Install guides (apt, dnf, brew, Docker) |
| [docs/TUTORIAL.md](docs/TUTORIAL.md) | Guided first hour |
| [docs/ECOSYSTEM.md](docs/ECOSYSTEM.md) | How all the pieces fit together |
| [docs/STDLIB.md](docs/STDLIB.md) | Stdlib reference (83 packages) |
| [docs/TELECOM.md](docs/TELECOM.md) | Telecom / IVA / FreeSWITCH / Asterisk |
| [docs/MCP.md](docs/MCP.md) | MCP integration |
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
