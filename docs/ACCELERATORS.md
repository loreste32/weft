# Accelerator hardware reporting

## Maturity status: experimental (CUDA validated)

GPU acceleration in Weft is **experimental**. As of 0.6.2:

- **CPU reference provider**: built and conformance-tested in CI on every commit.
- **CUDA provider**: validated on real hardware (see below). All 6 tensor ops pass conformance.
- **ROCm / MLX providers**: compile-verified only. No hardware validation yet.

### CUDA hardware validation (0.6.2)

| Field | Value |
|-------|-------|
| GPU | NVIDIA A2 (Ampere, compute capability 8.6) |
| VRAM | 15,356 MiB |
| Driver | 590.48.01 |
| CUDA toolkit | 12.0 (nvcc r12.0/compiler.32267302_0) |
| Host CPU | Intel Xeon Gold 6138 @ 2.00 GHz (80 threads) |
| OS | Ubuntu 24.04.3 LTS, kernel 6.8.0-90 |

**Operations tested and passed**:

| Operation | Status | DType | Tolerance |
|-----------|--------|-------|-----------|
| `tensor_matmul` | PASS | float32 | 1e-4 |
| `tensor_add` | PASS | float32 | exact |
| `tensor_sub` | PASS | float32 | exact |
| `tensor_mul` | PASS | float32 | exact |
| `tensor_div` | PASS | float32 | exact |
| `tensor_sum` | PASS | float32 | 1e-4 |

**Benchmarks** (2×2 matmul, full-reduction sum — small inputs, measures ABI overhead not throughput):

| Operation | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| `tensor_matmul` | 1,906,369 | 18,326 | 46 |
| `tensor_sum` | 1,513,349 | 18,155 | 39 |

**Execution reporting**: all ops report `device: "cuda:0"`, `fallback: false`, classification `honest`.

**Fallback frequency**: 0% — no device-claimed op fell back to CPU.

**Multi-stream / concurrency**: not supported. Single-stream, synchronous execution only.

**Failure behavior**: a provider that fails to load, build, or run is reported as `failed` or `unavailable`. Silent fallback is rejected.

### MLX hardware validation (0.6.2)

| Field | Value |
|-------|-------|
| SoC | Apple M4 |
| OS | macOS 15.6 (Sequoia), darwin 24.6.0 |
| mlx-c | 0.6.0_3 (Homebrew) |

**Operations tested and passed**:

| Operation | Status | DType | Tolerance |
|-----------|--------|-------|-----------|
| `tensor_matmul` | PASS | float32 | 1e-4 |
| `tensor_add` | PASS | float32 | exact |
| `tensor_sub` | PASS | float32 | exact |
| `tensor_mul` | PASS | float32 | exact |
| `tensor_div` | PASS | float32 | exact |

MLX does not declare `tensor_sum` (axis reduction API not compile-verified).

**Benchmark** (2×2 matmul — measures ABI overhead):

| Operation | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| `tensor_matmul` | 198,617 | 18,321 | 46 |

**Execution reporting**: all ops report `device: "mlx:0"`, `fallback: false`, classification `honest`.

**Not yet validated**: ROCm (requires AMD GPU). Broadcasting, axis reductions, convolutions, RNG, and linalg beyond matmul are not declared by any provider.

---

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

Silent fallback is **not allowed** — and it is enforced, not just documented.

1. A provider must report whether an operation ran on the **requested device**,
   **fell back to CPU**, or is **unavailable**. Reporting is wired end-to-end:
   - JSON results carry `device`, `requested_device`, and `fallback` fields.
   - The binary tensor path carries the same report through the additive ABI
     v1 export `weft_accel_exec_info` (see
     [`native/accelerator/README.md`](../native/accelerator/README.md)).
   - The host parses both into a typed `accelerator.ExecInfo` with status
     `device` / `fallback` / `unavailable` / `unreported` and validates
     honesty: a report claiming a device other than the requested one with
     `fallback: false` is rejected as an error.
   - Weft code reads the report via `accelerator.last_exec_info(plugin)`
     (also exposed as `warp.accelerator_last_exec_info` and
     `ml.exec_info`).
2. Unavailable hardware → `status=unavailable`. That is **not** a test failure.
3. A failed provider run (build, load, or numerical mismatch) → `status=failed`.
   It must not be hidden as a skip.
4. Warp CPU kernels are the explicit default when **no plugin is loaded**.
   Loading a plugin never silently reverts to CPU without reporting it.
5. Vendor providers must not claim device execution if they only returned a
   host-side result.
6. A provider that omits the reporting fields is `unreported` and fails
   conformance: `TestExternalProviderReporting` and
   `scripts/accelerator-conformance.sh` classify each provider as `honest`,
   `unreported`, or `contradictory`, and only `honest` passes. The
   classification is recorded per provider in the conformance JSON report.

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

## Operation coverage

The binary tensor ABI (`weft_accel_run_tensor`) transports dtype (12 fixed
codes), rank, shape, element strides, byte length, and data — so strided and
broadcast layouts are representable on the wire. What each provider actually
covers is a separate, manifest-declared claim, and the conformance gate
checks it per op: `TestExternalProviderTensorCoverage` probes every `tensor_*`
operation a manifest declares through `RunTensor` and fails the provider if a
declared op errors (declared-but-missing is a lie). The passing op set is
recorded in `reports/accelerator-conformance.json` under
`cpu_reference.coverage.ops_passed`.

Current coverage:

| Operation | CPU reference | Vendor (CUDA / ROCm / MLX) |
|-----------|---------------|-----------------------------|
| `tensor_matmul` | float64 rank-2 | float32 rank-2 |
| `tensor_add` / `tensor_sub` / `tensor_mul` / `tensor_div` | float16 + float32 + float64, same-shape, rank 1–2 | float32, same-shape, rank 1–2 |
| `tensor_sum` (full reduction → rank-0, NumPy `np.sum` semantics) | float16 + float32 + float64, rank 1–2 | CUDA/ROCm float32, rank 1–2; MLX not declared |

The CPU reference implements float16 (ABI code 12, binary16 storage) by
widening inputs to float32 for the computation and rounding results back to
half precision with round-to-nearest-even, matching the host tensor package's
NumPy 2.x `np.float16` conversion semantics.

Deliberate non-claims (providers reject these with explicit errors rather
than silent approximations):

- **Broadcasting.** Elementwise ops are strictly same-shape. The ABI can
  transport broadcastable layouts via strides, but no provider implements
  trailing-dimension broadcasting yet; a wrong broadcast is worse than a
  clear rejection.
- **Axis reductions.** `tensor_sum` is full-reduction only; no `axis` /
  `keepdims` parameter exists in the ABI.
- **Convolutions, RNG, and linalg beyond `matmul`** (solve, svd, eig, …) are
  not transported by any declared operation.
- **Vendor `tensor_sum` is correctness-only for now.** CUDA and ROCm declare
  a deterministic single-device-thread reduction; a parallel reduction and
  benchmark are still required before making performance claims. MLX remains
  undeclared until its `mlx-c` reduction API is compile-verified. Vendor
  provider diffs are compile-unverified on hosts without their SDKs.

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
same-shape contiguous binary `tensor_matmul` and elementwise `tensor_add` /
`tensor_sub` / `tensor_mul` / `tensor_div` for float32 rank-1/rank-2 tensors,
plus a CUDA/ROCm float32 full reduction.
The CPU reference additionally covers those elementwise ops in float64 and
full-reduction `tensor_sum` (rank-0 output). Broadcasting, axis reductions,
conv, RNG, and linalg beyond matmul remain separate coverage claims — see
"Operation coverage" above.

- CPU reference provider: `built` + conformance `passed` on ordinary CI hosts.
- Vendor providers on hosts without the SDK/device: `unavailable` / `not_run` —
  **not** a failure.
- Vendor providers on self-hosted hardware jobs: compile, load, health, and
  matmul (JSON + binary tensor where claimed) must pass with
  `fallback_occurred=false` for device-claimed ops.
- A green **compile alone** is not sufficient provider validation.

The scheduled/manual `.github/workflows/native-accelerators.yml` jobs now also
run `BenchmarkExternalProvider*` for binary matmul and any declared
`tensor_sum`, using the provider's manifest dtype, and upload raw benchmark
output as a per-vendor Actions artifact. These artifacts are evidence, not a
performance claim by themselves: a release must record runner, toolkit,
driver, device, dtype, tolerance, and peak-memory context alongside them.

## Release gate checklist

Use this checklist before shipping a release that claims accelerator, Warp,
DataFrame, or ML hardware support. Host-only CI covers the CPU path; vendor
claims need self-hosted jobs.

### Always (host CI / laptop)

- [ ] `make accelerator-conformance` — CPU reference builds; `health`,
      `identity`, `matmul`, and `tensor_matmul` pass under
      `WEFT_ACCELERATOR_PLUGIN`; the per-op coverage gate
      (`TestExternalProviderTensorCoverage`) passes every tensor op the
      manifest declares and records the op set in the report; the adversarial
      reporting gate (`TestExternalProviderReporting`) classifies the
      provider `honest`; report records `fallback: false` and
      `reporting: "honest"` for the CPU reference.
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
      `fallback: true` with `device: "cpu"` — never silent), and the
      provider's `reporting` classification is `honest` on both the JSON and
      the binary tensor path.
- [ ] Representative matmul / transfer wall times recorded against CPU
      baseline (`make bench-numerical` for host side).

### Honesty gates

- [ ] Do not mark Warp/DataFrame/ML as full NumPy/pandas/framework
      replacements without COMPATIBILITY.md level 1–4 pass.
- [x] ML `matmul` routes through a bound provider's binary `tensor_matmul`
      operation when both operands share that provider; the host reconstructs
      the result and checks the provider's execution report. Unbound,
      mismatched, and unsupported operations remain honest CPU fallback.
- [ ] Extend the same conformance boundary to the remaining ML forward and
      backward operations, with native device-memory ownership and hardware
      validation before making CUDA/ROCm/MLX performance claims.
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
