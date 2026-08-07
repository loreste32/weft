# Weft capability matrix

- **Generated:** 2026-08-07T19:37:27Z
- **Format:** `weft.capability.matrix` v1
- **Honesty:** Statuses are conservative. implemented ≠ complete NumPy/pandas/ML framework parity. See docs/COMPATIBILITY.md.

## Status vocabulary

| Status | Meaning |
|--------|---------|
| `implemented` | Present and covered at smoke/semantics level for the claim |
| `partial` | Present but missing options, edge cases, scale, or backend parity |
| `unsupported` | Not implemented or deliberately out of scope for replacement claims |

## Export surface (from weft.json)

| Package | Export count | Manifest |
|---------|--------------|----------|
| `warp` | 204 | `packages/warp/weft.json` |
| `dataframe` | 134 | `packages/dataframe/weft.json` |
| `ml` | 114 | `packages/ml/weft.json` |

## Claim summary

- implemented: **19** · partial: **13** · unsupported: **6**

## warp

| Claim | Status | Notes |
|-------|--------|-------|
| array creation (array/zeros/ones/arange) | `implemented` | Flat list + shape; typed constructors |
| fixed-width dtypes (bool/int/uint/float/object) | `implemented` | Packed host dtypes with range validation; promotion matches NumPy 2.x promote_types for all supported pairs; full astype casting table and complex/structured dtypes not claimed |
| host packed tensor storage (_tid) | `implemented` | Primary numeric storage via internal/tensor |
| elementwise arithmetic + broadcasting | `implemented` | Trailing broadcast; shape mismatch → Err |
| reductions (sum/mean/min/max/axis) | `implemented` | Axis opts: keepdims/initial/where/ddof yes; out no |
| statistics extras (histogram/bincount/cov/corrcoef/average/quantile) | `implemented` | NumPy half-open histogram with closed last bin; ddof=1 cov/corrcoef; linear-interpolation quantile |
| polynomial + searching helpers (polyfit/polyval/roots/searchsorted/digitize) | `partial` | Normal-equations polyfit (no SVD centering/scaling); analytic roots degree <= 2 only; 1-D searchsorted/digitize assume sorted/monotonic input |
| matmul / dot (1D/2D forms) | `implemented` | CPU; LU inv/solve O(n³) |
| strided views / transpose_view / advanced index | `partial` | Views + index API expanding; not full NumPy indexing |
| 1D FFT / IFFT / fft_freq | `partial` | 1D only; rfft/irfft/rfft_freq/fftshift/ifftshift included; power-of-2 Cooley–Tukey or naive; not multi-dim/sparse/masked |
| seeded RNG (default_rng) | `partial` | Deterministic per seed, independent generators; combined L'Ecuyer MRG, not PCG64 — no NumPy bit parity |
| sparse / masked arrays | `unsupported` | Not implemented |
| native accelerator dispatch (load/run/tensor) | `implemented` | Explicit path + capability; no silent load |
| CUDA / ROCm / MLX automatic kernels | `partial` | Vendor providers expose bounded float32 matmul + same-shape tensor_add; require explicit plugin path + hardware jobs |
| complete NumPy API replacement | `unsupported` | Experimental surface; not binary-compatible NumPy |

## dataframe

| Claim | Status | Notes |
|-------|--------|-------|
| from_rows / from_columns / CSV/JSON/SQL I/O | `implemented` | Row-list storage; quoted CSV; from_csv_opts/read_csv_opts dtype+null policies; read_sql/to_sql SQLite bridge via stdlib db (quoted identifiers, bound params) |
| filter / query / sort / head/tail/iloc | `implemented` | iloc scalar/list; loc_label/loc_set/iloc_set give label selection + assignment with pandas-observed broadcasting; positional loc kept for compat |
| groupby + aggregations + transform/size | `partial` | group_by with single/composite keys + per-column agg lists, group_by_transform, group_by_size, pivot_table aggfuncs; not full pandas groupby API |
| join / merge / concat | `implemented` | Common how-modes; not full multi-key parity |
| DataFrame ↔ Warp numeric interchange | `partial` | Tested 1D/2D copying path; rejects null/non-numeric values; zero-copy not claimed |
| Series + explicit index / MultiIndex foundation | `partial` | Series helpers + multi-level foundation; not complete MultiIndex |
| pivot / melt / rolling / expanding / ewm | `implemented` | rolling+expanding over sum/mean/count/min/max/first/last/std/var; ewm_mean/sum/var/std with alpha/span/halflife, adjust, ignore_na, bias matched to pandas 3.0 recursions (ewm_sum adjust=true only, as pandas); resample still open |
| nullable / categorical / datetime dtypes | `unsupported` | Null-aware stats only; no extension dtypes |
| Arrow / Parquet / columnar backend | `unsupported` | Row-list only |
| 100k+ row ETL scale | `partial` | Scale smoke exists; not memory-optimized for multi-GB |
| complete pandas API replacement | `unsupported` | Experimental; deliberate subset |

## ml

| Claim | Status | Notes |
|-------|--------|-------|
| vectors (dot/cosine/norm/topk) | `implemented` | Pure Weft |
| embeddings + local index (RAG helpers) | `partial` | Provider-backed embed; needs network/keys |
| classical linear / logistic fit + score | `implemented` | CPU minibatch; 100k-row train tested; accepts nested lists and packed Warp inputs |
| reverse-mode autodiff (scalars + warp) | `implemented` | Tape ops; not full framework |
| forward-mode autodiff (dual numbers / JVP) | `implemented` | Exact jvp/jacobian/derivative over scalars + warp arrays; jacobian costs one evaluation per input; nested duals give scalar second derivatives; three-way checked vs reverse mode + gradcheck |
| SGD / Adam optimizers | `implemented` | Scalar + Warp parameters; skip frozen params; grad clipping (global-norm PyTorch formula + elementwise value) |
| modules (linear/activations/sequential) + checkpoints | `implemented` | linear + relu/sigmoid/tanh/gelu/softmax; advisory; not PyTorch parity; named freeze/unfreeze; v2 checkpoints resume bit-identically (weights + Adam state + epoch) |
| differentiable losses + LR schedules + seeded batching | `implemented` | mse/bce/cross-entropy/huber gradchecked; step/exponential/cosine LR; seeded shuffle + drop_last batches |
| higher-order grads (create_graph / gradcheck / hvp) | `partial` | Scalar nested reverse-mode create_graph; array VJPs numeric; HVP finite-diff |
| device placement (cpu/cuda/rocm/mlx) | `partial` | Advisory tags; non-CPU → fallback:true, compute stays CPU |
| GPU training without external provider | `unsupported` | No fake GPU; needs native accelerator plugin |
| ONNX / Triton local inference | `partial` | Via mlinfer HTTP sidecars, not in-process |

## How to refresh

```sh
make capability-matrix
# or: python3 scripts/capability-matrix.py
```

Edit the hand-maintained `CLAIMS` table in `scripts/capability-matrix.py` when adding APIs or changing honesty status. Do not mark a claim `implemented` without tests or a documented smoke path.
