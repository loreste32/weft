# warp — validated NumPy-style arrays

`warp` provides row-major arrays represented as `{_warp: true, data, shape}`.
It is a portable CPU numerical layer for Weft with explicit error handling and
an optional native accelerator ABI.

## Contracts

- `array(data, shape)` requires a flat list and an exact element count.
- `from_list` accepts 1D lists or rectangular 2D lists; ragged input is an
  error.
- Binary operations support scalars, equal shapes, and NumPy-style trailing
  broadcasting. Incompatible shapes return `Err`.
- `matmul` and `dot` support 1D×1D, 2D×1D, 1D×2D, and 2D×2D forms.
- `set` returns a copy and does not mutate the source array.
- Empty statistics return `null` where a numeric identity is undefined;
  `sum([])` is `0` and `prod([])` is `1`.
- `sort` and `argsort` use a stable merge sort, so they are O(n log n) rather
  than the previous quadratic insertion sort.

## Native backends

Native libraries are never loaded implicitly. A program must have the
`accelerator` capability and call `warp.accelerator_load(path)`. The provider
ABI is documented in [`native/accelerator`](../native/accelerator):

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

CUDA, ROCm/HIP, and MLX are separate provider implementations. A provider
must declare its actual vendor and supported operations; the example library
is only an ABI smoke test.

## API surface

The package manifest is authoritative and exports the full public surface:
constructors, shape/indexing, element-wise math, comparisons, reductions,
statistics, linear algebra, manipulation, masking, inspection, and native
dispatch. Run `weft mod check packages/warp --tests` to verify the manifest and
the 79 regression tests.

## Deliberate limits

`warp` is not a complete binary-compatible NumPy replacement. It does not yet
implement every dtype, memory order, sparse matrix, masked array, FFT,
autodiff, or ufunc protocol. The CPU implementation is pure Weft and does not
promise BLAS/SIMD performance. Use native providers for large GPU workloads
and validate numerical equivalence for the operations you rely on.
