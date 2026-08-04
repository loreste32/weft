# Weft accelerator capability report

- **Generated:** unknown
- **Format:** `weft.accelerator.report` v1
- **Host:** Darwin / arm64 (Go: go version go1.26.1 darwin/arm64; Python: 3.14.6)

## Fallback policy

- Silent fallback allowed: **False**

## Providers

| Provider | Status | Conformance | Tool / toolkit | Detail |
|----------|--------|-------------|----------------|--------|
| `cpu_reference` | `built` | `passed` | /usr/bin/cc | ok |
| `cuda` | `unavailable` | `not_run` | — | Requires self-hosted CUDA runner (.github/workflows/native-accelerators.yml) |
| `rocm` | `unavailable` | `not_run` | — | Requires self-hosted ROCm runner |
| `mlx` | `unavailable` | `not_run` | — | Requires self-hosted Apple Silicon runner with mlx-c |

## Trust model (summary)

| Control | Environment variable |
|---------|----------------------|
| Hard disable | `WEFT_ACCELERATOR_DISABLE` |
| Path allowlist | `WEFT_ACCELERATOR_ALLOWLIST` |
| Require checksum | `WEFT_ACCELERATOR_REQUIRE_CHECKSUM` |
| Expected SHA-256 | `WEFT_ACCELERATOR_CHECKSUM` |

- Native providers are trusted host code and bypass the language sandbox.
- Registry packages cannot silently activate plugins; loads require an explicit path.
- Prefer allowlist + checksum verification in production.

## Benchmarks

- **Status:** `see scripts/bench-numerical.sh`
- **Note:** CPU numerical benchmarks are separate from provider hardware jobs

## Artifacts

- Machine-readable: `reports/accelerator-report.json`
- This summary: `reports/accelerator-report.md`

Hosts without GPUs report `unavailable` for vendor providers; that is expected and is not a CI failure.
