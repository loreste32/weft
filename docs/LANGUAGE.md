# Weft language reference

End-to-end description of the language as of **0.3.29**. For paste-ready recipes see [COOKBOOK.md](COOKBOOK.md). For a one-page cheatsheet see [SYNTAX.md](SYNTAX.md).

---

## 1. What Weft is

Weft is a small, braced scripting language that runs on a pure-Go stack VM:

```text
.weft source → lexer → parser → compiler → bytecode → VM
```

It is aimed at **LLM agent scripts**, **HTTP glue**, and **ops tooling**. The runtime is one binary; there is no Python interpreter on the hot path.

Design choices you will feel immediately:

| Choice | Consequence |
|--------|-------------|
| `Result` + `?` | Fallible I/O is visible; no try/catch |
| Concurrent by default | `map` / `filter` fan out; no `async`/`await` keywords |
| Closures by value | Captured locals are deep-copied at creation (safe under fan-out) |
| Gradual types | Annotations optional; `weft check` can still help |
| Vendor packages | Path/git install into `vendor/`, lockfile with hashes |

---

## 2. Programs and entry

A script is one or more top-level declarations. Execution starts at **`fn main`** (arity 0).

```weft
fn main {
    say("hello, weft")
}
```

Libraries use `pub fn` and usually have **no** `main`. Path imports load a file as a module map of exports.

Top-level forms:

| Form | Example |
|------|---------|
| Function | `fn name(a, b) { … }` · `pub fn …` |
| Type / struct | `type Point { x, y }` · `type Id = str` |
| Const | `const Max = 10` |
| Enum | `enum Status { Ok, Err }` · `pub enum …` |
| Import | `use greeter` · `use "./lib.weft" as L` · `import greeter as g` |

Comments: `//` to end of line.

---

## 3. Values and types

### Value kinds

| Kind | Literals / construction |
|------|-------------------------|
| `null` | `null` |
| `bool` | `true`, `false` |
| `int` | `42`, `-1`, `1_000`, `0xff`, `0b1010`, `0o755` |
| `float` | `3.14`, `1e-6`, `2.5E+3`, `1_000e-3` |
| `str` | `"hi"`, `` `raw` `` |
| `list` | `[1, 2, 3]` |
| `map` | `{"a": 1, "b": 2}` |
| `struct` | `Point{x: 1, y: 2}` or type defaults |
| `func` | `fn(x) { x + 1 }` or named `fn` |
| `Result` | `Ok(v)` / `Err(msg)` |
| `unit` | empty success / no value |

Lists and maps are mutable when held under a `mut` binding and mutated through ops like `push`. Prefer not to share mutable collections across concurrent tasks — pass copies or use channels.

### Optional type annotations

```weft
fn add(a: int, b: int) -> int { a + b }

fn load(path: str) -> Result {
    fs.read(path)?
}

x: int := 1
```

Types are gradual. `weft check` and `weft check --types` report issues and inferred bindings. There is no full sound type system yet.

Common type forms: `int`, `float`, `str`, `bool`, `[T]`, `{K: V}`, `Result`, `Result[T]`, `T?` (optional), named types.

---

## 4. Bindings

```weft
x := 1          // immutable binding (preferred)
mut n := 0      // reassignable
n = n + 1

let y = 2       // still works; prefer :=
const Max = 100 // top-level constant (literal)
```

Assignment targets: locals, fields, indexes (`xs[i] = v`, `m.key = v`).

---

## 5. Strings

| Form | Meaning |
|------|---------|
| `"hello $name"` | Insert a simple name |
| `"sum=${a + b}"` | Insert an expression |
| `"{\"x\":1}"` | Ordinary JSON — `{` does not start interpolation |
| `` `raw $notCode` `` | Raw string, no interpolation |
| `f"hi {name}"` | Accepted brace style (legacy-friendly) |

Concatenation: `"a" + "b"`. Length: `len(s)`.

---

## 6. Operators

Arithmetic: `+` `-` `*` `/` `%`  
Comparison: `==` `!=` `<` `<=` `>` `>=`  
Logic: `&&` `||` `!` (short-circuit)  
Nullish: `??`  
Pipeline: `x |> f` · `x |> f |> g`  
Error unwrap: `expr?`  

Indexing / fields: `xs[i]`, `obj.field`, `m["key"]`  
Call: `f(a, b)`

---

## 7. Control flow

### if

Statement or value:

```weft
if n > 0 {
    say("pos")
} else if n == 0 {
    say("zero")
} else {
    say("neg")
}

x := if n > 0 { 1 } else { 0 }
say(if ready { "go" } else { "wait" })
```

### match

First arm wins. Patterns: literals, `_`, idents/consts, field access (enums).

```weft
msg := match kind {
    "text" { text }
    "done" { "[done]" }
    Status.Ok { "good" }
    _ { "?" }
}
```

No match and no trailing `_` yields unit. Prefer an explicit `_` arm for defaults.

### loops

```weft
for x in xs {
    say(x)
}

mut i := 0
while i < 10 {
    i = i + 1
}

// break / continue work inside loops
```

`for` iterates lists (and other iterable values). `range(n)` / `range(start, end)` build numeric sequences.

### return / defer

```weft
fn early(x) {
    if x < 0 {
        return 0
    }
    x * 2
}

fn with_cleanup(ch) -> Result {
    defer close(ch)   // call expression only; LIFO on return / ? / fallthrough
    send(ch, 1)
    Ok(unit)
}
```

Last expression in a block is the value of the block / function when there is no explicit `return`.

---

## 8. Functions

```weft
fn double(x) { x * 2 }

fn greet(name) {
    "hello, $name"
}

fn main {
    say(double(21))
    say(greet("weft"))
}
```

- Parentheses are optional when there are no parameters: `fn main { … }`  
- `pub fn` marks exports for modules  
- Return type `-> Result` enables `?` and auto-wraps bare last values as `Ok(...)`

### Closures

Anonymous functions capture **outer locals by value**. Values are deep-copied when the closure is created, so later mutations of the outer binding do not change what the closure sees, and concurrent use does not share a mutable outer frame.

```weft
fn main {
    base := 10
    add := fn(x) { base + x }
    say(add(1))   // 11

    mut n := 1
    g := fn() { n }
    n = 99
    say(g())      // 1  (snapshot at creation)
}
```

Nested closures can capture names from outer scopes; free-variable analysis walks nested bodies.

Handlers for `map`, `http.serve`, `spawn`, etc. are ordinary function values:

```weft
label := "ok"
h := fn(x) { label + ":" + x }
say(map(["a", "b"], h))
```

---

## 9. Enums

Enums are **string-tagged maps** — simple and match-friendly, not algebraic sum types with payloads.

```weft
enum Status { Ok, Err, Pending }

fn main {
    say(Status.Ok)           // "Ok"
    s := Status.Pending
    say(match s {
        Status.Ok { "good" }
        Status.Err { "bad" }
        Status.Pending { "wait" }
        _ { "?" }
    })
}
```

`pub enum` is exported from modules the same way as `pub` types. Variants are strings, so `match "Ok" { Status.Ok { … } … }` also works.

---

## 10. Errors (`Result` and `?`)

There is **no try/catch** as the primary model. Fallible operations return `Result`.

```weft
fn load(path) -> Result {
    ensure(path != "", "path required")?
    text := fs.read(path).context("load")?
    text   // bare value becomes Ok(text) when return type is Result
}
```

| Piece | Role |
|-------|------|
| `Ok(v)` / `Err(msg)` | Construct success / failure |
| `expr?` | Unwrap Ok, or **return** Err from this function |
| `-> Result` | Enables `?`; wraps bare returns |
| `ensure(cond, msg)?` | Fail if condition false |
| `bail(msg)` / `bail(msg, kind)` | Build an Err Result |
| `r.ok` · `r.value` · `r.err` | Inspect without `?` |
| `r.unwrap_or(def)` · `r.context(msg)` · `r.or(other)` | Helpers |

Full guide: [ERRORS.md](ERRORS.md).

---

## 11. Types and structs

```weft
type Point {
    x: int
    y: int
}

type Id = str

fn main {
    p := Point{x: 1, y: 2}
    say(p.x)
}
```

Struct field access uses `.`. Type aliases rename existing types for documentation and checking.

---

## 12. Collections and pipelines

```weft
xs := [1, 2, 3]
push(xs, 4)
say(len(xs))
say(xs[0])

m := {"city": "Paris", "temp": 21}
say(m.city)

// concurrent by default (order preserved)
out := map(urls, fetch)
kept := filter(items, is_ok)

// sequential when side-effect order matters
out := seq_map(urls, fetch)

// pipelines
21 |> double |> say
```

Prelude helpers include `map`, `seq_map`, `filter`, `seq_filter`, `reduce`, `each`, `par_map`, `find`, `any`, `all`, `sort`, `reverse`, `unique`, `zip`, `flatten`, `enumerate`, `count`, `range`, `push`, `len`, `say` / `println`.

See [PIPELINES.md](PIPELINES.md) and [STDLIB.md](STDLIB.md).

---

## 13. Concurrency

No `async` / `await` keywords. Ordinary functions run on Go’s scheduler under the VM.

```weft
// Independent work
results := parallel([
    fn() { http.get(url_a)? },
    fn() { http.get(url_b)? },
])?

// First completed
v := race([fn() { slow() }, fn() { fast() }])?

// Deadline
v := timeout(2, fn() { slow_call() })?

// Background
h := spawn(work, arg1, arg2)
ok := h.await()?

// Task group
g := group()
g.go(fn() { 10 })
g.go(fn() { 20 })
rs := g.wait()?

// Channels
ch := channel(8)
spawn(producer, ch)
x := recv(ch)?
peek := try_recv(ch)?   // {ok, value}
```

Env: `WEFT_WORKERS=N` for default `map` / `filter` pool size.

**Rules:** prefer channels or copies over shared mutable maps; use `timeout` instead of busy waits; tool calls from agents fan out concurrently when the model returns several at once.

Full guide: [CONCURRENCY.md](CONCURRENCY.md).

---

## 14. Modules and packages

### Path import

```weft
use "./math.weft" as m
say(m.add(2, 3))
```

### Package import

```weft
use greeter
say(greeter.hello("weft"))
```

Resolution order (simplified):

1. Stdlib (`http`, `json`, `fs`, …)  
2. `vendor/<name>/`  
3. `WEFT_PATH`  
4. `packages/<name>/`  

### Authoring

```bash
weft new module greeter
weft mod check
weft get greeter ./path/to/greeter
weft install
```

- `pub` exports only (when any `pub` exists)  
- `weft.lock` records content hashes  
- Lite version constraints: `"version": "^0.1.0"`  
- Catalog: `weft packages list` · `WEFT_CATALOG_URL` for remote index  

Details: [packages.md](packages.md), [modules.md](modules.md).

---

## 15. Stdlib surface

Packages live in the binary. List them with:

```bash
weft stdlib
weft stdlib http
```

High-traffic packages: `fs`, `http`, `json`, `str`, `time`, `env`, `cli`, `llm`, `web`, `db`, `log`, `test`, `yaml` / `toml` / `ini`, `math`, `re`, `csv`, `table`, …

Overview: [STDLIB.md](STDLIB.md). Domain guides: [web.md](web.md), [cli.md](cli.md), [data.md](data.md), [LLM_PROVIDERS.md](LLM_PROVIDERS.md).

---

## 16. Tooling

| Command | Purpose |
|---------|---------|
| `weft run file.weft` | Execute |
| `weft` | REPL |
| `weft check path…` | Parse / type-check |
| `weft check --types` | Print inferred types |
| `weft test` | Run `*_test.weft` · `fn test_*` |
| `weft fmt` | Pretty-print from AST |
| `weft bench` | Microbench `fn bench_*` |
| `weft lsp` | Language server (stdio) |
| `weft doctor` | Environment readiness |
| `weft gen "task"` | LLM writes Weft |

Tests: [TESTING.md](TESTING.md). Tooling notes: [TOOLING.md](TOOLING.md).

---

## 17. What is intentionally missing

| Missing | Why |
|---------|-----|
| `try` / `catch` as primary errors | `Result` + `?` is the style |
| `async` / `await` keywords | Concurrent-by-default; no function coloring |
| Classes / inheritance | Structs + functions |
| NumPy / pandas / SciPy | Out of core; stay small |
| In-process GPU training | Orchestrate outside; optional train kit |
| Full algebraic enums | String-tagged enums only (for now) |
| Public package registry | Path/git + monorepo catalog first |

See [ROADMAP.md](ROADMAP.md) and [PRINCIPLES.md](PRINCIPLES.md).

---

## 18. Complete mini program

```weft
enum Kind { Text, Done }

fn handle(kind, text) {
    match kind {
        Kind.Text { text }
        Kind.Done { "[done]" }
        _ { "" }
    }
}

fn main -> Result {
    label := "evt"
    tag := fn(s) { label + ":" + s }

    items := ["a", "b", "c"]
    out := map(items, tag)
    say(out)

    say(handle(Kind.Text, "hello"))
    say(handle(Kind.Done, ""))

    raw := `{"city":"Paris"}`
    data := json.parse(raw)?
    say(data.city)
}
```

Next: [TUTORIAL.md](TUTORIAL.md) for a guided hour, then [COOKBOOK.md](COOKBOOK.md) and [examples/cookbook/](../examples/cookbook/) for real tasks.
