# Optional vendor providers

The providers in this directory are SDK-backed implementations of the Weft
ABI. They are deliberately separate shared libraries: the Weft executable does
not link CUDA, ROCm, or MLX, and a provider must only advertise hardware and
operations it actually supports.

All three providers implement the same bounded JSON `matmul` operation and
the binary `tensor_matmul` operation. The binary operation is the required path
for large batches; JSON remains a diagnostic/reference path:

```json
{
  "a": [1, 2, 3, 4], "a_shape": [2, 2],
  "b": [5, 6, 7, 8], "b_shape": [2, 2]
}
```

`matmul` returns `{"data":[19,22,43,50],"shape":[2,2]}`. The adapter is
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

The implementation uses HIP allocation, copies, kernel launch, synchronization,
and device-to-host transfer. It declares both `rocm` and `hip` vendors.

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
evaluates `mlx_matmul` on the default GPU stream, synchronizes before reading
the result, and frees every MLX handle. MLX uses unified memory, but the
provider still materializes results before returning them across the Weft ABI.

## Validation requirements

Before distributing a provider, run the native integration harness against the
actual library and publish its manifest, supported shapes/dtypes, device
requirements, error behavior, numerical tolerances, and benchmark results.
The reference providers are not a claim that every host has the corresponding
SDK or GPU; ordinary CI tests the ABI and CPU reference provider, while
vendor-hardware jobs must compile and run each provider on its own runner.

## Hardware CI

`.github/workflows/native-accelerators.yml` is a scheduled/manual conformance workflow. It is separate from pull-request CI because it requires self-hosted runners labeled `cuda`, `rocm`, and `mlx`. The CUDA runner needs `nvcc` and a working NVIDIA device; the ROCm runner needs `hipcc` and a working AMD device; the MLX runner needs Apple Silicon plus an `mlx-c` installation and repository variable `MLX_C_PREFIX`. Each job compiles the checked-in provider, loads it through Weft's `dlopen` ABI, and runs health plus 2×2 matmul checks. A green compile alone is not sufficient provider validation.
