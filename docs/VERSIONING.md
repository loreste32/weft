# Versioning

## Current line: **0.4.x**

| | |
|--|--|
| Toolchain | **`0.4.10`** (`pkg/weft.Version` / `weft version`) |
| VS Code extension | **`0.4.10`** (`editors/vscode/package.json`) |
| Branch | `main` |

0.3.x is **complete** (through the 0.3.30–0.3.35 era of patches). 0.4.x is the current development line: type annotations, DAP, Wasm playground, registry trust, reliability work.

```text
0.3.x (complete) → 0.4.0 → 0.4.10 → …
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

## What a 0.4.x patch is for

Bugfixes, reliability (fuzz/validate/race), stdlib/tooling, docs, gold corpus, small language ergonomics.

Breaking language changes should be rare and called out in [CHANGELOG.md](../CHANGELOG.md). There is **no LTS** yet — pin versions in production.

## Registry / packages

A public registry protocol and signing are **implemented**. Hosting of `registry.weftproject.dev` may still be partial; path/git and monorepo `packages/` always work. Namespace key trust: `weft registry trust` (see [STABILITY.md](STABILITY.md)).

## Before you tag

1. Align version strings above  
2. `go test ./...` and `scripts/ci.sh`  
3. `weft train eval --quiet` still healthy unless gold intentionally changed  
