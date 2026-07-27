# Weft

A small scripting language for agent scripts, HTTP glue, and light ops work.

[![CI](https://github.com/loreste32/weft/actions/workflows/ci.yml/badge.svg)](https://github.com/loreste32/weft/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

![Wifty — Weft mascot](assets/brand/wifty.jpg)

Weft ships as one Go binary, uses its own syntax (`:=`, braces, `Result`/`?`), and installs packages into `vendor/`.

It is early (0.3.x). Useful for small tools; not a finished ecosystem.

| | |
|--|--|
| CLI | `weft` |
| Files | `.weft` |
| Version | 0.3.30 (git `main`, through 0.3.35) |
| Docs | [docs/README.md](docs/README.md) |
| Ops notes | [docs/SYSOPS.md](docs/SYSOPS.md) |

## Status

Works for short scripts. Rough around the edges.

- lex, parse, compile, stack VM  
- `Result` / `?`, closures, string enums, `match`  
- concurrent map/filter, spawn, channels  
- path/git packages + vendor + lock  
- LLM hooks (OpenAI-compat, Anthropic, Ollama, vLLM)  
- basic tooling: `check`, `test`, `fmt`, `bench`, thin LSP  

Still rough: gradual types, incomplete fmt/LSP edges, shallow stdlib in places, no public package registry.

## Quick start

```bash
go build -o weft ./cmd/weft
# or: make install   -> ~/.local/bin/weft

./weft doctor
./weft run examples/hello.weft
./weft check examples/fib.weft --types
```

### Syntax

Short and braced. Full notes: [docs/SYNTAX.md](docs/SYNTAX.md).

```weft
fn weather(city) { "clear in $city" }

fn main -> Result {
    mut n := 0
    reply := llm.ask("Weather in Paris?", [
        llm.tool("weather", weather),
    ])?
    say("got $reply")
    21 |> weather |> say
}
```

| Weft | Meaning |
|------|---------|
| `x := 1` | bind |
| `mut n := 0` | reassignable |
| `use pkg` | import package |
| `say "hi"` / `say(x)` | print |
| `"hi $name"` | string interpolation |
| `fn main { }` | no empty `()` |
| `expr?` | propagate error |
| `x \|> f` | pipeline |

## CLI

```text
weft                         # REPL
weft run <file.weft>
weft check <file|dir>... [--types]
weft test [path...]
weft fmt | bench | stdlib
weft doctor | version | lsp
weft gen "task" [-o out.weft]
weft new module|app|cli <name>
weft get | install | list
```

`weft help` has the full list.

## Examples

The [examples/](examples/) directory has runnable scripts covering HTTP, CLI tools, databases, data pipelines, concurrency, and more. A few highlights:

```bash
weft run examples/webapp.weft           # small HTTP server
weft run examples/cli_tool.weft -- --help
weft run examples/sysops_host.weft -- info
weft run examples/pipeline_etl.weft
weft run examples/db_sqlite.weft
```

## Documentation

| Doc | |
|-----|--|
| [docs/README.md](docs/README.md) | Index |
| [docs/TUTORIAL.md](docs/TUTORIAL.md) | Guided first hour |
| [docs/LANGUAGE.md](docs/LANGUAGE.md) | Language reference |
| [docs/COOKBOOK.md](docs/COOKBOOK.md) | Recipes |
| [docs/SYSOPS.md](docs/SYSOPS.md) | Ops / runbooks |
| [docs/STDLIB.md](docs/STDLIB.md) | Stdlib map |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Now / next / never |

## Why this exists

We needed a small language for agent tools and ops glue: one binary, explicit errors, simple packages. Weft is that experiment. It may fit your scripts; it may not. Try the examples and decide.

## Develop

```bash
make test          # go test ./...
make ci            # gofmt, vet, tests, example smoke
make build         # ./weft
make install       # ~/.local/bin/weft
```

Contributing: [CONTRIBUTING.md](CONTRIBUTING.md) | Security: [SECURITY.md](SECURITY.md)

## License

Apache-2.0
