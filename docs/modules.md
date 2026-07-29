# Expanding Weft with modules

**Modules are how third parties grow the language surface** — new APIs, domain helpers, shared pipelines, optional domains — without forking the Go binary.

**How modules sit next to stdlib and agents:** [ECOSYSTEM.md](ECOSYSTEM.md).  
**Consuming packages:** [packages.md](packages.md). **Catalog:** [`packages/README.md`](../packages/README.md).

### Monorepo examples (never built-ins)

| Package | Role | Docs |
|---------|------|------|
| [`packages/mold`](../packages/mold) | structured models for agents | [MOLD.md](MOLD.md) |
| [`packages/ml`](../packages/ml) | embeddings / RAG / metrics | [ML.md](ML.md) |
| [`packages/tokensave`](../packages/tokensave) | memory / teach → train | package README |

Core stays lean; install only what an app needs.

Anyone can publish libraries that other Weft apps install with `weft get` — **no environment activation, no central registry required**. Modules are folders of `.weft` source + `weft.json`.

| You want… | Do this |
|-----------|---------|
| New functions for apps | `weft new module …` + `pub fn` |
| Multi-file library | `use "./util.weft" as util` inside the package |
| Module that uses another | declare `deps` in `weft.json` (transitive install) |
| Native I/O / OS APIs | already in stdlib; new ones need a Weft core PR |
| Foreign language packages | **no** — pure Weft or HTTP to external services |

## Author a module (5 minutes)

```bash
weft new module greeter
cd greeter
# edit lib.weft — mark the public API with `pub`
weft mod check
```

Scaffold layout:

```text
greeter/
  weft.json      # name, version, entry, exports
  lib.weft       # package entry (pub API)
  util.weft      # optional internal files
  README.md
```

### `weft.json` (module)

```json
{
  "name": "greeter",
  "version": "0.1.0",
  "type": "module",
  "description": "Friendly greetings",
  "entry": "lib.weft",
  "exports": ["hello", "greet"],
  "license": "Apache-2.0",
  "authors": ["you@example.com"],
  "repository": "https://github.com/you/weft-greeter",
  "keywords": ["demo"],
  "deps": {}
}
```

| Field | Purpose |
|-------|---------|
| `name` | Import name (`use greeter`) |
| `version` | Semver tag consumers pin |
| `type` | `module` (library) or `app` |
| `entry` | Main file (default `lib.weft`) |
| `exports` | Documented public API — checked by `weft mod check` |
| `deps` | Modules *this* module needs |

### Public API

```weft
// lib.weft
pub fn hello(name) {
    "hello, " + name
}

// not exported when any `pub` exists
fn internal_helper() { 1 }
```

Rules:

1. Prefer **`pub fn`** (and `pub type` / `pub enum`) for anything consumers call.
2. If **no** `pub` is present, all non-`main` functions export (legacy convenience).
3. Multi-file: `use "./util.weft" as util` inside the module; only the **entry** file’s exports are the package surface. Path imports **cannot leave the package directory** (escape is a hard error).
4. `weft mod check` parses **every** non-test `.weft` in the package (not only the entry), checks `exports` against pub symbols, and points at next steps.  
5. `weft mod check --tests` (or `-t`) also runs `weft test` on the package after a successful static check.  
6. Monorepo CI runs `mod check` (and `--tests` when `*_test.weft` exists) on every `packages/*` entry.

## Consumers install your module

### Path (local / monorepo)

```bash
weft get greeter ../greeter
# or
weft get greeter ./vendor-src/greeter
weft install
```

```weft
use greeter

fn main {
    say(greeter.hello("weft"))
}
```

### Git (recommended for open source)

```bash
git tag v0.1.0 && git push --tags

# consumers:
weft get greeter github.com/you/weft-greeter@v0.1.0
weft install
weft run main.weft
```

### Zip / URL

```bash
weft mod pack -o greeter-0.1.0.weftpkg.zip
# host the zip, then:
weft get greeter https://example.com/greeter-0.1.0.weftpkg.zip
```

## Validate & pack

```bash
weft mod check              # in module root
weft mod check ./greeter    # path
weft mod pack -o out.zip    # zip without vendor/.git
```

`weft mod check` verifies:

- Entry file exists and parses
- At least one export
- `exports` in `weft.json` match `pub fn` names
- Warnings for missing version/description

## Resolution order

When an app does `use greeter` / `import greeter`:

1. **Stdlib** (`http`, `web`, `llm`, …) — always in the binary  
2. **`vendor/greeter/`** — after `weft install`  
3. **`WEFT_PATH`** — colon-separated extra roots  
4. **`packages/greeter/`** — monorepo convention  

App project layout:

```text
myapp/
  weft.json
  weft.lock
  main.weft
  vendor/           # installed modules (commit for offline CI)
    greeter/
  packages/         # optional local modules without get
```

## Multi-module monorepo

```text
repo/
  packages/
    greeter/lib.weft
    httpkit/lib.weft
  apps/
    api/main.weft + weft.json → deps path ../../packages/greeter
```

```bash
cd apps/api
weft get greeter ../../packages/greeter
weft install
weft run main.weft
```

## Module deps (transitive)

Modules can depend on other modules. **`weft install` flattens the full graph into the app’s `vendor/`** — path deps resolve relative to the package that declared them (monorepo-friendly).

```json
{
  "name": "resultx",
  "type": "module",
  "deps": {
    "mathx": { "path": "../mathx" }
  }
}
```

```bash
# app only lists resultx — mathx is installed automatically
weft get resultx ../packages/resultx
weft install
# vendor/resultx + vendor/mathx
```

Cycles are allowed for install (each package once). Prefer acyclic graphs for runtime clarity.

Live example: [`examples/modules/`](../examples/modules/) (`mathx` multi-file → `resultx` depends on it → `demo` app).

## What not to do

| Avoid | Prefer |
|-------|--------|
| Shipping secrets in modules | `secrets` / env at the app |
| Mutating shared global state | Pure functions + passed args |
| Depending on foreign language packages | Pure Weft or HTTP APIs |
| Omitting `pub` on a large API | Explicit `pub` surface |

## What modules can and cannot expand

| Can expand | Cannot (v1) |
|------------|-------------|
| Functions, multi-file packages, `pub` APIs | New syntax / keywords |
| Types (`pub type`) exported on the package map | Native Go / C plugins |
| Composition via `deps` + transitive install | Binary ABI / shared libs |
| Domain “stdlib” for your team (`use billing`) | Hooking the VM from outside |

The runtime stdlib (`web`, `llm`, `db`, …) is compiled into the `weft` binary. If you need a new *host* capability (e.g. a new protocol), open a PR against `internal/stdlib`. Everything else — business logic, helpers, internal frameworks — belongs in modules.

## Capabilities (third-party modules)

Installed packages run with **restricted host access** by default. These stdlib packages are denied unless the module opts in:

| Package | Why restricted |
|---------|----------------|
| `sh` | process execution |
| `secrets` | credential material |
| `cli` | process exit / argv |
| `db` `redis` `mongo` | data stores / exfil |
| `nats` `amqp` | messaging / exfil |
| `socket` `email` | raw net / SMTP |
| `pickle` | arbitrary deserialize |

### Explicit packages

```json
{
  "name": "deploykit",
  "type": "module",
  "capabilities": ["sh"]
}
```

### Named profiles (shortcuts)

Prefer least privilege. Profiles expand to package lists:

| Profile | Grants |
|---------|--------|
| `none` | (empty — only unrestricted stdlib: `json`, `fs`, `http`, …) |
| `data` | `db` `redis` `mongo` `nats` `amqp` |
| `net` | `socket` `email` |
| `host` | `sh` `secrets` `cli` `socket` `email` `pickle` |
| `full` | `*` (everything) |

```json
{
  "name": "etlkit",
  "type": "module",
  "capability_profile": "data",
  "capabilities": ["@net"]
}
```

- `"capabilities": ["@data", "sh"]` — profile token + extra packages  
- `"capability_profile": "data"` — same as `@data`  
- `"capabilities": ["*"]` / profile `full` — full host (review carefully)  
- **Apps** and path-local scripts (`use "./x.weft"`) are unrestricted  
- Path imports cannot leave the package root  
- `weft install` is atomic for `vendor/`; `weft run` verifies `weft.lock` sums  
- Outbound HTTP blocks cloud metadata and RFC1918 by default (loopback always OK for local Ollama/vLLM; set `WEFT_HTTP_ALLOW_PRIVATE=1` for internal nets)  
- Package URL downloads are SSRF-checked, size-capped, and refuse zip-slip / symlinks

## CLI cheat sheet

```text
weft new module <name>     scaffold library
weft new app <name>        scaffold application
weft mod check [dir]       validate publishable module
weft mod pack [dir] -o z   zip for URL installs
weft get <name> <spec>     add dependency
weft install               vendor/ + weft.lock
weft list                  show project deps
weft packages list         monorepo catalog (packages/index.json)
weft registry install <name>   install from registry
weft list packages         same as packages list
```

Catalog env: `WEFT_PACKAGES=/path/to/packages` (or `…/index.json`) when not in monorepo.

## Example

```bash
weft new module coolmath
# …
weft mod check coolmath
cd myapp && weft get coolmath ../coolmath && weft install
```

See also [`docs/packages.md`](packages.md) (consumer package manager).
