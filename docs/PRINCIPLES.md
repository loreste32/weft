# Design principles

A few rules we try not to break. Syntax and the stdlib follow from these.

## 1. LLM work is in the language

Agents, tools, streaming, and basic structured decode belong in the language and stdlib — not only in a third-party framework. You should be able to write a simple agent without installing a stack.

## 2. Own syntax

Code should look like **Weft**. Examples, modules, tests, and model prompts use that surface — not a dialect of another language with different braces or keywords.

- Short keywords: `fn`, `use`, `say`, `mut`
- Bind with `:=` (rebind only `mut`)
- Braces required; no indent-sensitive silent bugs
- Strings: `"hi $name"` / `"${expr}"` (JSON-safe interpolation)
- Fields: `row.name` over `row["name"]` when the key is a name
- `?` for fallible work; last expression returns
- Full rules: [`docs/SYNTAX.md`](SYNTAX.md)

## 2b. Prefer short

If something needs three lines of ceremony for one idea, the API (or the example) is wrong.

| Prefer | Avoid |
|--------|--------|
| `fn weather(city)` | `fn weather(city: str) -> str` when types aren't needed |
| `-> Result` | `-> Result[unit]` unless the Ok type matters |
| `llm.ask(prompt, tools)` | multi-step builder / class ceremony |
| `llm.tool("x", fn)` | registries, decorators, config objects |
| `data.city` | `data["city"]` when key is a name |
| stdlib always in scope | mandatory `import` for every line |
| `return 42` in `-> Result` | mandatory `return Ok(42)` (both work) |
| `fn(x) { x + 1 }` | `fn(x) { return x + 1 }` |
| one `for` / inline `if` | helper fns that only wrap one expression |
| opts you need | dumping every option “for documentation” |
| `weft` (REPL) | ceremony-heavy notebooks for one-off prompts |
| `llm.stream` / `llm.extract` | framework-sized structured-output stacks |
| `mold` module when you need models | growing core stdlib for every schema library |

Keep demos short enough to scan. Cut spare comments and unused options.

## 3. Honest performance

Talk about cold start and a single binary. Don’t claim compute speedups without measurements. LLM latency is mostly the network anyway.

## 4. Small language, rich builtins

Keep the syntax small. Put most power in Go-registered stdlib (`http`, `llm`, `json`, `secrets`). One binary; no external package manager for core.

Scripts, packages, agents, and the default fine-tune path (OpenAI-compatible HTTP) run in pure Go. Optional `--backend trl` shells out to an external open-weight training toolchain — that path is opt-in, not required for Weft itself.

## 5. Predictable failures

I/O and model calls return `Result[T]`. `?` propagates. No `async`/`await` coloring. Task fan-out uses deep-copied args (or channels) instead of a shared mutable heap.

## 5b. Concurrent programming is the default

Weft is for agents and I/O fan-out. **Concurrent is normal**, not a special mode:

- Use `parallel` / `gather` / `par_map` / `spawn` / `race` / `timeout` / channels — every day.
- Ordinary `fn` runs concurrently when scheduled; **no `async` / `await`**.
- Args into tasks are **deep-copied** (no shared mutable heap across tasks).
- Multi-tool agent steps **fan out concurrently** by default.

An event-loop API with colored functions is intentionally out of scope. See [`docs/CONCURRENCY.md`](CONCURRENCY.md).

## 6. Ship slices

| Slice | Exit |
|-------|------|
| MVP-0 | `weft run examples/hello.weft` |
| MVP-1 | sequential agent + tools |
| Phase 2 | REPL, stream, server, checker |
| Phase 3 | `spawn` / channels / parallel tools |

## 7. Implementation bias

- Compiler/runtime in **Go**
- **Bytecode stack VM** for v1 (REPL, embed, single binary)
- Hand-rolled lexer + recursive-descent parser
- Borrow ideas from other embeddable Go VMs; do not fork them as the product surface

## Syntax at a glance

```weft
fn main {
    mut n := 0
    for x in [1, 2, 3] {
        n = n + x
    }
    say("sum=$n")
}
```

- Blocks: `{ }` required
- Strings: `"hi $name"` / `"${expr}"`
- Types optional: `fn f(x: int) -> Result[str]`
- Errors: `return Ok(v)` / `return Err(e)` / `expr?`
- Mutability: `x :=` (no rebind) vs `mut x :=` then `x =`; in-place mutation on collections is OK within a task
