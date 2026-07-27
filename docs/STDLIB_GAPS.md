# Stdlib Tier A / B (0.3.x)

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
| DB | `query` / `exec` / `begin` / `tx` | `db_test.go`, `TestDB_TxBeginPing` |
| CSV | header + comma dialect | `Comp_CSVHeaderDialect`, `TestCSV_AllPaths` |

## Tier B — complete

| Area | API | Tests |
|------|-----|--------|
| Binary pack | `binstruct.pack` / `unpack` / `size` | `binstruct_test.go`, `Comp_Binstruct*`, `TestAB_Binstruct` |
| Diffs | `difflib.unified_diff` / `ndiff` | `difflib_test.go`, `Comp_TestAssert*`, `TestAB_Difflib` |
| Stats | `math.quantile` / `mode` | `Comp_MathQuantileMode`, `TestAB_IPNetworkMath` |
| IP network | `ip.network` (+ parse/in_network/…) | `Comp_IPNetworkParse`, `TestAB_IPNetworkMath` |

## Tier C — permanent non-goals

**Not a backlog.** These stay out of the language/stdlib by design (Weft is ops/agent lite, not a Python clone):

| Non-goal | Why |
|----------|-----|
| GUI toolkits | Different product surface |
| `asyncio`-style event-loop API | Tasks + channels cover concurrency without a loop rewrite |
| Multiprocessing / shared memory | Host process model, not in-process forks |
| ctypes / mmap | Host escape; security + portability |
| venv / pip packaging model | Packages/vendor + capabilities already ship |
| Heavy scientific arrays (NumPy-class) | Explicit non-goal; keep `ml` light |
| Full mail *servers* | Client `email` parse/build/send is fine; MTA is not |
| pdb-as-stdlib | Debugger tooling is separate from stdlib |

If something looks like C, reject it or park it outside this map.

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
| `tier_ab_*.go` | A/B end-to-end + fullcover |
| `brokers_offline_cover_test.go` | redis miniredis, nats/amqp/mongo offline |
| `ws_webrtc_cover_test.go` | websocket frames + webrtc hub |
| `ollama_vllm_httptest_test.go` | ollama/vllm HTTP mocks |
| `pure_ops_more_cover_test.go` | archive/csv/email/json/table/db/socket/circuit |
| `llm_mock_cover_test.go` | LLMDo chat/ask/agent/stream |
| `cli_test.go` | flags + subcommands |

## Coverage

### A/B core packages (~99% statements)

| Package | Coverage |
|---------|----------|
| `shlex`, `signal`, `functools`, `traceback`, `copy` | **100%** |
| `binstruct`, `difflib` | **~97–100%** |

### Whole `internal/stdlib` (ongoing)

| Milestone | Statements |
|-----------|------------|
| Before fullcover push | ~47% |
| Pure packages + FS/CLI/HTTP | ~66% |
| + LLMDo + web/httptest | ~68% |
| + redis/miniredis, ws/webrtc, ollama/vllm | ~75% |
| + archive/csv/email/json/table/db/socket | ~79% |
| + web ServeHTTP/static/ws + llm helpers + viz | **~81%** |

Still thin without live brokers/daemons:

| Package | Why low |
|---------|---------|
| `amqp` wrap (queue ops) | Needs RabbitMQ |
| `mongo` collection ops | Needs MongoDB |
| `nats` full sub loop | Needs NATS (optional live test if present) |
| `llm` chatAnthropic HTTP | Needs mock or live Anthropic path |

Covered offline: **llm** via `LLMDo` + pure helpers, **http**/`web.ServeHTTP`/static/ws, **redis** via miniredis, **websocket** frames, **webrtc** hub fakes, **ollama/vllm** httptest, **sqlite** db/tx, **archive/csv/email/json/table/viz**.

Rule: every new behavior lands with tests. See CONTRIBUTING.md.
