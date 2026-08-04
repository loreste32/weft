# warp — NumPy-style arrays for Weft

`warp` is a validated, row-major N-dimensional array module. It provides a
portable pure-Weft CPU path, NumPy-style broadcasting and shape manipulation,
reductions, sorting, linear algebra, and explicit dispatch into a native
accelerator plugin. It is a compatibility layer under active expansion, not
yet a binary-compatible replacement for every NumPy API.

```weft
use warp

fn main -> Result {
    a := warp.from_list([[1, 2, 3], [4, 5, 6]])
    b := warp.array([10, 20, 30], [3])
    say(warp.to_list(warp.add(a, b)))
    say(warp.shape(warp.matmul(a, warp.array([1, 1, 1], [3]))))
}
```

## Execution backends

The CPU implementation is always available. Native execution is explicit and
capability-gated. Provider libraries implement the versioned ABI in
[`native/accelerator`](../../native/accelerator). CUDA, ROCm/HIP, and Apple
MLX are separate provider builds; the manifest must report the actual vendor,
supported operations, dtypes, layouts, and device constraints. The included
example provider only validates loading and JSON ownership—it is not a GPU
backend.

## API groups

- Creation: `array`, `array_typed`, `astype`, `dtype`, typed constructors,
  `from_list`, `zeros`, `ones`, `full`, `arange`,
  `linspace`, `eye`, `diag`, `rand`, `randn`, `randint`.
- Shape/indexing: `shape`, `size`, `ndim`, `reshape` (including one `-1`),
  `flatten`, `ravel`, `squeeze`, `expand_dims`, `transpose`, `T`, `get`,
  `set`, `row`, `col`, `slice`.
- Element-wise math: arithmetic, comparisons, `where`, trigonometric,
  exponential, logarithmic, rounding, clipping, and sign operations.
- Statistics: reductions, cumulative operations, percentiles, arbitrary-rank
  axis reductions, logical/finite predicates, `allclose`, and `nan_to_num`.
- Linear algebra: 1D/2D `dot` and `matmul`, norms, `outer`, `cross`, `trace`,
  `det`, `inv`, and `solve`.
- Native tensors: `accelerator_run_tensor` accepts bounded typed flat tensor
  descriptors and dispatches through the binary provider ABI.
- Manipulation: `concat`, axis-aware `concatenate`, `stack`, equal-section
  `split`, `take`, validated `vstack`/`hstack`, `tile`, `repeat`,
  `repeat_axis`, `flip`, `flip_axis`, stable O(n log n) `sort`/`argsort`,
  `unique`, and masking.

Arrays are immutable values: `set` and every arithmetic operation return new
arrays. Shape mismatches, ragged nested lists, invalid indices, and invalid
constructor parameters return errors instead of silently truncating data.
Common result typing is preserved: comparisons and logical predicates produce
`bool`, math functions produce `float64`, and arithmetic follows the supported
`bool`/`int64`/`float32`/`float64` promotion rules. Axis and cumulative
operations retain their array dtype where the result is an array.

## Strided views and indexing

`transpose_view(a, axes?)`, `reshape`, and `view(a, shape, element_strides, offset)` expose immutable logical views over shared backing storage. `element_strides` reports element units; `strides` reports byte units. `to_list`, `get`, arithmetic, reductions, sorting, masking, and linear algebra read views in logical order, including negative strides. `contiguous` materializes a row-major copy when a packed result is required.

`index(a, selectors)` accepts integer selectors (including negative indices), integer-position lists, boolean masks, and `null` full-axis selectors. Missing trailing selectors mean full axes. This is a bounded NumPy-style indexing surface; advanced multi-axis broadcasting, assignment views, and the complete slicing grammar remain outside the current compatibility profile.

## Boundaries

The CPU path is designed for scripting, ETL, classical ML, and moderate arrays.
LU-based matrix inversion/solving remains O(n³). This package does not claim
complete NumPy API, full casting-table or packed dtype semantics,
memory-layout, sparse-array, autodiff, or native-kernel compatibility. Use validated CUDA, ROCm, or MLX providers for
large GPU workloads and compare results against the CPU path on representative
tolerances.
