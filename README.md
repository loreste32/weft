# Weft

A scripting language for agent tools, HTTP glue, and ops work. One Go binary, no runtime dependencies.

[![CI](https://github.com/loreste32/weft/actions/workflows/ci.yml/badge.svg)](https://github.com/loreste32/weft/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

![Wifty — Weft mascot](assets/brand/wifty.jpg)

## What it is

Weft is a small language with its own syntax, a stack VM, and a broad stdlib baked into the binary. You write `.weft` files, run them with `weft run`, and ship scripts without setting up environments or installing interpreters.

It handles errors with `Result` / `?` instead of exceptions, runs concurrent work without `async`/`await`, and talks to LLMs, databases, and HTTP services out of the box.

| | |
|--|--|
| Version | 0.3.31 (`main` branch) |
| Install | `go build -o weft ./cmd/weft` |
| Docs | [docs/README.md](docs/README.md) |
| Security | [SECURITY.md](SECURITY.md) |

## Quick start

```bash
go build -o weft ./cmd/weft
./weft doctor
./weft run examples/hello.weft
./weft run examples/todoapp/main.weft   # web app with SQLite
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

**Language:** lex, parse, compile, stack VM. Closures (capture by value), sum types with payloads (`enum Shape { Circle(r) }`), `match` with destructuring, `defer`, `Result`/`?`. Concurrent `map`/`filter`, `spawn`, channels — no `async`/`await`.

**Stdlib (in the binary):** `http`, `web`, `json`, `db` (SQLite/Postgres/MySQL with auto JSON/JSONB parsing), `fs`, `sh`, `cli`, `llm` (OpenAI/Anthropic/Ollama/vLLM), `csv`, `yaml`, `pcap`, `crypto`, `re`, `time`, and [many more](docs/STDLIB.md). Run `weft stdlib` to see them all.

**Tooling:**
- `weft check [--types]` — type checking
- `weft test [--coverage]` — run `fn test_*` in `*_test.weft` files
- `weft fmt [--check]` — code formatter (CI-friendly with `--check`)
- `weft run [--watch]` — run scripts, auto-reload on file changes
- `weft notebook` — run `.weft` as cells, output HTML
- `weft bench` — microbenchmarks
- `weft debug <file>` — interactive source-level debugger
- `weft profile <file>` — execution profiler
- `weft lsp` — Language Server (completion, hover, rename, diagnostics)

**Packages:**
- Path and git imports into `vendor/`
- Package registry with ed25519 signing (`weft publish`, `weft registry install`)
- Monorepo catalog: `weft packages list`
- Capability system for third-party package sandboxing

**Optional modules** (not in the binary — install via `weft get`):

| Module | What it does |
|--------|-------------|
| [warp](packages/warp/) | N-dimensional array math (pure Weft) |
| [mold](packages/mold/) | Structured LLM JSON, validation, tool params |
| [ml](packages/ml/) | Embeddings, vectors, RAG index |
| [tokensave](packages/tokensave/) | Context thrift, memory, train data |

## CLI

```text
weft                              REPL (history saved to ~/.weft/history)
weft run <file.weft> [--watch]    run a script (--watch reloads on change)
weft check <file|dir> [--types]   type check
weft test [path] [--coverage]     run tests
weft fmt [--check] <file|dir>     format (--check for CI)
weft notebook <file> [-o out.html]
weft bench | stdlib | doctor | version | lsp
weft debug <file.weft>            debugger
weft profile <file.weft>          profiler

weft new module|app|cli <name>    scaffold a project
weft get <name> <path|git>        add a dependency
weft install                      install from weft.json
weft publish [--key name]         sign and upload to registry
weft registry search|install|keygen|serve
```

## Examples

```bash
weft run examples/hello.weft
weft run examples/todoapp/main.weft        # full web app: SQLite + JSON API + HTML
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
| [docs/README.md](docs/README.md) | Full docs index |
| [docs/TUTORIAL.md](docs/TUTORIAL.md) | Guided first hour |
| [docs/COOKBOOK.md](docs/COOKBOOK.md) | Paste-ready recipes |
| [docs/STDLIB.md](docs/STDLIB.md) | Stdlib map |
| [docs/ECOSYSTEM.md](docs/ECOSYSTEM.md) | How pieces fit together |
| [docs/web.md](docs/web.md) | HTTP servers and HTMX |
| [docs/packages.md](docs/packages.md) | Package manager and registry |
| [SECURITY.md](SECURITY.md) | Threat model and capabilities |
| [docs/ROADMAP.md](docs/ROADMAP.md) | What's next |

## Why this exists

We wanted a small language for agent tools and ops glue — one binary, explicit error handling, simple packages, and LLM integration without a heavy runtime. Weft is that experiment.

It might work for your scripts. It might not. Try the examples and decide.

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
