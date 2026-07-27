# Tooling

Small set of commands for day-to-day work. Nothing here replaces a full IDE or pytest ecosystem — it’s enough to check, test, and poke at the language.

## Commands

| Command | What it does |
|---------|----------------|
| `weft check <path>…` | Parse and type-check. Directories are fine; `main` is not required. |
| `weft test [path…]` | Run unit tests in `*_test.weft` / `fn test_*`. |
| `weft fmt <path>…` | Pretty-print from the AST (or whitespace-only if the file doesn’t parse). |
| `weft bench [path…]` | Rough microbenches: `*_bench.weft` / `fn bench_*`. |
| `weft stdlib [pkg]` | List stdlib packages, or members of one package. |
| `weft lsp` | Language server: diagnostics, completion, hover, definition, symbols, **format**. |
| `weft eval [dir]` | Smoke-run scripts that have `fn main`. |
| `weft train eval` | Score the embedded gold examples (parse/compile). |

## Format

`weft fmt` rewrites source from the parse tree: 4-space indent, spaces around operators, `say` instead of bare `println`, `use` for imports. Short enums and single-expression match arms stay on one line. Nested / wide maps and lists break across lines; string map keys stay quoted. Anonymous fns print as `fn(...) { … }`. Run it twice and you should get the same bytes.

If a file is broken, fmt won’t invent a parse — it just trims trailing space so you don’t lose work.

```bash
weft fmt examples/fmt_sample.weft
weft fmt ./packages/ml
weft fmt examples/cookbook
```

Editors using `weft lsp` get the same formatter via `textDocument/formatting` (format document / format-on-save if enabled).

## LSP (editors)

```bash
weft lsp   # stdio Language Server
```

| Feature | Notes |
|---------|--------|
| Diagnostics | Parse + gradual type check on open/change |
| Completion | Keywords, prelude, stdlib packages/members (with signatures), local fns/enums, **enum variants** after `Status.` |
| Hover | Packages, prelude, **stdlib member docs** (`llm.*`, `web.*`, `yaml.*`, `db.*`, …), enum variants |
| Signature help | Broad catalog (llm/http/web/fs/json/yaml/db/cli/table/test/…); active param tracks commas |
| Definition | Local `fn` / `enum` / `type` |
| Symbols | Outline for fn, type, enum |
| Formatting | Same engine as `weft fmt` |

VS Code / JetBrains: see `editors/`.

## Bench

Handy for “is this loop slower than that one?” on your machine. Not a formal suite.

```bash
weft bench examples/micro_bench.weft -n 5000
weft bench -run sort -n 10000
```

Files: `*_bench.weft` or `bench_*.weft`. Functions: `fn bench_name` with no arguments. Default is 1000 iterations after one warmup run. Output is ns/op (or µs/ms when it’s larger).

## Tests and errors

- Tests: [TESTING.md](TESTING.md)  
- Errors (`Result` / `?`): [ERRORS.md](ERRORS.md)  
