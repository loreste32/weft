# Stdlib Tier A / B (0.3.30)

Honest map of the ops/agent surface. Live list: `weft stdlib`.

## Tier A — complete

| Area | API | Tests |
|------|-----|--------|
| Shell tokens | `shlex.split` / `quote` / `join` | `shlex_test.go`, `ops_surface`, `Comp_Shlex*` |
| Process | `sh.run/capture/ok/shell/which/lines/code`; opts `dir` `env` `stdin` `timeout` `merge` `check` | `Comp_ShAllOpts`, `ops_surface` |
| Signals | `signal.listen` / `received` / `reset` | `Comp_Signal*`, `ops_surface` |
| Paths / bytes | `fs.stem` / `with_suffix` / `parents` / `read_bytes` / `write_bytes` | `Comp_FS*`, `ops_surface` |
| Prompt | `cli.prompt` | `TestAB_CLIPrompt` |
| Subcommands | `cli.parse` `commands` → `p.command` | `TestCLISubcommands`, `Comp_CLI*` |
| Secrets | `token_hex` / `token_urlsafe` / `compare` / require/get/from/unwrap | `Comp_SecretsAll`, `ops_surface` |
| Logging | `log.set_json` + field maps | `TestLogSetJSON`, `Comp_XMLHTMLCryptoLog` |
| Copy | `copy.copy` / `deepcopy` | `Comp_CopyShallowDeep`, `TestAB_Copy*` |
| Functools | `partial` / `once` | `Comp_Functools*`, `TestAB_Copy*` |
| Errors | `traceback.format` / `is_err` / `err_msg` | `Comp_TracebackAll`, `TestAB_Traceback` |
| Crypto | `hash` / `hmac_sha512` (+ existing) | `Comp_XMLHTMLCryptoLog`, `TestAB_Crypto*` |
| URL | `merge_query` / `path_unescape` (+ parse/build) | `Comp_URLAll` |
| XML | `find` / `findall` / `text` / `attr` | `Comp_XML*`, `ops_surface` |
| HTML | `links` (+ escape/strip_tags) | `Comp_XMLHTML*`, `ops_surface` |
| INI | `sections` / `has_section` (+ parse/get/save/load) | `Comp_INIFull` |
| Test | `test.assert` (+ eq/ok/…) | `Comp_TestAssert*`, `TestAB_TestAssert` |
| HTTP | timeout sec/`"5s"`, retries, headers, form, **`insecure`** | `TestAB_HTTP*` |
| DB | `query` / `exec` / `begin` / `tx` | existing `db_test.go` |
| CSV | header + comma dialect | `Comp_CSVHeaderDialect` |

## Tier B — complete

| Area | API | Tests |
|------|-----|--------|
| Binary pack | `binstruct.pack` / `unpack` / `size` | `binstruct_test.go`, `Comp_Binstruct*`, `TestAB_Binstruct` |
| Diffs | `difflib.unified_diff` / `ndiff` | `difflib_test.go`, `Comp_TestAssert*`, `TestAB_Difflib` |
| Stats | `math.quantile` / `mode` | `Comp_MathQuantileMode`, `TestAB_IPNetworkMath` |
| IP network | `ip.network` (+ parse/in_network/…) | `Comp_IPNetworkParse`, `TestAB_IPNetworkMath` |

## Tier C — non-goals

GUI, `asyncio` event-loop API, multiprocessing/shared memory, ctypes/mmap, venv/pip model, heavy scientific arrays, full mail servers, pdb-as-stdlib.

## Demos

```bash
weft run examples/tier_ab.weft -- demo
weft run examples/sysops_host.weft -- check -r git,sh
weft run examples/cli_tool.weft -- --help
```

## Test entry points

| File | Role |
|------|------|
| `shlex_test.go` | unit split/quote/join |
| `binstruct_test.go` | unit pack/unpack/errors |
| `difflib_test.go` | unit diffs |
| `ops_surface_test.go` | smoke integration |
| `tier_ab_test.go` | A/B end-to-end |
| `tier_ab_comprehensive_test.go` | opts, shallow copy, INI/CSV/URL/IP/math matrix |
| `cli_test.go` | flags + subcommands |

Rule: every new behavior lands with tests. See CONTRIBUTING.md.
