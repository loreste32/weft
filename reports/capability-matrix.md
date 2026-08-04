# Weft capability matrix

- **Generated:** 2026-08-04T12:35:48Z
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
| `warp` | 155 | `packages/warp/weft.json` |
| `dataframe` | 112 | `packages/dataframe/weft.json` |
| `ml` | 65 | `packages/ml/weft.json` |

## Claim summary

- implemented: **16** · partial: **11** · unsupported: **6**

## warp

| Claim | Status | Notes |
|-------|--------|-------|
| array creation (array/zeros/ones/arange) | `implemented` | Flat list + shape; typed constructors |
| fixed-width dtypes (bool/int/uint/float/object) | `implemented` | Packed host dtypes with range validation; not full NumPy casting table or complex/structured dtypes |
| host packed tensor storage (_tid) | `implemented` | Primary numeric storage via internal/tensor |
| elementwise arithmetic + broadcasting | `implemented` | Trailing broadcast; shape mismatch → Err |
| reductions (sum/mean/min/max/axis) | `implemented` | Axis opts partial (keepdims yes; where/initial/out no) |
| matmul / dot (1D/2D forms) | `implemented` | CPU; LU inv/solve O(n³) |
| strided views / transpose_view / advanced index | `partial` | Views + index API expanding; not full NumPy indexing |
| 1D FFT / IFFT / fft_freq | `partial` | 1D only; power-of-2 Cooley–Tukey or naive; not multi-dim/sparse/masked |
| sparse / masked arrays | `unsupported` | Not implemented |
| native accelerator dispatch (load/run/tensor) | `implemented` | Explicit path + capability; no silent load |
| CUDA / ROCm / MLX automatic kernels | `partial` | Vendor providers exist; require plugin path + hardware jobs |
| complete NumPy API replacement | `unsupported` | Experimental surface; not binary-compatible NumPy |

## dataframe

| Claim | Status | Notes |
|-------|--------|-------|
| from_rows / from_columns / CSV/JSON I/O | `implemented` | Row-list storage; quoted CSV |
| filter / query / sort / head/tail/iloc | `implemented` | iloc scalar/list; not full .loc label engine |
| groupby + aggregations + transform/size | `partial` | group_by, group_by_transform, group_by_size; not full pandas groupby API |
| join / merge / concat | `implemented` | Common how-modes; not full multi-key parity |
| DataFrame ↔ Warp numeric interchange | `partial` | Tested 1D/2D copying path; rejects null/non-numeric values; zero-copy not claimed |
| Series + explicit index / MultiIndex foundation | `partial` | Series helpers + multi-level foundation; not complete MultiIndex |
| pivot / melt / rolling / expanding | `implemented` | Present; window ops limited vs pandas |
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
| SGD / Adam optimizers | `implemented` | Scalar + Warp parameters |
| modules (linear/relu/sequential) + checkpoints | `implemented` | Advisory; not PyTorch parity |
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
