# warp — NumPy-style arrays for Weft

`warp` is a validated, row-major N-dimensional array module. Numeric arrays use
host packed tensors (`tensor` stdlib / `internal/tensor`) as primary storage,
with a portable list cache for views and object dtype. Supported packed dtypes
are `bool`, `int8`, `int16`, `int32`, `int64`, `uint8`, `uint16`, `uint32`,
`uint64`, `float32`, and `float64`; `uint64` values are limited by Weft's
signed runtime integer representation. It provides NumPy-style
broadcasting and shape manipulation, reductions, sorting, linear algebra, and
explicit dispatch into a native accelerator plugin. It is a compatibility layer
under active expansion, not yet a binary-compatible replacement for every NumPy
API. Use `storage_kind(a)` to inspect `"tensor"` vs `"list"` backing.

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
  `linspace`, `eye`, `diag`, `rand`, `randn`, `randint`, plus `default_rng(seed)`
  for independent deterministic generators (uniform/normal/integer draws,
  `shuffle`, `permutation`, `choice`; per-seed reproducible, not NumPy
  bit-compatible).
- Shape/indexing: `shape`, `size`, `ndim`, `reshape` (including one `-1`),
  `flatten`, `ravel`, `squeeze`, `expand_dims`, `transpose`, `T`, `get`,
  `set`, `row`, `col`, `slice`.
- Element-wise math: arithmetic, comparisons, `where`, trigonometric,
  exponential, logarithmic, rounding, clipping, and sign operations, plus
  `hypot`, `expm1`, `log1p`, `floor_divide`, `remainder` (NumPy floored-mod
  semantics, unlike Weft `%`/`mod_`), `square`, `reciprocal`,
  `deg2rad`/`rad2deg`, `copysign`, and ties-to-even `rint`.
- Statistics: reductions, cumulative operations, percentiles, arbitrary-rank
  axis reductions, logical/finite predicates, `allclose`, and `nan_to_num`.
  Reduction options: `sum_opts`/`prod_opts`/`min_opts`/`max_opts` accept
  `axis`, `keepdims`, `initial`, and `where`; `var_opts`/`std_opts` accept
  `ddof`; `cumsum_axis`/`cumprod_axis` accumulate along one axis.
  `histogram(a, bins, range)` (NumPy half-open bins with a closed last bin),
  `bincount`, `cov`/`corrcoef` (ddof=1), weighted `average`, and
  linear-interpolation `quantile` cover the NumPy statistics core.
- Polynomials (highest-degree-first coefficients): `polyfit` (least squares
  via the normal equations and the LU solver; underdetermined or singular fits
  are an explicit error rather than NumPy's RankWarning), `polyval`,
  `polyder`, `polyint`, and `roots` for degree ≤ 2 with an explicit error for
  higher degrees.
- Searching/binning: `searchsorted(a, v, side)` (the input is assumed sorted,
  as in NumPy — out-of-order data is not validated) and `digitize(x, bins,
  right)` with monotonicity validation.
- Linear algebra: 1D/2D `dot` and `matmul`, norms, `outer`, `cross`, `trace`,
  `det`, `inv`, `solve`, plus `slogdet` (sign/log-magnitude determinant,
  {0, -inf} for exactly singular matrices), `matrix_rank` (row-echelon pivot
  count with an optional tolerance — not NumPy's SVD-based default), and
  `cond` (Frobenius condition number, equal to `np.linalg.cond(a, "fro")`,
  not the 2-norm default; +inf when singular).
- Signal: 1D `fft_1d`/`ifft_1d` and `fft_freq`, plus the real-input suite
  `rfft`/`irfft`/`rfft_freq` and `fftshift`/`ifftshift`. Multi-dimensional
  transforms `fft2`/`ifft2` (last two axes) and `fftn`/`ifftn` (all axes or an
  explicit axis list) compose the 1-D engine per slice; complex input is a
  `{"re": array, "im": array}` map.
- Native tensors: `accelerator_run_tensor` accepts bounded typed flat tensor
  descriptors and dispatches through the binary provider ABI. Use `release(a)`
  when a long-running program no longer needs a packed array's host handle.
- Manipulation: `concat`, axis-aware `concatenate`, `stack`, equal-section
  `split`, `take`, `put` (returns a new array), validated `vstack`/`hstack`,
  `tile`, `repeat`, `repeat_axis`, `flip`, `flip_axis`, stable O(n log n)
  `sort`/`argsort` with axis-aware `sort_axis`/`argsort_axis`, `unique`, and
  `unique_opts` with `return_index`/`return_counts`, and masking.

Arrays are immutable values: `set` and every arithmetic operation return new
arrays. Shape mismatches, ragged nested lists, invalid indices, and invalid
constructor parameters return errors instead of silently truncating data.
Common result typing is preserved: comparisons and logical predicates produce
`bool`, math functions produce `float64`, and arithmetic result dtypes follow
NumPy 2.x `promote_types` rules for every supported dtype pair (including
mixed signed/unsigned pairs such as `int8 + uint8 → int16` and
`int64 + uint64 → float64`). Axis and cumulative
operations retain their array dtype where the result is an array.

## Strided views and indexing


`transpose_view(a, axes?)`, `reshape`, and `view(a, shape, element_strides, offset)` expose immutable logical views over shared backing storage. `element_strides` reports element units; `strides` reports byte units. `to_list`, `get`, arithmetic, reductions, sorting, masking, and linear algebra read views in logical order, including negative strides. `contiguous` materializes a row-major copy when a packed result is required.

`index(a, selectors)` accepts integer selectors (including negative indices), rectangular integer index arrays, boolean masks, `slice_selector(start, stop, step)` selectors, and `null` full-axis selectors. Missing trailing selectors mean full axes. Multiple integer index arrays broadcast using NumPy's right-aligned rules; contiguous advanced axes keep basic-axis order, while separated advanced axes move the broadcast dimensions to the front. `slice_step` and `slice_axis` implement Python/NumPy start/stop/step normalization, including negative steps, as immutable strided views. `set(a, selectors, value)` returns an immutable updated array and accepts the same selectors; scalar values and Warp arrays broadcast to the selected shape. Assignment through ellipsis/new-axis syntax remains outside the current compatibility profile.

## Boundaries

The CPU path is designed for scripting, ETL, classical ML, and moderate arrays.
LU-based matrix inversion/solving remains O(n³). This package does not claim
complete NumPy API, full casting-table or packed dtype semantics,
memory-layout, sparse-array, autodiff, or native-kernel compatibility. Use validated CUDA, ROCm, or MLX providers for
large GPU workloads and compare results against the CPU path on representative
tolerances.
