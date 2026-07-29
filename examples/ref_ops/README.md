# Reference app: ops

Host facts and **local** checks — no network by default.

## Threat model

| Input | Risk | Mitigation in this app |
|-------|------|------------------------|
| Env keys | Secret leakage | Keys ending in KEY/TOKEN/SECRET/PASSWORD print `***` |
| Paths | Path traversal / overread | Only reports exists/is_file/is_dir for the path you pass |
| `tools` | Depends on `sh.which` | Optional command; fails closed if tools missing |

Still **host-powered** — not a multi-tenant sandbox.

## Usage

```bash
weft run examples/ref_ops/main.weft -- info
weft run examples/ref_ops/main.weft -- env HOME
weft run examples/ref_ops/main.weft -- env OPENAI_API_KEY
weft run examples/ref_ops/main.weft -- path-check README.md
weft run examples/ref_ops/main.weft -- tools git,sh
```

## Tests

```bash
weft test examples/ref_ops
```
