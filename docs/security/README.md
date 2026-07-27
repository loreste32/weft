# Security documentation

Weft is a **host-power** scripting runtime (shell + HTTP toolkit), not a multi-tenant sandbox.

| | |
|--|--|
| Threat model + checklist | **[SECURITY.md](../../SECURITY.md)** |
| Where trust sits in the stack | [ECOSYSTEM.md](../ECOSYSTEM.md#trust-path) |

## Audits

| Report | Notes |
|--------|--------|
| [reaudit-2026-07-27.md](reaudit-2026-07-27.md) | White-hat re-audit after HTMX / mold / package work |

**Status of that re-audit’s P1/P2 items:** addressed on `main` (hostname-only LLM trust, Secret seal consistency, archive/email caps, mold depth/list caps, multipart part limit). Prefer [SECURITY.md](../../SECURITY.md) “Recent hardening” for the current list.

## Related

- Package capabilities: [modules.md](../modules.md)  
- Production notes: [PRODUCTION.md](../PRODUCTION.md)  
- Optional modules (`mold` is pure / no caps): [MOLD.md](../MOLD.md)
