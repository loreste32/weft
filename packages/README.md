# Weft packages (optional modules)

**Not stdlib. Not in the `weft` binary.**  
Same install model for all three: `weft get` → `vendor/` → `use name`.

How this fits the rest of Weft: **[docs/ECOSYSTEM.md](../docs/ECOSYSTEM.md)**.

## Catalog

| Module | Role | Caps (typical) | Docs |
|--------|------|----------------|------|
| [`mold`](mold/) | Structured models — validate LLM/API JSON, JSON Schema, tool params | none (pure) | [docs/MOLD.md](../docs/MOLD.md) |
| [`ml`](ml/) | Embeddings, vectors, RAG index, metrics | `@agent` + fs + env | [docs/ML.md](../docs/ML.md) |
| [`tokensave`](tokensave/) | Context thrift, memory, teach → train export | `@agent` + fs + env | [README](tokensave/README.md) |

Index file (not a public registry): [`index.json`](index.json).

```bash
weft packages list
weft packages get mold      # or ml / tokensave
weft install

# equivalent path form
weft get mold ./packages/mold
weft get ml ./packages/ml
weft get tokensave ./packages/tokensave
```

```weft
use mold

fn main -> Result {
    M := mold.model({"name": "str!"})?
    p := mold.parse(M, "{\"name\":\"Ada\"}")?
    say(p["name"])
}
```

## Agent stack (which module when)

```text
llm (stdlib)  →  chat / tools / stream
mold          →  shape & validate structured JSON
tokensave     →  thrift context + memory → train gold
ml            →  embeddings / RAG vectors
```

Full picture: [docs/ECOSYSTEM.md](../docs/ECOSYSTEM.md).

## Author another module

```bash
weft new module mykit
# edit lib.weft — pub fn …
weft mod check
# consumers: weft get mykit ./path-or-git@tag
```

Capabilities: [docs/modules.md](../docs/modules.md).  
Consumer package manager: [docs/packages.md](../docs/packages.md).  
Trust model: [SECURITY.md](../SECURITY.md).

## Non-goals for packages/

- Native GPU / binary wheels (use sidecars + HTTP)
- Replacing core (`http`, `llm`, `fs`, …)
