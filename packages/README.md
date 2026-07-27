# Weft packages (optional modules)

**Not core stdlib.** Nothing here is built into the `weft` binary — apps install what they need with `weft get` / `weft packages get`.

| Package | Role |
|---------|------|
| [`ml`](ml/) | embeddings, vectors, RAG index, metrics |
| [`tokensave`](tokensave/) | model brain — clarify asks, memory, train export (local **and** paid) |
| [`mold`](mold/) | pour LLM/API JSON into shape — validate, JSON Schema, tool params |

```bash
weft get ml ./packages/ml   # monorepo path
weft install
```

```weft
use ml
fn main -> Result {
    say(ml.topk([1.0, 0.0], [{"id":"a","vec":[1.0,0.0]}], 1))
}
```

## Catalog

Monorepo index (not a public registry): [`index.json`](index.json).

```bash
weft packages list              # or: weft list packages
weft packages get tokensave    # adds path dep from catalog
weft install

# or explicit path
weft get tokensave ./packages/tokensave
weft get ml ./packages/ml

# outside monorepo:
# export WEFT_PACKAGES=/path/to/weft/packages
```

## Author another module

```bash
weft new module mykit
# edit lib.weft — pub fn …
weft mod check
# consumers: weft get mykit ./path-or-git@tag
```

Capabilities: [`docs/modules.md`](../docs/modules.md) (`@data`, `@host`, …).  
No central registry required — path/git + lock is enough until discovery hurts.

## Non-goals for packages/

- Native GPU / binary wheels (use sidecars + HTTP)
- Replacing core (`http`, `llm`, `fs`, …)
