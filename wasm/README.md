# Weft browser Wasm runtime

This directory contains the client-side Weft interpreter used by the playground and embedders. It uses the stock Go `GOOS=js GOARCH=wasm` target and executes the same parser, compiler, bytecode validator, VM, and pure standard-library code as the native runtime.

## Build and test

```bash
make wasm          # generates wasm/weft.wasm and the matching wasm_exec.js
make wasm-test     # Go adapter tests, loader tests, and real Node/WASM tests
(cd wasm && npm ci && npx playwright install --with-deps chromium firefox && npm run test:browser)
make wasm-serve    # http://127.0.0.1:8765/playground.html
```

The generated binary is intentionally ignored by Git. Its size depends on the Go toolchain; the current Go 1.26 build is approximately 18 MB. `wasm_exec.js` must come from the same Go toolchain as `weft.wasm`.

The Playwright smoke test serves the generated runtime over HTTP and runs browser API checks in both Chromium and Firefox. A second suite, `playwright_adversarial_test.js`, runs adversarial checks against real local HTTP endpoints: deceptive and malformed `Content-Length` headers (via raw sockets), oversized declared lengths, gzip expansion, chunked streaming, redirects into over-limit bodies, request-body rejection before any bytes reach the server, repeated timeouts and their connection cleanup, repeated whole-program executions, and virtual-filesystem quota enforcement and reclamation. A third suite, `playwright_dataframe_test.js`, fetches the `warp` and `dataframe` package sources from the local server, inlines them into single browser programs, and asserts exact numerical results (elementwise/broadcast arithmetic, axis reductions, matmul, filter/sort/group_by/join, `to_warp` interchange, a 10,000-row scale smoke) plus the explicit capability errors for host-only tensor and accelerator packages. All three run through `npm run test:browser`. The generated Wasm binary is rebuilt before the test and is intentionally not committed.

## JavaScript API

```html
<script src="wasm_exec.js"></script>
<script src="weft.js"></script>
<script type="module">
  const weft = await Weft.load("weft.wasm");
  const result = await weft.runAsync(`fn main { say("hi") }`, { timeoutMs: 5000 });
  console.log(result.output, result.error);
</script>
```

`runAsync()` is the recommended API. It yields to the browser event loop and is required for browser-backed HTTP. `run()` remains available for small CPU-only snippets, but it is synchronous and can block the calling JavaScript task until the program finishes. Both return `{ output: string, error: string|null }` and enforce a 512,000-byte source limit plus a timeout from 1 to 30,000 milliseconds. The source limit is sized so pure-Weft packages (see below) can be inlined into a single program.

The global functions `runWeft(code, timeoutMs?)` and `runWeftAsync(code, timeoutMs?)` are installed after the Go runtime starts. `Weft.load()` handles concurrent callers as one initialization, waits for the exports, reports Go runtime failures, and falls back from streaming instantiation when the server sends an incorrect WASM MIME type.

## Browser capability boundary

Implemented browser capabilities:

- language core, type annotations, bytecode validation, lists/maps, JSON, strings, math, time, regular expressions, and pure standard-library packages;
- `http.get`, `post`, `put`, `patch`, `delete`, `request`, `fetch`, and `get_json` through the browser Fetch API when called from `runAsync()`; normal browser CORS and CSP rules apply. Request and response bodies are each capped at 32 MiB: an oversized request body is rejected before anything is sent, an over-limit declared `Content-Length` is rejected before the body is read, and the response stream reader enforces the same limit on decoded bytes as they arrive (gzip expansion counts). `Content-Length` is never trusted as the memory bound, and body-bearing responses without a readable stream are rejected. Per-request deadlines come from `timeout_ms` (milliseconds) or `timeout` (seconds) in the opts map; expiry aborts the fetch and returns a deadline-exceeded `Err`;
- `fs` through a process-local virtual filesystem, including read/write/list/stat/walk/temp-file operations. Paths are confined to the virtual root and capped at 4096 characters; storage is capped at 16 MiB per file, 64 MiB total, 10,000 files, 5,000 directories, and 15,000 combined entries. Files are not persisted to the user’s disk;
- browser-safe `os` environment and path helpers. Browser process identity is represented by neutral values.

Host-only packages return explicit capability errors in this target. This includes databases, brokers, raw sockets, DNS/TLS inspection, shell/process control, server listeners, packet capture, supervisor/cluster operations, and external LLM/provider integrations. `web` cannot listen in a browser; use the Fetch API from browser code or the native runtime for servers.

### Numerical packages in the browser (warp + dataframe)

The pure-Weft packages `packages/warp` (NumPy-style arrays) and `packages/dataframe` (pandas-inspired tables) **work in the browser**, with caveats:

- **Loading.** The browser compiles one self-contained source: `use "./lib.weft"` / `use warp` resolve through the host filesystem and package manager, neither of which exists in a browser. Fetch the package sources (e.g. from your static server) and inline them into a single program, as `wasm/playwright_dataframe_test.js` does. The combined warp + dataframe sources (~200 KB) fit the 512,000-byte source limit; the virtual-filesystem quotas do not apply because module loading never touches the virtual fs. `use tensor`, `use accelerator`, and `use math` inside the packages still resolve — those names are registered stdlib packages in the browser build.
- **Storage.** The host `tensor` stdlib is unavailable in the browser (`tensor.supported()` is `false`; every operation returns an `Err` saying host tensor storage is unavailable). warp detects this and falls back to its portable CPU list storage, so `storage_kind(a)` reports `"list"` instead of `"tensor"`. All CPU semantics — elementwise and broadcast arithmetic, axis reductions, matmul, dtype promotion, DataFrame filter/sort/group_by/agg/join, and the `to_warp`/`from_warp` interchange — behave identically to the host list-storage path. `wasm/playwright_dataframe_test.js` asserts exact values for all of these in Chromium and Firefox, plus a 10,000-row group_by + sort smoke test.
- **Performance.** List storage is interpreted Weft, not packed native tensors: expect host-tensor workloads to be one to two orders of magnitude slower in the browser. Keep frames in the low tens of thousands of rows and remember each run is bounded by the 30-second execution timeout.

Intentionally host-only in this target:

- **Native accelerator plugins** (`accelerator.load`/`run`/`run_tensor`, `warp.accelerator_*`) — browsers cannot load shared libraries; calls return explicit capability errors and `accelerator.supported()` is `false`.
- **Packed host tensor storage** (`tensor.*`) — unavailable as described above.
- **File-based and wire formats that need host I/O or drivers** — Parquet/SQL/database I/O, and any package reading datasets from disk. In the browser, ingest CSV/JSON/JSONL over HTTP (`http.get` + `dataframe.from_csv`/`from_json`) or from the virtual fs instead.
- **Large-scale compute budgets** — the 30-second per-run timeout and single-threaded Wasm VM bound what is practical; heavier jobs belong on the native runtime.

HTTP is deliberately not exposed through synchronous `run()`: waiting for a JavaScript Promise from the browser’s main task would deadlock the runtime. Use `await weft.runAsync(...)` for all code that performs HTTP.

For full host networking, disk I/O, databases, and server listeners, use the native runtime or `cmd/weft-playground`.
