# Weft

Weave agents into code.

[![CI](https://github.com/loreste32/weft/actions/workflows/ci.yml/badge.svg)](https://github.com/loreste32/weft/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

![Wifty — Weft mascot](assets/brand/wifty.jpg)

Weft is a small scripting language aimed at LLM agents, HTTP glue, and ops scripts. One Go binary, no Python on the core path, packages vendored like `go mod`. Syntax is its own (braces, `:=`, `?` for errors) — not Python with braces.

| | |
|--|--|
| CLI | `weft` |
| Files | `.weft` |
| Version | 0.3.27 (git `main`, patch line through 0.3.35) |
| Mascot | Wifty |
| Brand | [docs/BRAND.md](docs/BRAND.md) |

## Where we are (0.3.27)

Weft is **usable** for agent scripts, HTTP glue, and small ops tools. It is **not** a finished ecosystem or a CPython stand-in.

You get a single binary, a real language loop (parse → VM), `Result`/`?` errors, concurrent map/filter without `async`/`await`, **closures that capture outer locals by value**, string enums + richer `match`, vendored packages, LLM providers (OpenAI-compat, Anthropic, Ollama, vLLM), and day-to-day commands (`check`, `test`, `fmt`, `bench`). Optional private fine-tune stays on your GPU; cloud upload is opt-in.

Still rough: types are gradual, LSP/fmt are practical but not gofmt/IDE-grade, stdlib is broad-and-shallow, and there is no public package registry yet (path/git + monorepo catalog; optional `WEFT_CATALOG_URL` and lite `^`/`~`/`>=` constraints).

| Area | Notes |
|------|--------|
| Language | lex → parse → compile → stack VM; closures; `enum` / `match` |
| Syntax | `:=`, `mut`, `use`, `say`, `$interp`, `?` — [SYNTAX](docs/SYNTAX.md) |
| Check / test | `weft check`, `weft test` — [TESTING](docs/TESTING.md) |
| Errors | `Result` + `?` — [ERRORS](docs/ERRORS.md) |
| Tooling | fmt, bench, stdlib list, thin LSP — [TOOLING](docs/TOOLING.md) |
| Stdlib | I/O, http, llm, db, text/math helpers — `weft stdlib` |
| Concurrency | fan-out map/filter, spawn, channels — [CONCURRENCY](docs/CONCURRENCY.md) |
| Packages | path/git + vendor + lock; catalog; lite semver ranges |

## Where we hope to go

Through **0.3.35** we want the boring parts to feel ordinary: clearer errors, sturdier check/test/fmt, stdlib only where scripts hurt, better modules, a less painful editor story, and honest docs.

We are **not** aiming at NumPy, in-process training, or `async`/`await`. A public registry or fancier packaging only if path/git becomes a real tax.

**Documentation:** **[docs/README.md](docs/README.md)** (index) · **[docs/TUTORIAL.md](docs/TUTORIAL.md)** (first hour) · **[docs/LANGUAGE.md](docs/LANGUAGE.md)** (language reference) · **[docs/COOKBOOK.md](docs/COOKBOOK.md)** (recipes) · **[docs/STDLIB.md](docs/STDLIB.md)** (stdlib map) · **[examples/cookbook/](examples/cookbook/)** (runnable recipes).

Longer write-up: **[docs/ROADMAP.md](docs/ROADMAP.md)** (now / next / never). Version policy: [docs/VERSIONING.md](docs/VERSIONING.md). Ops notes: [docs/PRODUCTION.md](docs/PRODUCTION.md).

## Quick start

```bash
# build
go build -o weft ./cmd/weft
# or: make install

./weft doctor
./weft run examples/hello.weft
./weft run examples/weft_style.weft
./weft check examples/fib.weft --types
```

### Syntax

Short and braced. Full notes: [docs/SYNTAX.md](docs/SYNTAX.md).

```weft
fn weather(city) { "clear in $city" }   // last expr returns

fn main -> Result {
    mut n := 0
    reply := llm.ask("Weather in Paris?", [
        llm.tool("weather", weather),
    ])?
    say("got $reply")
    21 |> weather |> say                // pipeline
}
```

| Weft | Meaning |
|------|---------|
| `x := 1` | bind (prefer over `let`) |
| `mut n := 0` | reassignable |
| `use pkg` | import package |
| `say "hi"` / `say(x)` | print |
| `"hi $name"` / `"${a+b}"` | string insert (JSON-safe) |
| `fn main { }` | no empty `()` |
| `expr?` | propagate error |
| `x \|> f` | pipeline |
| `weft check --types` | type inference |

## CLI (common)

```text
weft                         # REPL
weft run <file.weft>
weft check <file|dir>… [--types]
weft test [path…]             # *_test.weft · fn test_*
weft fmt | bench | stdlib
weft doctor | version | lsp

weft train …                 # prepare / eval / private finetune / offline pack
weft gen "task" [-o out.weft]
weft new module|app|cli <name>
weft init | get | install | list | packages list
```

`weft help` has the full list.

### Ollama & vLLM (local)

```bash
export WEFT_PROVIDER=ollama
export OLLAMA_MODEL=llama3.2
weft ollama list
weft ollama chat "hello"
weft gen "print hello weft" -o hi.weft

export WEFT_PROVIDER=vllm
export VLLM_BASE_URL=http://127.0.0.1:8000/v1
weft vllm health && weft vllm list
```

```weft
reply := ollama.chat("Explain Result in one line")?
reply := vllm.chat({"model": "…", "prompt": "hi"})?
// or unified: WEFT_PROVIDER=ollama → llm.chat / weft gen
```

Docs: [`docs/LLM_LOCAL.md`](docs/LLM_LOCAL.md) · ML module: [`docs/ML.md`](docs/ML.md) · roadmap: [`docs/ROADMAP.md`](docs/ROADMAP.md)

### Fine-tune (private by default)

Training data can stay on machines you control — open frontier LoRA, air-gap packs, or a VPC fine-tune API.

```bash
# Merge confidential domain rows + train locally (nothing uploaded)
weft train prepare -o weft-sft --expand --from /secure/domain.jsonl
weft train finetune --private --preset qwen-7b

# Air-gapped GPU box kit
weft train offline -o weft-airgap --expand
weft train presets

# Public OpenAI only with explicit consent (uploads chat.jsonl)
export OPENAI_API_KEY=sk-...
weft train finetune --backend openai --allow-upload --wait

# English → Weft (points at any OpenAI-compatible base, incl. private)
weft gen "sum 1..5 and print it" -o sum.weft --run
```

Docs: [`docs/FINETUNE.md`](docs/FINETUNE.md) · [`llm-pack/README.md`](llm-pack/README.md)

### Packages & modules

Libraries are plain `.weft` folders with a `weft.json`. They install into `vendor/` (no venv).
```bash
# Author a module
weft new module greeter
cd greeter && weft mod check

# Consume it (path or git); install flattens transitive deps into vendor/
weft get greeter ../greeter
weft get greeter github.com/you/greeter@v0.1.0
weft install
```

```weft
use greeter
fn main { say(greeter.hello("weft")) }
```

Multi-file + deps demo: [`examples/modules/`](examples/modules/) · docs: [`docs/modules.md`](docs/modules.md) · [`docs/packages.md`](docs/packages.md).

### Web

Small HTTP apps, WebSockets, and a simple WebRTC signaling helper:
```weft
fn main {
    app := web.app()
    app.get("/api/hi", fn(req) { web.json({"ok": true}) })
    app.get("/users/:id", fn(req) { web.json({"id": req.params["id"]}) })
    app.ws("/ws/echo", fn(conn) {
        while true { conn.send(conn.recv()?)? }
    })
    hub := webrtc.hub()
    hub.attach(app, "/ws/signal")?
    app.listen(":8080")
}
```

```bash
weft run examples/webapp.weft       # API + static + chat + WebRTC
weft run examples/webrtc_call.weft  # open two browser tabs
```

Docs: [`docs/web.md`](docs/web.md)

### Data visualization

```weft
fn main -> Result {
    c := viz.bar({"a": 10, "b": 20}, {"title": "Sales"})
    viz.save("sales.svg", c)?
    viz.save("dash.html", viz.page("Report", [c, viz.line([1,3,2,5])]))?
    say(viz.spark([1, 3, 2, 5, 4]))
}
```

```bash
weft run examples/viz_charts.weft
weft run examples/viz_dashboard.weft   # live charts on :8080
```

Docs: [`docs/viz.md`](docs/viz.md)

### CLI / devops / data processing

```weft
fn main -> Result {
    p := cli.parse({
        "about": "myctl",
        "flags": {
            "env": {"short": "e", "default": "dev"},
            "verbose": {"short": "v", "bool": true},
        },
    })?
    if p.help { say(p.usage); cli.exit(0) }

    r := sh.run("git", ["status", "-sb"])?
    lines := fs.lines("data.jsonl")?
    say(len(lines), r.ok)
}
```

```bash
weft run examples/cli_tool.weft -- --help
weft run examples/data_pipeline.weft -- -i examples/data/users.jsonl -f ok -v
```

Docs: [`docs/cli.md`](docs/cli.md)

### Databases & messaging

```weft
c := db.open("sqlite:./app.db")?
c.exec("INSERT INTO users(name) VALUES (?)", ["Ada"])?
rows := c.query("SELECT * FROM users")?

r := redis.connect("redis://127.0.0.1:6379/0")?
r.set("k", "v", 60)?

nc := nats.connect("nats://127.0.0.1:4222")?
nc.publish("jobs", payload)?

ch := amqp.connect("amqp://guest:guest@localhost:5672/")?
ch.publish("", "queue", body)?

m := mongo.connect("mongodb://localhost:27017")?
m.collection("app", "users").insert({"name": "Ada"})?

res := graphql.query(url, `query { hello }`, {})?
```

```bash
weft run examples/db_sqlite.weft
weft run examples/data_stack.weft -- all-urls
```

Docs: [`docs/data.md`](docs/data.md)

### Pipelines / ETL

```weft
fn score(row) { /* pure */ row }

rows := jsonl.read("users.jsonl")?
active := table.where_truthy(rows, "ok")
slim := table.project(active, ["name", "role"])
out := map(slim, score) |> /* or */ table.sort(slim, "name", false)
jsonl.write("out.jsonl", out)?
// concurrent by default: map(rows, enrich) or map(rows, enrich, 8)
```

```bash
weft run examples/pipeline_etl.weft
weft run examples/pipeline_parallel.weft
```

Docs: [`docs/PIPELINES.md`](docs/PIPELINES.md)

### Concurrency

```weft
// fan-out
let r = parallel([fn() { work(1) }, fn() { work(2) }])?

// task group
let g = group()
g.go(fn() { 1 })
g.wait()?

// channels + closures (capture is by value)
fn prod(ch) { send(ch, 1); close(ch) }
fn main() -> Result {
    let ch = channel(1)
    spawn(prod, ch)
    println(recv(ch)?)
}
```

## Documentation map

| Doc | |
|-----|--|
| [docs/README.md](docs/README.md) | Index of all docs |
| [docs/TUTORIAL.md](docs/TUTORIAL.md) | Guided first hour |
| [docs/LANGUAGE.md](docs/LANGUAGE.md) | End-to-end language reference |
| [docs/COOKBOOK.md](docs/COOKBOOK.md) | Recipes (files, HTTP, agents, CLI, …) |
| [examples/cookbook/](examples/cookbook/) | Runnable offline recipes |
| [docs/STDLIB.md](docs/STDLIB.md) | Stdlib package map |
| [docs/SYNTAX.md](docs/SYNTAX.md) | Short cheatsheet |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Now / next / never |

## Why Weft

| Python pain | Weft |
|-------------|------|
| Slow cold start | Single binary, no site-packages |
| GIL | Goroutine tasks, no shared mutable heap |
| async coloring | `spawn` / `TaskGroup` (no function colors) |
| Packaging hell | Core stdlib in the binary + vendor packages |
| Indentation-only bugs | Required braces |
| Schema/tools bolted on | `TypeInfo`, tools, Agent in stdlib |

## Design principles

1. **LLM-native, not LLM-adjacent** — messages, tools, structured output, streaming are language concerns.
2. **Honest performance** — advertise startup and parallel I/O; measure compute before claiming wins.
3. **Small language, rich builtins** — keep syntax tight; put power in Go-registered stdlib.
4. **Predictable failures** — `Result[T]` + `?`, not exception soup.
5. **Ship slices** — MVP-0 is `hello`; MVP-1 is sequential agent-with-tools.

Full architecture: [`docs/weft-design.md`](docs/weft-design.md) · Brand: [`docs/BRAND.md`](docs/BRAND.md) · Principles: [`docs/PRINCIPLES.md`](docs/PRINCIPLES.md) · Docs hub: [`docs/README.md`](docs/README.md).

## Develop

```bash
make test          # go test ./...
make ci            # gofmt, vet, tests, example smoke, train pack
make build         # ./weft
make install       # ~/.local/bin/weft
```

```bash
bash scripts/ci.sh   # same gate as GitHub Actions
```

Contributing: [CONTRIBUTING.md](CONTRIBUTING.md) · Security: [SECURITY.md](SECURITY.md).

## License

Apache-2.0
