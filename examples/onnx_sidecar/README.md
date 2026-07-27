# ONNX sidecar (external process)

Weft never loads ONNX in-core. Call a sibling HTTP server.

```bash
weft run examples/onnx_sidecar/mock_server.weft   # terminal 1
weft run examples/onnx_sidecar/main.weft          # terminal 2
# ONNX_SIDECAR_URL=http://127.0.0.1:8091
```

**Contract:** `POST /v1/classify` `{"text":"…"}` → `{"label","score","model"}` · `GET /health`

```weft
r := http.post("${base}/v1/classify", json.stringify({"text": t}), {
    "timeout_ms": 2000, "retries": 2, "circuit": true,
})?
out := json.parse(r.body)?
say("${out.label} (${out.score})")
```

Real ONNX Runtime / Triton stays in the sidecar. Weft only does HTTP.  
See [`docs/ML.md`](../../docs/ML.md) · [`docs/SYNTAX.md`](../../docs/SYNTAX.md).
