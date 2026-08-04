#ifndef WEFT_ACCELERATOR_TENSOR_JSON_HPP
#define WEFT_ACCELERATOR_TENSOR_JSON_HPP

#include <cerrno>
#include <cstdlib>
#include <cstring>
#include <sstream>
#include <string>
#include <vector>

namespace weft_accel_provider {

struct matrix {
  std::vector<float> values;
  size_t rows = 0;
  size_t cols = 0;
};

inline const char* after_key(const char* json, const char* key) {
  std::string needle = std::string("\"") + key + "\"";
  const char* found = std::strstr(json, needle.c_str());
  if (found == nullptr) return nullptr;
  const char* colon = std::strchr(found + needle.size(), ':');
  return colon == nullptr ? nullptr : colon + 1;
}

inline bool number_array(const char* json, const char* key,
                         std::vector<double>& values, std::string& error) {
  const char* cursor = after_key(json, key);
  if (cursor == nullptr) {
    error = std::string("missing array field ") + key;
    return false;
  }
  while (*cursor != '[' && *cursor != '\0') ++cursor;
  if (*cursor != '[') {
    error = std::string("field ") + key + " must be an array";
    return false;
  }
  ++cursor;
  while (true) {
    while (*cursor == ' ' || *cursor == '\t' || *cursor == '\r' || *cursor == '\n') ++cursor;
    if (*cursor == ']') return true;
    char* end = nullptr;
    errno = 0;
    double value = std::strtod(cursor, &end);
    if (end == cursor || errno == ERANGE) {
      error = std::string("field ") + key + " contains an invalid number";
      return false;
    }
    values.push_back(value);
    cursor = end;
    while (*cursor == ' ' || *cursor == '\t' || *cursor == '\r' || *cursor == '\n') ++cursor;
    if (*cursor == ',') {
      ++cursor;
      continue;
    }
    if (*cursor == ']') return true;
    error = std::string("field ") + key + " must be comma-separated";
    return false;
  }
}

inline bool parse_matrix(const char* json, const char* data_key,
                         const char* shape_key, matrix& result,
                         std::string& error) {
  std::vector<double> values;
  std::vector<double> shape;
  if (!number_array(json, data_key, values, error) ||
      !number_array(json, shape_key, shape, error)) {
    return false;
  }
  if (shape.size() != 2 || shape[0] < 0 || shape[1] < 0 ||
      shape[0] != static_cast<size_t>(shape[0]) ||
      shape[1] != static_cast<size_t>(shape[1])) {
    error = std::string("shape ") + shape_key + " must contain two non-negative integers";
    return false;
  }
  result.rows = static_cast<size_t>(shape[0]);
  result.cols = static_cast<size_t>(shape[1]);
    if (result.rows > 1 << 20 || result.cols > 1 << 20 ||
        (result.rows != 0 && result.cols > (1 << 26) / result.rows) ||
      values.size() != result.rows * result.cols) {
    error = std::string("data and shape do not agree for ") + data_key;
    return false;
  }
  result.values.reserve(values.size());
  for (double value : values) result.values.push_back(static_cast<float>(value));
  return true;
}

// matrix_json renders one matmul result with mandatory execution reporting.
// Silent fallback is not allowed: a provider that computed on the host for a
// device request must pass device "cpu" and fallback true. The host rejects
// results whose device contradicts requested_device without fallback.
inline std::string matrix_json(const std::vector<float>& values,
                               size_t rows, size_t cols,
                               const std::string& device, bool fallback) {
  std::ostringstream output;
  output.precision(9);
  output << "{\"data\":[";
  for (size_t i = 0; i < values.size(); ++i) {
    if (i != 0) output << ',';
    output << values[i];
  }
  output << "],\"shape\":[" << rows << ',' << cols << "],\"device\":\"" << device
         << "\",\"requested_device\":\"" << device << "\",\"fallback\":"
         << (fallback ? "true" : "false") << '}';
  return output.str();
}

// exec_info_json renders the weft_accel_exec_info document for one finished
// operation. status is "device" or "fallback" ("unavailable" is produced by
// the host, not by a successful provider run).
inline std::string exec_info_json(const std::string& device, bool fallback) {
  return std::string("{\"device\":\"") + device + "\",\"requested_device\":\"" + device +
         "\",\"fallback\":" + (fallback ? "true" : "false") + ",\"status\":\"" +
         (fallback ? "fallback" : "device") + "\"}";
}

inline char* copy_output(const std::string& value) {
  char* result = static_cast<char*>(std::malloc(value.size() + 1));
  if (result != nullptr) std::memcpy(result, value.c_str(), value.size() + 1);
  return result;
}

}  // namespace weft_accel_provider

#endif
