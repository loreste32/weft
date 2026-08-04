# warp — validated NumPy-style arrays

`warp` provides row-major arrays represented as `{_warp: true, data, shape,
_dtype, _strides, _offset}`. Dtypes are logical metadata over ordinary Weft
values, not a claim of packed native storage. Strides and offsets support
immutable logical views over shared portable backing lists.
It is a portable CPU numerical layer for Weft with explicit error handling and
an optional native accelerator ABI.

## Contracts

- `array(data, shape)` requires a flat list and an exact element count.
- `array_typed`, `astype`, `dtype`, and typed constructors support `bool`,
  `int64`, `float32`, `float64`, and `object` with validated numeric casts.
- `from_list` accepts 1D lists or rectangular 2D lists; ragged input is an
  error.
- Binary operations support scalars, equal shapes, and NumPy-style trailing
  broadcasting. Incompatible shapes return `Err`.
- `reshape` accepts one inferred `-1` dimension; `squeeze` can produce a
  zero-dimensional array; negative axes are normalized where supported.
- `sum_axis`, `mean_axis`, `min_axis`, and `max_axis` accept negative axes and
  arbitrary rank, returning a scalar when a rank-1 axis is reduced.
- `sum_axis_opts`, `mean_axis_opts`, `min_axis_opts`, and `max_axis_opts` add
  explicit `keepdims` behavior for shape-stable reduction pipelines.
- `transpose`, `concatenate`, `stack`, equal-section `split`, and `take` use
  row-major shape-preserving semantics.
- `repeat_axis` and `flip_axis` provide explicit multidimensional axis
  operations while preserving the fixed-arity `repeat` and `flip` APIs.
- `matmul` and `dot` support 1D×1D, 2D×1D, 1D×2D, and 2D×2D forms.
- `set` returns a copy and does not mutate the source array.
- Empty statistics return `null` where a numeric identity is undefined;
  `sum([])` is `0` and `prod([])` is `1`.
- `sort` and `argsort` use stable merge sort, so they are O(n log n).

## Native backends

Native libraries are never loaded implicitly. A program must have the
`accelerator` capability and call `warp.accelerator_load(path)`. The provider
ABI is documented in [`native/accelerator`](../native/accelerator). CUDA,
ROCm/HIP, and MLX are separate provider implementations; each provider must
declare its actual vendor, supported operations, dtypes, layouts, and device
constraints. The checked-in example library is only an ABI smoke test.

For binary tensor dispatch, pass flat tensor descriptors to
`warp.accelerator_run_tensor(plugin, operation, inputs)`. Each descriptor has
`dtype`, `shape`, and `data`; the operation returns the same descriptor shape.
This path is bounded, validates dtype/shape/byte limits, and avoids JSON tensor
serialization. It is currently an explicit provider operation contract, not a
general NumPy device array or asynchronous stream API.

```weft
use warp

fn main -> Result {
    plugin := warp.accelerator_load("/opt/weft/libweft_accel_cuda.so")?
    output := warp.accelerator_run(plugin, "matmul", {
        "a": [1.0, 2.0, 3.0, 4.0], "a_shape": [2, 2],
        "b": [5.0, 6.0, 7.0, 8.0], "b_shape": [2, 2],
    })?
    say(output)
    warp.accelerator_close(plugin)?
}
```

## Strided views and indexing


Arrays carry element strides and an element offset. `transpose_view` and `view` are immutable zero-copy logical views; `strides` reports byte strides and `element_strides` reports element strides. Existing value-producing operations materialize logical view order, and `contiguous` creates a row-major copy. `slice_step`, `slice_axis`, and `slice_selector` implement normalized Python/NumPy stepped slices, including negative steps. `index` supports integer, negative, rectangular integer index arrays, boolean-mask, slice, and full-axis selectors. Multiple integer index arrays use NumPy right-aligned broadcasting; contiguous advanced axes preserve the basic-axis position, and separated advanced axes place the broadcast dimensions first. Assignment through advanced-index views and ellipsis/new-axis syntax are not implemented.

## API surface

The package manifest exports the complete public surface: creation,
shape/indexing, element-wise math, comparisons, reductions, statistics,
linear algebra, manipulation, masking, inspection, and native dispatch.
Run `weft mod check packages/warp --tests` to verify the manifest and its 86
regression tests.

## Deliberate limits

`warp` is not yet a complete binary-compatible NumPy replacement. It does not
yet implement every dtype, memory order, sparse matrix, masked array, FFT,
autodiff, or ufunc protocol. The logical dtype layer does not provide packed
memory or NumPy's full casting table. The CPU implementation is pure Weft and does not
promise BLAS/SIMD performance. Use native providers for large GPU workloads
and validate numerical equivalence for the operations you rely on.
