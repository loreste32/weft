# Security

## Supported versions

Weft is pre-1.0 (0.3.x). Fixes land on the current published line; there is no long-term support branch yet.

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Email the maintainer via the contact on the [GitHub profile](https://github.com/loreste32) (or open a private security advisory on this repository if available). Include:

- Affected version (`weft version`)
- Impact and a minimal reproduction if possible
- Whether the issue is already public

We will acknowledge receipt and work on a fix or coordinated disclosure.

## Scope notes

- Package installs from path/git are trusted like local code; treat untrusted sources carefully.
- Network helpers aim to avoid obvious SSRF mistakes but are not a hardened multi-tenant sandbox.
- LLM/tool agents can execute whatever your scripts allow — do not point them at secrets without review.

## Threat model (0.3.x)

Weft is a **host-power** scripting runtime (like a local shell + HTTP toolkit), not a multi-tenant sandbox.

| Trust boundary | Behavior |
|----------------|----------|
| App / path scripts | Full host access (`fs`, `sh`, `http`, `llm`, …) |
| Third-party packages (`vendor/`) | Restricted stdlib unless `capabilities` / `capability_profile` grants it |
| Outbound HTTP | Blocks RFC1918, CGNAT `100.64/10`, and `169.254/16` metadata by default; loopback allowed for local models |
| `http` `insecure: true` | Skips TLS verify only — **SSRF dial guards stay on** |
| Env API keys + `llm` | Env keys are **not** sent to untrusted `base_url` (allowlist: OpenAI/Anthropic hosts, localhost; extend with `WEFT_LLM_TRUST_HOSTS`) |
| Secrets | `Secret` values print as `***`; field access sealed — use `secrets.unwrap` |

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
2. Do not set `WEFT_HTTP_ALLOW_PRIVATE=1` on multi-tenant hosts.
3. Do not register shell tools on untrusted LLM prompts.
4. Prefer containers for untrusted code isolation.

### Recent hardening (security review)

- SSRF retained when `insecure: true`; socket dial IP-pins like HTTP
- `git clone` rejects dash-leading args; `--` before URL; CheckURL for https git
- Expanded package restricted set (`fs`/`http`/`env`/`llm`/…)
- `app.before` runs for routes, static, and WebSocket
- Response header CRLF strip; cookie `SameSite=Lax` default
- Tar extract caps + reject specials; Secret field seal
