# Stdlib: what we keep, what we won’t

Live list: `weft stdlib`.

## Keep (ops / agents actually use)

| Need | Package |
|------|---------|
| Shell args safely | `shlex` (split / quote / join) |
| Run commands | `sh` (+ `lines`, timeout duration string, `merge` opt) |
| Graceful stop flags | `signal` (listen / received / reset) |
| Tokens | `secrets.token_hex` / `token_urlsafe` / `compare` |
| JSON logs | `log.set_json` |
| Path stem | `fs.stem` |
| XML walk | `xml.find` / `findall` |
| HTML hrefs | `html.links` |
| Query merge | `url.merge_query` |

## Won’t add (or already deleted)

| Temptation | Why not |
|------------|---------|
| `copy` / `functools` / `traceback` packages | Prelude has `deepcopy`; rest was unused wrappers |
| `binstruct` / `difflib` | Rare for our scripts; big code for little use |
| Binascii aliases (`b2a_*`) | Same as existing base64/hex helpers |
| Fake `getpass` | Echoed stdin is not a password API |
| `crypto.hash` dispatcher | Call `sha256` / `md5` directly |
| Full stdlib parity | Binary bloat; not the product |

## Rule

If a script never needs it, don’t put it in the binary. Prefer `packages/*` or `sh` for one-offs.
