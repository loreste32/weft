# Weft native accelerator ABI

This directory defines the optional native provider boundary used by
`accelerator.load`. The host deliberately does not link vendor SDKs.

Build the included smoke provider:

```sh
# Linux
cc -shared -fPIC -I native/accelerator \
  native/accelerator/example.c -o /tmp/weft-accelerator-example.so

# macOS
cc -dynamiclib -fPIC -I native/accelerator \
  native/accelerator/example.c -o /tmp/weft-accelerator-example.dylib
```

Provider contract:

- export `weft_accel_manifest`, `weft_accel_run`, `weft_accel_free`, and
  `weft_accel_last_error`;
- report `abi: 1` and the actual vendor/runtime in the manifest;
- accept and return one UTF-8 JSON value per operation;
- allocate results in the provider and release them only in `weft_accel_free`;
- enforce provider-side tensor/device limits and return an error instead of
  partially writing an output.

CUDA providers may include the CUDA Runtime API and implement operations using
streams, device allocation, copies, and kernels. ROCm providers may use HIP or
rocBLAS, and MLX providers may use the MLX C++/Swift interface on Apple
Silicon. These are separate provider builds; the ABI does not pretend that a
CUDA binary runs on ROCm or that MLX is available on non-Apple hosts.

The example provider is only a loader smoke test. It is not a numerical
backend. A vendor provider must publish its own version, supported dtypes,
layouts, devices, operations, and numerical validation results.
