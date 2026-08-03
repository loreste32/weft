# warp — NumPy-style arrays for Weft

`warp` is a validated, row-major N-dimensional array module. It provides a
portable pure-Weft CPU path, broadcasting for common array shapes, reductions,
sorting, linear algebra, and explicit dispatch into a native accelerator
plugin. It is NumPy-inspired, not a drop-in replacement for every NumPy API.

```weft
use warp

fn main -> Result {
    a := warp.from_list([[1, 2, 3], [4, 5, 6]])
    b := warp.array([10, 20, 30], [3])
    say(warp.to_list(warp.add(a, b)))
    // [11, 22, 33, 14, 25, 36]
    say(warp.shape(warp.matmul(a, warp.array([1, 1, 1], [3]))))
    // [2]
}
```

## Execution backends

The CPU implementation is always available. Native execution is explicit and
capability-gated:

```weft
use warp

fn main -> Result {
    plugin := warp.accelerator_load("/opt/weft/libweft_accel_mlx.dylib")?
    result := warp.accelerator_run(plugin, "matmul", {
        "a": [1.0, 2.0, 3.0, 4.0], "a_shape": [2, 2],
        "b": [5.0, 6.0, 7.0, 8.0], "b_shape": [2, 2],
    })?
    say(result)
    warp.accelerator_close(plugin)?
}
```

Provider libraries implement the versioned ABI in
[`native/accelerator`](../../native/accelerator). CUDA, ROCm/HIP, and Apple
MLX are separate provider builds; the manifest must report the actual vendor,
supported operations, dtypes, layouts, and device constraints. The included
example provider only validates loading and JSON ownership—it is not a GPU
backend.

## API groups

- Creation: `array`, `from_list`, `zeros`, `ones`, `full`, `arange`,
  `linspace`, `eye`, `diag`, `rand`, `randn`, `randint`.
- Shape/indexing: `shape`, `size`, `ndim`, `reshape`, `flatten`, `ravel`,
  `squeeze`, `expand_dims`, `T`, `get`, `set`, `row`, `col`, `slice`.
- Element-wise math: arithmetic, comparisons, `where`, trigonometric,
  exponential, logarithmic, rounding, clipping, and sign operations.
- Statistics: reductions, cumulative operations, percentiles, axis reductions,
  `allclose`, `isnan`, `isinf`, and `nan_to_num`.
- Linear algebra: NumPy-style 1D/2D `dot` and `matmul`, norms, `outer`,
  `cross`, `trace`, `det`, `inv`, and `solve`.
- Manipulation: `concat`, validated `vstack`/`hstack`, `tile`, `repeat`,
  `flip`, stable O(n log n) `sort`/`argsort`, `unique`, and masking.
- Native dispatch: `accelerator_supported`, `accelerator_load`,
  `accelerator_run`, `accelerator_close`.

Arrays are immutable values: `set` and every arithmetic operation return new
arrays. Shape mismatches, ragged nested lists, invalid indices, and invalid
constructor parameters return errors instead of silently truncating data.

## Boundaries

The CPU path is designed for scripting, ETL, classical ML, and moderate arrays.
LU-based matrix inversion/solving remains O(n³). For very large workloads,
use a validated CUDA, ROCm, or MLX provider and compare results against the
CPU path on representative tolerances. This package does not claim complete
NumPy API, dtype, memory-layout, sparse-array, autodiff, or native-kernel
compatibility.
