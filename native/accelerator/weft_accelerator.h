#ifndef WEFT_ACCELERATOR_H
#define WEFT_ACCELERATOR_H

// Weft native accelerator ABI v1.
// Implementations may be written in C, C++, Rust, or Swift as long as they
// export these C symbols and return UTF-8 JSON.
#define WEFT_ACCELERATOR_ABI 1

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

#ifdef __cplusplus
}
#endif

#endif
