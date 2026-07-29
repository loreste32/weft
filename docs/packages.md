# Weft package manager

Weft installs pure source packages into the project — no global site-packages, no environment-activation ceremony.

**Big picture** (stdlib vs modules vs agents): [ECOSYSTEM.md](ECOSYSTEM.md).  
**Authoring modules:** [modules.md](modules.md). **Monorepo catalog:** [`packages/README.md`](../packages/README.md).

| Idea | Weft |
|------|------|
| Install scope | **Project `vendor/` only** |
| Activation | None — just `weft run` |
| Package form | Pure `.weft` source packages |
| Lockfile | **`weft.lock` with content hashes** |
| Runtime | Single binary; packages are files |

**Writing modules for others?** See [`docs/modules.md`](modules.md).

## Quick start (consumer)

```bash
weft init myapp
weft get greeter ./path/to/greeter     # local path
weft get util github.com/org/repo@v0.1 # git (needs git on PATH)
weft install                           # from weft.json → vendor/
weft run main.weft
```

## Quick start (author)

```bash
weft new module greeter
cd greeter && weft mod check
# share path or git tag — others: weft get greeter github.com/you/greeter@v0.1.0
```

## Manifest (`weft.json`)

```json
{
  "name": "myapp",
  "version": "0.1.0",
  "type": "app",
  "deps": {
    "greeter": { "path": "../packages/greeter" },
    "util": "github.com/acme/weft-util@v0.2.0",
    "data": { "url": "https://example.com/data-pkg.zip" }
  }
}
```

String shorthands:

- `./foo` / `../foo` → path
- `github.com/org/repo@tag` → git clone
- `https://…zip` → download archive

**Version constraints (lite):** object form may include `"version": "^1.2.0"`, `"~0.3.1"`, `">=0.2.0"`, or an exact semver. On install, that is checked against the dependency’s own `weft.json` `version`. Branch names that are not semver-like are not treated as constraints.

### Catalog discovery

```bash
weft packages list              # walk up for packages/index.json
weft packages list embed        # filter by name/summary
weft packages search rag        # same idea
weft packages info tokensave    # one entry (path, version, install hints)
weft registry install ml
weft registry install mold
weft registry install tokensave
```

#### Monorepo catalog (`packages/index.json`)

See the single catalog table in [`packages/README.md`](../packages/README.md) and the agent stack in [ECOSYSTEM.md](ECOSYSTEM.md).

| Name | One-liner | Docs |
|------|-----------|------|
| `mold` | structure & validate agent JSON | [MOLD.md](MOLD.md) |
| `ml` | embeddings / RAG | [ML.md](ML.md) |
| `tokensave` | context thrift + memory | [tokensave README](../packages/tokensave/) |

These are **modules**, not stdlib: nothing under `packages/` is compiled into the `weft` binary.

- `WEFT_PACKAGES` — path to a packages dir or `index.json`  
- `WEFT_CATALOG_URL` — HTTPS URL to a remote `index.json` (discovery only; install still uses path/git specs from the entry)

Unknown names get “did you mean?” suggestions when close.

`weft doctor` reports catalog discovery (`catalog`, `catalog_pkgs`), project deps, and vendor lock integrity when you are inside a project.

**Trust:** package installs are trusted like local code. Review `capabilities` / `capability_profile` before installing third-party modules ([SECURITY.md](../SECURITY.md), [modules.md](modules.md)).

## Layout of a package

```
greeter/
  weft.json         # name, version, entry, exports
  lib.weft          # preferred entry
  # or entry from weft.json / mod.weft / greeter.weft / src/lib.weft
```

```weft
// lib.weft
pub fn greet(name) { "hi " + name }
```

## Importing

```weft
use greeter                 // preferred
import greeter              // same
import "greeter"            // same
import greeter as g         // alias
use "./local.weft" as L     // path (always works)
```

Resolution order:

1. Stdlib (`http`, `web`, `llm`, …) — in the binary  
2. `vendor/<name>/`  
3. `WEFT_PATH`  
4. `packages/<name>/`

## Lockfile

`weft install` writes `weft.lock` with `sha256:` of each package tree. Commit both `weft.json` and `weft.lock`. Commit `vendor/` for fully offline builds, or re-install in CI.

**Integrity:** if `weft.lock` is present, `weft run` verifies vendor trees against lock sums and refuses to execute on mismatch (re-run `weft install`).

**Atomic install:** packages materialize in a staging tree; `vendor/` is swapped only on full success. A failed install does not half-update deps.

**Conflicts:** the same package name from two different sources (diamond deps) fails install — no silent first-wins.

**Reserved names:** you cannot vendor a package named like stdlib (`http`, `json`, …) or prelude globals (`map`, `filter`, …).

**Archives:** zip/tar installs reject path traversal, absolute paths, and symlinks.

**Capabilities:** third-party modules default-deny high-risk packages (`sh`, `secrets`, `cli`, `env`, `fs`, `http`, `llm`/`ollama`/`vllm`, `web`, `archive`, `graphql`, data-plane brokers, `socket`, `email`, `pickle`) unless listed under `capabilities` or a `capability_profile` (`@agent`, `@io`, `@data`, `@host`, `@full`, …). Apps and path-local scripts stay unrestricted.

**Network:** package URL fetches use SSRF-safe dialing (no metadata, no RFC1918 unless `WEFT_HTTP_ALLOW_PRIVATE=1`). Archives capped at 100 MiB download / 200 MiB uncompressed.

## CLI

```text
weft new module <name> | weft new app <name>
weft mod check [dir] | weft mod pack [dir]
weft init | get | install | list
```

## Transitive deps

`weft install` walks each package’s `weft.json` `deps` and vendors them into the **app** `vendor/` tree (flat). Path specs resolve against the package that declared them, so monorepos work:

```text
packages/mathx
packages/resultx  → deps.mathx = ../mathx
apps/demo         → deps.resultx = ../../packages/resultx
# install in apps/demo → vendor/resultx + vendor/mathx
```

## Public registry

Weft has a package registry with ed25519 signed packages. Override the default endpoint with `WEFT_REGISTRY`.

```bash
# browse and install from registry
weft registry search json
weft registry info mypkg
weft registry install mypkg@^1.0

# generate a signing key
weft registry keygen myname
weft registry keys

# publish (validates, signs, uploads)
weft publish --key myname
```

Signatures are verified on install. Unsigned packages are still installable.

Auth: set `WEFT_REGISTRY_TOKEN` for publish access.

## Not yet

- Binary native extensions (expand via pure `.weft` modules)
- Hosted default registry endpoint (client is ready, server not yet deployed)
