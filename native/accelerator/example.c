#include "weft_accelerator.h"

#include <stdlib.h>
#include <string.h>

static const char *last_error = "";

const char *weft_accel_manifest(void) {
    return "{\"name\":\"weft-example\",\"version\":\"1.0.0\","
           "\"abi\":1,\"vendors\":[\"example\"],"
           "\"operations\":[\"health\",\"identity\"]}";
}

const char *weft_accel_last_error(void) { return last_error; }

int weft_accel_run(const char *operation, const char *input_json,
                   char **output_json) {
    if (operation == NULL || input_json == NULL || output_json == NULL) {
        last_error = "operation, input, and output are required";
        return 1;
    }
    if (strcmp(operation, "health") == 0) {
        const char *health = "{\"ok\":true}";
        *output_json = (char *)malloc(strlen(health) + 1);
        if (*output_json == NULL) {
            last_error = "allocation failed";
            return 2;
        }
        strcpy(*output_json, health);
        return 0;
    }
    if (strcmp(operation, "identity") == 0) {
        *output_json = (char *)malloc(strlen(input_json) + 1);
        if (*output_json == NULL) {
            last_error = "allocation failed";
            return 2;
        }
        strcpy(*output_json, input_json);
        return 0;
    }
    last_error = "unknown operation";
    return 3;
}

void weft_accel_free(char *output_json) { free(output_json); }
