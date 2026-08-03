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

The conformance harness will pin one NumPy and one pandas release per support
branch. The exact versions belong in the lockfile and CI artifact; unpinned
“latest” comparisons are not release evidence. Python is allowed as an
offline reference oracle for tests, but it is not a Weft runtime dependency.

## Required foundations

### `warp`

- `internal/tensor` now provides the first typed/strided host primitive and a
  binary provider ABI; the public `packages/warp` module still uses its
  portable list representation until the runtime bridge is complete;
- typed, strided storage with views, offsets, C/F order, and buffer export;
- complete dtype promotion/casting and scalar behavior;
- basic, advanced, boolean, and broadcasted indexing;
- reduction options (`axis`, `keepdims`, `where`, `initial`, `out`, `ddof`);
- ufunc-like dispatch, FFT/random/masked/sparse modules, and complete linear
  algebra coverage;
- binary tensor transport for native providers instead of JSON for large data.

### `dataframe`

- columnar storage with nullable, categorical, datetime, timezone, and
  extension dtypes;
- label-aware Series arithmetic, reindexing, `.loc`/`.iloc`/scalar access,
  duplicate indexes, and MultiIndex;
- complete groupby/aggregation/transform/window/join/reshape semantics;
- Arrow/Parquet/SQL-class I/O and memory-efficient 100k+ row execution.

### ML and providers

- tensor/autograd/optimizer/model serialization APIs;
- device selection, streams, asynchronous execution, memory pools, and
  deterministic fallback behavior;
- real hardware conformance jobs for every declared vendor provider.

## Release gates

- differential tests against the pinned NumPy/pandas oracle;
- property/fuzz tests for indexing, broadcasting, dtypes, joins, and nulls;
- scale benchmarks and memory-limit tests;
- CPU plus real CUDA/ROCm/MLX provider runs;
- generated API/claim audit with no stale or unsupported documentation.
