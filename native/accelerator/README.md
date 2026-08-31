# Weft native accelerator ABI

This directory defines the optional native provider boundary used by
`accelerator.load`. The Weft host deliberately does not link vendor SDKs.
Providers are explicitly selected shared libraries and must declare the
operations they implement.

## Trust model

Native providers are **trusted host code**. `dlopen` loads them into the Weft
process and they bypass the language sandbox completely.

| Control | Environment variable |
|---------|----------------------|
| Hard disable | `WEFT_ACCELERATOR_DISABLE=1` |
| Path allowlist | `WEFT_ACCELERATOR_ALLOWLIST` (files or directories; `:`/`,`/`;` separated) |
| Require checksum | `WEFT_ACCELERATOR_REQUIRE_CHECKSUM=1` |
| Expected SHA-256 | `WEFT_ACCELERATOR_CHECKSUM` or sidecar `<plugin-path>.sha256` |

Registry packages cannot silently activate plugins: loads require an explicit
filesystem path from application code, plus the `accelerator` capability for
third-party modules. Production servers should set an allowlist and require
checksums. Generate a capability report with `make accelerator-report`.

## Optional binary tensor transport

Providers may additionally export `weft_accel_run_tensor` and
`weft_accel_free_tensor` and declare the operation in their manifest. Tensor
descriptors carry one of the ABI dtypes (`bool`, `int8`, `int16`, `int32`,
`int64`, `uint8`, `uint16`, `uint32`, `uint64`, `float16`, `float32`, or
`float64`), rank,
shape, element strides, byte length, and data. Inputs are borrowed for the
duration of the call; output storage remains provider-owned until the free
callback. The host validates dtype, rank, shape, byte length, and the 256 MiB
boundary before constructing a Weft tensor. The initial host contract requires
C-contiguous output; providers may reject unsupported layouts explicitly.

The reference provider implements float64 `tensor_matmul`, bounded same-shape
elementwise `tensor_add` / `tensor_sub` / `tensor_mul` / `tensor_div`
(float16, float32, and float64), and full-reduction `tensor_sum` (float16,
float32, and float64) for rank-1/rank-2 tensors. float16 uses ABI dtype code
12 with binary16 storage: the reference provider widens inputs to float32 for
the computation and rounds results back to half precision with
round-to-nearest-even, matching the host tensor package's NumPy 2.x
conversion semantics. `tensor_sum` returns a rank-0 tensor holding one
element of the input dtype, matching NumPy `np.sum` full-reduction semantics.
Vendor providers implement binary symbols for float32 `tensor_matmul` and the
same-shape elementwise ops; `tensor_sum` is currently reference-only because a
correct parallel reduction is not part of the vendors' validated pattern set.
Each provider must publish its supported dtype/layout matrix before being used
for large training batches. Broadcasting is deliberately not claimed by any
provider: the ABI transports strides, but every elementwise op rejects
non-same-shape inputs with an explicit error rather than guessing.

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

## Execution reporting (required for conformance)

Every operation must report where it ran. Silent fallback is not allowed.

- JSON operations include `"device"`, `"requested_device"`, and `"fallback"`
  fields in every result object.
- Providers additionally export `char *weft_accel_exec_info(void)` (additive
  to ABI v1; the version is not bumped). It returns a newly allocated JSON
  object describing the most recent `weft_accel_run` or
  `weft_accel_run_tensor` call:
  `{"device":"cuda:0","requested_device":"cuda:0","fallback":false,"status":"device"}`.
  The host releases the string through `weft_accel_free`.

The host parses these fields into a typed `ExecInfo` with status `device`
(ran on requested device), `fallback` (fell back to CPU), `unavailable`
(no provider ran the op), or `unreported` (provider said nothing). A
contradictory report — a device other than the requested one with
`fallback: false`, or a `status` that disagrees with the fallback flag — is
rejected as an error. A missing export or malformed document is
`unreported`, which fails `make accelerator-conformance` and the
`TestExternalProviderReporting` gate; the conformance report classifies each
provider as `honest`, `unreported`, or `contradictory`.

## Reference provider

The checked-in C provider is a portable ABI and numerical reference. It has no
GPU dependency and implements `health`, `identity`, validated float64 `matmul`
JSON operations, and binary `tensor_matmul`, same-shape elementwise
`tensor_add` / `tensor_sub` / `tensor_mul` / `tensor_div`, and
full-reduction `tensor_sum` (rank-0 output, NumPy `np.sum` semantics). The
elementwise ops accept contiguous same-shape float16, float32, or float64
tensors of rank 1 or 2; they are explicitly same-shape and unsupported
broadcasting must return an error. Health and matmul JSON include explicit `device` /
`fallback` fields so hosts never treat silent CPU fallback as device success:

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

The JSON adapter remains a correctness and ABI reference for diagnostics. The
reference provider declares `tensor_matmul`, the four same-shape elementwise
ops, and `tensor_sum`; the CUDA, ROCm/HIP, and MLX providers declare
`tensor_matmul` plus the four elementwise ops (float32 — they reject ABI
dtype code 12, float16, along with every other dtype outside their claim).
Vendor hardware jobs
must still validate them on real devices before large training batches are
considered release-ready.
