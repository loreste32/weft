# Versioning

## Current line: **0.6.x**

| | |
|--|--|
| Toolchain | **`0.6.3`** (`pkg/weft.Version` / `weft version`) |
| VS Code extension | **`0.6.3`** (`editors/vscode/package.json`) |
| Branch | `main` |

0.3.x–0.5.x are **complete**. 0.6.x is the current development line: forward autodiff, loc/iloc assignment, warp NumPy surface, ML training, SQL bridge, signed releases.

```text
0.3.x (complete) → 0.4.0 → 0.6.3 → …
```

## Where the number lives

| Place | Role |
|-------|------|
| `pkg/weft/weft.go` → `Version` | **source of truth** |
| `README.md` | people-facing |
| `editors/vscode/package.json` | extension (keep in step) |
| `docs/ROADMAP.md` | “where we are” |
| this file | branch policy |

Bump them together when you cut a release.

## What a 0.6.x patch is for

Bugfixes, reliability (fuzz/validate/race), stdlib/tooling, docs, gold corpus, small language ergonomics.

Breaking language changes should be rare and called out in [CHANGELOG.md](../CHANGELOG.md). There is **no LTS** yet — pin versions in production.

## Registry / packages

A public registry protocol and signing are **implemented**. Hosting of `registry.weftproject.dev` may still be partial; path/git and monorepo `packages/` always work. Namespace key trust: `weft registry trust` (see [STABILITY.md](STABILITY.md)).

## Before you tag

1. Align version strings above  
2. `go test ./...` and `scripts/ci.sh`  
3. `weft train eval --quiet` still healthy unless gold intentionally changed  
