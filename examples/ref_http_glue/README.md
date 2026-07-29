# Reference app: HTTP glue

Minimal client-side API orchestration — offline `demo`, optional live `get`.

## Threat model

- `get` performs outbound HTTP with host SSRF guards (see SECURITY.md).
- Do not pass untrusted URLs without understanding `WEFT_HTTP_ALLOW_PRIVATE`.
- Host-powered: not a sandbox.

## Usage

```bash
weft run examples/ref_http_glue/main.weft -- demo
weft run examples/ref_http_glue/main.weft -- get https://example.com
```

## Tests

```bash
weft test examples/ref_http_glue
```
