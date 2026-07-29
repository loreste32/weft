# Stability & positioning (0.4.x)

Weft is a **host-powered scripting runtime** for:

1. **AI-agent scripts** (tools, structured LLM output, memory)
2. **HTTP / API glue**
3. **System-operations automation**

It is **not** positioned as a general replacement for Python, Go, or Rust. The credible niche is replacing the stack of *Python agent glue + Bash + small HTTP services + a separate package environment*.

## Honest maturity

| Area | Status |
|------|--------|
| Language concept | Strong (Result/`?`, concurrency without async, single binary) |
| Agent/ops positioning | Strong |
| Implementation breadth | High (language + stdlib + packages + tooling) |
| Ecosystem maturity | Early |
| Production readiness | Usable for **pinned, tested** tools — not multi-tenant sandbox |

Trust model: Weft runs with the privileges of the process. Capability profiles and package signatures reduce risk; they do **not** make Weft a secure multi-tenant isolator. See [SECURITY.md](../SECURITY.md) and [ECOSYSTEM.md](ECOSYSTEM.md).

## Concurrent-by-default collections

`map` / `filter` fan out concurrently. **Result order is preserved** for the returned list. That is intentional for network/IO-bound work.

**Not preserved / not safe to assume:**

- Order of **side effects** inside the callback (logging, I/O, shared counters)
- Outer `mut` reassignment from a closure — captures are **by value**; reassignment errors as const

**Use `seq_map` / `seq_filter`** when the callback is stateful, talks to a shared DB transaction, is rate-limited, or must be deterministically debugged.

## Bytecode validation vs VM safety

- Compiler output is validated (`ValidateChunk`: indices, jump targets must be instruction starts).
- The VM must **not panic** on stack underflow or truncated/misaligned operands — it returns a runtime error.
- Validation does **not** fully prove stack balance; corrupt hand-crafted chunks should still fail safely.
- User-function **under-arity** is a hard runtime error (`wrong number of arguments to f: have N, want M`). Extra args are still ignored (type checker may warn).

## REPL

Session REPL (`weft` / `weft repl`): top-level `fn`/`type`/`const`/`enum` bind without `main`. Multi-line continues for open braces and trailing operators; `:history [filter]`, `!N` re-run, `:cancel` aborts. History: `~/.weft/history`. Line editing still external (`rlwrap`).

## Error-message quality (0.4.x)

Prefer actionable diagnostics over generic token names:

| Situation | Message shape |
|-----------|----------------|
| Too few args to `fn` | `wrong number of arguments to name: have N, want M` |
| Unterminated string | `… got unterminated string` (not bare `ILLEGAL`) |
| Rust-style `use a::b` | `invalid use path: write use a or use "path" (not pkg::name)` |
| Missing `}` before `else` | `expected } before else (missing closing brace?)` |
| Empty `match x {}` | parses; compile: `match has no arms` |

## Compatibility

- **0.4.x** may still break APIs; pin toolchain and package versions.
- Signatures prove *which key* signed a package, not *who* that key belongs to — use `weft registry trust` / `WEFT_REQUIRE_TRUST=1`.
- No LTS promise yet.

## Reliability focus (current priority)

Prefer proving the core over adding packages:

1. Fuzzing (lex, parse, compile→validate)
2. Bytecode structural validation
3. Concurrency stress (`go test -race`)
4. Cross-platform release smoke
5. Spec-like language docs kept in sync with `weft version`
6. Benchmarks vs Python for glue scripts (`make bench` + `make bench-glue`)
7. Binary size: **`go build -tags slim`** / `make build-slim` (~19MB vs ~42MB; no SQL/brokers)

## Compatibility corpus

Pinned scripts: `testdata/compat/*.weft` + matching `*.out`.  
CI runs `go test ./pkg/weft/ -run TestCompatCorpus`. Extend when locking behavior; do not silently weaken goldens.

Current cases lock: arith, closures, hello, json round-trip, `seq_map`/`seq_filter`, match/enum, `Result`/`?`, optional types, **concurrent `map` result order**, strings, channels, Result field accessors, list indexing.

### Adding a golden

1. Write `testdata/compat/foo.weft` (must have `fn main`, offline, deterministic).  
2. `weft run testdata/compat/foo.weft > testdata/compat/foo.out`  
3. `go test ./pkg/weft/ -run TestCompatCorpus` and `go test ./internal/format/ -run TestCompatFormatRoundTrip`  
4. Note the behavior in CHANGELOG if it locks a semantic.

## Glue benchmarks

| Target | What |
|--------|------|
| `make bench` | Go microbenches (`internal/vm`, compile) — fib, map, JSON, strings |
| `make bench-glue` | Wall-time Weft vs Python3 on paired scripts in `testdata/bench/` |

Paired workloads (must print **identical** output):

| Name | Shape |
|------|--------|
| `json_roundtrip` | 5k JSON stringify/parse loops |
| `seq_map` | sequential map over 20k ints |
| `str_split_join` | split/join/upper loops |
| `fib` | recursive fib(28) — language core, not IO glue |

CI gates **output parity** only (not wall times). For statistical timing:

```bash
hyperfine -w 2 -r 5 \
  './weft run testdata/bench/json_roundtrip.weft' \
  'python3 testdata/bench/json_roundtrip.py'
```

Expect: pure recursion (fib) slower than CPython; JSON/string glue closer. Numbers vary by host — do not treat a single run as a SLA.

Illustrative one-shot on Apple M4 (Darwin arm64, Weft 0.4.1) — order-of-magnitude only:

| Workload | Weft | CPython |
|----------|------|---------|
| json_roundtrip (5k) | ~40ms | ~45ms |
| seq_map (20k) | ~55ms | ~30ms |
| str_split_join (8k loops) | ~120ms | ~35ms |
| fib(28) | ~240ms | ~50ms |

JSON glue is competitive; recursion and string-heavy loops still lag. That is expected for a young VM.

### Reference apps

| App | Role |
|-----|------|
| `examples/ref_agent_ops` | CLI + pure pipelines + optional LLM |
| `examples/ref_http_glue` | JSON/HTTP client glue |
| `examples/ref_ops` | Host facts, env redaction, path checks |

Each has a README threat model and `*_test.weft`. CI runs `weft test` on all three.

## Namespace key rotation

| Side | How |
|------|-----|
| Registry | First publish pins `namespaces/<ns>.json`. Rotate: `POST /v1/namespaces/<ns>/keys` with Bearer token and `{"public_key":"…","retire":"…optional"}`, then publish with the new key. |
| Client trust | `weft registry trust-rotate <ns> <new-pubkey> [--retire <old>]` |

## Commands

```bash
# structural + unit
go test ./internal/compile/ ./internal/vm/ ./internal/parse/ ./internal/lex/ -count=1

# short fuzz (also in scripts/ci.sh)
make fuzz-smoke

# race smoke (also in scripts/ci.sh)
make race-smoke

# compatibility goldens
go test ./pkg/weft/ -run TestCompatCorpus

# bytecode always validated at end of compile
weft check examples/hello.weft

# slim binary + multi-GOOS compile check
make build-slim
make release-smoke

# reference apps (pinned patterns)
weft test examples/ref_agent_ops
weft run examples/ref_agent_ops/main.weft -- status
weft test examples/ref_http_glue
weft run examples/ref_http_glue/main.weft -- demo
weft test examples/ref_ops

# glue benches (local; CI checks Weft==Python outputs only)
make bench
make bench-glue
```

## GitHub Actions

Workflow: `.github/workflows/ci.yml`

| Job | When | What |
|-----|------|------|
| `ci` | every PR/push | full `scripts/ci.sh` (incl. bench pair parity) |
| `reliability` | every PR/push | race, fuzz, compat, format roundtrip, slim, refs, bench parity |
| `release-smoke` | every PR/push | full/slim sizes + GOOS compile matrix |
| `fuzz-deep` | weekly schedule + manual | 2m fuzz each target |

Format roundtrip: `go test ./internal/format/ -run TestCompatFormatRoundTrip`.
