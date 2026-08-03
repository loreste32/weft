#include "../../accelerator/weft_accelerator.h"
#include "tensor_json.hpp"

#include <mlx/c/mlx.h>

#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

using weft_accel_provider::copy_output;
using weft_accel_provider::matrix;
using weft_accel_provider::parse_matrix;

static std::string last_error;

extern "C" const char* weft_accel_manifest(void) {
  return "{\"name\":\"weft-mlx\",\"version\":\"1.0.0\","
         "\"abi\":1,\"vendors\":[\"mlx\"],"
         "\"operations\":[\"health\",\"matmul\",\"tensor_matmul\"],"
         "\"metadata\":{\"runtime\":\"mlx-c\",\"dtype\":\"float32\",\"tensor_abi\":\"1\"}}";
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
    if (mlx_device_count(&devices, MLX_GPU) != 0) {
      last_error = "mlx_device_count failed";
      return 1;
    }
    *output_json = copy_output(std::string("{\"ok\":true,\"backend\":\"mlx\",\"devices\":") +
                               std::to_string(devices) + "}");
    if (*output_json == nullptr) { last_error = "result allocation failed"; return 1; }
    return 0;
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
  int a_shape[2] = {static_cast<int>(a.rows), static_cast<int>(a.cols)};
  int b_shape[2] = {static_cast<int>(b.rows), static_cast<int>(b.cols)};
  mlx_array a_array = mlx_array_new_data(a.values.data(), a_shape, 2, MLX_FLOAT32);
  mlx_array b_array = mlx_array_new_data(b.values.data(), b_shape, 2, MLX_FLOAT32);
  mlx_stream stream = mlx_default_gpu_stream_new();
  mlx_array result = mlx_array_new();
  int status = 0;
  if (a_array.ctx == nullptr || b_array.ctx == nullptr || stream.ctx == nullptr) {
    last_error = "MLX array or GPU stream allocation failed";
    status = 1;
  } else if (mlx_matmul(&result, a_array, b_array, stream) != 0 ||
             mlx_array_eval(result) != 0 || mlx_synchronize(stream) != 0) {
    last_error = "MLX matmul evaluation failed";
    status = 1;
  }
  std::vector<float> values;
  if (status == 0) {
    const float* data = mlx_array_data_float32(result);
    if (data == nullptr) {
      last_error = "MLX returned no float32 data";
      status = 1;
    } else {
      values.assign(data, data + a.rows * b.cols);
    }
  }
  mlx_array_free(result);
  mlx_array_free(a_array);
  mlx_array_free(b_array);
  mlx_stream_free(stream);
  if (status != 0) return 1;
  *output_json = copy_output(weft_accel_provider::matrix_json(values, a.rows, b.cols));
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
    last_error = "MLX tensor_matmul requires contiguous float32 matrices";
    return 1;
  }
  size_t rows = static_cast<size_t>(inputs[0].shape[0]);
  size_t inner = static_cast<size_t>(inputs[0].shape[1]);
  size_t cols = static_cast<size_t>(inputs[1].shape[1]);
  if (inputs[1].shape[0] != static_cast<int64_t>(inner) || rows == 0 || inner == 0 || cols == 0 ||
      rows > SIZE_MAX / inner || inner > SIZE_MAX / cols || rows > SIZE_MAX / cols ||
      inputs[0].bytes != rows * inner * sizeof(float) ||
      inputs[1].bytes != inner * cols * sizeof(float)) {
    last_error = "MLX tensor_matmul shapes or byte lengths are invalid";
    return 1;
  }
  int a_shape[2] = {static_cast<int>(rows), static_cast<int>(inner)};
  int b_shape[2] = {static_cast<int>(inner), static_cast<int>(cols)};
  mlx_array a_array = mlx_array_new_data(inputs[0].data, a_shape, 2, MLX_FLOAT32);
  mlx_array b_array = mlx_array_new_data(inputs[1].data, b_shape, 2, MLX_FLOAT32);
  mlx_stream stream = mlx_default_gpu_stream_new();
  mlx_array result = mlx_array_new();
  int status = 0;
  if (a_array.ctx == nullptr || b_array.ctx == nullptr || stream.ctx == nullptr || result.ctx == nullptr) {
    last_error = "MLX tensor array or GPU stream allocation failed";
    status = 1;
  } else if (mlx_matmul(&result, a_array, b_array, stream) != 0 ||
             mlx_array_eval(result) != 0 || mlx_synchronize(stream) != 0) {
    last_error = "MLX tensor matmul evaluation failed";
    status = 1;
  }
  const float* data = nullptr;
  if (status == 0) {
    data = mlx_array_data_float32(result);
    if (data == nullptr) {
      last_error = "MLX returned no tensor float32 data";
      status = 1;
    }
  }
  size_t output_bytes = rows * cols * sizeof(float);
  if (status == 0) {
    output->dtype = WEFT_TENSOR_FLOAT32;
    output->rank = 2;
    output->shape = static_cast<int64_t*>(std::calloc(2, sizeof(int64_t)));
    output->strides = static_cast<int64_t*>(std::calloc(2, sizeof(int64_t)));
    output->data = std::malloc(output_bytes);
    output->bytes = output_bytes;
    if (output->shape == nullptr || output->strides == nullptr || output->data == nullptr) {
      last_error = "MLX tensor output allocation failed";
      status = 1;
    } else {
      output->shape[0] = static_cast<int64_t>(rows);
      output->shape[1] = static_cast<int64_t>(cols);
      output->strides[0] = static_cast<int64_t>(cols);
      output->strides[1] = 1;
      std::memcpy(output->data, data, output_bytes);
    }
  }
  mlx_array_free(result);
  mlx_array_free(a_array);
  mlx_array_free(b_array);
  mlx_stream_free(stream);
  if (status != 0) {
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
