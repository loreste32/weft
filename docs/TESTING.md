# Testing Weft scripts

You can test `.weft` code with `weft test` without writing Go. It’s a small runner: discover files, call zero-arg `test_*` functions, stop on the first failed assert.

## Quick start

```bash
weft test                              # current tree
weft test examples/stdlib_test.weft
weft test -run math                    # name substring
weft test -q                           # summary only
weft test --race                       # data race detection
weft test --mem                        # memory allocation tracking
weft test --timeout 30                 # per-test timeout (seconds)
weft test --coverage                   # coverage report
```

## Layout

| Piece | Rule |
|-------|------|
| File | `*_test.weft` or `test_*.weft` |
| Function | `fn test_something` with **no parameters** |
| Asserts | `test.*` (stdlib; no import needed) |

```weft
fn test_add {
    test.eq(1 + 1, 2)
    test.is_true(len([1, 2]) == 2)
}

fn helper {
    // ignored — name must start with test_
}
```

No `fn main` in test files.

## Asserts

| Call | |
|------|--|
| `test.eq` / `test.ne` | equality |
| `test.is_true` / `test.is_false` | truthiness (`true`/`false` are keywords, so the names are a bit awkward) |
| `test.ok` / `test.err` | `Result` |
| `test.contains` | string or list |
| `test.approx(a, b, eps?)` | floats (default ε `1e-9`) |
| `test.is_null` | null |
| `test.fail` / `test.skip` | hard fail or skip |

A failed assert ends that `test_*` function and prints `FAIL file::name`.

## Pass/fail semantics

A `test_*` function fails when it **raises** a runtime error (e.g. a failed `test.eq`) **or** when it **returns an `Err` Result** — including `Err` propagated by `?`:

```weft
fn test_parse_config {
    cfg := parse_config("bad")?   // Err propagates → this test FAILs
    test.eq(cfg.name, "x")
}
```

Returning `Ok(...)` or any non-`Result` value passes. Use `test.ok` / `test.eq` for assertions — don't let a `?` silently decide the outcome unless a propagated error is exactly what should fail the test.

## Related

| Tool | Role |
|------|------|
| `weft check` | types only, no run |
| `weft eval [dir]` | run example scripts that have `main` |
| `weft train eval` | gold corpus (for the language pack) |
| `go test ./…` | tests for the **implementation** in Go |

## Package-relative tests

Each test file gets `ProjectDir` set to the package/project root (or the test’s directory). Relative `use "./lib.weft"` resolves next to the package without a full vendor install.

Authors can also run tests as part of validation:

```bash
weft mod check . --tests    # static check + weft test
weft mod check . -t -q      # quiet test summary
```

## Modules

`weft new module foo` drops a sample `foo_test.weft` that imports `./lib.weft`:

```weft
use "./lib.weft" as mod

fn test_hello {
    test.eq(mod.hello("weft"), …)
}
```

Each test file is compiled on its own, so you need that path import (or similar) to reach package code.

## Typecheck notes

- `weft check` is fine on modules and tests (no `main` required).
- If a function is declared with a non-`Result` return type, `?` is a type error.

## Benchmarks

Convention: files `*_bench.weft` or `bench_*.weft`, functions `fn bench_*` (zero args).

```bash
weft bench                             # discover and run
weft bench -n 5000                     # 5000 iterations per bench
weft bench --save baseline.json        # export results
weft bench --compare baseline.json     # regression tracking
weft bench -run fib                    # filter by name
```

## Linting

```bash
weft lint                              # current tree
weft lint src/                         # specific path
```

Checks: parse errors, unused imports, trailing whitespace, long lines, TODO markers, missing entry points.

## CI

```bash
weft test -q --race
weft check --strict examples packages
weft lint
weft bench
```
