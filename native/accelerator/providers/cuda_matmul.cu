#include "../../accelerator/weft_accelerator.h"
#include "tensor_json.hpp"

#include <cuda_runtime.h>

#include <string>
#include <vector>

using weft_accel_provider::copy_output;
using weft_accel_provider::matrix;
using weft_accel_provider::matrix_json;
using weft_accel_provider::parse_matrix;

static std::string last_error;

__global__ void weft_matmul_kernel(const float* a, const float* b, float* out,
                                   size_t rows, size_t inner, size_t cols) {
  size_t row = blockIdx.y * blockDim.y + threadIdx.y;
  size_t col = blockIdx.x * blockDim.x + threadIdx.x;
  if (row >= rows || col >= cols) return;
  float value = 0.0f;
  for (size_t i = 0; i < inner; ++i) value += a[row * inner + i] * b[i * cols + col];
  out[row * cols + col] = value;
}

static bool cuda_ok(cudaError_t status, const char* operation) {
  if (status == cudaSuccess) return true;
  last_error = std::string(operation) + ": " + cudaGetErrorString(status);
  return false;
}

extern "C" const char* weft_accel_manifest(void) {
  return "{\"name\":\"weft-cuda\",\"version\":\"1.0.0\","
         "\"abi\":1,\"vendors\":[\"cuda\"],"
         "\"operations\":[\"health\",\"matmul\"],"
         "\"metadata\":{\"runtime\":\"cuda-runtime\",\"dtype\":\"float32\"}}";
}

extern "C" const char* weft_accel_last_error(void) { return last_error.c_str(); }

extern "C" int weft_accel_run(const char* operation, const char* input_json,
                               char** output_json) {
  if (operation == nullptr || input_json == nullptr || output_json == nullptr) {
    last_error = "operation, input, and output are required";
    return 1;
  }
  *output_json = nullptr;
  last_error.clear();
  if (std::strcmp(operation, "health") == 0) {
    int devices = 0;
    if (!cuda_ok(cudaGetDeviceCount(&devices), "cudaGetDeviceCount")) return 1;
    *output_json = copy_output(std::string("{\"ok\":true,\"backend\":\"cuda\",\"devices\":") +
                               std::to_string(devices) + "}");
    return *output_json == nullptr;
  }
  if (std::strcmp(operation, "matmul") != 0) {
    last_error = "unsupported operation";
    return 1;
  }
  matrix a, b;
  std::string error;
  if (!parse_matrix(input_json, "a", "a_shape", a, error) ||
      !parse_matrix(input_json, "b", "b_shape", b, error)) {
    last_error = error;
    return 1;
  }
  if (a.cols != b.rows) {
    last_error = "matmul inner dimensions do not match";
    return 1;
  }
  float *device_a = nullptr, *device_b = nullptr, *device_out = nullptr;
  size_t a_bytes = a.values.size() * sizeof(float);
  size_t b_bytes = b.values.size() * sizeof(float);
  size_t out_bytes = a.rows * b.cols * sizeof(float);
  if (!cuda_ok(cudaMalloc(&device_a, a_bytes), "cudaMalloc(a)") ||
      !cuda_ok(cudaMalloc(&device_b, b_bytes), "cudaMalloc(b)") ||
      !cuda_ok(cudaMalloc(&device_out, out_bytes), "cudaMalloc(out)")) {
    cudaFree(device_a); cudaFree(device_b); cudaFree(device_out);
    return 1;
  }
  bool ok = cuda_ok(cudaMemcpy(device_a, a.values.data(), a_bytes, cudaMemcpyHostToDevice), "cudaMemcpy(a)") &&
            cuda_ok(cudaMemcpy(device_b, b.values.data(), b_bytes, cudaMemcpyHostToDevice), "cudaMemcpy(b)");
  if (ok) {
    dim3 block(16, 16);
    dim3 grid((b.cols + block.x - 1) / block.x, (a.rows + block.y - 1) / block.y);
    weft_matmul_kernel<<<grid, block>>>(device_a, device_b, device_out, a.rows, a.cols, b.cols);
    ok = cuda_ok(cudaGetLastError(), "kernel launch") && cuda_ok(cudaDeviceSynchronize(), "cudaDeviceSynchronize");
  }
  std::vector<float> result(a.rows * b.cols);
  if (ok) ok = cuda_ok(cudaMemcpy(result.data(), device_out, out_bytes, cudaMemcpyDeviceToHost), "cudaMemcpy(result)");
  cudaFree(device_a); cudaFree(device_b); cudaFree(device_out);
  if (!ok) return 1;
  *output_json = copy_output(matrix_json(result, a.rows, b.cols));
  if (*output_json == nullptr) { last_error = "result allocation failed"; return 1; }
  return 0;
}

extern "C" void weft_accel_free(char* output_json) { std::free(output_json); }
