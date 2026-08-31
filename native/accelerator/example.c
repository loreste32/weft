#include "weft_accelerator.h"

#include <errno.h>
#include <math.h>
#include <stdarg.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static const char *last_error = "";

/* Execution reporting state; populated after copy_string is defined below. */
static char last_exec_info[192] = "";

static char *copy_string(const char *value) {
    size_t size;
    char *copy;
    if (value == NULL) return NULL;
    size = strlen(value) + 1;
    copy = (char *)malloc(size);
    if (copy != NULL) memcpy(copy, value, size);
    return copy;
}

static void fail(const char *message) { last_error = message; }

/* Execution reporting for the most recent run. The CPU reference always
   computes on the host and never falls back, so it reports device "cpu"
   with fallback false. Empty until the first call. */
static void record_exec_info(const char *device, const char *requested, int fallback) {
    snprintf(last_exec_info, sizeof(last_exec_info),
             "{\"device\":\"%s\",\"requested_device\":\"%s\",\"fallback\":%s,\"status\":\"%s\"}",
             device, requested, fallback ? "true" : "false",
             fallback ? "fallback" : "device");
}

char *weft_accel_exec_info(void) { return copy_string(last_exec_info); }

const char *weft_accel_manifest(void) {
    return "{\"name\":\"weft-example\",\"version\":\"1.2.0\","
           "\"abi\":1,\"vendors\":[\"example\"],"
           "\"operations\":[\"health\",\"identity\",\"matmul\",\"tensor_matmul\","
           "\"tensor_add\",\"tensor_sub\",\"tensor_mul\",\"tensor_div\",\"tensor_sum\"],"
           "\"metadata\":{\"tensor_abi\":\"1\",\"dtype\":\"float64\",\"dtypes\":\"float16,float32,float64\"}}";
}

const char *weft_accel_last_error(void) { return last_error; }

static const char *value_after_key(const char *json, const char *key) {
    char needle[64];
    const char *found;
    const char *colon;
    int written = snprintf(needle, sizeof(needle), "\"%s\"", key);
    if (written < 0 || (size_t)written >= sizeof(needle)) return NULL;
    found = strstr(json, needle);
    if (found == NULL) return NULL;
    colon = strchr(found + written, ':');
    return colon == NULL ? NULL : colon + 1;
}

static int parse_number_array(const char *json, const char *key,
                              double **values_out, size_t *length_out) {
    const char *cursor = value_after_key(json, key);
    double *values = NULL;
    size_t length = 0;
    size_t capacity = 0;
    if (cursor == NULL) {
        fail("missing array field");
        return 0;
    }
    while (*cursor != '[' && *cursor != '\0') cursor++;
    if (*cursor != '[') {
        fail("array field must be a JSON array");
        return 0;
    }
    cursor++;
    for (;;) {
        char *end;
        double value;
        while (*cursor == ' ' || *cursor == '\t' || *cursor == '\r' || *cursor == '\n') cursor++;
        if (*cursor == ']') break;
        errno = 0;
        value = strtod(cursor, &end);
        if (end == cursor || errno == ERANGE) {
            fail("array contains an invalid number");
            free(values);
            return 0;
        }
        if (length == capacity) {
            size_t next = capacity == 0 ? 16 : capacity * 2;
            double *grown = (double *)realloc(values, next * sizeof(double));
            if (grown == NULL) {
                fail("array allocation failed");
                free(values);
                return 0;
            }
            values = grown;
            capacity = next;
        }
        values[length++] = value;
        cursor = end;
        while (*cursor == ' ' || *cursor == '\t' || *cursor == '\r' || *cursor == '\n') cursor++;
        if (*cursor == ',') {
            cursor++;
            continue;
        }
        if (*cursor != ']') {
            fail("array must contain comma-separated numbers");
            free(values);
            return 0;
        }
        break;
    }
    *values_out = values;
    *length_out = length;
    return 1;
}

static int parse_shape(const char *json, const char *key, size_t shape[2]) {
    double *values = NULL;
    size_t length = 0;
    int ok = parse_number_array(json, key, &values, &length);
    if (!ok) return 0;
    if (length != 2 || values[0] < 0 || values[1] < 0 ||
        values[0] != (size_t)values[0] || values[1] != (size_t)values[1]) {
        fail("matmul shapes must contain two non-negative integers");
        free(values);
        return 0;
    }
    shape[0] = (size_t)values[0];
    shape[1] = (size_t)values[1];
    free(values);
    return 1;
}

typedef struct {
    char *data;
    size_t length;
    size_t capacity;
} output_buffer;

static int append_format(output_buffer *buffer, const char *format, ...) {
    va_list args;
    va_list copy;
    int needed;
    size_t required;
    va_start(args, format);
    va_copy(copy, args);
    needed = vsnprintf(NULL, 0, format, copy);
    va_end(copy);
    if (needed < 0) {
        va_end(args);
        return 0;
    }
    required = buffer->length + (size_t)needed + 1;
    if (required > buffer->capacity) {
        size_t capacity = buffer->capacity == 0 ? 256 : buffer->capacity;
        while (capacity < required) capacity *= 2;
        char *grown = (char *)realloc(buffer->data, capacity);
        if (grown == NULL) {
            va_end(args);
            return 0;
        }
        buffer->data = grown;
        buffer->capacity = capacity;
    }
    vsnprintf(buffer->data + buffer->length,
              buffer->capacity - buffer->length, format, args);
    va_end(args);
    buffer->length += (size_t)needed;
    return 1;
}

static int run_matmul(const char *input_json, char **output_json) {
    double *a = NULL;
    double *b = NULL;
    size_t a_length = 0;
    size_t b_length = 0;
    size_t a_shape[2];
    size_t b_shape[2];
    output_buffer output = {0};
    size_t row;
    size_t col;
    if (!parse_number_array(input_json, "a", &a, &a_length) ||
        !parse_number_array(input_json, "b", &b, &b_length) ||
        !parse_shape(input_json, "a_shape", a_shape) ||
        !parse_shape(input_json, "b_shape", b_shape)) {
        free(a);
        free(b);
        return 0;
    }
    if (a_shape[1] != b_shape[0] || a_length != a_shape[0] * a_shape[1] ||
        b_length != b_shape[0] * b_shape[1]) {
        fail("matmul data lengths and inner dimensions must agree");
        free(a);
        free(b);
        return 0;
    }
    if (!append_format(&output, "{\"data\":[")) {
        fail("matmul output allocation failed");
        free(a);
        free(b);
        return 0;
    }
    for (row = 0; row < a_shape[0]; row++) {
        for (col = 0; col < b_shape[1]; col++) {
            size_t inner;
            double value = 0.0;
            for (inner = 0; inner < a_shape[1]; inner++) {
                value += a[row * a_shape[1] + inner] * b[inner * b_shape[1] + col];
            }
            if (!append_format(&output, "%s%.17g", (row == 0 && col == 0) ? "" : ",", value)) {
                fail("matmul output allocation failed");
                free(a);
                free(b);
                free(output.data);
                return 0;
            }
        }
    }
    if (!append_format(&output,
                       "],\"shape\":[%zu,%zu],\"device\":\"cpu\",\"requested_device\":\"cpu\",\"fallback\":false}",
                       a_shape[0], b_shape[1])) {
        fail("matmul output allocation failed");
        free(a);
        free(b);
        free(output.data);
        return 0;
    }
    free(a);
    free(b);
    *output_json = output.data;
    return 1;
}

int weft_accel_run(const char *operation, const char *input_json,
                   char **output_json) {
    if (operation == NULL || input_json == NULL || output_json == NULL) {
        fail("operation, input, and output are required");
        return 1;
    }
    *output_json = NULL;
    last_error = "";
    record_exec_info("cpu", "cpu", 0);
    if (strcmp(operation, "health") == 0) {
        /* Explicit device + fallback reporting: CPU reference never falls back. */
        *output_json = copy_string(
            "{\"ok\":true,\"backend\":\"example-cpu\",\"device\":\"cpu\","
            "\"fallback\":false,\"requested_device\":\"cpu\"}");
    } else if (strcmp(operation, "identity") == 0) {
        *output_json = copy_string(input_json);
    } else if (strcmp(operation, "matmul") == 0) {
        return run_matmul(input_json, output_json) ? 0 : 1;
    } else {
        fail("unsupported operation");
        return 1;
    }
    if (*output_json == NULL) {
        fail("result allocation failed");
        return 1;
    }
    return 0;
}

void weft_accel_free(char *output_json) { free(output_json); }

static int tensor_c_contiguous(const weft_accel_tensor_input *input) {
    int64_t expected = 1;
    int rank;
    if (input == NULL || input->shape == NULL || input->strides == NULL) return 0;
    rank = (int)input->rank;
    while (rank > 0) {
        rank--;
        if (input->strides[rank] != expected) return 0;
        if (input->shape[rank] < 0) return 0;
        if (input->shape[rank] != 0 && expected > INT64_MAX / input->shape[rank]) return 0;
        expected *= input->shape[rank];
    }
    return 1;
}

/* Bounded same-shape contract shared by every elementwise binary op. The
   reference provider deliberately does not implement broadcasting: inputs
   must have identical ranks and shapes, be C-contiguous, and carry exactly
   count * item_size bytes. */
static int tensor_elementwise_shape(const weft_accel_tensor_input *left,
                                    const weft_accel_tensor_input *right,
                                    size_t item_size, size_t *count_out) {
    size_t count = 1;
    uint32_t axis;
    if (left == NULL || right == NULL || count_out == NULL || left->rank == 0 ||
        left->rank != right->rank || left->shape == NULL || right->shape == NULL ||
        left->strides == NULL || right->strides == NULL ||
        !tensor_c_contiguous(left) || !tensor_c_contiguous(right)) {
        return 0;
    }
    if (left->rank > 2) return 0;
    for (axis = 0; axis < left->rank; axis++) {
        if (left->shape[axis] != right->shape[axis] || left->shape[axis] < 0) return 0;
        if (left->shape[axis] != 0 && count > SIZE_MAX / (size_t)left->shape[axis]) return 0;
        count *= (size_t)left->shape[axis];
    }
    if (count > SIZE_MAX / item_size ||
        left->bytes != count * item_size || right->bytes != count * item_size ||
        (count != 0 && (left->data == NULL || right->data == NULL))) {
        return 0;
    }
    *count_out = count;
    return 1;
}

typedef enum {
    ELEMENTWISE_ADD,
    ELEMENTWISE_SUB,
    ELEMENTWISE_MUL,
    ELEMENTWISE_DIV
} elementwise_op;

static float apply_op_float(elementwise_op op, float a, float b) {
    switch (op) {
    case ELEMENTWISE_ADD: return a + b;
    case ELEMENTWISE_SUB: return a - b;
    case ELEMENTWISE_MUL: return a * b;
    default: return a / b;
    }
}

static double apply_op_double(elementwise_op op, double a, double b) {
    switch (op) {
    case ELEMENTWISE_ADD: return a + b;
    case ELEMENTWISE_SUB: return a - b;
    case ELEMENTWISE_MUL: return a * b;
    default: return a / b;
    }
}

/* IEEE 754 binary16 (float16) conversions. The reference provider implements
   float16 as float32 compute with binary16 storage: inputs widen exactly to
   float, the operation runs in single precision, and the result rounds back
   to half with the same round-to-nearest-even, subnormal, and
   overflow-to-infinity semantics as the host tensor package
   (internal/tensor/float16.go, NumPy 2.x np.float16). */
static float half_to_float(uint16_t bits) {
    unsigned sign = bits >> 15;
    unsigned exp = (bits >> 10) & 0x1f;
    unsigned mant = bits & 0x3ff;
    float value;
    if (exp == 0x1f) {
        value = mant != 0 ? NAN : INFINITY;
    } else if (exp == 0) {
        value = ldexpf((float)mant, -24); /* subnormal or zero */
    } else {
        value = ldexpf((float)(mant | 0x400), (int)exp - 25);
    }
    return sign != 0 ? -value : value;
}

static uint16_t round_to_nearest_even(uint64_t keep, uint64_t rem, unsigned shift) {
    uint64_t halfway = (uint64_t)1 << (shift - 1);
    if (rem > halfway || (rem == halfway && (keep & 1) != 0)) keep++;
    return (uint16_t)keep;
}

/* Round a float (widened to double, which is exact) to its binary16 bit
   pattern. Mirrors internal/tensor Float16FromFloat64: round to nearest,
   ties to even, subnormals handled, overflow yields ±Inf. */
static uint16_t float_to_half(float f) {
    uint64_t dbits;
    uint16_t sign;
    int exp;
    uint64_t mant;
    int e;
    uint64_t sig;
    double wide = (double)f;
    memcpy(&dbits, &wide, sizeof(dbits));
    sign = (uint16_t)(dbits >> 63) << 15;
    exp = (int)(dbits >> 52) & 0x7ff;
    mant = dbits & 0xfffffffffffffULL;
    if (exp == 0x7ff) {
        return (uint16_t)(sign | (mant != 0 ? 0x7e00 : 0x7c00)); /* NaN quieted / ±Inf */
    }
    if (exp == 0) return sign; /* float64 subnormals are far below the half range */
    e = exp - 1023;
    sig = mant | ((uint64_t)1 << 52);
    if (e > 15) return (uint16_t)(sign | 0x7c00); /* magnitude >= 2^16 overflows to Inf */
    if (e >= -14) {
        /* Normal half: keep the implicit bit plus 10 mantissa bits. */
        uint16_t keep = round_to_nearest_even(sig >> 42, sig & (((uint64_t)1 << 42) - 1), 42);
        if (keep == 0x800) { /* mantissa carry: rounds up to the next power of two */
            keep = 0x400;
            e++;
            if (e > 15) return (uint16_t)(sign | 0x7c00);
        }
        return (uint16_t)(sign | (uint16_t)(e + 15) << 10 | (keep & 0x3ff));
    }
    if (e < -25) return sign; /* below half of the smallest subnormal (2^-25 midpoint) */
    {
        /* Subnormal half: the quantum is 2^-24. */
        unsigned shift = (unsigned)(28 - e);
        uint16_t keep = round_to_nearest_even(sig >> shift, sig & (((uint64_t)1 << shift) - 1), shift);
        return (uint16_t)(sign | keep);
    }
}

/* Same-shape elementwise binary op (tensor_add, tensor_sub, tensor_mul,
   tensor_div) for float16, float32, or float64 tensors of rank 1 or 2.
   Non-same-shape inputs are rejected outright: broadcasting is not part of
   this provider's coverage claim. float16 inputs widen to float32 for the
   computation and the result rounds back to binary16 storage. */
static int run_tensor_elementwise(elementwise_op op, const char *name,
                                  const weft_accel_tensor_input *inputs,
                                  weft_accel_tensor_output *output) {
    size_t item_size;
    size_t count;
    uint32_t axis;
    if (inputs[0].dtype != inputs[1].dtype ||
        (inputs[0].dtype != WEFT_TENSOR_FLOAT64 && inputs[0].dtype != WEFT_TENSOR_FLOAT32 &&
         inputs[0].dtype != WEFT_TENSOR_FLOAT16)) {
        fail("reference elementwise operations require matching float16, float32, or float64 dtypes");
        return 0;
    }
    item_size = inputs[0].dtype == WEFT_TENSOR_FLOAT64 ? sizeof(double)
              : inputs[0].dtype == WEFT_TENSOR_FLOAT16 ? sizeof(uint16_t)
              : sizeof(float);
    if (!tensor_elementwise_shape(&inputs[0], &inputs[1], item_size, &count)) {
        static char message[128];
        snprintf(message, sizeof(message),
                 "reference %s requires contiguous same-shape float16/float32/float64 tensors of rank 1 or 2",
                 name);
        fail(message);
        return 0;
    }
    output->dtype = inputs[0].dtype;
    output->rank = inputs[0].rank;
    output->shape = (int64_t *)calloc(output->rank, sizeof(int64_t));
    output->strides = (int64_t *)calloc(output->rank, sizeof(int64_t));
    output->data = count == 0 ? NULL : malloc(count * item_size);
    output->bytes = count * item_size;
    if (output->shape == NULL || output->strides == NULL || (count != 0 && output->data == NULL)) {
        fail("elementwise output allocation failed");
        weft_accel_free_tensor(output);
        return 0;
    }
    memcpy(output->shape, inputs[0].shape, output->rank * sizeof(int64_t));
    output->strides[output->rank - 1] = 1;
    for (axis = output->rank - 1; axis > 0; axis--) {
        output->strides[axis - 1] = output->strides[axis] * output->shape[axis];
    }
    if (inputs[0].dtype == WEFT_TENSOR_FLOAT64) {
        const double *left = (const double *)inputs[0].data;
        const double *right = (const double *)inputs[1].data;
        double *result = (double *)output->data;
        for (size_t index = 0; index < count; index++) {
            result[index] = apply_op_double(op, left[index], right[index]);
        }
    } else if (inputs[0].dtype == WEFT_TENSOR_FLOAT16) {
        const uint16_t *left = (const uint16_t *)inputs[0].data;
        const uint16_t *right = (const uint16_t *)inputs[1].data;
        uint16_t *result = (uint16_t *)output->data;
        for (size_t index = 0; index < count; index++) {
            result[index] = float_to_half(apply_op_float(op, half_to_float(left[index]),
                                                             half_to_float(right[index])));
        }
    } else {
        const float *left = (const float *)inputs[0].data;
        const float *right = (const float *)inputs[1].data;
        float *result = (float *)output->data;
        for (size_t index = 0; index < count; index++) {
            result[index] = apply_op_float(op, left[index], right[index]);
        }
    }
    return 1;
}

/* Full-reduction sum over a contiguous float16/float32/float64 tensor of rank
   1 or 2. The output is a rank-0 tensor (NumPy `np.sum` semantics for a full
   reduction), holding one element of the input dtype. The accumulation runs
   in double precision for stability; the stored value matches the input
   dtype, with float16 rounding through the same binary16 conversion as the
   elementwise ops. shape/strides are allocated (but empty of dimensions) so
   hosts never see NULL arrays for a rank-0 result. */
static int run_tensor_sum(const weft_accel_tensor_input *input,
                          weft_accel_tensor_output *output) {
    size_t item_size;
    size_t count = 1;
    uint32_t axis;
    double total = 0.0;
    if (input->dtype != WEFT_TENSOR_FLOAT64 && input->dtype != WEFT_TENSOR_FLOAT32 &&
        input->dtype != WEFT_TENSOR_FLOAT16) {
        fail("reference tensor_sum requires a float16, float32, or float64 tensor");
        return 0;
    }
    if (input->rank == 0 || input->rank > 2 || input->shape == NULL ||
        input->strides == NULL || !tensor_c_contiguous(input)) {
        fail("reference tensor_sum requires a contiguous tensor of rank 1 or 2");
        return 0;
    }
    for (axis = 0; axis < input->rank; axis++) {
        if (input->shape[axis] < 0) {
            fail("reference tensor_sum shape must be non-negative");
            return 0;
        }
        if (input->shape[axis] != 0 && count > SIZE_MAX / (size_t)input->shape[axis]) {
            fail("reference tensor_sum shape is too large");
            return 0;
        }
        count *= (size_t)input->shape[axis];
    }
    item_size = input->dtype == WEFT_TENSOR_FLOAT64 ? sizeof(double)
              : input->dtype == WEFT_TENSOR_FLOAT16 ? sizeof(uint16_t)
              : sizeof(float);
    if (count > SIZE_MAX / item_size || input->bytes != count * item_size ||
        (count != 0 && input->data == NULL)) {
        fail("reference tensor_sum byte length does not match its shape");
        return 0;
    }
    if (input->dtype == WEFT_TENSOR_FLOAT64) {
        const double *values = (const double *)input->data;
        for (size_t index = 0; index < count; index++) total += values[index];
    } else if (input->dtype == WEFT_TENSOR_FLOAT16) {
        const uint16_t *values = (const uint16_t *)input->data;
        for (size_t index = 0; index < count; index++) total += (double)half_to_float(values[index]);
    } else {
        const float *values = (const float *)input->data;
        for (size_t index = 0; index < count; index++) total += (double)values[index];
    }
    output->dtype = input->dtype;
    output->rank = 0;
    output->shape = (int64_t *)calloc(1, sizeof(int64_t));
    output->strides = (int64_t *)calloc(1, sizeof(int64_t));
    output->data = malloc(item_size);
    output->bytes = item_size;
    if (output->shape == NULL || output->strides == NULL || output->data == NULL) {
        fail("tensor_sum output allocation failed");
        weft_accel_free_tensor(output);
        return 0;
    }
    if (input->dtype == WEFT_TENSOR_FLOAT64) {
        *(double *)output->data = total;
    } else if (input->dtype == WEFT_TENSOR_FLOAT16) {
        *(uint16_t *)output->data = float_to_half((float)total);
    } else {
        *(float *)output->data = (float)total;
    }
    return 1;
}

int weft_accel_run_tensor(const char *operation,
                          const weft_accel_tensor_input *inputs,
                          size_t input_count,
                          weft_accel_tensor_output *output) {
    const double *a;
    const double *b;
    double *result;
    size_t rows;
    size_t inner;
    size_t cols;
    size_t row;
    size_t col;
    size_t k;
    size_t bytes;
    if (operation == NULL || inputs == NULL || output == NULL) {
        fail("tensor operation requires inputs and an output");
        return 1;
    }
    memset(output, 0, sizeof(*output));
    record_exec_info("cpu", "cpu", 0);
    if (strcmp(operation, "tensor_add") == 0 || strcmp(operation, "tensor_sub") == 0 ||
        strcmp(operation, "tensor_mul") == 0 || strcmp(operation, "tensor_div") == 0) {
        elementwise_op kind = ELEMENTWISE_ADD;
        if (strcmp(operation, "tensor_sub") == 0) kind = ELEMENTWISE_SUB;
        if (strcmp(operation, "tensor_mul") == 0) kind = ELEMENTWISE_MUL;
        if (strcmp(operation, "tensor_div") == 0) kind = ELEMENTWISE_DIV;
        if (input_count != 2) {
            fail("elementwise tensor operations require two inputs");
            return 1;
        }
        if (!run_tensor_elementwise(kind, operation, inputs, output)) return 1;
        return 0;
    }
    if (strcmp(operation, "tensor_sum") == 0) {
        if (input_count != 1) {
            fail("tensor_sum requires one input");
            return 1;
        }
        if (!run_tensor_sum(&inputs[0], output)) return 1;
        return 0;
    }
    if (strcmp(operation, "tensor_matmul") != 0) {
        fail("unsupported tensor operation");
        return 1;
    }
    if (input_count != 2) {
        fail("tensor_matmul requires two inputs");
        return 1;
    }
    if (inputs[0].dtype != WEFT_TENSOR_FLOAT64 || inputs[1].dtype != WEFT_TENSOR_FLOAT64 ||
        inputs[0].rank != 2 || inputs[1].rank != 2 ||
        !tensor_c_contiguous(&inputs[0]) || !tensor_c_contiguous(&inputs[1])) {
        fail("reference tensor matmul requires contiguous float64 matrices");
        return 1;
    }
    rows = (size_t)inputs[0].shape[0];
    inner = (size_t)inputs[0].shape[1];
    if (inputs[1].shape[0] != (int64_t)inner) {
        fail("tensor matmul inner dimensions do not agree");
        return 1;
    }
    cols = (size_t)inputs[1].shape[1];
    if (rows != 0 && inner > SIZE_MAX / rows) {
        fail("tensor matmul shape is too large");
        return 1;
    }
    if (rows * inner != inputs[0].bytes / sizeof(double) ||
        (inner != 0 && cols > SIZE_MAX / inner) ||
        inner * cols != inputs[1].bytes / sizeof(double) ||
        (rows != 0 && cols > SIZE_MAX / rows) ||
        rows * cols > SIZE_MAX / sizeof(double)) {
        fail("tensor matmul byte lengths do not match shapes");
        return 1;
    }
    bytes = rows * cols * sizeof(double);
    output->dtype = WEFT_TENSOR_FLOAT64;
    output->rank = 2;
    output->shape = (int64_t *)calloc(2, sizeof(int64_t));
    output->strides = (int64_t *)calloc(2, sizeof(int64_t));
    output->data = malloc(bytes);
    if (output->shape == NULL || output->strides == NULL || (bytes != 0 && output->data == NULL)) {
        fail("tensor matmul output allocation failed");
        return 1;
    }
    output->shape[0] = (int64_t)rows;
    output->shape[1] = (int64_t)cols;
    output->strides[0] = (int64_t)cols;
    output->strides[1] = 1;
    output->bytes = bytes;
    a = (const double *)inputs[0].data;
    b = (const double *)inputs[1].data;
    result = (double *)output->data;
    for (row = 0; row < rows; row++) {
        for (col = 0; col < cols; col++) {
            double value = 0.0;
            for (k = 0; k < inner; k++) value += a[row * inner + k] * b[k * cols + col];
            result[row * cols + col] = value;
        }
    }
    return 0;
}

void weft_accel_free_tensor(weft_accel_tensor_output *output) {
    if (output == NULL) return;
    free(output->shape);
    free(output->strides);
    free(output->data);
    memset(output, 0, sizeof(*output));
}
