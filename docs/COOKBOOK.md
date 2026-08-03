# Weft cookbook

Paste-ready recipes for common jobs. All examples target **0.4.x** Weft.

| Before you paste | |
|------------------|--|
| How pieces fit | [ECOSYSTEM.md](ECOSYSTEM.md) |
| Language rules | [LANGUAGE.md](LANGUAGE.md) |
| Stdlib map | [STDLIB.md](STDLIB.md) |
| First hour | [TUTORIAL.md](TUTORIAL.md) |

**Runnable offline samples** live in [`examples/cookbook/`](../examples/cookbook/) (`01_hello.weft` … `14_mold.weft`). Ops/A–B surface demo: [`examples/tier_ab.weft`](../examples/tier_ab.weft). This page keeps the wider set, including network/LLM/server sketches.

Run a snippet by saving it as `x.weft` and:

```bash
weft run x.weft
# with CLI args after --
weft run x.weft -- --help
# or from the repo:
weft run examples/cookbook/01_hello.weft
weft test examples/cookbook -q
```

---

## Contents

1. [Hello and style](#1-hello-and-style)  
2. [Files and text](#2-files-and-text)  
3. [JSON and config](#3-json-and-config)  
4. [HTTP client](#4-http-client)  
5. [HTTP server](#5-http-server)  
6. [Errors without try/catch](#6-errors-without-trycatch)  
7. [Enums and match](#7-enums-and-match)  
8. [Closures and handlers](#8-closures-and-handlers)  
9. [Lists, map, filter](#9-lists-map-filter)  
10. [Concurrency](#10-concurrency)  
11. [CLI tools](#11-cli-tools)  
12. [Environment and secrets](#12-environment-and-secrets)  
13. [LLM and agents](#13-llm-and-agents) (incl. [structured models / mold](#structured-models-mold-module))  
14. [Packages and multi-file](#14-packages-and-multi-file)  
15. [Testing](#15-testing)  
16. [Time, strings, regex](#16-time-strings-regex)  
17. [Data: CSV and SQLite](#17-data-csv-and-sqlite)  
18. [Logging and production-ish scripts](#18-logging-and-production-ish-scripts)  
19. [Packet captures (pcap)](#19-packet-captures-pcap)  
20. [Troubleshooting](#20-troubleshooting)  

---

## 1. Hello and style

### Minimal

```weft
fn main {
    say("hello, weft")
}
```

### Bindings, loops, pipeline

```weft
fn double(x) { x * 2 }

fn main {
    mut n := 0
    for x in [1, 2, 3] {
        n = n + x
    }
    name := "weft"
    say("sum=$n hi $name double=${double(21)}")
    21 |> double |> say
}
```

**Prefer:** `:=`, `mut`, `say`, last-expression return, `"hi $name"`.  
**Avoid:** empty ceremony, `try/catch` style, inventing foreign names.  
**Note:** `say` works as a statement and as a pipeline sink (`|> say`).

---

## 2. Files and text

### Read / write

```weft
fn main -> Result {
    text := fs.read("notes.txt")?
    fs.write("notes.bak", text)?
    fs.append("log.txt", "ran once\n")?
    say(len(text))
}
```

### Paths and listing

```weft
fn main -> Result {
    root := fs.cwd()
    path := fs.join(root, "data", "users.jsonl")
    say(fs.base(path), fs.ext(path))

    if fs.exists(path) {
        say(fs.size(path))
    }

    for p in fs.glob("**/*.weft") {
        say(p)
    }
}
```

### Line-oriented processing

```weft
fn main -> Result {
    lines := fs.lines("access.log")?
    mut n := 0
    for line in lines {
        if str.contains(line, "ERROR") {
            n = n + 1
        }
    }
    say("errors=$n")
}
```

### Temp file

```weft
fn main -> Result {
    p := fs.temp_file("weft-", ".txt")?
    fs.write(p, "scratch")?
    say(p)
    fs.remove(p)?
}
```

---

## 3. JSON and config

### Parse and field access

```weft
fn main -> Result {
    raw := `{"city":"Paris","temp":21,"ok":true}`
    data := json.parse(raw)?
    say(data.city, data.temp)
    say(json.pretty(data))
}
```

### Read JSON file, write pretty

```weft
fn main -> Result {
    data := json.parse(fs.read("config.json")?)?
    data.version = "0.4.10"
    fs.write("config.json", json.pretty(data) + "\n")?
}
```

### YAML / TOML / INI

```weft
fn main -> Result {
    y := yaml.parse(fs.read("app.yaml")?)?
    t := toml.parse(fs.read("app.toml")?)?
    i := ini.parse(fs.read("app.ini")?)?
    say(y, t, i)
}
```

### Nested get with default

```weft
fn main -> Result {
    data := json.parse(`{"user":{"name":"Ada"}}`)?
    // Prefer field access when keys are plain identifiers
    say(data.user.name)
    // Dynamic path + default when missing
    say(json.get(data, "user.name")?)
    say(json.get(data, "user.missing", "anon")?)
}
```

---

## 4. HTTP client

### GET body

```weft
fn main -> Result {
    body := http.get("https://httpbin.org/get")?
    say(str.slice(body, 0, 200))
}
```

### JSON API

```weft
fn main -> Result {
    // one-shot GET + parse
    data := http.get_json("https://httpbin.org/json")?
    say(json.pretty(data))

    // or manual:
    // r := http.get(url)?
    // data := json.parse(r.body)?
}
```

### POST JSON

```weft
fn main -> Result {
    payload := json.stringify({"hello": "weft"})
    body := http.post("https://httpbin.org/post", payload)?
    say(str.slice(body, 0, 200))
}
```

### Fan-out several URLs

```weft
fn fetch_one(url) -> Result {
    body := http.get(url)?
    len(body)
}

fn main -> Result {
    urls := [
        "https://example.com",
        "https://httpbin.org/get",
    ]
    sizes := map(urls, fetch_one)?
    say(sizes)
}
```

---

## 5. HTTP server

### Tiny server

```weft
fn main {
    say("http://127.0.0.1:8080")
    http.serve(":8080", fn(req) {
        if req.path == "/health" {
            return http.json({"ok": true})
        }
        return http.text(200, "hello from weft")
    })
}
```

### Route table with match

```weft
fn main {
    http.serve(":8080", fn(req) {
        match req.path {
            "/health" { http.json({"ok": true}) }
            "/version" { http.json({"version": "0.4.10"}) }
            _ { http.text(404, "not found") }
        }
    })
}
```

For static files, SSE, multi-page apps: [web.md](web.md) and `examples/webapp.weft`.

---

## 6. Errors without try/catch

### Propagate with `?`

```weft
fn load(path) -> Result {
    ensure(path != "", "path required")?
    fs.read(path).context("load")?
}

fn main -> Result {
    text := load("README.md")?
    say(len(text))
}
```

### Defaults and inspection

```weft
fn main {
    n := int.parse("nope").unwrap_or(0)
    say("n=$n")

    r := fs.read("missing.txt")
    if r.is_err {
        say("kind=${r.err.kind} msg=${r.err.message}")
    } else {
        say(r.value)
    }
}
```

### Bail and ensure

```weft
fn check_user(name) -> Result {
    ensure(name != "", "name required", "user")?
    if name == "root" {
        return bail("not allowed", "user")
    }
    Ok(name)
}
```

Full guide: [ERRORS.md](ERRORS.md).

---

## 7. Enums and match

### Status machine

```weft
enum Status { Pending, Running, Done, Failed }

fn label(s) {
    match s {
        Status.Pending { "…" }
        Status.Running { "→" }
        Status.Done { "✓" }
        Status.Failed { "✗" }
        _ { "?" }
    }
}

fn main {
    say(label(Status.Running))
    // variants are strings
    say(Status.Done == "Done")
}
```

### Event handler

```weft
enum Kind { Text, Done, Error }

fn handle(kind, text) {
    match kind {
        Kind.Text { text }
        Kind.Done { "[done]" }
        Kind.Error { "err: $text" }
        _ { "" }
    }
}

fn main {
    say(handle(Kind.Text, "hi"))
    say(handle(Kind.Done, ""))
}
```

---

## 8. Closures and handlers

### Capture label for map

```weft
fn main {
    prefix := "item"
    out := map([1, 2, 3], fn(n) { "$prefix-$n" })
    say(out)
}
```

### By-value capture (snapshot)

```weft
fn main {
    mut n := 1
    g := fn() { n }
    n = 99
    say(g())   // 1 — frozen when g was created
}
```

### HTTP handler as closure

```weft
fn main {
    app := "demo"
    http.serve(":8080", fn(req) {
        http.json({"app": app, "path": req.path})
    })
}
```

---

## 9. Lists, map, filter

### Concurrent map (default)

```weft
fn work(x) { x * x }

fn main {
    say(map([1, 2, 3, 4], work))
    // cap workers:
    say(map([1, 2, 3, 4], work, 2))
}
```

### Filter

```weft
fn main {
    xs := [1, 2, 3, 4, 5, 6]
    evens := filter(xs, fn(n) { n % 2 == 0 })
    say(evens)
}
```

### Sequential when order of side effects matters

```weft
fn main -> Result {
    paths := ["a.txt", "b.txt"]
    // write in order:
    seq_map(paths, fn(p) {
        fs.append("out.log", p + "\n")?
    })?
}
```

### Build a list

```weft
fn main {
    mut xs := []
    for i in range(5) {
        push(xs, i)
    }
    say(xs)
}
```

---

## 10. Concurrency

### parallel / gather

```weft
fn main -> Result {
    results := parallel([
        fn() { 1 + 1 },
        fn() { 2 * 3 },
        fn() { 10 - 1 },
    ])?
    say(results)
}
```

### race and timeout

```weft
fn slow() {
    time.sleep(2)
    "slow"
}

fn fast() { "fast" }

fn main -> Result {
    say(race([fn() { slow() }, fn() { fast() }])?)
    say(timeout(1, fn() { slow() }))  // may Err on deadline
}
```

### Channels

```weft
fn producer(ch) {
    send(ch, 1)
    send(ch, 2)
    close(ch)
}

fn main -> Result {
    ch := channel(2)
    spawn(producer, ch)
    a := recv(ch)?
    b := recv(ch)?
    say(a, b)
}
```

### Task group

```weft
fn main -> Result {
    g := group()
    g.go(fn() { 10 })
    g.go(fn() { 20 })
    say(g.wait()?)
}
```

More: [CONCURRENCY.md](CONCURRENCY.md), `examples/channels.weft`, `examples/parallel.weft`.

---

## 11. CLI tools

### Flags and subcommands

```weft
// weft run tool.weft -- greet Ada
// weft run tool.weft -- --help

fn main -> Result {
    p := cli.parse({
        "about": "demo — sample CLI",
        "flags": {
            "verbose": {"short": "v", "bool": true, "help": "verbose"},
            "env": {"short": "e", "default": "dev", "help": "environment"},
        },
    })?

    if p.help {
        say(p.usage)
        cli.exit(0)
    }

    args := p.args
    if len(args) == 0 {
        io.eprintln("missing command")
        say(p.usage)
        cli.exit(2)
    }

    cmd := args[0]
    if p.verbose {
        io.eprintln("env=${p.env} cmd=$cmd")
    }

    if cmd == "greet" {
        mut name := "world"
        if len(args) > 1 {
            name = args[1]
        }
        say("hello, $name (${p.env})")
        return Ok(unit)
    }

    io.eprintln("unknown command: $cmd")
    cli.exit(2)
}
```

See [cli.md](cli.md) and `examples/cli_tool.weft`. Scaffold: `weft new cli mytool`.

---

## 12. Environment and secrets

```weft
fn main -> Result {
    home := env.get("HOME")
    if home == null {
        say("HOME unset")
    } else {
        say("home=$home")
    }

    // fail if missing
    token := env.require("API_TOKEN")?

    // never print the secret itself
    say("token length=${len(token)}")
}
```

Production notes: [PRODUCTION.md](PRODUCTION.md).

---

## 13. LLM and agents

Runnable offline sample: `examples/cookbook/13_agent.weft` (uses the eval mock when no API key is set).

### One-shot chat

```weft
fn main -> Result {
    reply := llm.chat("Explain Result and ? in one short paragraph")?
    say(reply)

    // system + model opts
    say(llm.chat("hi", {"system": "Be terse."})?)

    // multi-turn messages
    say(llm.chat([
        {"role": "system", "content": "You write short answers."},
        {"role": "user", "content": "What is Weft?"},
    ])?)
}
```

### Tools / agent

```weft
fn add(a, b) { a + b }
fn note(msg) { "note:$msg" }

fn main -> Result {
    // tools only
    reply := llm.ask("What is 2+3? Use tools.", [
        llm.tool("add", add, "add two numbers a and b"),
        llm.tool("note", note, "record a short note"),
    ])?
    say(reply)

    // tools + opts (system, max_steps, model, …)
    reply2 := llm.ask("2+3?", [
        llm.tool("add", add, "add two numbers"),
    ], {
        "system": "Prefer tools for arithmetic.",
        "max_steps": 8,
    })?
    say(reply2)
}
```

### Streaming

```weft
fn main -> Result {
    // token events
    for e in llm.stream("one word: hi")? {
        if e.kind == "text" { print(e.text) }
    }
    say("")

    // or collect into one string
    say(llm.stream_text("one word: hi")?)
}
```

### Local Ollama

```bash
export WEFT_PROVIDER=ollama
export OLLAMA_MODEL=llama3.2
weft ollama list
weft run agent.weft
```

```weft
fn main -> Result {
    say(ollama.chat("hello from weft")?)
}
```

### Generate Weft with the model

```bash
weft gen "sum 1..5 and print it" -o sum.weft --run
```

Providers: [LLM_PROVIDERS.md](LLM_PROVIDERS.md) · local: [LLM_LOCAL.md](LLM_LOCAL.md).  
Examples: `examples/realworld/tool_agent.weft`, `examples/ollama_chat.weft`.

### Structured models (`mold` module)

Optional module in the agent stack (with `llm` / `tokensave` / `ml`): [ECOSYSTEM.md](ECOSYSTEM.md) · full guide [MOLD.md](MOLD.md).

```bash
weft get mold
```

```weft
use mold

fn main -> Result {
    Args := mold.model({
        "city": mold.str({"desc": "city name"})?,
        "units": "str?",
    })?

    // validate tool args the model returned
    a := mold.parse(Args, "{\"city\":\"Paris\"}")?
    say(a["city"])

    // fenced model output
    out := mold.extract(Args, "```json\n{\"city\":\"Oslo\"}\n```")?

    // describe for providers
    say(json.stringify(mold.tool_params(Args)))
}
```

Runnable: `examples/cookbook/14_mold.weft`.

---

## 14. Packages and multi-file

### Path import next to the script

```text
app/
  main.weft
  lib/
    math.weft
```

```weft
// main.weft
use "./lib/math.weft" as math

fn main {
    say(math.add(2, 3))
}
```

```weft
// lib/math.weft
pub fn add(a, b) { a + b }
```

### Install a path dependency

```bash
weft init myapp
cd myapp
weft get greeter ../packages/greeter
weft install
```

```weft
use greeter

fn main {
    say(greeter.hello("weft"))
}
```

### Author a module

```bash
weft new module greeter
cd greeter
# edit lib.weft — pub fn …
weft mod check
```

### Catalog (monorepo)

```bash
weft packages list
weft get ml
```

Optional remote discovery: `WEFT_CATALOG_URL=https://…/index.json`.

Details: [packages.md](packages.md), [modules.md](modules.md).

---

## 15. Testing

### File layout

```text
math_test.weft   # or test_math.weft
```

```weft
fn test_add {
    test.eq(1 + 1, 2)
    test.is_true(len([1, 2]) == 2)
}

fn test_parse {
    n := int.parse("3").unwrap_or(0)
    test.eq(n, 3)
}
```

```bash
weft test
weft test -run add
weft test -q
```

### Testing a local library

```weft
use "./lib.weft" as lib

fn test_hello {
    test.eq(lib.hello("x"), "hello, x")
}
```

The runner sets project/package directory so relative `use` works.

See [TESTING.md](TESTING.md).

---

## 16. Time, strings, regex

```weft
fn main {
    now := time.now()
    say(time.iso(now))
    say(time.format(now, "2006-01-02"))

    s := "  Hello, Weft  "
    say(str.trim(s))
    say(str.lower(s))
    say(str.split("a,b,c", ","))

    m := re.find(`[0-9]+`, "ab42cd")
    say(m)
}
```

---

## 17. Data: CSV and SQLite

### CSV report sketch

```weft
fn main -> Result {
    rows := csv.read("metrics.csv")?
    say(len(rows))
    // table helpers: weft stdlib table
}
```

### SQLite

```weft
fn main -> Result {
    c := db.open("sqlite:app.db")?
    c.exec("CREATE TABLE IF NOT EXISTS items(id INTEGER PRIMARY KEY, name TEXT, qty INTEGER)")?
    c.exec("INSERT INTO items(name, qty) VALUES (?, ?)", ["widgets", 10])?
    rows := c.query("SELECT id, name, qty FROM items ORDER BY name")?
    for row in rows {
        say(row.id, row.name, row.qty)
    }
    c.close()?
}
```

Runnable samples: `examples/csv_report.weft`, `examples/db_sqlite.weft`, `examples/data_pipeline.weft`. Guide: [data.md](data.md).

---

## 18. Logging and production-ish scripts

```weft
fn main -> Result {
    log.info("starting worker")
    // do work with ?
    path := env.get("INPUT")
    if path == null {
        return bail("INPUT required", "config")
    }
    raw := fs.read(path).context("read input")?
    log.info("read bytes=${len(raw)}")
    Ok(unit)
}
```

Tips:

- Prefer `?` and `.context("…")` over silent ignores  
- Do not print secrets  
- Cap concurrency with `WEFT_WORKERS` under load  
- Commit `weft.lock` (and optionally `vendor/`) for reproducible deploys  

See [PRODUCTION.md](PRODUCTION.md).

---

## 19. Packet captures (pcap)

### Build a SYN packet and write a pcap file

```weft
use pcap

fn main -> Result {
    pkt := pcap.ethernet({
        "dst": "ff:ff:ff:ff:ff:ff",
        "src": "00:11:22:33:44:55",
        "payload": pcap.ipv4({
            "src": "192.168.1.100",
            "dst": "93.184.216.34",
            "payload": pcap.tcp({
                "src_port": 49152,
                "dst_port": 443,
                "flags": "SYN",
            }),
        }),
    })
    pcap.write("handshake.pcap", [pkt])?
    say("wrote handshake.pcap — open in Wireshark")
}
```

### DNS-style UDP packet

```weft
use pcap

fn main -> Result {
    pkt := pcap.ethernet({
        "dst": "aa:bb:cc:dd:ee:ff",
        "src": "00:11:22:33:44:55",
        "payload": pcap.ipv4({
            "src": "10.0.0.1",
            "dst": "8.8.8.8",
            "proto": 17,
            "payload": pcap.udp({
                "src_port": 12345,
                "dst_port": 53,
                "payload": "fake DNS query",
            }),
        }),
    })
    pcap.write("dns.pcap", [pkt])?
}
```

### Read and inspect a pcap

```weft
use pcap

fn main -> Result {
    pkts := pcap.read("capture.pcap")?
    say(len(pkts), "packets")
    for p in pkts {
        say("ts:", p.ts, "len:", p.len)
    }
}
```

### Raw bytes from hex

```weft
use pcap

fn main -> Result {
    raw := pcap.hex("ffffffffffff 001122334455 0800 4500002800000000 4006 0000 c0a80101 0a000001")
    pcap.write("raw.pcap", [raw])?
}
```

---

## 20. Troubleshooting

| Symptom | What to try |
|---------|-------------|
| `no main function` | Add `fn main { … }` or run a library via tests/import |
| `?` outside Result | Mark `fn … -> Result` or handle with `if r.ok` |
| Import not found | `weft install`; check `vendor/`; path `use "./x.weft"` |
| Closure sees old value | By design — capture is by value; re-create the fn after updates |
| map order / races | Outputs keep order; don’t mutate shared maps across tasks |
| LLM fails offline | Set mock/`WEFT_PROVIDER`, or use `weft train eval` without live |
| Unknown stdlib name | `weft stdlib` / `weft stdlib pkg` — don’t invent APIs |
| Types look wrong | `weft check file.weft --types` |

```bash
weft doctor
weft check .
weft test -q
weft version
```

---

## Where next

| Want… | Go to |
|-------|--------|
| First hour | [TUTORIAL.md](TUTORIAL.md) |
| Full language rules | [LANGUAGE.md](LANGUAGE.md) |
| Stdlib map | [STDLIB.md](STDLIB.md) |
| Offline runnable recipes | [examples/cookbook/](../examples/cookbook/) |
| Concurrency depth | [CONCURRENCY.md](CONCURRENCY.md) |
| Agents / providers | [LLM_PROVIDERS.md](LLM_PROVIDERS.md) |
| Repo examples | `examples/` |
| Honest limits | [ROADMAP.md](ROADMAP.md) |
