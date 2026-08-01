# Tutorial: your first hour with Weft

A guided path from install to a small real script. Work in order; each step is short. For paste-ready recipes later, use the [cookbook](COOKBOOK.md) and [examples/cookbook/](../examples/cookbook/).

**Time:** about 60 minutes · **Needs:** Go toolchain (to build `weft`) · **Offline-friendly:** yes for steps 1–8

---

## Before you start

```bash
git clone <your-weft-repo>   # or open this monorepo
cd weft                     # repo root
go build -o weft ./cmd/weft
./weft doctor
./weft version
```

You should see something like `weft 0.3.x`. Put `./weft` on your `PATH` if you like (`make install` installs to `~/.local/bin/weft`).

Docs map: [README.md](README.md) · Language: [LANGUAGE.md](LANGUAGE.md) · Recipes: [COOKBOOK.md](COOKBOOK.md)

---

## Minute 0–5 — Hello

Create `hello.weft`:

```weft
fn main {
    say("hello, weft")
}
```

```bash
./weft run hello.weft
# hello, weft
```

**What just happened:** lex → parse → compile → stack VM. Entry is always `fn main` (no args).

Also try the repo sample:

```bash
./weft run examples/hello.weft
./weft run examples/cookbook/01_hello.weft
```

---

## Minute 5–15 — Bindings, functions, strings

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

| Piece | Meaning |
|-------|---------|
| `x := 1` | Immutable bind |
| `mut n := 0` then `n = …` | Reassignable |
| `fn double(x) { x * 2 }` | Last expression is the return value |
| `"hi $name"` / `"${expr}"` | String insert (JSON `{` is not magic) |
| `x \|> f` | Pipeline (`|> say` works) |

```bash
./weft run examples/cookbook/02_style.weft
./weft check examples/cookbook/02_style.weft --types
```

`weft check --types` prints inferred bindings — useful while learning.

---

## Minute 15–25 — Errors with `Result` and `?`

Weft has **no try/catch**. Fallible work returns `Result`. `?` unwraps `Ok` or returns `Err` from the current function.

```weft
fn load(path) -> Result {
    ensure(path != "", "path required")?
    fs.read(path).context("load")?
}

fn main -> Result {
    // default when parse fails
    n := int.parse("nope").unwrap_or(0)
    say("default n=$n")

    r := load("")
    if r.is_err {
        say("kind=${r.err.kind} msg=${r.err.message}")
    }

    text := load("README.md")?
    say("readme bytes=${len(text)}")
}
```

Rules of thumb:

1. Functions that use `?` should declare `-> Result`.  
2. Prefer `expr?` over manual `if r.ok` when failure should abort the function.  
3. Use `.context("…")` to layer messages.

```bash
./weft run examples/cookbook/05_errors.weft
```

More: [ERRORS.md](ERRORS.md).

---

## Minute 25–35 — Lists, map, concurrency

`map` and `filter` fan out **by default** (order of results is preserved). No `async`/`await`.

```weft
fn work(x) { x * x }

fn main -> Result {
    say(map([1, 2, 3, 4], work))

    results := parallel([
        fn() { 1 + 1 },
        fn() { 2 * 3 },
    ])?
    say(results)

    ch := channel(2)
    spawn(fn(c) {
        send(c, 1)
        send(c, 2)
        close(c)
    }, ch)
    say(recv(ch)?, recv(ch)?)
}
```

```bash
./weft run examples/cookbook/08_map_filter.weft
./weft run examples/cookbook/09_parallel.weft
./weft run examples/cookbook/10_channels.weft
```

Env: `WEFT_WORKERS=N` caps default concurrency. Details: [CONCURRENCY.md](CONCURRENCY.md).

---

## Minute 35–45 — Enums, match, closures

### Enums and sum types

```weft
enum Status { Ok, Err, Pending }
enum Shape { Circle(r), Rect(w, h), Point }

fn main {
    s := Status.Pending
    say(match s {
        Status.Ok { "good" }
        Status.Err { "bad" }
        Status.Pending { "wait" }
        _ { "?" }
    })

    area := match Shape.Circle(5) {
        Shape.Circle(r) { 3.14 * r * r }
        Shape.Rect(w, h) { w * h }
        Shape.Point { 0 }
    }
    say("area=$area")
}
```

### Closures (capture by value)

```weft
fn main {
    base := 10
    add := fn(x) { base + x }
    say(add(1))   // 11

    mut n := 1
    g := fn() { n }
    n = 99
    say(g())      // 1 — snapshot at creation
}
```

```bash
./weft run examples/cookbook/06_enums_match.weft
./weft run examples/cookbook/07_closures.weft
```

---

## Minute 45–55 — JSON, files, multi-file

### JSON

```weft
fn main -> Result {
    raw := `{"city":"Paris","temp":21}`
    data := json.parse(raw)?
    say(data.city, data.temp)
    say(json.pretty(data))
}
```

### Files

```weft
fn main -> Result {
    p := fs.temp_file("weft-", ".txt")?
    fs.write(p, "scratch\n")?
    say(fs.read(p)?)
    fs.remove(p)?
}
```

### Path import

```text
examples/cookbook/lib/math.weft
examples/cookbook/11_path_import.weft
```

```weft
// 11_path_import.weft
use "./lib/math.weft" as math

fn main {
    say(math.add(2, 40))
}
```

```bash
./weft run examples/cookbook/03_json.weft
./weft run examples/cookbook/04_files.weft
./weft run examples/cookbook/11_path_import.weft
```

Packages with `vendor/`: [packages.md](packages.md). Try later: `weft new module greeter`.

### Optional catalog modules (later)

Not in the binary — same install path for all. Full map: [ECOSYSTEM.md](ECOSYSTEM.md).

```bash
weft packages list
weft get mold
./weft run examples/cookbook/14_mold.weft   # structured models for agents
```

| Module | Doc |
|--------|-----|
| `mold` | [MOLD.md](MOLD.md) — validate LLM JSON, tool params |
| `ml` | [ML.md](ML.md) — embeddings / RAG |
| `tokensave` | context thrift / memory |

---

## Minute 55–60 — Check, test, CLI

### Static check

```bash
./weft check examples/cookbook/
./weft check examples/fib.weft --types
```

### Unit tests

```weft
// examples/cookbook/math_test.weft
use "./lib/math.weft" as math

fn test_add {
    test.eq(math.add(2, 40), 42)
}

fn test_truth {
    test.is_true(1 + 1 == 2)
}
```

```bash
./weft test examples/cookbook -q
```

### Tiny CLI

```bash
./weft run examples/cookbook/12_cli.weft -- --help
./weft run examples/cookbook/12_cli.weft -- greet Ada
```

---

## Optional extras (after the hour)

| Want… | Do this |
|-------|---------|
| HTTP client | [COOKBOOK §4](COOKBOOK.md#4-http-client) (needs network) |
| Tiny server | `./weft run examples/server.weft` then open `:8080` |
| LLM / tools | `export WEFT_PROVIDER=ollama` · [LLM_LOCAL.md](LLM_LOCAL.md) |
| Agent sample | `examples/realworld/tool_agent.weft` |
| Full recipe book | [COOKBOOK.md](COOKBOOK.md) + [examples/cookbook/](../examples/cookbook/) |
| Language depth | [LANGUAGE.md](LANGUAGE.md) |

---

## Mental model to keep

```text
source  →  parse  →  compile  →  VM
errors  =  Result + ?     (not try/catch)
fan-out =  map / parallel / spawn   (not async/await)
capture =  by value at closure creation
packages = vendor/ + lock, not a global site-packages
```

## If something breaks

| Symptom | Fix |
|---------|-----|
| `no main function` | Add `fn main { … }` |
| `?` error about Result | Add `-> Result` on that function |
| Import missing | Path `use "./x.weft"` or `weft install` for packages |
| Unknown API | `./weft stdlib` / `./weft stdlib fs` |

```bash
./weft doctor
./weft test examples/cookbook -q
```

You are done with the hour. Next: skim [LANGUAGE.md](LANGUAGE.md), then solve a real task from [COOKBOOK.md](COOKBOOK.md).
