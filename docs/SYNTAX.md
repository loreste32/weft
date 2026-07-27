# Syntax

Weft is short and braced. It is not Python and not JavaScript on purpose — the surface is small so agents and people can learn one dialect.

**Tutorial:** [TUTORIAL.md](TUTORIAL.md) · **Language:** [LANGUAGE.md](LANGUAGE.md) · **Recipes:** [COOKBOOK.md](COOKBOOK.md) · **Runnable:** [examples/cookbook/](../examples/cookbook/) · **Index:** [README.md](README.md)

## Feel

| Prefer | Instead of |
|--------|------------|
| `fn` + braces | Indentation-only structure |
| `x := 1` | `let` on every line |
| `"hi $name"` / `"${expr}"` | Python f-strings (or `{` that breaks JSON) |
| `use greeter` | Long import ceremony |
| `fn main { }` | Empty `()` |
| `expr?` | try/except around every I/O call |
| last expression returns | `return` on every one-liner |

## Cheatsheet

```weft
// bind
x := 1              // immutable binding
mut n := 0          // reassignable
n = n + 1

// functions — () optional when no params
fn double(x) { x * 2 }
fn main {
    say("answer ${double(21)}")
}

// control
if n > 0 { say("pos") } else { say("nonpos") }
// if as value (RHS / args) — not only last-stmt
x := if n > 0 { 1 } else { 0 }
say(if x == 1 { "ok" } else { "no" })
// match (literals, enum fields, consts, `_`) — value or statement; first arm wins
msg := match kind {
    "text" { text }
    "done" { "" }
    _ { "?" }
}
for x in [1, 2, 3] { n = n + x }
while n < 10 { n = n + 1 }

// enum — string-tagged map (Status.Ok == "Ok")
enum Status { Ok, Err, Pending }
s := Status.Pending
say(match s {
    Status.Ok { "good" }
    Status.Err { "bad" }
    Status.Pending { "wait" }
    _ { "?" }
})

// closures — capture outer locals by value (deep-copied at creation)
base := 10
add := fn(x) { base + x }
say(add(1))  // 11

// defer call — LIFO on return / ? early return / fallthrough
// (call expression only; args evaluated at the defer site)
defer close(ch)

// errors — Result + ? (no try/catch)
fn load(path) -> Result {
    ensure(path != "", "path required")?
    text := fs.read(path).context("load")?
    text                            // → Ok(text)
}
// r.ok / r.err / r.unwrap_or(def) / bail("msg") / Error.wrap(cause, msg)
// full guide: docs/ERRORS.md

// packages
use greeter
use "./math.weft" as m

// concurrency
ch := channel(1)
spawn(fn(c) { send(c, 1); close(c) }, ch)
v := recv(ch)?
```

## Keywords

**Core:** `fn` `mut` `if` `else` `match` `for` `in` `while` `return` `break` `continue` `defer`  
**Bind:** `let` still works; prefer `:=`  
**Modules:** `use` (preferred) · `import` (alias) · `as` · `pub`  
**Types:** `type` `struct` · `const` · `enum`  
**Literals:** `true` `false` `null`

## Numbers

| Form | Example |
|------|---------|
| int | `42`, `-1`, `1_000_000` |
| hex | `0xff`, `0xFF`, `0xde_ad` |
| binary | `0b1010`, `0B11` |
| octal | `0o755`, `0O644` |
| float | `3.14`, `0.5`, `3.14_15` |
| scientific | `1e-6`, `2.5E+3`, `1_000e-3` |

Scientific form always yields a float (even `1e3`). Prefixed bases are always ints.  
`_` may separate digits (`1_000`, `0xFF_00`); not at the end, and not next to `.` as `3._14`.

## Strings

- `"hello $name"` — simple name insert  
- `"sum=${a + b}"` — expression insert  
- `"{\"x\":1}"` — plain JSON, no surprise interp  
- `` `raw $notCode` `` — no interpolation  
- `f"hi {name}"` — still accepted (brace style)

## Type inference

Annotations are optional. Weft infers from usage:

```weft
fn main {
    x := 1          // x : int
    ys := [1, 2]    // ys : [int]
    s := "hi $x"    // s : str
    t := json.parse("{}")?  // t unwrapped from Result
}

fn add(a: int, b: int) -> int { a + b }  // checked when annotated
```

```bash
weft check file.weft           # infer + report errors
weft check file.weft --types   # also print inferred bindings
```

## Out of scope (by choice)

No indentation-only blocks, no required semicolons, no classes/inheritance, no `async`/`await` keywords (`task.await()` is fine). Missing packages fail loudly. Types are optional on bindings.

## Prefer / avoid (for examples and models)

| Prefer | Avoid when you can |
|--------|--------------------|
| `x := 1` · `mut n := 0` | `let` on every line |
| `use greeter` | long import ceremony |
| `"hi $name"` | string soup or f-string-only style |
| `data.city` | `data["city"]` for plain names |
| `fs.read(p)?` | try/except around every I/O |
| last expr returns | `return` on every one-liner |
| `gather` / `spawn` | `async def` / `await` |
| `say(x)` | `print` / `console.log` as the house style |

## Keep it short

Dense on purpose. Skip padding:

```weft
// good
fn main -> Result {
    say(fs.read("a.txt")?)
}

// bad — ceremony for nothing
fn main -> Result {
    path := "a.txt"
    result := fs.read(path)
    if !result.ok {
        return Err(result.err)
    }
    text := result.value
    say(text)
    return Ok(1)
}
```

Rules of thumb: last-expr return · `?` over manual Result checks · skip type noise · skip empty `()` · skip helpers that call one line.  


## Full programs

```weft
use greeter

fn weather(city) { "clear in $city" }

fn main -> Result {
    reply := llm.ask("Weather in Paris?", [
        llm.tool("weather", weather),
    ])?
    say(reply)
}
```
