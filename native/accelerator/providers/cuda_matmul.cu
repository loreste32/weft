#include "../../accelerator/weft_accelerator.h"
#include "tensor_json.hpp"

#include <cuda_runtime.h>

#include <cstring>
#include <string>
#include <vector>

using weft_accel_provider::copy_output;
using weft_accel_provider::matrix;
using weft_accel_provider::matrix_json;
using weft_accel_provider::parse_matrix;

static std::string last_error;

// Execution reporting for the most recent run (weft_accel_exec_info). This
// provider never falls back to CPU: device ops either run on the CUDA device
// or fail with an error, so fallback is always false here.
static std::string last_exec_info;

static std::string cuda_device_name() {
  int device = 0;
  if (cudaGetDevice(&device) != cudaSuccess) return "cuda";
  return std::string("cuda:") + std::to_string(device);
}

static void record_exec_info(bool fallback) {
  last_exec_info = weft_accel_provider::exec_info_json(cuda_device_name(), fallback);
}

extern "C" char* weft_accel_exec_info(void) {
  return copy_output(last_exec_info);
}

__global__ void weft_matmul_kernel(const float* a, const float* b, float* out,
                                   size_t rows, size_t inner, size_t cols) {
  size_t row = blockIdx.y * blockDim.y + threadIdx.y;
  size_t col = blockIdx.x * blockDim.x + threadIdx.x;
  if (row >= rows || col >= cols) return;
  float value = 0.0f;
  for (size_t i = 0; i < inner; ++i) value += a[row * inner + i] * b[i * cols + col];
  out[row * cols + col] = value;
}

__global__ void weft_add_kernel(const float* a, const float* b, float* out,
                                size_t count) {
  size_t index = blockIdx.x * blockDim.x + threadIdx.x;
  if (index < count) out[index] = a[index] + b[index];
}

// Bounded same-shape elementwise ops beyond add. op selects subtract (0),
// multiply (1), or divide (2); the host-side wrapper below mirrors the
// tensor_add shape checks, launch bounds, copies, and reporting exactly.
__global__ void weft_elementwise_kernel(const float* a, const float* b, float* out,
                                        size_t count, int op) {
  size_t index = blockIdx.x * blockDim.x + threadIdx.x;
  if (index >= count) return;
  if (op == 0) out[index] = a[index] - b[index];
  else if (op == 1) out[index] = a[index] * b[index];
  else out[index] = a[index] / b[index];
}

static bool cuda_ok(cudaError_t status, const char* operation) {
  if (status == cudaSuccess) return true;
  last_error = std::string(operation) + ": " + cudaGetErrorString(status);
  return false;
}

extern "C" const char* weft_accel_manifest(void) {
  return "{\"name\":\"weft-cuda\",\"version\":\"1.1.0\","
         "\"abi\":1,\"vendors\":[\"cuda\"],"
         "\"operations\":[\"health\",\"matmul\",\"tensor_matmul\",\"tensor_add\","
         "\"tensor_sub\",\"tensor_mul\",\"tensor_div\"],"
         "\"metadata\":{\"runtime\":\"cuda-runtime\",\"dtype\":\"float32\",\"tensor_abi\":\"1\"}}";
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
    std::string device = cuda_device_name();
    record_exec_info(false);
    *output_json = copy_output(std::string("{\"ok\":true,\"backend\":\"cuda\",\"devices\":") +
                               std::to_string(devices) + ",\"device\":\"" + device +
                               "\",\"requested_device\":\"" + device + "\",\"fallback\":false}");
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
  record_exec_info(false);
  *output_json = copy_output(matrix_json(result, a.rows, b.cols, cuda_device_name(), false));
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

static bool tensor_add_shape(const weft_accel_tensor_input& left,
                             const weft_accel_tensor_input& right,
                             size_t* count) {
  if (left.rank == 0 || left.rank > 2 || left.rank != right.rank ||
      left.shape == nullptr || right.shape == nullptr || left.strides == nullptr ||
      right.strides == nullptr || !tensor_c_contiguous(left) || !tensor_c_contiguous(right)) {
    return false;
  }
  size_t elements = 1;
  for (uint32_t axis = 0; axis < left.rank; ++axis) {
    if (left.shape[axis] != right.shape[axis] || left.shape[axis] < 0) return false;
    size_t dimension = static_cast<size_t>(left.shape[axis]);
    if (dimension != 0 && elements > SIZE_MAX / dimension) return false;
    elements *= dimension;
  }
  if (elements == 0 || elements > SIZE_MAX / sizeof(float) ||
      left.bytes != elements * sizeof(float) || right.bytes != elements * sizeof(float) ||
      left.data == nullptr || right.data == nullptr) return false;
  *count = elements;
  return true;
}

// Bounded same-shape float32 elementwise driver for tensor_sub, tensor_mul,
// and tensor_div. This is the tensor_add flow with the op-tagged
// weft_elementwise_kernel in place of weft_add_kernel; tensor_add keeps its
// own path above so the hardware-tested code is unchanged.
static int cuda_run_tensor_elementwise(const char* name, int op,
                                       const weft_accel_tensor_input* inputs,
                                       weft_accel_tensor_output* output) {
  size_t count = 0;
  if (inputs[0].dtype != WEFT_TENSOR_FLOAT32 || inputs[1].dtype != WEFT_TENSOR_FLOAT32 ||
      !tensor_add_shape(inputs[0], inputs[1], &count)) {
    last_error = std::string("CUDA ") + name +
                 " requires contiguous same-shape float32 tensors of rank 1 or 2";
    return 1;
  }
  size_t output_bytes = count * sizeof(float);
  float *device_a = nullptr, *device_b = nullptr, *device_out = nullptr;
  bool ok = cuda_ok(cudaMalloc(&device_a, output_bytes), "cudaMalloc(elementwise a)") &&
            cuda_ok(cudaMalloc(&device_b, output_bytes), "cudaMalloc(elementwise b)") &&
            cuda_ok(cudaMalloc(&device_out, output_bytes), "cudaMalloc(elementwise out)");
  if (ok) ok = cuda_ok(cudaMemcpy(device_a, inputs[0].data, output_bytes, cudaMemcpyHostToDevice), "cudaMemcpy(elementwise a)") &&
              cuda_ok(cudaMemcpy(device_b, inputs[1].data, output_bytes, cudaMemcpyHostToDevice), "cudaMemcpy(elementwise b)");
  if (ok) {
    constexpr size_t block_size = 256;
    size_t blocks = (count + block_size - 1) / block_size;
    if (blocks > 2147483647u) {
      last_error = std::string("CUDA ") + name + " is too large for the provider launch bounds";
      ok = false;
    } else {
      weft_elementwise_kernel<<<static_cast<unsigned int>(blocks), block_size>>>(
          static_cast<const float*>(device_a), static_cast<const float*>(device_b), device_out, count, op);
      ok = cuda_ok(cudaGetLastError(), "elementwise kernel launch") &&
           cuda_ok(cudaDeviceSynchronize(), "elementwise synchronize");
    }
  }
  output->dtype = WEFT_TENSOR_FLOAT32;
  output->rank = inputs[0].rank;
  output->shape = static_cast<int64_t*>(std::calloc(output->rank, sizeof(int64_t)));
  output->strides = static_cast<int64_t*>(std::calloc(output->rank, sizeof(int64_t)));
  output->data = std::malloc(output_bytes);
  output->bytes = output_bytes;
  if (ok && (output->shape == nullptr || output->strides == nullptr || output->data == nullptr)) {
    last_error = std::string("CUDA ") + name + " output allocation failed";
    ok = false;
  }
  if (ok) {
    std::memcpy(output->shape, inputs[0].shape, output->rank * sizeof(int64_t));
    std::memcpy(output->strides, inputs[0].strides, output->rank * sizeof(int64_t));
    ok = cuda_ok(cudaMemcpy(output->data, device_out, output_bytes, cudaMemcpyDeviceToHost),
                 "cudaMemcpy(elementwise result)");
  }
  cudaFree(device_a);
  cudaFree(device_b);
  cudaFree(device_out);
  if (!ok) {
    weft_accel_free_tensor(output);
    return 1;
  }
  record_exec_info(false);
  return 0;
}

extern "C" int weft_accel_run_tensor(const char* operation,
                                       const weft_accel_tensor_input* inputs,
                                       size_t input_count,
                                       weft_accel_tensor_output* output) {
  if (operation == nullptr || inputs == nullptr || output == nullptr || input_count != 2) {
    last_error = "tensor operation requires two inputs and an output";
    return 1;
  }
  std::memset(output, 0, sizeof(*output));
  if (std::strcmp(operation, "tensor_sub") == 0 || std::strcmp(operation, "tensor_mul") == 0 ||
      std::strcmp(operation, "tensor_div") == 0) {
    int op = 0;
    if (std::strcmp(operation, "tensor_mul") == 0) op = 1;
    if (std::strcmp(operation, "tensor_div") == 0) op = 2;
    return cuda_run_tensor_elementwise(operation, op, inputs, output);
  }
  if (std::strcmp(operation, "tensor_add") == 0) {
    size_t count = 0;
    if (inputs[0].dtype != WEFT_TENSOR_FLOAT32 || inputs[1].dtype != WEFT_TENSOR_FLOAT32 ||
        !tensor_add_shape(inputs[0], inputs[1], &count)) {
      last_error = "CUDA tensor_add requires contiguous same-shape float32 tensors of rank 1 or 2";
      return 1;
    }
    size_t output_bytes = count * sizeof(float);
    float *device_a = nullptr, *device_b = nullptr, *device_out = nullptr;
    bool ok = cuda_ok(cudaMalloc(&device_a, output_bytes), "cudaMalloc(add a)") &&
              cuda_ok(cudaMalloc(&device_b, output_bytes), "cudaMalloc(add b)") &&
              cuda_ok(cudaMalloc(&device_out, output_bytes), "cudaMalloc(add out)");
    if (ok) ok = cuda_ok(cudaMemcpy(device_a, inputs[0].data, output_bytes, cudaMemcpyHostToDevice), "cudaMemcpy(add a)") &&
                cuda_ok(cudaMemcpy(device_b, inputs[1].data, output_bytes, cudaMemcpyHostToDevice), "cudaMemcpy(add b)");
    if (ok) {
      constexpr size_t block_size = 256;
      size_t blocks = (count + block_size - 1) / block_size;
      if (blocks > 2147483647u) {
        last_error = "CUDA tensor_add is too large for the provider launch bounds";
        ok = false;
      } else {
        weft_add_kernel<<<static_cast<unsigned int>(blocks), block_size>>>(
            static_cast<const float*>(device_a), static_cast<const float*>(device_b), device_out, count);
        ok = cuda_ok(cudaGetLastError(), "tensor add kernel launch") &&
             cuda_ok(cudaDeviceSynchronize(), "tensor add synchronize");
      }
    }
    output->dtype = WEFT_TENSOR_FLOAT32;
    output->rank = inputs[0].rank;
    output->shape = static_cast<int64_t*>(std::calloc(output->rank, sizeof(int64_t)));
    output->strides = static_cast<int64_t*>(std::calloc(output->rank, sizeof(int64_t)));
    output->data = std::malloc(output_bytes);
    output->bytes = output_bytes;
    if (ok && (output->shape == nullptr || output->strides == nullptr || output->data == nullptr)) {
      last_error = "CUDA tensor_add output allocation failed";
      ok = false;
    }
    if (ok) {
      std::memcpy(output->shape, inputs[0].shape, output->rank * sizeof(int64_t));
      std::memcpy(output->strides, inputs[0].strides, output->rank * sizeof(int64_t));
      ok = cuda_ok(cudaMemcpy(output->data, device_out, output_bytes, cudaMemcpyDeviceToHost),
                   "cudaMemcpy(add result)");
    }
    cudaFree(device_a);
    cudaFree(device_b);
    cudaFree(device_out);
    if (!ok) {
      weft_accel_free_tensor(output);
      return 1;
    }
    record_exec_info(false);
    return 0;
  }
  if (std::strcmp(operation, "tensor_matmul") != 0 ||
      inputs[0].dtype != WEFT_TENSOR_FLOAT32 || inputs[1].dtype != WEFT_TENSOR_FLOAT32 ||
      inputs[0].rank != 2 || inputs[1].rank != 2 ||
      !tensor_c_contiguous(inputs[0]) || !tensor_c_contiguous(inputs[1])) {
    last_error = "CUDA tensor_matmul requires contiguous float32 matrices";
    return 1;
  }
  size_t rows = static_cast<size_t>(inputs[0].shape[0]);
  size_t inner = static_cast<size_t>(inputs[0].shape[1]);
  size_t cols = static_cast<size_t>(inputs[1].shape[1]);
  if (inputs[1].shape[0] != static_cast<int64_t>(inner) || rows == 0 || inner == 0 || cols == 0 ||
      rows > SIZE_MAX / inner || inner > SIZE_MAX / cols || rows > SIZE_MAX / cols ||
      inputs[0].bytes != rows * inner * sizeof(float) ||
      inputs[1].bytes != inner * cols * sizeof(float)) {
    last_error = "CUDA tensor_matmul shapes or byte lengths are invalid";
    return 1;
  }
  size_t output_bytes = rows * cols * sizeof(float);
  float *device_a = nullptr, *device_b = nullptr, *device_out = nullptr;
  bool ok = cuda_ok(cudaMalloc(&device_a, inputs[0].bytes), "cudaMalloc(tensor a)") &&
            cuda_ok(cudaMalloc(&device_b, inputs[1].bytes), "cudaMalloc(tensor b)") &&
            cuda_ok(cudaMalloc(&device_out, output_bytes), "cudaMalloc(tensor out)");
  if (ok) ok = cuda_ok(cudaMemcpy(device_a, inputs[0].data, inputs[0].bytes, cudaMemcpyHostToDevice), "cudaMemcpy(tensor a)") &&
              cuda_ok(cudaMemcpy(device_b, inputs[1].data, inputs[1].bytes, cudaMemcpyHostToDevice), "cudaMemcpy(tensor b)");
  if (ok) {
    dim3 block(16, 16);
    dim3 grid((cols + block.x - 1) / block.x, (rows + block.y - 1) / block.y);
    weft_matmul_kernel<<<grid, block>>>(device_a, device_b, device_out, rows, inner, cols);
    ok = cuda_ok(cudaGetLastError(), "tensor kernel launch") && cuda_ok(cudaDeviceSynchronize(), "tensor synchronize");
  }
  output->dtype = WEFT_TENSOR_FLOAT32;
  output->rank = 2;
  output->shape = static_cast<int64_t*>(std::calloc(2, sizeof(int64_t)));
  output->strides = static_cast<int64_t*>(std::calloc(2, sizeof(int64_t)));
  output->data = std::malloc(output_bytes);
  output->bytes = output_bytes;
  if (ok && (output->shape == nullptr || output->strides == nullptr || output->data == nullptr)) {
    last_error = "CUDA tensor output allocation failed";
    ok = false;
  }
  if (ok) {
    output->shape[0] = static_cast<int64_t>(rows);
    output->shape[1] = static_cast<int64_t>(cols);
    output->strides[0] = static_cast<int64_t>(cols);
    output->strides[1] = 1;
    ok = cuda_ok(cudaMemcpy(output->data, device_out, output_bytes, cudaMemcpyDeviceToHost), "cudaMemcpy(tensor result)");
  }
  cudaFree(device_a);
  cudaFree(device_b);
  cudaFree(device_out);
  if (!ok) {
    weft_accel_free_tensor(output);
    return 1;
  }
  record_exec_info(false);
  return 0;
}

extern "C" void weft_accel_free_tensor(weft_accel_tensor_output* output) {
  if (output == nullptr) return;
  std::free(output->shape);
  std::free(output->strides);
  std::free(output->data);
  std::memset(output, 0, sizeof(*output));
}
