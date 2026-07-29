# Weft browser Wasm runtime

Client-side Weft interpreter for the playground and embedders.

## Build

```bash
make wasm          # → wasm/weft.wasm (+ wasm_exec.js)
make wasm-serve    # http://127.0.0.1:8765/playground.html
```

Uses stock Go (`GOOS=js GOARCH=wasm`), not TinyGo. Artifact is ~13MB.

## API (`weft.js`)

```html
<script src="wasm_exec.js"></script>
<script src="weft.js"></script>
<script type="module">
  const weft = await Weft.load("weft.wasm");
  const { output, error } = weft.run(`fn main { say("hi") }`);
</script>
```

`runWeft(code, timeoutMs?)` is also available globally after load.

## What works

Language core, lists/maps, json/str/math/time/re, type annotations, most pure stdlib.

## Stubs (return clear errors)

`http`, `web`, `db`, `fs` I/O to host disk may still call `os` (browser-limited), `sh`, `llm`, brokers, etc.

Server playground (`cmd/weft-playground`) remains available for full-stdlib sandboxes.
