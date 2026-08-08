# Where we are, and where we hope to go

Weft is for agent scripts, telecom, HTTP glue, and ops tooling. It stays small on purpose.

## Where we are now (0.6.0)

Weft is on the **0.6.x** line (0.3.x complete — see [VERSIONING.md](VERSIONING.md)). Positioning and maturity: [STABILITY.md](STABILITY.md). You can build the binary, write real scripts, and run them on a single Go runtime.

**Language**

- Lex → parse → compile → stack VM  
- Own syntax (`:=`, `mut`, `use`, `say`, `?`, match, defer, enum)  
- **Closures** capture outer locals by value (deep-copied — safe under concurrency)  
- **Sum types with payloads**: `enum Shape { Circle(r), Rect(w,h) }` + destructuring in `match`  
- Errors via `Result` + `?` (no try/catch)  
- Concurrency without `async`/`await` (map/filter fan-out, spawn, channels, race, timeout)  
- Scientific floats (`1e-6`), hex/bin/oct ints, digit separators  

**83 stdlib packages** (in the binary)

- LLM: `llm` (OpenAI/Anthropic/Ollama/vLLM), `ollama`, `vllm` — chat, tools, streaming, agents  
- AI integration: `mcp` (Model Context Protocol client + server), `deepgram` (streaming STT), `elevenlabs` (streaming TTS), `mlinfer` (ONNX/Triton/HuggingFace inference)  
- Web: `http`, `web` (HTMX, SSE, cookies, `app.before`), `ws`, `webrtc`  
- DevOps: `sysinfo` (CPU/memory/disk/uptime), `proc` (process list/kill), `netutil` (port scan/DNS/TCP ping), `sh`, `fs`, `signal`, `secrets`, `log`  
- Data: `db` (SQLite/Postgres/MySQL), `csv`, `json`, `jsonl`, `yaml`, `toml`, `xml`, `ini`, `redis`, `mongo`, `nats`, `amqp`, `graphql`  
- CLI/ops: `cli` (flags, subcommands), `env`, `platform`, `shlex`, `crypto`, `pcap`, `email`, `socket`  
- Collections: `str`, `math`, `time`, `re`, `iter`, `collections`, `heap`, `bisect`, `pipe`, `functools`, `copy`, `traceback`  
- Full list: `weft stdlib`  

**23 registry modules** at [registry.weftproject.dev](https://registry.weftproject.dev)

| Module | What |
|--------|------|
| `telecom` | IVA voice agents, FreeSWITCH ESL, Asterisk ARI, STT/TTS, DTMF, routing, queues, CDR |
| `mold` | Structured LLM JSON, validation, JSON Schema, tool params |
| `ml` | Embeddings, vectors, RAG index, metrics |
| `tokensave` | Context thrift, memory, teach → train export |
| `warp` | N-dimensional array math |
| `retry` | Exponential backoff for flaky operations |
| `semver` | Semver parsing, comparison, constraints |
| `cache` | In-memory key-value cache with TTL |
| `color` | ANSI terminal colors for CLI tools |
| `jwt` | JWT token decode and inspection |
| `http_router` | Routing with path params, middleware, groups, CORS |
| `template` | String templating with placeholders, loops, HTML escaping |
| `validate` | Data validation for forms/APIs |
| `cron` | Recurring task scheduler |
| `auth` | HMAC, password hashing, tokens, OAuth helpers |
| `queue` | In-process job queue with retries, dead-letter |
| `config` | Unified config: .env/JSON/YAML/TOML with validation |
| `logger` | Structured logging: levels, JSON/text, child loggers |
| `router` | HTTP routing, path params, middleware, CORS |

Plus 4 local ML-stack packages in `packages/` (`dataframe`, `embed`, `experiment`, `metrics`) — install via path/git.

**Registry and packages**

- Public registry hosted at **registry.weftproject.dev** with web UI  
- Mandatory ed25519 signing on all publishes  
- Version immutability (no overwrites)  
- Capability sandboxing for third-party packages  
- `weft registry search|info|install|keygen|keys|serve`  
- `weft publish --key <name>` with signature verification  

**Tooling**

- `weft check [--types]`, `test [--coverage]`, `fmt [--check]`, `bench`  
- `weft debug <file>` — debugger · `weft profile <file>` — profiler  
- `weft notebook <file> [-o out.html]` — cells to HTML  
- `weft mcp serve <file>` — expose functions as MCP tools  
- `weft update` — self-update binary · `weft upgrade` — upgrade packages  
- `weft gen "task" -o out.weft` — LLM generates Weft from English  
- `weft train prepare|finetune|eval` — private fine-tuning pipeline  
- LSP: completion, hover, signatures, definition, references, rename, extract-function, auto-import, diagnostics, format  
- VS Code and JetBrains editor plugins  

**Distribution**

- Website: [weftproject.dev](https://weftproject.dev) with docs, cookbook, download, registry  
- One-line install: `curl -fsSL https://weftproject.dev/install.sh | sh`  
- APT repo (Ubuntu/Debian): `apt install weft`  
- DNF repo (Fedora/RHEL): `dnf install weft`  
- Homebrew formula  
- Dockerfile for containers  
- GitHub Release with binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 — automated on `v*` tags (`.github/workflows/release.yml`)  
- macOS binaries ad-hoc signed for Gatekeeper  

**Still rough or incomplete**

- Type checking is gradual, not a full sound system  
- `weft fmt` covers the common style; still not every edge case  
- LSP is usable daily; not IDE-grade refactoring  
- Stdlib is broad-and-shallow: good for glue, not a full OS platform  
- Binary is convenience-first (~40MB with drivers); not a minimal embed  
- Concurrent-by-default `map`/`filter` need discipline (`seq_map` for stateful work)  
- Package signatures prove key identity, not human ownership (trust store helps)  
- DAP `evaluate` resolves identifiers only — no expression evaluation, `setVariable`, or restart yet  
- Windows `sysinfo` memory/disk return "not implemented"; Unix is full  
- Browser Wasm playground supports the language core, browser `fetch`, bounded virtual `fs`, and async execution; host-only packages return explicit capability errors
- Slim build stubs the db/broker packages (clear runtime error, but no build-time warning)  
- CI executes on Linux only; macOS/Windows are cross-compiled, never run  

In one line: **useful for agents, telecom, and ops scripts when versions are pinned and tested; not a finished ecosystem.**

## Where we hope to go

The **0.3.x line is complete** (0.3.31–0.6.0). Everything shipped.

**Completed in 0.3.x:** changelog page, `weft doc`, `weft lint`, `weft build`, `weft test --race/--mem/--timeout`, `cluster`/`governor`/`supervisor` stdlib, `deepgram`/`elevenlabs`/`mlinfer`, MCP, telecom with FreeSWITCH/Asterisk, website with 36 doc pages. (0.4.0 then added the `http_router`, `template`, `validate`, `cron` registry modules — 23 total.)

## 0.6.x — make it solid

**Shipped (0.4.0–0.6.0):** optional type annotations + `--strict`, DAP debugging, browser Wasm playground, registry namespace trust, telecom SIP REFER / WebRTC bridge, VS Code 0.6.0 (LSP types + DAP), bytecode validation, fuzz/race/bench smoke targets, grouped imports, registry auto-fetch, third-party git imports, LSP references/rename/extract/auto-import, REPL tab completion + multi-line polish, compat corpus expansion, glue benchmarks vs Python, reference apps, tag-triggered release workflow, crypto.argon2id + crypto.pbkdf2, ESL Content-Length frame parser, LU decomposition for warp det/inv/solve, maturity labels for all 81+23 packages, supply-chain tests, benchmark CI publishing.

**Reliability (priority now — prove the core):**
- Language/VM fuzzing and malformed-input testing (`make fuzz-smoke`) — done (smoke + weekly deep)  
- Race detector + concurrency stress (`make race-smoke`) — done  
- Cross-platform reproducible releases — done (`make release-smoke` + tag-triggered `.github/workflows/release.yml` publishing binaries, checksums, and the VSIX)  
- Compatibility / gold corpus discipline — done (`testdata/compat`, still expand)  
- Benchmarks vs Python for glue scripts — done (`make bench` + `make bench-glue`)  
- Optional stdlib build tags / binary size — done (`make build-slim`)  
- Formatter + LSP edge cases — partial (format corpus green; locals + multi-file rename + extract)  
- Error-message hardening — partial (arity, Illegal lit, use::, else-brace, empty match)

**Language maturity:**
- Harden error messages further (more edge cases)
- REPL: multi-line, history, tab completion / ↑↓ — done (TTY); pipes still Scanner-based

**IDE & tooling:**
- LSP: locals, multi-file rename, extract-function — done  
- VS Code extension 0.6.0 VSIX packaged; Marketplace publish needs `VSCE_PAT`

**Release & platform gaps (next):**
- macOS + Windows CI runners — today those targets are cross-compiled, never executed  
- Dockerfile built and smoke-tested in CI (currently unverified)  
- Windows `sysinfo` memory/disk implementation  
- DAP: real expression evaluation, `setVariable`, exception breakpoints  
- Live-broker test coverage for `amqp`/`mongo`/`nats` ([STDLIB_GAPS.md](STDLIB_GAPS.md))  
- LSP tests for rename / references / extract / auto-import  
- Bring `install.sh` / APT / DNF / Homebrew packaging automation in-repo (maintained out-of-tree today; this repo alone cannot reproduce those channels)  
- Audit registry.weftproject.dev contents against `packages/` — publish any of the 23 modules that are missing  

**Scale & adoption:**
- Key rotation policy for namespaces  
- More telecom (SIP REFER already partially in-module)  
- More production-quality reference apps (initial set shipped in 0.4.2; polish and expand)  

## Numerical, dataframe, and native-runtime replacement program

This is the authoritative remaining-work checklist for the Python numerical and
tabular replacement program. The existing `warp` and `dataframe` packages are
useful validated subsets; they are not yet drop-in replacements for all of
NumPy or pandas. Until the gates below are met, documentation must say
“NumPy-style,” “pandas-inspired,” or name the supported subset.

### Replacement definition

“100% replacement” means that every API and behavior in Weft’s declared
compatibility profile has a tested equivalent, including values, shapes,
dtypes, casting, errors, missing values, ownership, and resource cleanup. It
does not mean copying NumPy or pandas’ private APIs. The profile, deliberate
deviations, unsupported APIs, oracle versions, and platform requirements must
be generated and published before a release can make that claim.

### Progress snapshot (foundations landed)

Shipped toward N1–N5 (still not a drop-in NumPy/pandas/PyTorch replacement):

- Host tensor storage + integer dtypes + free-list pool; Warp numeric `_tid` path
- Cron channels; accelerator trust env; publishable accelerator reports
- DataFrame multi-level index foundation; 100k-row scale bench smoke
- ML modules/optimizers/checkpoints/gradcheck; finite-diff HVP/second derivative;
  packed Warp trainer inputs; advisory device tags with honest CPU fallback
- Expanded differential conformance (10 fixtures + property smoke)
- dataframe describe/rank `weft check` fix via local sort

Further closed in-tree: 1D FFT suite + differential fixture; groupby transform/size
and MultiIndex swap/drop; scalar nested `create_graph` double-backward; CPU
accelerator conformance + capability matrix + scale benches (100k–250k, optional 1M);
tested DataFrame ↔ Warp copying interchange (zero-copy/columnar paths remain open).

Still open for absolute “100% replacement”: full NumPy ufunc/casting tables,
complete MultiIndex hierarchical ops, array nested reverse-mode, and **published**
CUDA/ROCm/MLX numbers on self-hosted runners every tagged release (tooling ready;
hardware is environment-dependent).

### P0 — correctness and truthful capability reporting

- ~~Remove silent fallback claims~~ **Done (wired + enforced):** providers
  report per-op whether work ran on the requested device, fell back to CPU, or
  is unavailable — via JSON `device`/`requested_device`/`fallback` fields and
  the additive ABI v1 export `weft_accel_exec_info` for the binary tensor
  path. The host parses these into a typed `ExecInfo`, rejects contradictory
  reports as errors, and surfaces them through `accelerator.last_exec_info`,
  `warp.accelerator_last_exec_info`, and `ml.exec_info`. Conformance is
  adversarial: `TestExternalProviderReporting` plus
  `scripts/accelerator-conformance.sh` classify providers as
  `honest`/`unreported`/`contradictory` and fail anything but `honest`.
  **Done for the first operation:** `ml.matmul` now routes two arrays carrying
  the same bound plugin through the binary `tensor_matmul` ABI, reconstructs a
  Warp result, and surfaces the provider's execution report. Unbound,
  mismatched, or unsupported values use host Warp; provider failures are
  explicit errors. Remaining: route additional forward/backward ML operations,
  implement real device-memory ownership, and validate CUDA/ROCm/MLX providers
  on their native hardware.
- ~~Keep all native and browser limits explicit~~ **Done:** request and
  response bodies are capped at 32 MiB on host and browser (over-limit
  requests rejected pre-send; over-limit responses are explicit errors, never
  truncated); browser additionally refuses unbounded no-stream responses and
  never trusts `Content-Length`. Adversarial coverage: boundary-exact tests on
  the host; deceptive/malformed Content-Length, gzip expansion, redirects,
  repeated timeouts with abort-cleanup, and virtual-fs quota lifecycle in real
  Chromium/Firefox (`wasm/playwright_adversarial_test.js`).
- ~~Finish ESL confidence work~~ **Done (mock harness):** black-box process
  tests cover authentication, event-before-reply ordering, coalesced and
  byte-fragmented TCP frames, 10-way command concurrency, a 260-command flood
  enforcing the 256-request cap, timeout cleanup, and client- and
  server-initiated close; ARI has a mock REST+WebSocket suite (which exposed
  and fixed a never-compiled ARI client). Live FreeSWITCH, Asterisk ARI, and
  SIPp scenarios remain environment-gated for release validation.
- ~~Keep vendor copies, package manifests, examples, generated capability
  data, and documentation synchronized~~ **Done:** `check-vendor-sync.sh`,
  `check-catalog-sync.sh`, and `capability-matrix.py --check` all gate CI;
  stale committed capability reports fail the build.

### N1 — NumPy-compatible CPU semantics

- ~~Maintain a generated NumPy/pandas/Weft capability matrix~~ **Done:**
  `scripts/capability-matrix.py` (pinned oracles numpy 2.4.3 / pandas 3.0.1 /
  scikit-learn 1.9.0); `--check` fails CI on stale committed reports.
- **Partially done:** dtype promotion now matches NumPy 2.4.3
  `promote_types` for all 121 supported pairs (exhaustive differential
  fixture + Go matrix test); casts are range-checked (float→int overflow is
  an explicit `Err`). Open: float16 packed storage; object dtype packed
  storage. Complex, datetime, timedelta, structured, and byte-order semantics
  are documented as unsupported in `docs/COMPATIBILITY.md`.
- Complete shape and memory semantics: zero-sized dimensions, broadcasting,
  strides, offsets, contiguous and non-contiguous views, memory order,
  read-only/copy behavior, ownership, aliasing, serialization, and exact
  failure cases.
- Complete indexing and assignment: ellipsis, new axes, mixed basic and
  advanced selectors, broadcasted integer and boolean indexing, `take`,
  `put`, views versus copies, duplicate targets, and exact failure behavior.
  **Progress:** `put` (flat C-order, negative wrap, cycling values,
  last-wins duplicates, out-of-range `Err`) alongside the existing `take`,
  broadcasted integer/boolean advanced indexing, and immutable
  copy-on-write semantics. Open: ellipsis/newaxis selector tokens.
- Expand the numerical surface: ufunc families, reductions and accumulators,
  random generators with deterministic seeds, FFT, polynomial helpers,
  statistics, sorting/searching, masked arrays, sparse formats, and linear
  algebra edge cases. **Progress:** seeded `default_rng`
  (random/normal/integers/shuffle/permutation/choice — per-seed
  deterministic, not PCG64 bit-parity, documented); `initial`/`where`/
  `ddof`/axis-accumulator reduction options; `hypot`/`expm1`/`log1p`/
  `floor_divide`/`remainder`/`square`/`reciprocal`/`deg2rad`/`rad2deg`/
  `copysign`/`rint`; `rfft`/`irfft`/`rfft_freq`/`fftshift`/`ifftshift`;
  `sort_axis`/`argsort_axis`/`unique_opts`. Open: polynomial helpers,
  histogram/corrcoef statistics, searchsorted/partition, multi-dim FFT,
  eig/SVD/QR, masked arrays and sparse formats (documented unsupported).
- Add property, fuzz, and differential tests for every declared API, including
  dtype results, exceptions, empty inputs, non-contiguous inputs, and numerical
  tolerances against pinned NumPy. **Progress:** the conformance property
  harness now actually executes Weft (25 seeded cases: broadcasting, axis
  reductions on transposed views, matmul, reshape, comparisons at 1e-10);
  differential fixtures cover dtypes, errors, edges, strides, reductions, FFT,
  and sklearn linear/logistic fits. Open: fuzz-style randomized shape coverage
  beyond the seeded set.

### N2 — pandas-compatible tabular semantics and scale

- Complete `Index`, `MultiIndex`, `Series`, and `DataFrame` alignment,
  duplicate-label behavior, nullable dtypes, categorical data, timezone-aware
  values, ordering, and missing-value rules.
- Cover `loc`/`iloc`, assignment and broadcasting, groupby/agg/transform,
  joins/merges, pivot/reshape, rolling/expanding/resampling, ranking, window
  statistics, sorting, and stable error behavior. **Progress:** multi-column
  `group_by` with per-column agg lists, `pivot_table` with aggfuncs and
  `fill_value`, `rank_opts` (average/min/max/dense/first + na_option + pct),
  multi-key `merge` with `suffixes`; ~~label-based `loc` and loc/iloc
  assignment~~ **Done:** `loc_label` (scalar/list/boolean-mask/inclusive
  label-slice selectors, pandas error wording) and `loc_set`/`iloc_set`
  with pandas-observed broadcasting; positional `loc` kept for compat.
  **Done:** ewm window statistics — `ewm_mean`/`ewm_sum`/`ewm_var`/`ewm_std`
  with alpha/span/halflife, `adjust`, `ignore_na`, and `bias` matched to the
  pandas 3.0 recursions step-for-step (incl. the com==1 adjust=false branch
  and sum's adjust=true-only rule); `var` added to the rolling/expanding op
  set. Open: resample.
- Add production I/O coverage for CSV, JSON/JSONL, Parquet/Arrow-compatible
  interchange, SQL, and chunked streaming with explicit type and null policies.
  **Progress:** `read_sql`/`to_sql` via the `db` stdlib (sqlite, transactional,
  identifier-quoted); `from_csv_opts`/`read_csv_opts` with strict per-column
  dtypes and null sentinels. Open: Parquet/Arrow (documented unsupported),
  chunked streaming.
- Provide a tested DataFrame-column ↔ Warp-array interchange path, including
  zero-copy cases and cases that must copy due to layout, dtype, or ownership.
- Establish performance and memory budgets for 100k, 1M, and 10M-row workloads,
  including wide frames, joins, groupby, repeated operations, cancellation,
  failure cleanup, and peak allocation reporting.
- Add browser/WASM DataFrame execution tests and document which formats and
  capabilities intentionally remain host-only.

### N3 — practical ML and training compatibility

- Finish reverse- and forward-mode autodiff, higher-order derivatives,
  gradient checking, broadcasting, views/aliases, checkpointing, numerical
  stability diagnostics, and deterministic seed behavior.
  **Progress:** forward-mode autodiff landed as dual numbers (`dual`, `jvp`,
  `jacobian`, `derivative`, `fwd_*` ops) over scalars and warp arrays, with
  exact JVPs three-way checked against reverse mode and `gradcheck`, and
  nested duals for scalar second derivatives. Open: views/aliases, numerical
  stability diagnostics, array-level higher-order gradients.
- Complete the training surface: modules/layers, activations, losses,
  optimizers, schedulers, batching/data loaders, metrics, serialization,
  checkpoint resume, parameter freezing, and clear device placement.
  **Progress:** differentiable `mse_loss`/`binary_cross_entropy`/
  `cross_entropy`/`huber_loss`, `sigmoid`/`tanh`/`gelu`/`softmax` ops +
  modules, `step_lr`/`exponential_lr`/`cosine_lr`, seeded shuffled `batches`
  with `shuffle`/`seed` on the trainers — all gradchecked. Open: parameter
  freezing helper, checkpoint-resume flow, gradient clipping.
- Add classical ML algorithms and preprocessing with scikit-learn differential
  coverage, including sparse and categorical inputs where supported.
  **Progress:** sklearn 1.9.0 differential fixture (linear/logistic fits,
  standardize) in the pinned conformance harness. Open: k-means/KNN/trees;
  sparse/categorical inputs (documented unsupported).
- Validate end-to-end training and inference on CPU with a 100k+ row
  DataFrame-to-model pipeline; retain an explicit, tested CPU fallback when a
  native backend is unavailable.
- Add numerical, convergence, determinism, checkpoint, and leak tests for
  every supported model path before claiming production readiness.

### N4 — native CUDA, ROCm, and Apple MLX plugin runtime

- Stabilize a versioned plugin ABI for provider identity, capability discovery,
  devices, dtypes, shapes, strides, allocation/ownership, host↔device
  transfers, streams, synchronization, cancellation, errors, and logging.
- Build a real NVIDIA provider against the CUDA Runtime API and required kernel
  and math-library primitives; test it on pinned CUDA toolkits and NVIDIA
  hardware.
- Build a real AMD provider against ROCm/HIP and its required memory, stream,
  kernel, and math-library paths; test it on pinned ROCm versions and AMD
  hardware.
- Build a native Apple MLX provider with explicit macOS/device capability
  reporting and tests on supported Apple silicon; absence of MLX must be a
  clear unavailable result, never a false success.
- Define and test provider coverage for elementwise and broadcast operations,
  reductions, matmul, convolutions, transfers, random generation, linear
  algebra, DataFrame kernels, and synchronization. Verify numerical parity
  with the CPU oracle within declared tolerances.
- Add allocator, stream ordering, device-loss, dtype/stride, multi-device,
  concurrent-request, cancellation, leak, and repeated-run tests. A provider
  is not release-ready if it only loads or returns a CPU result under a native
  provider name.

### N5 — WASM and real-browser coverage

- ~~Enforce request-body limits~~ **Done** (32 MiB, pre-send, all methods;
  adversarially tested — the server sees zero bytes for over-limit posts).
- ~~For non-streaming responses, reject body-bearing responses when there is
  no trustworthy bounded stream~~ **Done** (stream-enforced decoded-byte
  limit; `Content-Length` never trusted as a memory guarantee).
- ~~Test malformed and deceptive `Content-Length`, compressed expansion,
  streaming, redirects, aborts, timeouts, cancellation cleanup, repeated
  timed-out executions~~ **Done** (`wasm/playwright_adversarial_test.js`,
  real local endpoints incl. raw sockets; per-browser divergence recorded).
- ~~Add Playwright CI on both Chromium and Firefox~~ **Done** (existing
  suite + adversarial suite + DataFrame/Warp execution suite run on both
  browsers in the `browser-wasm` job).
- ~~Keep parser-only and local mock-server tests in ordinary CI; concurrent
  socket tests in a separate job~~ **Done** (`telecom-parser` vs
  `telecom-dispatcher` jobs).
- ~~Keep a short PR fuzz smoke test and add a scheduled Linux job with
  meaningful fuzz duration~~ **Done** (`fuzz-smoke` per PR in `ci.sh`;
  weekly `fuzz-deep` job, 5m per target across lex/parse/compile/vm).

### N6 — CI, reproducibility, and release gates

- Run unit, parser, differential, property, browser, WASM, and resource
  lifecycle tests on every change. A capability-gated hardware test may report
  “unavailable,” but must never hide a failed test as a skip. **Done:** the
  `ci`/`conformance`/`reliability`/`wasm`/`browser-wasm`/`telecom-*` jobs run
  per change; `WEFT_LIVE_REQUIRED=1` makes broker-absent a failure in the
  live-services job; hardware-provider load failures are `t.Fatal` when the
  plugin env is set.
- Add reproducible self-hosted or hosted jobs for CUDA, ROCm, and MLX, plus
  real FreeSWITCH, Asterisk ARI, and SIPp integration environments.
  **Partial:** `native-accelerators.yml` runs provider conformance on labeled
  self-hosted runners (compile + conformance + reporting gate). Live
  FreeSWITCH/ARI/SIPp environments remain open (environment-gated).
- Freeze and verify offline dependency installation, exact Git/toolkit pins,
  generated lock/manifests, SBOMs, signed artifacts, checksums, and rebuild
  reproducibility. **Progress:** `reproducible-build-check.sh` (offline
  `GOPROXY=off` build after `go mod download` + `go mod verify`,
  byte-identical across checkout paths with `-trimpath -buildvcs=false`),
  `sbom.sh` published as `SBOM.json` in releases, SHA256SUMS retained,
  keyless cosign signing of all release blobs on tags (GitHub OIDC,
  `sigstore/cosign-installer`, bundles published alongside), and release
  notes link the actual build run plus the CI run for the tagged commit.
- Publish benchmark results for 100k+ DataFrame processing, model training,
  transfers, and accelerator operations with versions, hardware, tolerances,
  and peak memory recorded. **Partial:** `benchmarks` job publishes Go + glue
  benches; `bench-scale.sh` covers 100k–250k (1M optional). Open: 10M-row
  budgets, accelerator bench numbers from hardware runners.
- The final replacement gate requires zero unexplained differential failures,
  documented deviations for every unsupported API, green scale and cleanup
  benchmarks, and successful declared workflows on CPU plus every claimed
  accelerator. **Status:** deviations published in `docs/COMPATIBILITY.md`;
  differential suite green; accelerator claims limited to CPU until hardware
  runners publish numbers.

### Previous checklist (historical, superseded)

The block below is retained only for historical context from earlier roadmap
edits. The checklist above is authoritative; do not use this block to assess
release readiness.

This is a package/runtime program, not a promise to put heavyweight scientific dependencies into the small `weft` core binary. `packages/warp` and `packages/dataframe` are useful validated profiles today; they are not yet drop-in replacements for all of NumPy and pandas. Until the gates below are green, documentation must use “NumPy-style,” “pandas-inspired,” or a named supported subset rather than “100% replacement.”

### Progress snapshot (implemented foundations)

Shipped toward N1–N5 (still not a drop-in NumPy/pandas/PyTorch replacement):

- Host packed tensors (`internal/tensor` + stdlib `tensor`) as Warp primary numeric storage
- Cron concurrency fixed via channels
- Native plugin trust model (disable/allowlist/checksum)
- Expanded NumPy/pandas differential fixtures + property smoke
- ML modules (linear/relu/sequential), optimizers, checkpoints, seeds
- Vendor sync CI check and accelerator capability reports
- CPU numerical bench script (`make bench-numerical`)

Remaining for full replacement gates: complete ufunc/indexing matrix, MultiIndex,
scale budgets at 1M–10M rows, real CUDA/ROCm/MLX published numbers every release,
higher-order autodiff, and zero unexplained differential failures.

### N1 — compatibility contract and complete CPU semantics

- Maintain a generated capability matrix for NumPy, pandas, Weft, and every deliberate deviation; pin the oracle versions and record expected error/NaN/null behavior.
- Complete array semantics: all scalar dtypes and casting rules, complex numbers, datetime/timedelta, structured data, byte order, views, ownership, strides, zero-sized dimensions, memory order, object-free serialization, and exact shape/broadcast validation.
- Complete indexing semantics: ellipsis, new axes, mixed basic/advanced indexing, broadcasted integer and boolean indexing, assignment, take/put, views versus copies, and aliasing tests.
- Complete numerical surface: ufunc families, reductions and accumulators, random generators, FFT, polynomial helpers, statistics, sorting/searching, masked arrays, sparse formats, and linear-algebra edge cases.
- Add package-level property tests and differential tests for every supported API, including exceptions and dtype results, against pinned NumPy versions.

### N2 — dataframe replacement semantics and scale

- Complete `Index`, `MultiIndex`, `Series`, and `DataFrame` alignment, duplicate-label, nullable-dtype, categorical, timezone, and missing-value behavior.
- Cover `loc`/`iloc`, assignment, broadcasting, groupby/agg/transform, joins/merges, pivot/reshape, rolling/expanding/resampling, ranking, window statistics, and stable ordering.
- Add production IO coverage for CSV, JSON/JSONL, Parquet/Arrow-compatible interchange, SQL, and chunked streaming with explicit type and null policies.
- Establish performance and memory budgets for 100k, 1M, and 10M-row workloads; include repeated operations, wide frames, joins, groupby, and failure/cleanup behavior.
- Provide a stable interchange path between dataframe columns and Warp arrays without copies when layout and dtype permit, with tests when copies are required.

### N3 — ML and autodiff completeness

- Finish reverse- and forward-mode autodiff, higher-order derivatives, gradient checking, broadcasting, view/alias behavior, checkpointing, and numerical stability diagnostics.
  **Progress:** forward mode (dual-number JVP/`jacobian`/`derivative`) implemented and cross-validated against the reverse-mode tape and `gradcheck`; scalar nested reverse mode (`create_graph`) and finite-diff HVP exist. Open: view/alias semantics, stability diagnostics.
- Implement the training surface needed for practical replacement: modules/layers, losses, optimizers and schedulers, batching, metrics, serialization, checkpoint resume, and deterministic seeds.
- Add classical ML algorithms and preprocessing with scikit-learn differential coverage, including sparse and categorical inputs where supported.
- Validate end-to-end model training on CPU and each native backend, including a 100k+ row dataframe-to-model pipeline and explicit CPU fallback behavior.

### N4 — native accelerator plugin runtime

- Stabilize a versioned plugin ABI for devices, dtypes, shapes, strides, allocation/ownership, host-device transfers, streams, synchronization, errors, capability discovery, and cancellation.
- Implement and test a real CUDA provider using the CUDA Runtime API plus the required kernel/library primitives; compile and run it against pinned toolkit versions on NVIDIA hardware.
- Implement and test a real ROCm/HIP provider with the corresponding memory, stream, kernel, and math-library paths on AMD hardware.
- Implement and test a native Apple MLX provider with explicit macOS/device capability reporting and no silent claim of support when MLX is unavailable.
- Current native providers cover bounded same-shape contiguous `tensor_add` and `tensor_matmul` (rank-1/rank-2 float tensors); broadcasting, reductions, and the remaining operation families still require provider-level coverage.
- Define operation coverage and fallback rules for every provider: elementwise/broadcast operations, reductions, matmul, convolutions, transfers, random, linalg, dataframe kernels, and synchronization.
- Add allocator, stream-ordering, device-loss, dtype/stride, multi-device, concurrent-request, and leak tests. A provider is not release-ready if it only loads or returns a CPU result.

### N5 — release confidence and CI

- Keep fast parser/unit/differential tests on every change; add scheduled longer fuzzing, randomized property tests, and leak/resource-lifecycle runs.
- Add hardware CI or reproducible self-hosted jobs for NVIDIA CUDA, AMD ROCm, and Apple MLX, with capability-gated tests that report “unavailable” separately from “failed.”
- Add browser Playwright coverage for WASM numerical/dataframe execution, HTTP limits, cancellation, repeated timeouts, and filesystem/resource cleanup; WASM must document its intentional native-backend limits.
- Publish reproducible benchmark results, exact dependency/toolkit pins, SBOMs, signed artifacts, and a compatibility report for every release.
- The final replacement gate requires zero unexplained differential failures across the declared API, documented deviations for unsupported APIs, green scale benchmarks, and successful model/dataframe workflows on CPU plus every claimed accelerator.

**Core binary exclusions (still apply)**

- Heavy scientific array / dataframe stacks  
- In-process deep-learning training  
- Notebook as the main loop  
- Full enterprise cloud SDKs  
- `async`/`await` keywords (would undo concurrent-by-default)  

## How we decide what goes in core

Before adding to the **core binary**, we ask:

1. Do most agent/ops scripts need this?  
2. Could it be a `packages/*` module instead?  
3. Does it force GPU, huge native deps, or a second language on the hot path?  
4. Does it fight small-language principles?  

Rule of thumb: **HTTP + agents + local LLM → core. Embeddings/RAG → module. GPU train → orchestrate outside. SIP → module. ML inference → stdlib (HTTP client only).**

## Related

- [README.md](README.md) — documentation index  
- [TUTORIAL.md](TUTORIAL.md) · [LANGUAGE.md](LANGUAGE.md) · [COOKBOOK.md](COOKBOOK.md) · [STDLIB.md](STDLIB.md)  
- [TELECOM.md](TELECOM.md) · [MCP.md](MCP.md) · [ECOSYSTEM.md](ECOSYSTEM.md)  
- Runnable recipes: [examples/cookbook/](../examples/cookbook/)  
- [PRINCIPLES.md](PRINCIPLES.md) — product rules  
- [PRODUCTION.md](PRODUCTION.md) — timeouts, secrets, deploy sketch  
- [TOOLING.md](TOOLING.md) · [TESTING.md](TESTING.md) · [ERRORS.md](ERRORS.md)  
- [ML.md](ML.md) · [FINETUNE.md](FINETUNE.md) · [modules.md](modules.md)  
