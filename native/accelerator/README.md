# Weft native accelerator ABI

This directory defines the optional native provider boundary used by
`accelerator.load`. The Weft host deliberately does not link vendor SDKs.
Providers are explicitly selected shared libraries and must declare the
operations they implement.

## ABI v1

Every provider exports:

- `weft_accel_manifest()` — process-lifetime UTF-8 JSON with `name`, `version`,
  `abi`, non-empty unique `vendors`, and non-empty unique `operations`.
- `weft_accel_run(operation, input_json, output_json)` — one bounded JSON
  operation. It returns zero on success and allocates the output through the
  provider.
- `weft_accel_free(output_json)` — releases provider-owned output memory.
- `weft_accel_last_error()` — optional diagnostic text after a failed run.

The host validates ABI version, manifest fields, input/output JSON, and the
256 MiB boundary. It rejects operations that are not declared by the manifest.
The complete C declaration is [`weft_accelerator.h`](weft_accelerator.h).

## Reference provider

The checked-in C provider is a portable ABI and numerical reference. It has no
GPU dependency and implements `health`, `identity`, and validated float64
`matmul` JSON operations:

```sh
# Linux
cc -std=c11 -shared -fPIC -I native/accelerator \
  native/accelerator/example.c -o /tmp/weft-accelerator-example.so

# macOS
cc -std=c11 -dynamiclib -fPIC -I native/accelerator \
  native/accelerator/example.c -o /tmp/weft-accelerator-example.dylib
```

## Vendor providers

[`providers/`](providers/) contains SDK-backed implementations with the same
`matmul` contract:

- `cuda_matmul.cu` uses CUDA Runtime allocation, copies, a checked kernel
  launch, synchronization, and device-to-host transfer.
- `rocm_matmul.hip` uses HIP allocation, copies, kernel launch, synchronization,
  and device-to-host transfer.
- `mlx_matmul.cpp` uses the official MLX C API, evaluates on the default GPU
  stream, synchronizes before reading, and frees every MLX handle.

Build commands, SDK requirements, and validation expectations are in
[`providers/README.md`](providers/README.md). These optional providers cannot
be compiled or hardware-tested on hosts without their SDK/device; ordinary CI
tests the portable provider, loader, manifest contract, and shared tensor
adapter. Vendor-specific CI must run on CUDA, ROCm, and Apple Silicon runners.

The JSON adapter is a correctness and ABI reference, not the final high-volume
transport. A production provider should add a binary tensor path before moving
large training batches across the plugin boundary.
