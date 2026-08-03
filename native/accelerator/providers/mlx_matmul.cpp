#include "../../accelerator/weft_accelerator.h"
#include "tensor_json.hpp"

#include <mlx/c/mlx.h>

#include <cstdlib>
#include <string>
#include <vector>

using weft_accel_provider::copy_output;
using weft_accel_provider::matrix;
using weft_accel_provider::parse_matrix;

static std::string last_error;

extern "C" const char* weft_accel_manifest(void) {
  return "{\"name\":\"weft-mlx\",\"version\":\"1.0.0\","
         "\"abi\":1,\"vendors\":[\"mlx\"],"
         "\"operations\":[\"health\",\"matmul\"],"
         "\"metadata\":{\"runtime\":\"mlx-c\",\"dtype\":\"float32\"}}";
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
