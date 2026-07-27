# Errors

Weft sticks to **`Result` and `?`**. There is no try/catch. That keeps fallible I/O visible in the source, which helps both people and models.

## Basics

| | |
|--|--|
| `Ok(value)` | success |
| `Err(msg)` / `Err(msg, kind)` | failure |
| `Error` | struct: `message`, `kind`, `code`, `cause`, `at` |
| `expr?` | unwrap Ok, or **return** Err from this function |
| `-> Result` | allows `?`; bare last values become `Ok(...)` |

```weft
fn load(path) -> Result {
    ensure(path != "", "path required")?
    text := fs.read(path)?
    text
}
```

## Building failures

```weft
Err("boom")
Err("not found", "fs")
Error.new("boom", "custom")
Error.wrap(cause, "while saving")
Error.with("paywall", {"kind": "http", "code": 402})

return bail("stop", "user")
ensure(n > 0, "n must be positive")?
```

| Field | |
|-------|--|
| `message` | text |
| `kind` | coarse class (`fs`, `http`, `user`, …) |
| `code` | optional number/string |
| `cause` | nested error |
| `at` | filled by `?` when it can (`file:line in fn`) |

## Without `?`

```weft
r := fs.read(path)
if r.ok {
    say(r.value)
} else {
    say(r.err.kind, r.err.message)
}

n := int.parse(s).unwrap_or(0)
r := primary().or(secondary())
cfg := fs.read(path).context("load config")?
```

## Result helpers

| | |
|--|--|
| `r.ok` / `r.is_err` / `r.value` / `r.err` | inspect |
| `r.unwrap_or(def)` | value or default |
| `r.context(msg)` / `r.expect(msg)` | annotate Err, still a `Result` |
| `r.or(other)` | first Ok wins |
| `r.unwrap()` | value or hard fail — prefer `?` when you can |
| `is_ok` / `is_err` | free functions |
| `ensure` / `bail` | prelude |

## What we don’t do

- **try/catch** — reserved words only; not implemented, and not planned as the main style  
- **Auto-unwrap** on every call — that would hide I/O failures  
- **Double wrap** — if something is already a `Result`, return doesn’t wrap it again  

Crashes and type bugs surface as `RuntimeError` with a small stack. Domain failures stay as `Err(...)`.

More syntax notes: [SYNTAX.md](SYNTAX.md). Asserts in tests: [TESTING.md](TESTING.md).
