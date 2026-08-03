#ifndef WEFT_ACCELERATOR_H
#define WEFT_ACCELERATOR_H

// Weft native accelerator ABI v1.
// Implementations may be written in C, C++, Rust, or Swift as long as they
// export these C symbols and return UTF-8 JSON.
#define WEFT_ACCELERATOR_ABI 1

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Must return a process-lifetime JSON object, for example:
// {"name":"mlx","version":"0.1.0","abi":1,
//  "vendors":["mlx"],"operations":["health","matmul"]}
const char *weft_accel_manifest(void);

// The provider owns *output_json and must release it from weft_accel_free.
// Return zero on success. input_json is one JSON value.
int weft_accel_run(const char *operation, const char *input_json,
                   char **output_json);

// Optional diagnostic text for a non-zero weft_accel_run result.
const char *weft_accel_last_error(void);

void weft_accel_free(char *output_json);

// Optional binary tensor ABI. Providers that advertise `tensor_matmul` must
// export these symbols. Shapes are dimensions; strides are element strides.
// The provider owns output storage until weft_accel_free_tensor is called.
enum {
    WEFT_TENSOR_BOOL = 1,
    WEFT_TENSOR_INT64 = 2,
    WEFT_TENSOR_FLOAT32 = 3,
    WEFT_TENSOR_FLOAT64 = 4
};

typedef struct {
    uint32_t dtype;
    uint32_t rank;
    const int64_t *shape;
    const int64_t *strides;
    const void *data;
    size_t bytes;
} weft_accel_tensor_input;

typedef struct {
    uint32_t dtype;
    uint32_t rank;
    int64_t *shape;
    int64_t *strides;
    void *data;
    size_t bytes;
} weft_accel_tensor_output;

int weft_accel_run_tensor(const char *operation,
                          const weft_accel_tensor_input *inputs,
                          size_t input_count,
                          weft_accel_tensor_output *output);

void weft_accel_free_tensor(weft_accel_tensor_output *output);

#ifdef __cplusplus
}
#endif

#endif
