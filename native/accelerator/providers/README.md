# Optional vendor providers

The providers in this directory are SDK-backed implementations of the Weft
ABI. They are deliberately separate shared libraries: the Weft executable does
not link CUDA, ROCm, or MLX, and a provider must only advertise hardware and
operations it actually supports.

All three providers implement the same bounded JSON `matmul` operation and
binary `tensor_matmul` plus the same-shape elementwise operations
`tensor_add`, `tensor_sub`, `tensor_mul`, and `tensor_div`. CUDA and ROCm
also implement the full-reduction `tensor_sum` operation. The binary
operations are the required paths for large batches; JSON remains a
diagnostic/reference path. The elementwise ops accept contiguous, same-shape
float32 tensors of rank 1 or 2 and do not claim NumPy-style broadcasting —
non-same-shape inputs are rejected with an explicit error:

```json
{
  "a": [1, 2, 3, 4], "a_shape": [2, 2],
  "b": [5, 6, 7, 8], "b_shape": [2, 2]
}
```

## Operation coverage

| Operation | CPU reference (`../example.c`) | CUDA | ROCm/HIP | MLX |
|-----------|-------------------------------|------|----------|-----|
| `health`, `matmul` (JSON) | yes (float64) | yes (float32) | yes (float32) | yes (float32) |
| `tensor_matmul` | yes (float64) | yes (float32) | yes (float32) | yes (float32) |
| `tensor_add` / `tensor_sub` / `tensor_mul` / `tensor_div` | yes (float32 + float64) | yes (float32) | yes (float32) | yes (float32) |
| `tensor_sum` (full reduction → rank-0) | yes (float32 + float64) | yes (float32) | yes (float32) | no |
| broadcasting elementwise | no (explicit rejection) | no (explicit rejection) | no (explicit rejection) | no (explicit rejection) |

CUDA and ROCm currently use a deterministic single-device-thread reduction.
That is a correctness/conformance implementation, not a performance claim;
large reductions still need a parallel-kernel benchmark before release
claims. MLX does not declare `tensor_sum` until its installed `mlx-c` axis
contract is compile-verified. The conformance gate
(`TestExternalProviderTensorCoverage`) fails any declared op that errors.
Only the CPU reference row is exercised in ordinary CI; vendor rows must be
compiled and run on their labeled hardware runners.

`matmul` returns `{"data":[19,22,43,50],"shape":[2,2]}` plus mandatory
execution reporting fields: `"device"` (e.g. `"cuda:0"`, `"rocm:0"`,
`"mlx:0"`), `"requested_device"`, and `"fallback"`. `health` returns the same
fields. Each provider also exports `weft_accel_exec_info` (see
[`../weft_accelerator.h`](../weft_accelerator.h)) so the binary tensor path
carries the same report. These providers never fall back to CPU: a device
failure is an error return, not a silent host-side result. The adapter is
intentionally a correctness and ABI reference; production providers should add
a binary tensor path before moving very large batches through JSON.

## CUDA

Requires the CUDA Toolkit and a compatible GPU:

```sh
nvcc -std=c++17 -shared -Xcompiler -fPIC \
  -I native/accelerator/providers \
  -I native/accelerator \
  native/accelerator/providers/cuda_matmul.cu \
  -o libweft_accel_cuda.so
```

The implementation uses CUDA Runtime allocation, host/device copies, a checked
kernel launch, synchronization, and device-to-host copy. It reports the CUDA
runtime provider and float32 operation in its manifest.

## ROCm/HIP

Requires ROCm HIP and a compatible AMD GPU:

```sh
hipcc -std=c++17 -shared -fPIC \
  -I native/accelerator/providers \
  -I native/accelerator \
  native/accelerator/providers/rocm_matmul.hip \
  -o libweft_accel_rocm.so
```

The implementation uses HIP allocation, copies, matmul and elementwise kernel
launches, synchronization, and device-to-host transfer. It declares both
`rocm` and `hip` vendors.

## Apple MLX

Requires the MLX C package and CMake-built MLX libraries on Apple Silicon:

```sh
c++ -std=c++17 -shared -fPIC \
  -I path/to/mlx-c \
  -I native/accelerator/providers \
  -I native/accelerator \
  native/accelerator/providers/mlx_matmul.cpp \
  -lmlxc -lmlx \
  -o libweft_accel_mlx.dylib
```

The implementation uses the official MLX C API, creates float32 arrays,
evaluates `mlx_matmul` and `mlx_add` on the default GPU stream, synchronizes
before reading results, and frees every MLX handle. MLX uses unified memory, but
the provider still materializes results before returning them across the Weft ABI.

## Validation requirements

Before distributing a provider, run the native integration harness against the
actual library and publish its manifest, supported shapes/dtypes, device
requirements, error behavior, numerical tolerances, and benchmark results.
The reference providers are not a claim that every host has the corresponding
SDK or GPU; ordinary CI tests the ABI and CPU reference provider, while
vendor-hardware jobs must compile and run each provider on its own runner.

## Hardware CI

Hardware conformance covers health, 2×2 JSON matmul, binary tensor_matmul,
and every tensor op the provider declares (`TestExternalProviderTensorCoverage`
probes each manifest entry and fails declared-but-broken ops). A successful
compile alone is not provider validation.

`.github/workflows/native-accelerators.yml` is a scheduled/manual conformance workflow. It is separate from pull-request CI because it requires self-hosted runners labeled `cuda`, `rocm`, and `mlx`. The CUDA runner needs `nvcc` and a working NVIDIA device; the ROCm runner needs `hipcc` and a working AMD device; the MLX runner needs Apple Silicon plus an `mlx-c` installation and repository variable `MLX_C_PREFIX`. Each job compiles the checked-in provider, loads it through Weft's `dlopen` ABI, and runs health plus 2×2 matmul checks. A green compile alone is not sufficient provider validation.
