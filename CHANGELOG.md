# Changelog

All notable changes to Weft. Releases at [github.com/loreste32/weft/releases](https://github.com/loreste32/weft/releases).

## [0.5.1] — 2026-08-04

### Added
- **DataFrame multi-key semantics** — `group_by` accepts a list of key columns and per-column agg lists (`{"salary": ["sum","mean"]}`); new `pivot_table` with real aggfuncs over duplicate cells + `fill_value`; new `rank_opts` (average/min/max/dense/first, na_option, pct, direction); `merge` accepts multi-key `left_on`/`right_on` lists and `merge_opts` adds `suffixes`. Composite group keys use type-tagged JSON encoding (collision-proof against separator/type-punning values). 25 new adversarial tests (118 total).
- **NumPy dtype-promotion parity** — both promotion tables (Go `promoteDType`, Weft `_promote_dtype`) now match pinned NumPy 2.4.3 `promote_types` for all 121 dtype pairs (mixed signed/unsigned same-width → next-width signed; float32 survives only vs bool/≤16-bit ints), locked by an exhaustive differential fixture and a Go matrix test; `int8+uint8→int16` etc.
- **Truthful accelerator execution reporting** — providers now report whether an op ran on the requested device, fell back, is unavailable, or failed to report: typed `ExecInfo` in `internal/accelerator`, additive `weft_accel_exec_info` ABI export (implemented by the CPU reference and the CUDA/ROCm/MLX providers), `accelerator.last_exec_info` in the stdlib, and `ml.device_with_plugin`/`ml.exec_info` surfacing it to Weft code. Conformance is mandatory and adversarial: missing fields classify as `unreported`, contradictory device/fallback claims fail as `contradictory`, both failing `scripts/accelerator-conformance.sh`.
- **sklearn differential + real property conformance** — `testdata/conformance/ml_case.weft` compares `ml.linear_fit`/`ml.logistic_fit` against scikit-learn 1.9.0 (pinned) with documented GD-vs-closed-form tolerances; the old oracle-only property smoke was replaced by 25 seeded property cases that actually execute Weft (broadcasting, reductions on views, matmul, reshape/transpose, comparisons) at 1e-10 against NumPy.
- **Browser WASM adversarial suite** — `wasm/playwright_adversarial_test.js` runs against real local endpoints (node http + raw sockets) on Chromium and Firefox: deceptive/malformed Content-Length, gzip expansion past the 32 MiB limit, slow streaming, redirects, 10× repeated timeouts with abort-cleanup assertions, request-body pre-send rejection (server sees zero bytes), and virtual-fs quota/reclamation lifecycle; wired into `npm run test:browser` so CI runs it unchanged.
- **ESL/ARI adversarial harness** — black-box process tests for server-initiated close (mid-command and mid-event-wait), byte-level TCP fragmentation, coalesced frame boundaries, 10-way command concurrency, and a 260-command flood; first ARI mock test suite (REST + WebSocket round-trips, auth failure, malformed-event survival, clean shutdown).
- **Reproducible-build and supply-chain gates** — `scripts/reproducible-build-check.sh` (offline `GOPROXY=off` install after `go mod download` + `go mod verify`, byte-identical builds from two checkout paths with `-trimpath -buildvcs=false`) and `scripts/sbom.sh` (pinned dependency SBOM); both wired into release smoke and the release workflow, which now also publishes `SBOM.json`.
- **Capability-matrix freshness gate** — `python3 scripts/capability-matrix.py --check` fails CI when committed `reports/capability-matrix.{md,json}` drift from the generator (`make capability-matrix` now refreshes both files).
- **Cross-platform and Docker CI** — macOS/Windows jobs now execute the core unit suites, compat corpus, and a build+run smoke (previously cross-compiled, never run); a `docker-image` job builds the Dockerfile and smoke-tests `version`/`doctor`/`run` in the container.
- **Live broker CI** — `brokers-live` job runs NATS/RabbitMQ/MongoDB service containers with `WEFT_LIVE_REQUIRED=1` (unreachable broker = failure, never a hidden skip); new live tests cover AMQP declare/publish/consume and Mongo insert/find/count/delete round-trips.
- **Published compatibility deviations** — `docs/COMPATIBILITY.md` documents pinned oracle versions, platform requirements, and the declared warp/dataframe/ml unsupported surface and deliberate deviations.
- **Host packed tensors** — `internal/tensor` plus stdlib `tensor` provide typed, strided host storage with integer and float dtypes, a free-list memory pool (`Acquire`/`Release`), and Warp primary numeric storage (`storage_kind` reports `"tensor"` vs `"list"`).
- **Accelerator trust model** — optional fail-closed plugin loads via `WEFT_ACCELERATOR_DISABLE`, `WEFT_ACCELERATOR_ALLOWLIST`, `WEFT_ACCELERATOR_REQUIRE_CHECKSUM`, and `WEFT_ACCELERATOR_CHECKSUM` (or a `<plugin>.sha256` sidecar); native providers still require an explicit path and accelerator capability.
- **Accelerator capability reports** — `make accelerator-report` / `publish-accelerator-report` emit JSON and markdown under `reports/` with fallback policy, optional numerical benches, and honest unavailable provider status without GPUs (`docs/ACCELERATORS.md`).
- **CPU numerical and scale benches** — `bench-numerical` and `bench-scale` (100k-row dataframe groupby/sort smoke) record wall times for local and CI-adjacent comparison.
- **Vendor sync tooling** — `make vendor-sync` / `vendor-check` keep example vendor trees aligned with `packages/warp` and `packages/ml`.
- **DataFrame ↔ ML boundary** — DataFrames can copy validated numeric columns into packed Warp arrays, and ML classical trainers accept packed Warp feature/target arrays for an explicit CPU pipeline.
- **Conformance expansion** — NumPy/pandas differential fixtures (Warp edges, strides, dtype promotion, errors, reductions; dataframe index/missing) via `scripts/conformance/run.py` (10 fixtures + property smoke).
- **Warp NumPy surface** — host-tensor `add`/`mul`/`matmul` fast path; `argmin`/`argmax`; `nansum`/`nanmean`; `atleast_*`; `broadcast_to`; `pad`; `isclose`; **1D FFT/IFFT** (`fft_1d`/`ifft_1d`/`fft_freq`, power-of-2 Cooley–Tukey or naive); `diff`/`gradient`/`trapz`.
- **DataFrame multi-level + groupby** — `set_multi_index`, `swaplevel`, `droplevel`, multi-key `loc_labels`/`reindex`; **`group_by_transform`**, **`group_by_size`**; rolling/expanding retained.
- **ML nested reverse-mode (scalar)** — `backward(node, create_graph)` builds differentiable scalar VJPs so double-backward yields exact second derivatives; `grad_fn`; finite-diff HVP retained; advisory device tags with honest CPU fallback.
- **Release gates** — `accelerator-conformance` (CPU plugin JSON+tensor), `capability-matrix` report, multi-fixture `bench-scale` with soft budgets (250k DF rows default; 1M via env).
- **cron package documentation** — channel control protocol and deep-copy capture limits.

### Fixed
- **Float→int cast silent saturation** — `warp._cast_value` range-checked after `floor`/`ceil` had already saturated to int64 extremes, so `1e30` → int64 silently became a clamped value; now returns `Err` pre-truncation, matching NumPy 2.4.3 `OverflowError`.
- **`ml.device()` silently broken** — `type_of` returns `"str"` but device.weft checked `"string"`, so `ml.device("cpu")` always errored and `?` silently aborted the non-Result test fns (masked by the test-runner hole, now closed); also fixed missing-map-key hard errors in probe helpers and a double-wrapped `Ok(Ok(...))` return.
- **`accelerator.load` manifest unusable from Weft** — `goToValue` stringified the manifest struct; now a real map.
- **Browser fetch error messages** — network-layer failures surfaced as `Err(... "<object>")` because the reject callback stringified the JS Error object; the real `.message` is now propagated.
- **Test runner silent-pass** — a `test_*` fn that returned an `Err` Result (including `?` propagation) was reported as passed; the runner now fails it with the carried message, documented in `docs/TESTING.md`. Sweep of all in-repo suites found zero tests that had been silently passing.
- **ARI client was broken end-to-end** — `packages/telecom/ari.weft` had never compiled (immutable-binding reassignments), every REST call used a nonexistent `http.fetch(url, opts)` form, and `listen` called a nonexistent `ws_conn.read()`; fixed and now covered by the mock ARI suite.
- **ESL request-queue cap enforced** — the documented 256 pending-request limit was never checked; overflow now returns an explicit "too many pending requests" error instead of unbounded growth.
- **Native HTTP request-body limit** — `http.post`/`put`/`patch`/`fetch` now reject bodies over 32 MiB before sending (browser parity), and the 32 MiB response limit has adversarial tests (over-limit → explicit `Err`, exact-boundary succeeds).
- **Dockerfile clean-clone build** — dropped `COPY vendor/` (vendor is gitignored; the file was unbuildable from a fresh checkout), added `go mod verify`, reproducible-build flags, and an in-build `weft version` smoke; new `.dockerignore` shrinks the build context.
- **Numerical runtime ownership and dtype safety** — pooled tensor temporaries are released on error/strided native calls; fixed-width dtypes preserve integer precision; Warp exposes explicit packed-handle release; ML finite-difference probes restore parameters on failure.
- **cron concurrency** — jobs no longer rely on mutating a shared stats map across `spawn`; each job owns counters and answers `stop` / `stats` / `close` over a command channel.
- **cron exports** — `weft.json` lists `close` alongside `every`, `at`, `schedule`, `stop`, `stop_all`, `stats`, and `wait`.
- **dataframe `weft check`** — `describe` / `rank` no longer call host `sort` (undefined under module check); use package-local `_sort_values` merge-sort instead.

## [0.4.10] — 2026-08-03

### Added
- **Browser WASM runtime** — async execution, browser Fetch HTTP, virtual filesystem, browser-safe OS helpers
- **WASM filesystem hardening** — path traversal returns explicit errors (not silent alias), 4096-char path limit, 10k file cap, 5k dir cap, 15k total entry limit
- **Browser CI** — Playwright-based integration tests with Chromium and Firefox
- **Module tests** — 259 total Weft test cases across 22 modules (12 previously untested: cache, color, config, http_router, jwt, logger, metrics, queue, retry, semver, template, validate)

### Security
- **Self-update checksum verification** — `weft update` downloads `checksums.txt` and verifies SHA-256 before replacing the binary; rejects mismatches with "possible tampering" error
- **Install script checksum verification** — `install.sh` verifies SHA-256 using sha256sum/shasum before installing
- **netsafe HTTP client** — all update and registry HTTP uses `netsafe.SafeHTTPClient` with SSRF protection (was raw `http.Client`)
- **Removed unused URL field** from `VersionInfo` struct (prevented future server-controlled download URL misuse)

### Fixed
- **8 module source bugs found by new tests**: logger/queue/config empty-map field access crashes, queue/config list append with `+` operator, semver parameter reassignment, retry immutable bindings, color nonexistent `re.replace_all`
- **WASM path traversal** — `../secret` now returns error instead of silently becoming root
- **Deep fuzz** — cache cleaned between targets to prevent baseline timeout

## [0.4.9] — 2026-08-02

### Changed
- **Socket deadlines**: `conn.read()` no longer installs a hidden 60-second deadline that overrides caller-set deadlines — the critical ESL timeout fix
- **Socket deadline API**: `set_read_deadline`, `set_write_deadline`, `read_timeout`, `write_timeout` accept fractional seconds (e.g. 0.05s for sub-second ESL timeouts)
- **Socket deadline safety**: NaN, Inf, negative, zero, and overflow values are all rejected with clear errors
- **Socket deadline versioning**: temporary deadlines (`read_timeout`, `write_timeout`, `read_all_timeout`) track versions and restore the caller's previous deadline correctly
- **Socket locking**: `close()` does not acquire read mutex (interrupts blocked reads immediately); deadline setters use `deadlineMu` for safe concurrent access
- **ESL Content-Length validation**: strict digit-only parser with duplicate/negative/float/oversized rejection
- **ESL header normalization**: case-insensitive matching for Content-Type, Content-Length, Reply-Text
- **ESL dispatcher**: single-reader coordinator architecture with command/event channel separation
- **ESL timeout**: absolute deadline, no restart on non-event frames, always cleared on exit
- **Auth validation**: salt/hash hex encoding and length checked; Argon2 version 19 enforced; excessive params rejected
- **Router**: dispatch() extracted for testability; wildcard prefix validation; URL-decoded params; HEAD strips body; OPTIONS includes HEAD+OPTIONS in Allow; 405 includes Allow header

### Added

- **Browser WASM runtime**: async `runAsync()` execution with direct deadline enforcement, browser Fetch-backed HTTP, bounded virtual filesystem, browser-safe OS helpers, robust loader fallback, and CI-backed Node/WASM integration tests
- **Go-level ESL wire fixture tests**: Content-Length framing, CRLF support, coalesced frames, deadline honoring, black-box process dispatcher test
- **Socket regression tests**: deadline honoring (~1s not 60s), concurrent read+write, close interrupts blocked read, clear_read_deadline restores blocking
- **Crypto limit tests**: Argon2id/PBKDF2 parameter bounds at Go level
- **Auth tests**: 20 Weft tests covering hash/verify roundtrip, PBKDF2, unknown algorithm rejection, malformed records, version validation, needs_rehash, HMAC, Basic/Bearer auth
- **Router tests**: dispatch middleware ordering and short-circuit, HEAD/OPTIONS/405/Allow, group+wildcard with URL decoding
- **Warp tests**: det/inv validation for non-square, solve matrix RHS row-major, inv roundtrip 3x3
- **CI**: dedicated telecom-parser and telecom-dispatcher jobs
- 1406 Go tests total

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
