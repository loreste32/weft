#include "weft_accelerator.h"

#include <errno.h>
#include <stdarg.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static const char *last_error = "";

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

const char *weft_accel_manifest(void) {
    return "{\"name\":\"weft-example\",\"version\":\"1.1.0\","
           "\"abi\":1,\"vendors\":[\"example\"],"
           "\"operations\":[\"health\",\"identity\",\"matmul\"]}";
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
    if (!append_format(&output, "],\"shape\":[%zu,%zu]}", a_shape[0], b_shape[1])) {
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
    if (strcmp(operation, "health") == 0) {
        *output_json = copy_string("{\"ok\":true,\"backend\":\"example-cpu\"}");
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
