# Stdlib coverage (honest map)

Weft’s stdlib is **broad and shallow on purpose**. This page maps Tier A/B work against glue/ops needs, and lists **Tier C** as permanent non-goals.

Live inventory: `weft stdlib` / `weft stdlib <pkg>`.

## Tier A — agents / ops (implemented lite)

| Area | Package / API | Status |
|------|----------------|--------|
| Process | `sh.run/capture/ok/shell/which/lines/combined`, timeout as seconds or duration string, env map/list, merge stdout | lite |
| Shell tokens | `shlex.split/quote/join` | lite |
| Signals | `signal.listen/received/reset/pid` (poll flags, not VM callbacks) | lite |
| Paths / bytes | `fs.read_bytes/write_bytes/parents/with_suffix/stem` + existing fs | good |
| Secrets tokens | `secrets.token_hex/token_urlsafe/compare` | lite |
| Passwords | `cli.getpass` (line read; use TTY carefully) | lite |
| Logging | `log.*` + `log.set_json` | lite |
| Copy | `copy.copy` / `copy.deepcopy` | lite |
| Partial/once | `functools.partial/once/identity` | lite |
| Errors | `traceback.format_err/format/is_err/err_msg` | lite |
| Crypto | `crypto.hash/hmac_sha512` + existing hashes | lite |
| URL | `url.merge_query/path_unescape` + existing parse/build | lite |
| XML | `xml.find/findall/text/attr` | lite |
| HTML | `html.text/links` + escape/strip | lite |
| Test | `test.assert/raises` + existing asserts | lite |
| HTTP | existing `get/post/fetch` (timeouts/retries via opts on fetch) | lite |
| Time zones | existing `time.zone/parse_in/format_in` | lite |
| CSV dialects | `csv` header/comma opts | lite |
| DB | `db.open` + conn methods (see `weft stdlib` / docs/data) | shallow |

## Tier B — nicer stdlib feel (implemented lite)

| Area | Package / API | Status |
|------|----------------|--------|
| Binary pack | `binstruct.pack/unpack/size` (`<>!=` endian, common codes; not named `struct` — keyword) | lite |
| Diffs | `difflib.unified_diff/ndiff` | lite |
| Binascii | `base64.b2a_hex/a2b_hex/b2a_base64/a2b_base64` | lite |
| Statistics | `math.quantile/mode` + mean/stdev/… | lite |
| IP networks | `ip.network/compress` + existing | lite |

## Tier C — permanent non-goals (do not implement in core)

| Area | Why |
|------|-----|
| GUI (`tkinter`, turtle, IDLE) | Wrong product |
| `asyncio` event-loop API | Weft concurrency is tasks/channels by design |
| `multiprocessing` / shared memory | Use `sh` processes if needed |
| `ctypes` / `mmap` / C extensions | Binary size and safety |
| Full packaging (`venv`, `pip`, `ensurepip`) | Different package model (`vendor/`) |
| Scientific stacks (array/matrix cores) | Outside scope |
| Full mail server / IMAP/POP product | Optional later as packages if earned |
| Language internals (`ast`, `dis`, `gc` dump) | Not user stdlib |
| Debugger product (`pdb`) | Tooling later, not stdlib parity |

## Rule of thumb

1. Most agent/ops scripts need it?  
2. Can it live in `packages/*`?  
3. Huge native deps? → no.  

**HTTP + agents + local LLM + ops glue → core. Everything else earns its keep.**
