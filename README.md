# Weft

Weave agents into code.

[![CI](https://github.com/loreste32/weft/actions/workflows/ci.yml/badge.svg)](https://github.com/loreste32/weft/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

![Wifty — Weft mascot](assets/brand/wifty.jpg)

Weft is a small scripting language for agent scripts, HTTP glue, and light ops work. It ships as one Go binary, uses its own syntax (`:=`, braces, `Result`/`?`), and installs packages into `vendor/`.

It is early (0.3.x). Useful for small tools; not a finished ecosystem.

| | |
|--|--|
| CLI | `weft` |
| Files | `.weft` |
| Version | 0.3.27 (git `main`, through 0.3.35) |
| Docs | [docs/README.md](docs/README.md) |
| Ops notes | [docs/SYSOPS.md](docs/SYSOPS.md) |

## Where we are (0.3.27)

Works well enough for short agent scripts, HTTP glue, workers, and small CLIs:

- lex → parse → compile → stack VM  
- `Result` / `?`, closures (capture by value), string enums, `match`  
- concurrent map/filter, spawn, channels (no `async`/`await`)  
- path/git packages + vendor + lock; small monorepo catalog  
- LLM hooks (OpenAI-compat, Anthropic, Ollama, vLLM)  
- day-to-day tools: `check`, `test`, `fmt`, `bench`, thin LSP  

Still rough: gradual types, incomplete fmt/LSP edges, shallow stdlib in places, no public package registry.

| Area | Notes |
|------|--------|
| Language | stack VM; closures; `enum` / `match` |
| Syntax | `:=`, `mut`, `use`, `say`, `$interp`, `?` — [SYNTAX](docs/SYNTAX.md) |
| Check / test | `weft check`, `weft test` — [TESTING](docs/TESTING.md) |
| Errors | `Result` + `?` — [ERRORS](docs/ERRORS.md) |
| Tooling | fmt, bench, stdlib list, LSP — [TOOLING](docs/TOOLING.md) |
| Stdlib | I/O, http, llm, db, helpers — `weft stdlib` |
| Concurrency | map/filter fan-out, spawn, channels — [CONCURRENCY](docs/CONCURRENCY.md) |
| Packages | path/git + vendor + lock |
| Ops | `sh` / `fs` / `cli` / `env` — [SYSOPS](docs/SYSOPS.md) |

## Where we hope to go

Through **0.3.35**: clearer errors, more reliable check/test/fmt, stdlib only where scripts hurt, slightly better editing, honest docs. No rush to 1.0.

Out of scope for core: heavy scientific compute, in-process training stacks, `async`/`await`. A public registry only if path/git becomes painful.

More detail: [docs/ROADMAP.md](docs/ROADMAP.md) · [docs/VERSIONING.md](docs/VERSIONING.md) · [docs/PRODUCTION.md](docs/PRODUCTION.md)

## Quick start

```bash
# build
go build -o weft ./cmd/weft
# or: make install   → ~/.local/bin/weft

./weft doctor
./weft run examples/hello.weft
./weft run examples/sysops_host.weft -- info
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

### Fine-tune (optional, private by default)

If you train, data can stay on machines you control (local LoRA, air-gap pack, or explicit cloud upload).

```bash
# Merge confidential domain rows + train locally (nothing uploaded)
weft train prepare -o weft-sft --expand --from /secure/domain.jsonl
weft train finetune --private --preset qwen-7b

# Air-gapped GPU box kit
weft train offline -o weft-airgap --expand
weft train presets

# Cloud only with explicit consent (uploads chat.jsonl)
export OPENAI_API_KEY=sk-...
weft train finetune --backend openai --allow-upload --wait

# English → Weft (any OpenAI-compatible base, including private)
weft gen "sum 1..5 and print it" -o sum.weft --run
```

Docs: [`docs/FINETUNE.md`](docs/FINETUNE.md) · [`llm-pack/README.md`](llm-pack/README.md)

### Packages & modules

Libraries are plain `.weft` folders with a `weft.json`. They install into `vendor/` next to your app.

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

### CLI / sysops / data processing

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
weft run examples/sysops_host.weft -- check -r git,sh
weft run examples/data_pipeline.weft -- -i examples/data/users.jsonl -f ok -v
```

Docs: [`docs/cli.md`](docs/cli.md) · [`docs/SYSOPS.md`](docs/SYSOPS.md)

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
| [docs/SYSOPS.md](docs/SYSOPS.md) | Ops / runbooks / host checks |
| [examples/cookbook/](examples/cookbook/) | Runnable offline recipes |
| [docs/STDLIB.md](docs/STDLIB.md) | Stdlib package map |
| [docs/SYNTAX.md](docs/SYNTAX.md) | Short cheatsheet |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Now / next / never |

## Why this exists

We needed a small language for agent tools and ops glue: one binary, explicit errors, simple packages. Weft is that experiment. It may fit your scripts; it may not. Try the examples and decide.

Rough tradeoffs we chose:

- small syntax over a large language  
- `Result` / `?` over exceptions  
- concurrent helpers without `async`/`await`  
- stdlib in the binary + `vendor/` packages  

Principles and design notes: [docs/PRINCIPLES.md](docs/PRINCIPLES.md) · [docs/weft-design.md](docs/weft-design.md) · [docs/BRAND.md](docs/BRAND.md).

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
