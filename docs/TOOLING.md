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
| `weft notebook <file> [-o out.html]` | Run `.weft` as cells, output HTML. |
| `weft debug <file>` | Interactive source-level debugger. |
| `weft profile <file>` | Execution profiler. |
| `weft build [dir] [-o out]` | Produce standalone executable (no weft needed on target). |
| `weft test --race` | Detect data races in concurrent code. |
| `weft test --mem` | Track memory allocations per test. |
| `weft test --timeout N` | Per-test timeout in seconds. |
| `weft mcp serve <file>` | Run Weft functions as MCP tools (for AI assistants). |
| `weft update` | Self-update weft binary to latest version. |
| `weft upgrade` | Upgrade installed packages to latest registry versions. |
| `weft lsp` | Language server: diagnostics, completion, hover, definition, symbols, **format**. |
| `weft eval [dir]` | Smoke-run scripts that have `fn main`. |
| `weft train eval` | Score the embedded gold examples (parse/compile). |
| `weft` / `weft repl` | Interactive REPL (session bindings, multi-line, history). |

## REPL

```bash
weft          # or: weft repl
```

| | |
|--|--|
| Expressions | `1 + 2`, `map([1,2], fn(x) { x*x })` |
| Definitions | `fn` / `type` / `const` / `enum` bind into the session (no `main` required) |
| Multi-line | Continues while braces/parens/brackets or quotes are open; also after trailing ops (`1 +`) |
| `:cancel` / `:c` | Abort multi-line buffer (`:c` at primary prompt still clears the screen) |
| `:history` | Last entries; `:history text` filters by substring |
| `!N` / `:!N` | Re-run history entry **N** (1-based) |
| `:stdlib [pkg]` | Browse stdlib |
| History file | `~/.weft/history` |
| Interactive TTY | **Tab** completes keywords / stdlib / `:commands`; **↑/↓** history; Ctrl-C cancels line |

Pipes and tests still use plain line mode (no raw terminal).

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
| Completion | Keywords, prelude, stdlib packages/members (with signatures), local fns/enums, **params/lets**, **enum variants** after `Status.` |
| Hover | Packages, prelude, **stdlib member docs** (`llm.*`, `web.*`, `yaml.*`, `db.*`, …), enum variants, inferred types |
| Signature help | Broad catalog (llm/http/web/fs/json/yaml/db/cli/table/test/…); active param tracks commas |
| Definition | Same-file `fn` / `enum` / `type` / **params** / **lets** / for-vars |
| Highlight | `documentHighlight` for the identifier under the cursor |
| References | Same-file |
| Rename | Locals: same-file; **top-level** fn/type/enum/const: open buffers + workspace `.weft` |
| Code actions | **Extract function** on a selection (`refactor.extract.function`) |
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
