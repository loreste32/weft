# Reference app: agent + ops

A small, **production-shaped** Weft tool — not a feature dump.

## What it demonstrates

| Concern | Choice |
|---------|--------|
| Errors | `-> Result` + `?` |
| Concurrency | `map` only for **pure** transforms; no outer `mut` in callbacks |
| LLM | Optional; skipped without API key (CI-friendly) |
| Trust | Host-powered runtime — run only on trusted machines |
| Pinning | Document `weft version` in CI; pin this directory |

## Threat model (honest)

- **Not** multi-tenant safe. Runs with your user privileges.
- Reads local files you pass on the CLI.
- Does not shell out by default.
- If you add `sh`/`llm` later, treat inputs as untrusted.

## Commands

```bash
# from repo root (weft on PATH)
weft version   # pin this in CI

weft run examples/ref_agent_ops/main.weft -- status
weft run examples/ref_agent_ops/main.weft -- hash README.md
weft run examples/ref_agent_ops/main.weft -- summarize README.md
weft run examples/ref_agent_ops/main.weft -- pipeline README.md
```

## Tests

```bash
weft test examples/ref_agent_ops
# or
go test ./examples/ref_agent_ops/ -count=1   # if harness present
```

`*_test.weft` exercises status/hash/pipeline offline.

## Slim binary

This app works on the **slim** build (no SQL/brokers):

```bash
go build -tags slim -o weft-slim ./cmd/weft
./weft-slim run examples/ref_agent_ops/main.weft -- status
```
