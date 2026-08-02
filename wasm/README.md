# Weft browser Wasm runtime

This directory contains the client-side Weft interpreter used by the playground and embedders. It uses the stock Go `GOOS=js GOARCH=wasm` target and executes the same parser, compiler, bytecode validator, VM, and pure standard-library code as the native runtime.

## Build and test

```bash
make wasm          # generates wasm/weft.wasm and the matching wasm_exec.js
make wasm-test     # Go adapter tests, loader tests, and real Node/WASM tests
make wasm-serve    # http://127.0.0.1:8765/playground.html
```

The generated binary is intentionally ignored by Git. Its size depends on the Go toolchain; the current Go 1.26 build is approximately 18 MB. `wasm_exec.js` must come from the same Go toolchain as `weft.wasm`.

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

`runAsync()` is the recommended API. It yields to the browser event loop and is required for browser-backed HTTP. `run()` remains available for small CPU-only snippets, but it is synchronous and can block the calling JavaScript task until the program finishes. Both return `{ output: string, error: string|null }` and enforce a 100,000-byte source limit plus a timeout from 1 to 30,000 milliseconds.

The global functions `runWeft(code, timeoutMs?)` and `runWeftAsync(code, timeoutMs?)` are installed after the Go runtime starts. `Weft.load()` handles concurrent callers as one initialization, waits for the exports, reports Go runtime failures, and falls back from streaming instantiation when the server sends an incorrect WASM MIME type.

## Browser capability boundary

Implemented browser capabilities:

- language core, type annotations, bytecode validation, lists/maps, JSON, strings, math, time, regular expressions, and pure standard-library packages;
- `http.get`, `post`, `put`, `patch`, `delete`, `request`, `fetch`, and `get_json` through the browser Fetch API when called from `runAsync()`; normal browser CORS and CSP rules apply;
- `fs` through a process-local virtual filesystem, including read/write/list/stat/walk/temp-file operations; files are not persisted to the user’s disk and are capped at 16 MiB per file and 64 MiB total;
- browser-safe `os` environment and path helpers. Browser process identity is represented by neutral values.

Host-only packages return explicit capability errors in this target. This includes databases, brokers, raw sockets, DNS/TLS inspection, shell/process control, server listeners, packet capture, supervisor/cluster operations, and external LLM/provider integrations. `web` cannot listen in a browser; use the Fetch API from browser code or the native runtime for servers.

HTTP is deliberately not exposed through synchronous `run()`: waiting for a JavaScript Promise from the browser’s main task would deadlock the runtime. Use `await weft.runAsync(...)` for all code that performs HTTP.

For full host networking, disk I/O, databases, and server listeners, use the native runtime or `cmd/weft-playground`.
