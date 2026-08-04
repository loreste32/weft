# Accelerator hardware reporting

How Weft records native accelerator capability, fallback behavior, and
per-release hardware results.

The ABI and provider build notes live in
[`native/accelerator/README.md`](../native/accelerator/README.md) and
[`native/accelerator/providers/README.md`](../native/accelerator/providers/README.md).
This document covers **reporting**, **fallback policy**, and the **env trust model**.

## Commands

| Target | Script | Output |
|--------|--------|--------|
| `make accelerator-report` | `scripts/accelerator-report.sh` | JSON (default `/tmp/weft-accelerator-report.json`) |
| `make publish-accelerator-report` | `scripts/publish-accelerator-report.sh` | `reports/accelerator-report.json` + `.md` |
| `make accelerator-conformance` | `scripts/accelerator-conformance.sh` | `reports/accelerator-conformance.json` |
| `make capability-matrix` | `scripts/capability-matrix.py` | `reports/capability-matrix.md` |
| `make bench-numerical` | `scripts/bench-numerical.sh` | CPU numerical wall-time JSON |
| `make bench-scale` | `scripts/bench-scale.sh` | `reports/scale-bench.json` |

Report and conformance scripts are **safe without GPUs**. Missing toolchains or
devices are recorded as `unavailable` (or `not_run` for conformance), never
rewritten as a passing skip. A real build/load/numerical failure is `failed`.

Optional env for the report scripts:

| Variable | Effect |
|----------|--------|
| `WEFT_BIN` | Path to a `weft` binary (built if missing) |
| `WEFT_NUMERICAL_BENCH` | Embed an existing numerical-bench JSON |
| `WEFT_ACCEL_RUN_BENCH=1` | Run `scripts/bench-numerical.sh` inline when no prior JSON exists |
| `WEFT_ACCELERATOR_PLUGIN` | Path to a provider `.so`/`.dylib` for external conformance tests |
| `WEFT_SCALE_STRICT=1` | Fail scale benches on soft budget misses (default: warn only) |

## Fallback policy

Silent fallback is **not allowed**.

1. A provider must report whether an operation ran on the **requested device**,
   **fell back to CPU**, or is **unavailable**.
2. Unavailable hardware → `status=unavailable`. That is **not** a test failure.
3. A failed provider run (build, load, or numerical mismatch) → `status=failed`.
   It must not be hidden as a skip.
4. Warp CPU kernels are the explicit default when **no plugin is loaded**.
   Loading a plugin never silently reverts to CPU without reporting it.
5. Vendor providers must not claim device execution if they only returned a
   host-side result.

Status vocabulary used in the JSON report:

| Status | Meaning |
|--------|---------|
| `available` | Toolchain/device present on this host |
| `configured` | Required env/prefix set (e.g. `MLX_C_PREFIX`) |
| `built` | Reference provider compiled on this host |
| `unavailable` | Toolchain/device not present — not a failure |
| `build_failed` | Compile attempted and failed |
| `failed` | Conformance/load/run failed |
| `not_run` | Conformance not executed on this host |
| `passed` | Conformance checks succeeded |
| `skipped` | Conformance skipped because the provider was not built |

## Env trust model (summary)

Native providers are **trusted host code**. `dlopen` loads them into the Weft
process and they fully bypass the language sandbox. Full ABI notes:
[`native/accelerator/README.md`](../native/accelerator/README.md). Also
[`SECURITY.md`](../SECURITY.md).

| Control | Environment variable |
|---------|----------------------|
| Hard disable | `WEFT_ACCELERATOR_DISABLE=1` |
| Path allowlist | `WEFT_ACCELERATOR_ALLOWLIST` (files or directories; `:` / `,` / `;` separated) |
| Require checksum | `WEFT_ACCELERATOR_REQUIRE_CHECKSUM=1` |
| Expected SHA-256 | `WEFT_ACCELERATOR_CHECKSUM` or sidecar `<plugin-path>.sha256` |

Rules of thumb:

1. Treat provider shared libraries as trusted host code.
2. Registry packages cannot silently activate plugins; application code must
   pass an explicit filesystem path.
3. Production servers should set an allowlist and require checksums.
4. Capability grant `accelerator` is still required for third-party modules.

## Publishing per-release hardware results

For every release that claims CUDA, ROCm/HIP, or Apple MLX support, publish a
capability report (and any vendor-job logs) that includes the fields below.
Host-only CI still runs `make accelerator-report` / `publish-accelerator-report`
so absence of GPUs is visible as `unavailable`.

### How to publish

1. On each hardware runner (CPU reference, CUDA, ROCm, MLX), build the matching
   provider from `native/accelerator/providers/`.
2. Run the native integration harness (`WEFT_ACCELERATOR_PLUGIN=… go test
   ./internal/accelerator -run 'TestExternalProvider'`).
3. Capture wall-time and memory for representative matmul / transfer workloads
   (and `make bench-numerical` for CPU baselines).
4. Run:

   ```sh
   make publish-accelerator-report
   # or, with fresh numerical numbers:
   WEFT_ACCEL_RUN_BENCH=1 make publish-accelerator-report
   ```

5. Attach or commit `reports/accelerator-report.json` and
   `reports/accelerator-report.md` (or release-asset equivalents). Prefer
   release artifacts over large binary dumps in git — see `.gitignore` notes
   for `reports/*` junk.

Vendor hardware jobs live in `.github/workflows/native-accelerators.yml`
(scheduled / manual; self-hosted runners labeled `cuda`, `rocm`, `mlx`).

### Required fields

Every published per-release hardware result must record:

| Field | Description |
|-------|-------------|
| `driver_toolkit` | Driver and toolkit versions (e.g. CUDA 12.x + driver, ROCm HIP, mlx-c prefix) |
| `tests` | Which conformance / integration tests ran (health, matmul, tensor_matmul, …) |
| `tolerances` | Numerical tolerances used for float comparisons |
| `cpu_vs_gpu` | Wall time (and optionally throughput) for the same workload on CPU vs device |
| `transfer_overhead` | Host↔device copy cost when materializing results across the ABI |
| `memory` | Peak host and device memory where measurable |
| `fallback_occurred` | Explicit boolean: did any claimed device op fall back to CPU? |

These map to the `publish_fields.required_for_release_claim` array in the JSON
report. The machine-readable report always includes host info, provider
statuses, trust-model env names, fallback policy, and optional numerical bench
results when available.

### What “green” means

The current vendor provider contract includes JSON matmul plus bounded,
same-shape contiguous binary `tensor_matmul` and `tensor_add` for float32
rank-1/rank-2 tensors. Broadcasting and other elementwise operations remain
separate coverage claims.

- CPU reference provider: `built` + conformance `passed` on ordinary CI hosts.
- Vendor providers on hosts without the SDK/device: `unavailable` / `not_run` —
  **not** a failure.
- Vendor providers on self-hosted hardware jobs: compile, load, health, and
  matmul (JSON + binary tensor where claimed) must pass with
  `fallback_occurred=false` for device-claimed ops.
- A green **compile alone** is not sufficient provider validation.

## Release gate checklist

Use this checklist before shipping a release that claims accelerator, Warp,
DataFrame, or ML hardware support. Host-only CI covers the CPU path; vendor
claims need self-hosted jobs.

### Always (host CI / laptop)

- [ ] `make accelerator-conformance` — CPU reference builds; `health`,
      `identity`, `matmul`, and `tensor_matmul` pass under
      `WEFT_ACCELERATOR_PLUGIN`; report records `fallback: false` for the CPU
      reference.
- [ ] `make accelerator-report` / `make publish-accelerator-report` — vendors
      without toolchains show `unavailable` / `not_run` (not a failure).
- [ ] `make capability-matrix` — refresh `reports/capability-matrix.md`; no
      stale “implemented” claims for missing APIs.
- [ ] Host Go tests: `go test ./internal/accelerator -count=1` (loads the
      reference library; asserts fallback fields on health/matmul).
- [ ] `make bench-scale` — warp + dataframe scale fixtures complete; soft
      budgets may warn; run failures fail the script.
- [ ] Trust-model docs still match env names (`WEFT_ACCELERATOR_DISABLE`,
      allowlist, checksum).

### When claiming a vendor backend (CUDA / ROCm / MLX)

- [ ] Provider builds on the labeled self-hosted runner
      (`.github/workflows/native-accelerators.yml`).
- [ ] `WEFT_ACCELERATOR_PLUGIN=<vendor lib> go test ./internal/accelerator
      -run 'TestExternalProvider'` (JSON health + matmul).
- [ ] Tensor path when claimed:
      `-run 'TestExternalProviderTensor'`.
- [ ] Published report includes every field in
      `publish_fields.required_for_release_claim` (driver/toolkit, tests,
      tolerances, cpu_vs_gpu, transfer_overhead, memory,
      `fallback_occurred`).
- [ ] Device-claimed ops report `fallback_occurred=false` (or explicit
      `fallback: true` with `device: "cpu"` — never silent).
- [ ] Representative matmul / transfer wall times recorded against CPU
      baseline (`make bench-numerical` for host side).

### Honesty gates

- [ ] Do not mark Warp/DataFrame/ML as full NumPy/pandas/framework
      replacements without COMPATIBILITY.md level 1–4 pass.
- [ ] ML `to_device` non-CPU paths remain advisory with `fallback: true`
      unless a real plugin is bound and measured.
- [ ] Capability matrix statuses stay conservative (`partial` /
      `unsupported` preferred over optimistic `implemented`).

## Related docs

- [native/accelerator/README.md](../native/accelerator/README.md) — ABI, trust table, reference provider
- [native/accelerator/providers/README.md](../native/accelerator/providers/README.md) — CUDA / ROCm / MLX builds
- [WARP.md](WARP.md) — Warp arrays and accelerator dispatch
- [ML.md](ML.md) — ML package + accelerator boundary
- [COMPATIBILITY.md](COMPATIBILITY.md) — release gates
- [SECURITY.md](../SECURITY.md) — native plugin security
- [ROADMAP.md](ROADMAP.md) — N4/N6 hardware and reporting gates
