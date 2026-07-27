# Stdlib Tier A / B (0.3.29)

Honest map of the completed ops/agent surface. Live list: `weft stdlib`.

## Tier A — done (with tests)

| Area | API |
|------|-----|
| Shell tokens | `shlex.split` / `quote` / `join` |
| Process | `sh.run/capture/ok/shell/which/lines/code`; opts: `dir`, `env`, `stdin`, `timeout` (sec or `"5s"`), `merge`, `check` |
| Signals | `signal.listen` / `received` / `reset` |
| Paths / bytes | `fs.stem` / `with_suffix` / `parents` / `read_bytes` / `write_bytes` |
| Prompt | `cli.prompt` (line from stdin) |
| Secrets | `secrets.token_hex` / `token_urlsafe` / `compare` (+ require/get/from/unwrap) |
| Logging | `log.set_json` + field maps on log calls |
| Copy | `copy.copy` / `copy.deepcopy` |
| Functools | `functools.partial` / `once` |
| Errors | `traceback.format` / `is_err` / `err_msg` |
| Crypto | `crypto.hash` / `hmac_sha512` (+ existing hashes/hmac_sha256) |
| URL | `url.merge_query` / `path_unescape` (+ parse/build/…) |
| XML | `xml.find` / `findall` / `text` / `attr` |
| HTML | `html.links` (+ escape/strip_tags) |
| INI | `ini.sections` / `has_section` (+ parse/get/…) |
| Test | `test.assert` (+ existing eq/ok/…) |
| HTTP | timeout as duration string; existing retries/headers/form |
| DB | existing `query` / `exec` / `begin` / `tx` (already deep enough for glue) |
| CSV | existing header/comma dialects |

## Tier B — done (with tests)

| Area | API |
|------|-----|
| Binary pack | `binstruct.pack` / `unpack` / `size` |
| Diffs | `difflib.unified_diff` / `ndiff` |
| Stats | `math.quantile` / `mode` (+ mean/stdev/…) |
| IP network | `ip.network` (+ parse/in_network/…) |

## Tier C — permanent non-goals

GUI, `asyncio` event-loop API, multiprocessing/shared memory, ctypes/mmap, venv/pip model, heavy scientific arrays, full mail servers, pdb-as-stdlib.

## Tests

| File | Covers |
|------|--------|
| `shlex_test.go` | unit split/quote/join |
| `binstruct_test.go` | unit pack/unpack |
| `difflib_test.go` | unit diffs |
| `ops_surface_test.go` | integration for kept ops |
| `tier_ab_test.go` | end-to-end A/B surface |

Rule: every new behavior lands with tests. See CONTRIBUTING.md.
