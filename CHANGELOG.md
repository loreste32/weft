# Changelog

All notable changes to Weft. Releases at [github.com/loreste32/weft/releases](https://github.com/loreste32/weft/releases).

## [0.4.2] — 2026-07-29

### Added
- **Compat corpus expansion** — lock concurrent `map` result order, strings, channels, Result field accessors, list indexing (`testdata/compat/`)
- **Glue benchmarks vs Python** — paired workloads in `testdata/bench/`; `make bench-glue` (wall time); CI checks Weft==Python output parity
- **Go microbenches** — JSON round-trip + string split/join loops in `internal/vm`
- **Reference apps** — `examples/ref_agent_ops`, `ref_http_glue`, `ref_ops` with offline tests in CI
- **Release smoke** — full/slim sizes + multi-GOOS compile (`scripts/release-smoke.sh`)
- **STABILITY.md** — positioning, concurrency caveats, golden/ref process, GH Actions matrix
- **REPL multi-line polish** — continue on trailing operators (`1 +` …); `:cancel` aborts buffer; `:history <text>` filter; `!N` / `:!N` re-run history entry
- **LSP locals** — go-to-definition for params/lets/for-vars; completion includes local bindings; `documentHighlight` for the identifier under the cursor
- **LSP multi-file rename** — top-level fn/type/enum/const renames across open buffers and workspace `.weft` files
- **LSP extract function** — `refactor.extract.function` code action on a selection
- **REPL tab completion** — interactive TTY: Tab for keywords/stdlib/`:`cmds, ↑/↓ history (via `golang.org/x/term`)
- **VS Code extension 0.4.2** — `editors/vscode/weft-0.4.2.vsix` (weft-lsp 0.4.2)

### Fixed
- **Call under-arity** — missing args no longer become null then fail with `numeric op on int and null`; report `wrong number of arguments to name: have N, want M` at the call site
- **Parse diagnostics** — unterminated strings show as such (not `ILLEGAL`); `use pkg::name` gets a clear path hint; missing `}` before `else` is called out; empty `match x {}` parses (compile still requires arms)
- **REPL top-level `fn`/`type`/`const`** — no longer errors with `no main function`; definitions bind into the session env

## [0.4.0] — 2026-07-29

### Added
- **Interactive playground** at [weftproject.dev/playground.html](https://weftproject.dev/playground.html) — try Weft in the browser with 8 examples, share links, server-side sandbox
- **Browser Wasm runtime** — `make wasm` → `wasm/weft.wasm` + `wasm/playground.html` (client-side; network/db/llm packages stubbed)
- **Registry namespace trust** — `weft registry trust|untrust|trusts`, server pins namespace signing keys, `/v1/namespaces.json`
- **Telecom SIP REFER / WebRTC bridge** — blind/attended REFER, bridge_and_refer, WebRTC session refer/originate hooks
- **VS Code extension 0.4.1** — DAP + typed LSP; `editors/vscode/weft-0.4.1.vsix`
- **Reliability foundation** — bytecode `ValidateChunk`, lex/parse/compile fuzz, VM concurrency stress, `make race-smoke|fuzz-smoke|bench`, docs/STABILITY.md
- **`weft doc`** — generate API docs from `pub fn` declarations and doc comments
- **Better error messages** — parse errors now show the source line with a caret pointing at the problem
- **REPL improvements** — `:stdlib`, `:stdlib <pkg>`, `:history`, `:clear`, `:version` commands
- **4 new registry modules:** `http_router` (routing with path params, middleware, groups, CORS), `template` (string templating with placeholders, loops, HTML escaping), `validate` (data validation for forms/APIs), `cron` (recurring task scheduler with intervals and daily times)
- **Playground server** (`cmd/weft-playground`) — sandboxed execution with 5s timeout, 10KB limit
- 14 registry modules total
- 0.3.x line complete → entering 0.4.x

### Changed
- Roadmap updated: 0.3.x marked complete, 0.4.x goals set (LSP refactoring, type system, Wasm, VS Code marketplace)

## [0.3.35] — 2026-07-29

### Added
- **`cluster` stdlib** — distributed state backed by Redis: locks, node registry with heartbeat, rate limiting, atomic counters, pub/sub
- **`governor` stdlib** — token budgeting, cost tracking, execution timeouts for LLM calls
- **`supervisor` stdlib** — Erlang-style process supervision (one_for_one, one_for_all, rest_for_one), actor processes with mailboxes
- **`weft lint`** — static analysis: parse errors, unused imports, trailing whitespace, line length, TODO markers, missing entry points
- 76 stdlib packages total

### Fixed
- `weft update` handles permission denied with temp dir fallback and sudo prompt

## [0.3.34] — 2026-07-29

### Added
- **`weft build`** — produce standalone executables (embeds runtime + script + vendor, no weft needed on target)
- **`weft test --race`** — data race detection for concurrent code
- **`weft test --mem`** — memory allocation tracking per test
- **`weft test --timeout N`** — per-test timeout in seconds
- **`deepgram` stdlib** — streaming STT via WebSocket (Nova-2, interim results, VAD)
- **`elevenlabs` stdlib** — streaming TTS via WebSocket (Turbo v2.5, bidirectional for lowest latency)
- **`mlinfer` stdlib** — ML inference clients for ONNX Runtime, Triton, HuggingFace (classify, embed, detect, batch)
- 73 stdlib packages total
- BUILDING.md with 9 end-to-end app examples

### Fixed
- Registry install extracts archives to vendor/ directly (no broken temp paths)
- Archive extraction detects format by magic bytes, not file extension
- Registry search shows only latest version by default (`--all` for history)
- All docs updated: `weft get` → `weft registry install`

## [0.3.33] — 2026-07-28

### Added
- **`mcp` stdlib** — Model Context Protocol client + server (expose Weft functions as MCP tools for AI assistants)
- **`sysinfo` stdlib** — CPU, memory, disk, uptime, load average, network interfaces
- **`proc` stdlib** — process list, find, kill, exists
- **`netutil` stdlib** — port check, TCP ping, DNS lookup, port scan
- **`telecom` module** — IVA voice agents, FreeSWITCH ESL, Asterisk ARI, STT/TTS, DTMF, routing (DID/time/skills/geo), queues, CDR, dial plan, SSML
- **`weft update`** — self-update binary from weftproject.dev
- **`weft upgrade`** — upgrade installed packages to latest registry versions
- 5 new registry modules: `retry`, `semver`, `color`, `cache`, `jwt`
- Public registry at registry.weftproject.dev with mandatory ed25519 signing, version immutability, name validation
- Website at weftproject.dev with searchable docs (33 pages), cookbook (22 recipes), download page
- APT repo (Ubuntu/Debian) and DNF repo (Fedora/RHEL)
- GitHub Release with binaries for 5 platforms
- Dockerfile for container builds
- macOS binaries ad-hoc signed for Gatekeeper
- `robots.txt`, `sitemap.xml`, Open Graph, JSON-LD for search engines and AI crawlers

### Security
- Registry requires auth token for all publishes
- Registry verifies ed25519 signatures on upload
- Versions are immutable (no overwrites)
- Stdlib/prelude name collisions blocked
- Client rejects unsigned packages by default (`WEFT_ALLOW_UNSIGNED=1` to override)

### Fixed
- Lock file paths updated from old repo name
- Tutorial and security audit paths fixed
- Sum types with payloads moved from "future" to current in roadmap

## [0.3.31] — 2026-07-27

### Added
- `weft debug` — interactive source-level debugger
- `weft profile` — execution profiler
- `weft notebook` — run `.weft` as cells, output HTML
- Sum types with payloads (`enum Shape { Circle(r), Rect(w,h) }`)
- Match with destructuring (`Shape.Circle(r) { ... }`)
- Security re-audit with threat model

## [0.3.30] and earlier

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full history of the 0.3.x line.
