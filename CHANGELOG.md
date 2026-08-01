# Changelog

All notable changes to Weft. Releases at [github.com/loreste32/weft/releases](https://github.com/loreste32/weft/releases).

## [0.4.8] — 2026-08-01

### Added
- **crypto.argon2id** — Argon2id password hashing (Go native binding via golang.org/x/crypto)
- **crypto.pbkdf2** — PBKDF2-HMAC-SHA-256 key derivation (Go native binding)
- **auth** module upgraded — `hash_password` now uses Argon2id by default; `hash_password_pbkdf2` added; `verify_password` auto-detects algorithm (argon2id/pbkdf2/legacy sha256-10k)
- **ESL frame parser** — telecom ESL rewritten with proper Content-Length framing, partial-read buffering, command/event separation, and queued async event delivery
- **warp LU decomposition** — `det`, `inv`, `solve` now work for any size matrix (was limited to 2x2/3x3); O(n³) with partial pivoting
- **Maturity labels** — every stdlib package (81) and registry module (23) now has an explicit `experimental`/`beta`/`stable` designation via `StdlibMaturity()` and `weft.json` `"maturity"` field
- **Supply-chain tests** — auto-fetch local-wins, redirect-to-private-IP rejection, private git URL rejection, maturity field parsing
- **Weekly deep fuzz** — CI fuzz-deep job bumped from 2 minutes to 5 minutes per target (20 min total)
- **Benchmark publishing** — CI publishes Go benchmark and Weft-vs-Python glue results as artifacts on every push to main
- 4 new Go tests, 6 new Weft tests (1378 Go tests, 68 warp tests, 62 dataframe tests)

### Changed
- auth: password hashing upgraded from iterative SHA-256 to Argon2id with stored parameters
- auth: `verify_password` rejects unknown algorithms (was silent SHA-256 fallback)
- auth: bounded Argon2id memory (1MB-4GB) and PBKDF2 iterations (10k-10M) to prevent DoS
- auth: `needs_rehash()` detects outdated password parameters
- warp: det/inv/solve generalized via LU decomposition (2x2 fast path preserved)
- warp: solve() matrix RHS properly assembled row-major (was column-major)
- warp: det/inv/solve validate square 2D input and data/shape consistency
- telecom/esl: frame-level protocol handling with Content-Length, \r\n\r\n support
- telecom/esl: memory limits — 64KB headers, 10MB body, 128 headers, 10k event queue, 16MB buffer
- telecom/esl: outbound_server passes initial read into frame parser (no data loss)
- router: wildcard matching validates prefix segments before accepting
- router: query strings stripped, trailing slashes normalized
- router: HEAD matches GET routes, OPTIONS returns 204 with Allow header
- router: 405 for path-match/method-mismatch (was 404)
- stdlib: unknown packages default to "experimental" maturity (was "stable")

## [0.4.7] — 2026-07-31

### Added
- **5 new stdlib packages** (81 total):
  - `encoding` — hex, base32, URL encoding/decoding
  - `compress` — gzip/gunzip, deflate/inflate with 100 MiB decompression limit
  - `dns` — full DNS client: A/AAAA, SRV, CNAME, NS, MX, TXT, reverse (PTR)
  - `tls` — certificate inspection, chain verification, expiry monitoring
  - `os` — unified OS operations: env, paths, user info, filesystem, platform
- **5 new registry modules** (23 total):
  - `auth` — HMAC signing, password hashing, token generation, Basic/Bearer parsing, OAuth helpers
  - `queue` — in-process job queue with workers, retries, dead-letter, priority levels
  - `config` — unified config loader for .env/JSON/YAML/TOML with env overlay, dot-path access, validation
  - `logger` — structured logging with levels (debug→fatal), JSON/text output, context fields, child loggers
  - `router` — HTTP routing with path params, middleware chains, groups, CORS, rate limiting, auth middleware
- **`weft bench --compare`** — compare bench results against a saved baseline; `--save` exports JSON
- **`weft outdated`** — check installed packages against the registry for newer versions
- 14 new tests (1374 total across 21 packages)

## [0.4.6] — 2026-07-30

### Added
- **stdlib live probe** — `weft stdlib sysinfo.memory` now runs the function and shows human-readable output (bytes → GiB, labeled fields)
- **sysinfo enriched** — memory/disk return human-readable sizes, units, and scoped labels
- **proc hardening** — rejects dangerous inputs (PID 0, negative signals, blank search)
- **mlinfer hardening** — URL validation, response size cap (32 MiB), input sanitization
- 18 new tests (1360 total across 21 packages)

## [0.4.5] — 2026-07-30

### Added
- **`weft info`** — comprehensive system report: memory, disk, uptime, load, network interfaces, configured services, weft version
- **`weft sysinfo`** — alias for `weft info`
- **Helpful CLI suggestions** — typo a command and get the right one (`weft search` → `weft registry search`, 30+ mappings)
- **`weft stdlib` grouped by category** — LLM/AI, Web, Data, DevOps, Network, etc.
- **`weft stdlib <pkg>`** — shows signatures and descriptions for all members
- **`weft stdlib <pkg.member>`** — shows signature, description, and live output
- **REPL** — `help`, `exit`, `quit` work without colon prefix
- **Help entries for all 76 stdlib packages** — sysinfo, proc, netutil, mcp, deepgram, elevenlabs, mlinfer, governor, supervisor, cluster, and 30+ more

## [0.4.4] — 2026-07-29

### Added
- **Release workflow** — `.github/workflows/release.yml`: pushing a `v*` tag builds full + slim binaries for 5 platforms with SHA256 checksums, packages the VS Code VSIX, and publishes a GitHub Release; the tag must match `pkg/weft.Version`
- **VS Code extension 0.4.3** — `editors/vscode/weft-0.4.3.vsix` (matches language 0.4.3)

### Fixed
- **Wasm version drift** — the playground build derives its version from `pkg/weft.Version` via ldflags (was hardcoded `0.4.1-wasm`)

## [0.4.3] — 2026-07-29

### Added
- **Grouped imports** — `use { "mold" "telecom" "cache" }`
- **Registry auto-fetch** — a package missing from `vendor/` is downloaded from the registry on first `use`; disable with `WEFT_NO_AUTO_FETCH=1`
- **Third-party git imports** — `use "github.com/user/repo"` auto-clones into `vendor/`; URL imports like `use "weftproject.dev/mold"` extract the package name
- **LSP auto-import** — code action adds the `use` statement when you type `pkg.member`; covers all 76 stdlib packages and the registry modules

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
- **DAP debugger** — `weft debug --dap` speaks the Debug Adapter Protocol; VS Code adapter wired in `editors/vscode`

### Fixed
- **Call under-arity** — missing args no longer become null then fail with `numeric op on int and null`; report `wrong number of arguments to name: have N, want M` at the call site
- **Parse diagnostics** — unterminated strings show as such (not `ILLEGAL`); `use pkg::name` gets a clear path hint; missing `}` before `else` is called out; empty `match x {}` parses (compile still requires arms)
- **REPL top-level `fn`/`type`/`const`** — no longer errors with `no main function`; definitions bind into the session env

## [0.4.1] — 2026-07-29

### Added
- **LSP find-references** — `textDocument/references` (find all usages of an identifier)
- **Telecom SIP REFER** — blind + attended transfer via FreeSWITCH (`sip_refer`)
- **Telecom WebRTC bridge** — `webrtc_signal_server` (browser-to-SIP signaling), `click_to_call` (originate + bridge via ARI)
- **VS Code extension 0.4.1** — `editors/vscode/weft-0.4.1.vsix`
- **telecom module 0.3.0** published to the registry

## [0.4.0] — 2026-07-29

### Added
- **Interactive playground** at [weftproject.dev/playground.html](https://weftproject.dev/playground.html) — try Weft in the browser with 8 examples, share links, server-side sandbox
- **Browser Wasm runtime** — `make wasm` → `wasm/weft.wasm` + `wasm/playground.html` (client-side; network/db/llm packages stubbed)
- **Registry namespace trust** — `weft registry trust|untrust|trusts`, server pins namespace signing keys, `/v1/namespaces.json`
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
