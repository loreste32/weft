#include "tensor_json.hpp"

#include <cassert>
#include <string>

int main() {
  using namespace weft_accel_provider;
  matrix a;
  matrix b;
  std::string error;
  const char* input = "{\"a\":[1,2,3,4],\"a_shape\":[2,2],"
                      "\"b\":[5,6,7,8],\"b_shape\":[2,2]}";
  assert(parse_matrix(input, "a", "a_shape", a, error));
  assert(parse_matrix(input, "b", "b_shape", b, error));
  assert(a.rows == 2 && a.cols == 2 && a.values[3] == 4.0f);
  assert(b.rows == 2 && b.cols == 2 && b.values[0] == 5.0f);
  assert(!parse_matrix("{\"a\":[1],\"a_shape\":[2,2]}", "a", "a_shape", a, error));
  assert(matrix_json({19.0f, 22.0f, 43.0f, 50.0f}, 2, 2, "cuda:0", false) ==
         "{\"data\":[19,22,43,50],\"shape\":[2,2],\"device\":\"cuda:0\","
         "\"requested_device\":\"cuda:0\",\"fallback\":false}");
  assert(exec_info_json("cpu", true) ==
         "{\"device\":\"cpu\",\"requested_device\":\"cpu\",\"fallback\":true,"
         "\"status\":\"fallback\"}");
  return 0;
}
