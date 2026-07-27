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
