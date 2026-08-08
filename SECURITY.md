# Security

How trust boundaries sit next to language, stdlib, and modules: **[docs/ECOSYSTEM.md](docs/ECOSYSTEM.md#trust-path)**.

## Supported versions

Weft is pre-1.0 (0.6.x). Fixes land on the current published line; there is no long-term support branch yet.

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Email the maintainer via the contact on the [GitHub profile](https://github.com/loreste32) (or open a private security advisory on this repository if available). Include:

- Affected version (`weft version`)
- Impact and a minimal reproduction if possible
- Whether the issue is already public

We will acknowledge receipt and work on a fix or coordinated disclosure.

## Scope notes

- Registry packages are verified by ed25519 signature and sha256 checksum on install.
- Package installs from path/git are trusted like local code; treat untrusted sources carefully.
- Network helpers aim to avoid obvious SSRF mistakes but are not a hardened multi-tenant sandbox.
- LLM/tool agents can execute whatever your scripts allow — do not point them at secrets without review.

## Threat model

Weft is a **host-power** scripting runtime (like a local shell + HTTP toolkit), not a multi-tenant sandbox.

| Trust boundary | Behavior |
|----------------|----------|
| App / path scripts | Full host access (`fs`, `sh`, `http`, `llm`, …) |
| Third-party packages (`vendor/`) | Restricted stdlib unless `capabilities` / `capability_profile` grants it |
| Outbound HTTP | Blocks RFC1918, CGNAT `100.64/10`, and `169.254/16` metadata by default; loopback allowed for local models |
| `http` `insecure: true` | Skips TLS verify only — **SSRF dial guards stay on** |
| Env API keys + `llm` | Env keys are **not** sent to untrusted `base_url` (allowlist: OpenAI/Anthropic hosts, localhost; extend with `WEFT_LLM_TRUST_HOSTS`) |
| Secrets | `Secret` values print as `***`; field access sealed — use `secrets.unwrap` |
| Registry packages | Archives verified by ed25519 signature + sha256 checksum; SSRF-safe download; key names reject path traversal |
| VM execution | Call depth capped at 10,000 frames (infinite recursion → clean error, not crash) |

### Capability profiles (packages)

| Profile | Grants (summary) |
|---------|------------------|
| *(empty)* / `@none` | pure helpers only |
| `@io` | `fs`, `archive` |
| `@http` | `http` |
| `@llm` | `llm`, `ollama`, `vllm` |
| `@agent` | models + `http` |
| `@data` | db/redis/mongo/nats/amqp |
| `@net` | socket/email/http |
| `@host` / `@full` | broad / all restricted |

Apps that `use` modules needing models or disk must declare grants on the **module** `weft.json`.

### Operator checklist

1. Only install packages you trust; review `capabilities`.
2. Pin signing keys: `weft registry trust <namespace> <pubkey>` (or `--key localname`). Set `WEFT_REQUIRE_TRUST=1` so installs fail for untrusted namespaces.
3. Do not set `WEFT_HTTP_ALLOW_PRIVATE=1` on multi-tenant hosts.
4. Do not register shell tools on untrusted LLM prompts.
5. Prefer containers for untrusted code isolation.

### Recent hardening (security review)

- SSRF retained when `insecure: true`; socket dial IP-pins like HTTP
- `git clone` rejects dash-leading args; `--` before URL; CheckURL for https git
- Expanded package restricted set (`fs`/`http`/`env`/`llm`/…)
- `app.before` runs for routes, static, and WebSocket
- Response header CRLF strip; cookie `SameSite=Lax` default
- Tar/zip/gunzip extract caps + reject specials
- Secret seal: VM field get/set **and** `json.get` / `asMap` (use `secrets.unwrap`)
- Env API keys: **hostname-only** trust (no path/substring spoof); `WEFT_LLM_TRUST_HOSTS` exact/suffix
- SMTP `from`/`to`/`subject` strip CR/LF/NUL (header injection)
- HTMX OOB ids restricted to `[A-Za-z0-9_-]`
- Multipart form: max 1024 parts (part storms)
- `mold` module: max nest depth 32, max list length 10_000
- CGNAT `100.64/10` blocked with private nets
- Registry: SSRF-safe fetch/download, sha256 checksum verification, archive URL path traversal rejection
- Signing: key name validation (rejects `/`, `\`, `..`, leading `-`/`.`)
- VM: 10,000-frame call depth cap (stack overflow → error, not crash)
- DB: JSON/JSONB columns auto-parsed safely via `encoding/json` (no eval)
- DAP/LSP framing (`internal/jsonrpc`): 8 KiB max header line, 32 KiB total header bytes, 64-header cap, 10 MiB body limit, duplicate Content-Length rejected, strict `strconv.Atoi` parsing (no partial matches like `123junk`)

### Native accelerator plugins

Native providers are loaded with `accelerator.load(path)` via `dlopen`. They run **in-process** and fully bypass the language sandbox.

| Control | Env / mechanism |
|---------|-----------------|
| Hard disable | `WEFT_ACCELERATOR_DISABLE=1` |
| Path allowlist | `WEFT_ACCELERATOR_ALLOWLIST` (colon/comma/semicolon-separated files or directories) |
| Require checksum | `WEFT_ACCELERATOR_REQUIRE_CHECKSUM=1` |
| Expected digest | `WEFT_ACCELERATOR_CHECKSUM=<64 hex>` or sidecar `<plugin>.sha256` |

Rules of thumb:

1. Treat provider shared libraries as **trusted host code**.
2. Do not let registry packages silently load plugins; application code must pass an explicit path.
3. Production servers should set an allowlist and require checksums.
4. Capability grant `accelerator` is still required for third-party modules.
