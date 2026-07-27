# Versioning (0.3.x)

We’re staying on branch **`0.3.1`** and only bumping the patch number until **0.3.35**.

```text
0.3.1 → 0.3.2 → … → 0.3.35
```

Current toolchain string: **0.3.27** (`pkg/weft.Version`, also `weft version`).

No `0.4.x` until that ceiling is intentional. After 0.3.35 we can open a new line if we need breaking changes.

## Where the number lives

| Place | |
|-------|--|
| `pkg/weft/weft.go` → `Version` | source of truth |
| `README.md` | people-facing |
| `editors/vscode/package.json` | extension (keep in step) |
| `docs/PRODUCTION.md` | short note |
| this file | branch policy |

Bump them together when you cut a patch.

## What a patch is for

Bugfixes, stdlib additions, tooling, docs, gold corpus, small language ergonomics. Not for renaming the world or a public package registry product.

## Before you tag a bump

1. Still on branch `0.3.1`, version `0.3.N` with N ≤ 35  
2. `go test ./...` and `scripts/ci.sh` pass  
3. `weft train eval --quiet` still looks right (100% unless you meant to change gold)  
