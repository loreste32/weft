#include "../../accelerator/weft_accelerator.h"
#include "tensor_json.hpp"

#include <hip/hip_runtime.h>

#include <cstring>
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

static bool hip_ok(hipError_t status, const char* operation) {
  if (status == hipSuccess) return true;
  last_error = std::string(operation) + ": " + hipGetErrorString(status);
  return false;
}

extern "C" const char* weft_accel_manifest(void) {
  return "{\"name\":\"weft-rocm\",\"version\":\"1.0.0\","
         "\"abi\":1,\"vendors\":[\"rocm\",\"hip\"],"
         "\"operations\":[\"health\",\"matmul\",\"tensor_matmul\"],"
         "\"metadata\":{\"runtime\":\"hip\",\"dtype\":\"float32\",\"tensor_abi\":\"1\"}}";
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
    if (!hip_ok(hipGetDeviceCount(&devices), "hipGetDeviceCount")) return 1;
    *output_json = copy_output(std::string("{\"ok\":true,\"backend\":\"rocm\",\"devices\":") +
                               std::to_string(devices) + "}");
    return *output_json == nullptr;
  }
  if (std::strcmp(operation, "matmul") != 0) { last_error = "unsupported operation"; return 1; }
  matrix a, b;
  std::string error;
  if (!parse_matrix(input_json, "a", "a_shape", a, error) ||
      !parse_matrix(input_json, "b", "b_shape", b, error)) { last_error = error; return 1; }
  if (a.cols != b.rows) { last_error = "matmul inner dimensions do not match"; return 1; }
  float *device_a = nullptr, *device_b = nullptr, *device_out = nullptr;
  size_t a_bytes = a.values.size() * sizeof(float), b_bytes = b.values.size() * sizeof(float);
  size_t out_bytes = a.rows * b.cols * sizeof(float);
  if (!hip_ok(hipMalloc(&device_a, a_bytes), "hipMalloc(a)") ||
      !hip_ok(hipMalloc(&device_b, b_bytes), "hipMalloc(b)") ||
      !hip_ok(hipMalloc(&device_out, out_bytes), "hipMalloc(out)")) {
    hipFree(device_a); hipFree(device_b); hipFree(device_out); return 1;
  }
  bool ok = hip_ok(hipMemcpy(device_a, a.values.data(), a_bytes, hipMemcpyHostToDevice), "hipMemcpy(a)") &&
            hip_ok(hipMemcpy(device_b, b.values.data(), b_bytes, hipMemcpyHostToDevice), "hipMemcpy(b)");
  if (ok) {
    dim3 block(16, 16);
    dim3 grid((b.cols + block.x - 1) / block.x, (a.rows + block.y - 1) / block.y);
    hipLaunchKernelGGL(weft_matmul_kernel, grid, block, 0, 0, device_a, device_b, device_out, a.rows, a.cols, b.cols);
    ok = hip_ok(hipGetLastError(), "kernel launch") && hip_ok(hipDeviceSynchronize(), "hipDeviceSynchronize");
  }
  std::vector<float> result(a.rows * b.cols);
  if (ok) ok = hip_ok(hipMemcpy(result.data(), device_out, out_bytes, hipMemcpyDeviceToHost), "hipMemcpy(result)");
  hipFree(device_a); hipFree(device_b); hipFree(device_out);
  if (!ok) return 1;
  *output_json = copy_output(matrix_json(result, a.rows, b.cols));
  if (*output_json == nullptr) { last_error = "result allocation failed"; return 1; }
  return 0;
}

extern "C" void weft_accel_free(char* output_json) { std::free(output_json); }

static bool tensor_c_contiguous(const weft_accel_tensor_input& input) {
  int64_t expected = 1;
  for (int rank = static_cast<int>(input.rank) - 1; rank >= 0; --rank) {
    if (input.shape == nullptr || input.strides == nullptr || input.shape[rank] < 0 ||
        input.strides[rank] != expected) return false;
    expected *= input.shape[rank];
  }
  return true;
}

extern "C" int weft_accel_run_tensor(const char* operation,
                                       const weft_accel_tensor_input* inputs,
                                       size_t input_count,
                                       weft_accel_tensor_output* output) {
  if (operation == nullptr || inputs == nullptr || output == nullptr || input_count != 2) {
    last_error = "tensor_matmul requires two inputs and an output";
    return 1;
  }
  std::memset(output, 0, sizeof(*output));
  if (std::strcmp(operation, "tensor_matmul") != 0 ||
      inputs[0].dtype != WEFT_TENSOR_FLOAT32 || inputs[1].dtype != WEFT_TENSOR_FLOAT32 ||
      inputs[0].rank != 2 || inputs[1].rank != 2 ||
      !tensor_c_contiguous(inputs[0]) || !tensor_c_contiguous(inputs[1])) {
    last_error = "ROCm tensor_matmul requires contiguous float32 matrices";
    return 1;
  }
  size_t rows = static_cast<size_t>(inputs[0].shape[0]);
  size_t inner = static_cast<size_t>(inputs[0].shape[1]);
  size_t cols = static_cast<size_t>(inputs[1].shape[1]);
  if (inputs[1].shape[0] != static_cast<int64_t>(inner) || rows == 0 || inner == 0 || cols == 0 ||
      rows > SIZE_MAX / inner || inner > SIZE_MAX / cols || rows > SIZE_MAX / cols ||
      inputs[0].bytes != rows * inner * sizeof(float) ||
      inputs[1].bytes != inner * cols * sizeof(float)) {
    last_error = "ROCm tensor_matmul shapes or byte lengths are invalid";
    return 1;
  }
  size_t output_bytes = rows * cols * sizeof(float);
  float *device_a = nullptr, *device_b = nullptr, *device_out = nullptr;
  bool ok = hip_ok(hipMalloc(&device_a, inputs[0].bytes), "hipMalloc(tensor a)") &&
            hip_ok(hipMalloc(&device_b, inputs[1].bytes), "hipMalloc(tensor b)") &&
            hip_ok(hipMalloc(&device_out, output_bytes), "hipMalloc(tensor out)");
  if (ok) ok = hip_ok(hipMemcpy(device_a, inputs[0].data, inputs[0].bytes, hipMemcpyHostToDevice), "hipMemcpy(tensor a)") &&
              hip_ok(hipMemcpy(device_b, inputs[1].data, inputs[1].bytes, hipMemcpyHostToDevice), "hipMemcpy(tensor b)");
  if (ok) {
    dim3 block(16, 16);
    dim3 grid((cols + block.x - 1) / block.x, (rows + block.y - 1) / block.y);
    hipLaunchKernelGGL(weft_matmul_kernel, grid, block, 0, 0, device_a, device_b, device_out, rows, inner, cols);
    ok = hip_ok(hipGetLastError(), "tensor kernel launch") && hip_ok(hipDeviceSynchronize(), "tensor synchronize");
  }
  output->dtype = WEFT_TENSOR_FLOAT32;
  output->rank = 2;
  output->shape = static_cast<int64_t*>(std::calloc(2, sizeof(int64_t)));
  output->strides = static_cast<int64_t*>(std::calloc(2, sizeof(int64_t)));
  output->data = std::malloc(output_bytes);
  output->bytes = output_bytes;
  if (ok && (output->shape == nullptr || output->strides == nullptr || output->data == nullptr)) {
    last_error = "ROCm tensor output allocation failed";
    ok = false;
  }
  if (ok) {
    output->shape[0] = static_cast<int64_t>(rows);
    output->shape[1] = static_cast<int64_t>(cols);
    output->strides[0] = static_cast<int64_t>(cols);
    output->strides[1] = 1;
    ok = hip_ok(hipMemcpy(output->data, device_out, output_bytes, hipMemcpyDeviceToHost), "hipMemcpy(tensor result)");
  }
  hipFree(device_a);
  hipFree(device_b);
  hipFree(device_out);
  if (!ok) {
    weft_accel_free_tensor(output);
    return 1;
  }
  return 0;
}

extern "C" void weft_accel_free_tensor(weft_accel_tensor_output* output) {
  if (output == nullptr) return;
  std::free(output->shape);
  std::free(output->strides);
  std::free(output->data);
  std::memset(output, 0, sizeof(*output));
}
