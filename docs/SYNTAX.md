# Syntax

Weft is short and braced. The surface is small so agents and people can learn one dialect.

Full language notes: [LANGUAGE.md](LANGUAGE.md). Cookbook: [COOKBOOK.md](COOKBOOK.md).

## Cheatsheet

| Construct | Notes |
|-----------|--------|
| `x := 1` | bind (immutable rebind) |
| `mut n := 0` then `n = 1` | reassignable |
| `use pkg` / `use "./x.weft" as x` | packages / path modules |
| `say "hi"` / `say(x)` | print |
| `"hi $name"` / `"${expr}"` | string interpolation (JSON-safe) |
| `fn main { }` | empty `()` optional |
| `expr?` | propagate `Result` error |
| `x \|> f` / `x \|> f(extra)` | pipeline |
| `match x { ... }` | string enums / patterns |
| braces `{ }` | required on blocks |

Avoid brace-style placeholders in strings: write `"hi $name"`, not `"hi {name}"`.

## Style

- Prefer `:=` and `say` over verbose ceremony.
- Prefer `data.city` when the key is a fixed name.
- Prefer last-expression returns and `?` over wrapper noise.
- Prefer ordinary `fn` + `parallel`/`spawn` over inventing colored async syntax.

See [PRINCIPLES.md](PRINCIPLES.md) for the product rules behind these choices.
