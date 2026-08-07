# NumPy/pandas compatibility program

This document defines what “replacement” means for Weft. The goal is
behavioral compatibility with pinned NumPy and pandas releases, not the
ability to import Python packages inside the Weft runtime.

## Compatibility levels

1. **Surface** — the operation exists, is discoverable from the package
   manifest, and accepts the documented argument forms.
2. **Semantics** — values, shapes, dtypes, index alignment, mutation rules,
   exceptions, NaN/null behavior, and edge cases match the pinned reference.
3. **Scale** — representative 100k-row DataFrames and large tensor workloads
   complete within the documented resource and latency budgets.
4. **Backend** — CPU, CUDA, ROCm/HIP, and Apple MLX produce equivalent results
   within declared tolerances and expose the same capability/error model.

We do not call a package a replacement until all four levels pass for the
declared compatibility profile. A function being present is not evidence of
replacement status.

## Compatibility profile

The checked-in differential harness is `scripts/conformance/run.py`. It runs
the Weft programs in `testdata/conformance/` and compares their JSON results
with pinned offline NumPy, pandas, and scikit-learn oracles from
`scripts/conformance/requirements.txt`. The GitHub
`numpy-pandas-conformance` job installs those exact versions and gates changes
to `main`. The harness also includes a seeded property suite
(`warp_property_case.weft` driven per-case over stdin) and an `ml` fixture:
because `ml.linear_fit`/`logistic_fit` are gradient descent while sklearn
fits are closed-form/lbfgs, learned coefficients, predictions, and metrics
are compared at abs_tol=1e-2 (observed agreement is ~1e-9 after
standardizing features; `ml.standardize` itself is exact and checked at the
strict 1e-10 tolerance).

Python is only the reference oracle; it is not a Weft runtime dependency. The
current corpus is a representative smoke suite, not proof of complete API
parity. Add a case to both the Weft fixture and the Python oracle before
claiming support for a new operation, dtype, edge case, or performance tier.

## Required foundations

### `warp`

- `internal/tensor` is the packed host storage engine; `packages/warp` now
  uses host tensor handles (`_tid`) as primary storage for numeric dtypes,
  with a portable list cache for views/indexing and object dtype;
- typed, strided storage with views, offsets, C/F order, and buffer export;
- complete dtype promotion/casting and scalar behavior;
- basic, advanced, boolean, and broadcasted indexing;
- reduction option `out=` (axis, keepdims, where, initial, and ddof are
  implemented);
- ufunc-like dispatch machinery and masked/sparse array modules (the ufunc
  coverage itself — including hypot, expm1, log1p, floor_divide, remainder,
  square, reciprocal, deg2rad/rad2deg, copysign, rint — and a seeded
  `default_rng` random module now exist), and complete linear algebra
  coverage;
- binary tensor transport for native providers instead of JSON for large data.

### `dataframe`

- columnar storage with nullable, categorical, datetime, timezone, and
  extension dtypes;
- label-aware Series arithmetic, reindexing, `.loc`/`.iloc`/scalar access,
  duplicate indexes, and MultiIndex;
- complete groupby/aggregation/transform/window/join/reshape semantics
  (rolling/expanding share the `sum/mean/count/min/max/first/last/std/var`
  op set, and `ewm_mean`/`ewm_sum`/`ewm_var`/`ewm_std` replicate the pandas
  3.0 ewm recursions with `alpha`/`span`/`halflife`, `adjust`, `ignore_na`,
  and `bias`; resample and time-based windows remain open);
- Arrow/Parquet/SQL-class I/O and memory-efficient 100k+ row execution.

The current DataFrame slice preserves explicit indexes through filtering,
sorting, `head`, `tail`, and positional selection; `iloc` accepts scalar,
negative, and list positions. These behaviors are covered by the pinned
conformance fixture, but they do not imply complete pandas indexing parity.

### ML and providers

- `packages/ml` has reverse-mode autodiff, SGD/Adam, linear/sequential modules,
  checkpoints, and deterministic seeds; sparse/complex tensors, higher-order
  gradients, device placement, and async pools remain open;
- device selection, streams, asynchronous execution, memory pools, and
  deterministic fallback behavior;
- real hardware conformance jobs for every declared vendor provider.

## Oracle versions and platform requirements

- Differential oracle: **numpy==2.4.3**, **pandas==3.0.1**,
  **scikit-learn==1.9.0** (pinned in
  `scripts/conformance/requirements.txt`; installed only in the
  `numpy-pandas-conformance` CI job, never at runtime).
- CPU semantics are the reference on Linux and macOS (CI executes on Linux;
  maintainers run the same suites on macOS). Windows is cross-compiled but not
  executed in CI.
- CUDA, ROCm, and Apple MLX providers are **environment-gated**: they require
  the matching GPU/toolkit and the self-hosted runners in
  `.github/workflows/native-accelerators.yml`. A provider that is absent must
  report "unavailable"; it must never report success from CPU under a native
  provider name.

## Declared deviations and unsupported surface

These are deliberate. Each entry is either a documented behavioral difference
or an explicitly unsupported API; silent divergence is a bug, an entry here is
not. When one of these is implemented, remove the entry and add conformance
coverage in the same change.

### `warp` vs NumPy

- **Unsupported dtypes:** complex, datetime64, timedelta64, structured/record
  arrays, and explicit byte-order (endianness) dtypes. No `float16` yet;
  reduced-precision floats are `float32` only.
- **`object` dtype** is declared and functional but stored on the portable
  list path, not packed host storage; performance and some view semantics
  differ from numeric dtypes.
- **uint64 above int64 max** is not representable: Weft numbers are int64 /
  float64, so casts and literals above `2^63-1` cannot round-trip exactly.
- **No Fortran-order storage**; all packed tensors are C-contiguous with
  strided views on top.
- **No read-only arrays** and **no mutable aliasing views**: `set` produces a
  new array; writing through a view does not mutate the base. NumPy code that
  relies on `v = a[::2]; v[:] = 0` mutating `a` behaves differently.
- **Masked arrays and sparse formats are unsupported** (no `numpy.ma`, no
  sparse matrices).
- **Seeded RNG algorithm differs from NumPy.** `warp.default_rng(seed)` uses a
  combined L'Ecuyer (1988) multiple-recursive generator, not NumPy's PCG64, so
  streams are **not** bit-compatible with `np.random.default_rng(seed)` for
  the same seed. What is guaranteed: the same seed reproduces the identical
  sequence across runs and processes, generators are independent of each other
  and of the global `random` stdlib state, and uniform/normal/integer draws
  meet distribution smoke bounds (locked by `warp_random_case.weft`, which
  checks deterministic properties only — never cross-implementation values).
- **Weft `%` (and `warp.mod_`) is truncated remainder**, carrying the sign of
  the dividend like Go/C (`-7 % 3 == -1`), while `np.mod`/`np.remainder`
  follows floored division and carries the sign of the divisor. Use
  `warp.remainder` for NumPy semantics (`remainder(-7, 3) == 2`); `mod_` is
  kept as-is for backwards compatibility. `%` also only accepts ints, while
  `remainder` works on floats.
- **`warp.reciprocal` always returns a float** (`1 / x`), unlike NumPy's
  integer reciprocal which truncates (`np.reciprocal(2) == 0`). `x == 0`
  yields `+inf`.
- **`warp.copysign` treats zero as positive** on the sign argument: Weft
  floats cannot reliably distinguish `-0.0` from `0.0` (division by zero
  raises), so `copysign(x, 0.0)` matches the `+0.0` NumPy case only.
- **`warp.put` returns a new array** rather than mutating in place (warp
  arrays are immutable); indexing, value-cycling, and last-write-wins
  semantics otherwise match `np.put` on the flattened C-order data.
- **`warp.polyfit` solves the normal equations** (LU), not NumPy's centered
  SVD least squares, so badly conditioned high-degree fits lose accuracy
  relative to `np.polyfit`. Underdetermined fits (fewer points than
  `deg + 1`) and singular systems are explicit errors, where NumPy only
  issues a `RankWarning` and continues.
- **`warp.roots` is analytic for degree ≤ 2 only**; higher degrees return an
  explicit unsupported error rather than an iterative companion-matrix solve.
  Degree-2 results can differ from NumPy's eigenvalue-based roots in the last
  ulps (e.g. repeated roots), so the conformance fixture uses well-separated
  roots.
- **`warp.searchsorted` assumes sorted input without validating it**, exactly
  like `np.searchsorted`; results on out-of-order data are unspecified on
  both sides.
- **`warp.bincount` rejects non-integer values** with an error; NumPy
  currently casts them with a deprecation warning.
- **`loc`-style label indexing, ellipsis (`...`) and `newaxis` selector tokens
  are not implemented**; missing trailing selectors mean "all".
- Error fixtures in the conformance corpus assert error *presence*; NumPy
  exception *types* (e.g. `LinAlgError`) are not mirrored.

### `dataframe` vs pandas

- **No nullable extension dtypes** (`Int64`, `boolean`, `string[python]`):
  missing values are Weft `null` in row maps; there is no NaN-vs-NA
  distinction, no categorical dtype, and no timezone-aware datetimes.
- **Storage is row-list**, not columnar; zero-copy DataFrame ↔ Warp
  interchange is impossible in this layout, so the tested interchange path
  always copies and rejects null/non-numeric values.
- **Duplicate labels:** reindexing with duplicate labels keeps first-match
  fill where pandas raises; `loc_labels`/`reindex` lookup keeps first-match
  where pandas returns all matches. Label selection (`loc_label`) and
  assignment (`loc_set`) do follow pandas: scalar selection returns every
  matching row, and assignment updates every matching row.
- **`loc(t, start, stop)` remains positional half-open slicing** (kept for
  backwards compatibility). Label-based pandas `.loc` semantics live in
  `loc_label(t, row_sel, col_sel?)`: scalar/list/boolean-mask row selectors
  and inclusive `{"from", "to"}` label slices (insertion points on monotonic
  indexes, `Err` on non-monotonic missing bounds and non-unique single-level
  left bounds — pandas wording), plus column selection. Assignment with
  pandas-observed broadcasting is `loc_set`/`iloc_set` (scalar broadcast,
  per-row/per-column lists, single-element-list broadcast; other lengths are
  explicit `Err`s). Deviations: unknown columns are rejected instead of
  created, and a bare list row selector on a multi-level index is a label
  list (wrap a full key: `[["a", 1]]`).
- **No Parquet/Arrow I/O and no SQL bridge**; CSV/JSON/JSONL only, with
  type-inferring parse (no explicit dtype/null policy arguments yet).

### `ml` vs PyTorch / scikit-learn

- **Forward-mode autodiff is dual-number based** (`jvp` / `jacobian` /
  `derivative` over scalars and warp arrays): exact JVPs at one function
  evaluation per input direction, so it suits few-input functions; reverse
  mode remains the cheap path for few-output losses. There is no JIT-fused
  vmap/jvp composition. Nested `create_graph` reverse mode is scalar —
  array-level higher-order gradients are numeric (finite difference), and
  nested duals cover the scalar second-derivative case.
- **No gradient checkpointing, anomaly detection, or sparse/autodiff-aware
  device placement.** Device tags remain advisory without a plugin; bound
  providers currently dispatch only tensor matmul, and every provider report
  is validated for truthful fallback/device status.
- **Classical ML coverage is linear/logistic regression** plus preprocessing;
  sparse and categorical estimator inputs are unsupported.

## Release gates

- differential tests against the pinned NumPy/pandas oracle;
- property/fuzz tests for indexing, broadcasting, dtypes, joins, and nulls;
- scale benchmarks and memory-limit tests;
- CPU plus real CUDA/ROCm/MLX provider runs;
- generated API/claim audit with no stale or unsupported documentation.

## Engineering gates added in-tree

- Channel-based `cron` (no spawn-captured mutability).
- Accelerator trust model (disable / allowlist / checksum).
- Host `tensor` stdlib package + Warp `_tid` storage.
- Expanded differential fixtures (edges, indexes) + property smoke.
- `scripts/check-vendor-sync.sh` fails CI on example vendor drift.
- `scripts/check-catalog-sync.sh` fails CI on catalog/manifest path or version drift.
- `scripts/accelerator-report.sh` publishes capability/status JSON.
- `scripts/accelerator-conformance.sh` gates CPU reference health/identity/matmul/tensor_matmul (vendors optional).
- `scripts/capability-matrix.py` writes an honest Warp/DataFrame/ML claim matrix.
- `scripts/capability-matrix.py --check` fails CI when the committed matrix is stale.
- `scripts/reproducible-build-check.sh` gates offline install (`GOPROXY=off`
  after `go mod download` + `go mod verify`) and byte-identical rebuilds from
  two checkout paths with `-trimpath -buildvcs=false`.
- `scripts/sbom.sh` emits the pinned dependency SBOM (module graph + go.sum
  hashes); the release workflow publishes it as `SBOM.json`.
- `scripts/bench-scale.sh` multi-fixture scale budgets (soft warn; `WEFT_SCALE_STRICT=1` hard).
