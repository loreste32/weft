//go:build cgo && (darwin || linux || freebsd)

package accelerator

/*
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

typedef void *weft_accel_handle;
typedef const char *(*weft_accel_manifest_fn)(void);
typedef int (*weft_accel_run_fn)(const char *, const char *, char **);
typedef const char *(*weft_accel_error_fn)(void);
typedef void (*weft_accel_free_fn)(char *);
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
typedef int (*weft_accel_run_tensor_fn)(const char *, const weft_accel_tensor_input *,
                                        size_t, weft_accel_tensor_output *);
typedef void (*weft_accel_free_tensor_fn)(weft_accel_tensor_output *);
typedef char *(*weft_accel_exec_info_fn)(void);

// Optional additive ABI v1 export: execution reporting for the last run.
// Returns NULL when the provider does not export weft_accel_exec_info.
static char *weft_accel_exec_info_query(weft_accel_handle handle) {
    weft_accel_exec_info_fn fn = (weft_accel_exec_info_fn)dlsym(handle, "weft_accel_exec_info");
    return fn == NULL ? NULL : fn();
}

static char *weft_accel_strdup(const char *value) {
    if (value == NULL) return NULL;
    size_t size = strlen(value) + 1;
    char *copy = (char *)malloc(size);
    if (copy != NULL) memcpy(copy, value, size);
    return copy;
}

static weft_accel_handle weft_accel_open(const char *path, char **err_out) {
    dlerror();
    void *handle = dlopen(path, RTLD_NOW | RTLD_LOCAL);
    const char *err = dlerror();
    if (err != NULL && err_out != NULL) {
        *err_out = weft_accel_strdup(err);
    }
    return handle;
}

static void weft_accel_close(weft_accel_handle handle) {
    if (handle != NULL) dlclose(handle);
}

static const char *weft_accel_manifest(weft_accel_handle handle) {
    weft_accel_manifest_fn fn = (weft_accel_manifest_fn)dlsym(handle, "weft_accel_manifest");
    return fn == NULL ? NULL : fn();
}

static int weft_accel_has_required(weft_accel_handle handle) {
    return dlsym(handle, "weft_accel_manifest") != NULL &&
           dlsym(handle, "weft_accel_run") != NULL &&
           dlsym(handle, "weft_accel_free") != NULL;
}

static int weft_accel_has_tensor(weft_accel_handle handle) {
    return dlsym(handle, "weft_accel_run_tensor") != NULL &&
           dlsym(handle, "weft_accel_free_tensor") != NULL;
}

static int weft_accel_run(weft_accel_handle handle, const char *op,
                          const char *input, char **output, char **err_out) {
    weft_accel_run_fn fn = (weft_accel_run_fn)dlsym(handle, "weft_accel_run");
    if (fn == NULL) {
        if (err_out != NULL) *err_out = weft_accel_strdup("missing weft_accel_run symbol");
        return -1;
    }
    int rc = fn(op, input, output);
    if (rc != 0 && err_out != NULL) {
        weft_accel_error_fn error_fn = (weft_accel_error_fn)dlsym(handle, "weft_accel_last_error");
        const char *message = error_fn == NULL ? "native accelerator operation failed" : error_fn();
        *err_out = weft_accel_strdup(message == NULL ? "native accelerator operation failed" : message);
    }
    return rc;
}

static void weft_accel_free(weft_accel_handle handle, char *output) {
    weft_accel_free_fn fn = (weft_accel_free_fn)dlsym(handle, "weft_accel_free");
    if (fn != NULL && output != NULL) fn(output);
}

static int weft_accel_run_tensor(weft_accel_handle handle, const char *op,
                                 const weft_accel_tensor_input *inputs,
                                 size_t input_count, weft_accel_tensor_output *output,
                                 char **err_out) {
    weft_accel_run_tensor_fn fn = (weft_accel_run_tensor_fn)dlsym(handle, "weft_accel_run_tensor");
    if (fn == NULL) {
        if (err_out != NULL) *err_out = weft_accel_strdup("missing weft_accel_run_tensor symbol");
        return -1;
    }
    int rc = fn(op, inputs, input_count, output);
    if (rc != 0 && err_out != NULL) {
        weft_accel_error_fn error_fn = (weft_accel_error_fn)dlsym(handle, "weft_accel_last_error");
        const char *message = error_fn == NULL ? "native tensor operation failed" : error_fn();
        *err_out = weft_accel_strdup(message == NULL ? "native tensor operation failed" : message);
    }
    return rc;
}

static void weft_accel_free_tensor(weft_accel_handle handle, weft_accel_tensor_output *output) {
    weft_accel_free_tensor_fn fn = (weft_accel_free_tensor_fn)dlsym(handle, "weft_accel_free_tensor");
    if (fn != NULL && output != NULL) fn(output);
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/loreste/weft/internal/tensor"
)

type nativeSharedLibrary struct {
	handle C.weft_accel_handle
}

func nativeSupported() bool { return true }

func loadNative(path string) (nativePlugin, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var cErr *C.char
	handle := C.weft_accel_open(cpath, &cErr)
	if handle == nil {
		if cErr != nil {
			defer C.free(unsafe.Pointer(cErr))
			return nil, fmt.Errorf("load accelerator plugin: %s", C.GoString(cErr))
		}
		return nil, fmt.Errorf("load accelerator plugin %q failed", path)
	}
	if C.weft_accel_has_required(handle) == 0 {
		C.weft_accel_close(handle)
		return nil, fmt.Errorf("accelerator plugin %q is missing a required ABI symbol", path)
	}
	return &nativeSharedLibrary{handle: handle}, nil
}

func (p *nativeSharedLibrary) manifest() ([]byte, error) {
	value := C.weft_accel_manifest(p.handle)
	if value == nil {
		return nil, fmt.Errorf("missing weft_accel_manifest symbol")
	}
	return []byte(C.GoString(value)), nil
}

// execInfo implements nativeExecInfoPlugin. A missing symbol or null result
// is an error here; the registry maps that to StatusUnreported.
func (p *nativeSharedLibrary) execInfo() ([]byte, error) {
	value := C.weft_accel_exec_info_query(p.handle)
	if value == nil {
		return nil, fmt.Errorf("missing weft_accel_exec_info symbol")
	}
	defer C.weft_accel_free(p.handle, value)
	return []byte(C.GoString(value)), nil
}

func (p *nativeSharedLibrary) run(op string, input []byte) ([]byte, error) {
	cop := C.CString(op)
	defer C.free(unsafe.Pointer(cop))
	cinput := C.CString(string(input))
	defer C.free(unsafe.Pointer(cinput))
	var output *C.char
	var cErr *C.char
	if rc := C.weft_accel_run(p.handle, cop, cinput, &output, &cErr); rc != 0 {
		if cErr != nil {
			defer C.free(unsafe.Pointer(cErr))
			return nil, fmt.Errorf("%s", C.GoString(cErr))
		}
		return nil, fmt.Errorf("native accelerator returned status %d", int(rc))
	}
	if output == nil {
		return nil, fmt.Errorf("native accelerator returned no result")
	}
	defer C.weft_accel_free(p.handle, output)
	return []byte(C.GoString(output)), nil
}

func (p *nativeSharedLibrary) runTensor(operation string, inputs []*tensor.Tensor) (*tensor.Tensor, error) {
	if len(inputs) == 0 || len(inputs) > MaxTensorInputs {
		return nil, fmt.Errorf("native tensor input count must be between 1 and %d", MaxTensorInputs)
	}
	cop := C.CString(operation)
	defer C.free(unsafe.Pointer(cop))
	inputMemory := C.calloc(C.size_t(len(inputs)), C.size_t(C.sizeof_weft_accel_tensor_input))
	if inputMemory == nil {
		return nil, fmt.Errorf("allocate tensor input descriptors")
	}
	defer C.free(inputMemory)
	inputArray := (*[1 << 20]C.weft_accel_tensor_input)(inputMemory)[:len(inputs):len(inputs)]
	allocations := make([]unsafe.Pointer, 0, len(inputs)*3)
	defer func() {
		for _, allocation := range allocations {
			C.free(allocation)
		}
	}()
	for i, input := range inputs {
		shape := input.Shape()
		strides := input.Strides()
		cShape := C.malloc(C.size_t(len(shape)) * C.size_t(C.sizeof_int64_t))
		cStrides := C.malloc(C.size_t(len(strides)) * C.size_t(C.sizeof_int64_t))
		data := input.Bytes()
		var cData unsafe.Pointer
		if len(data) > 0 {
			cData = C.CBytes(data)
		}
		if (len(shape) > 0 && cShape == nil) ||
			(len(strides) > 0 && cStrides == nil) ||
			(len(data) > 0 && cData == nil) {
			return nil, fmt.Errorf("allocate tensor input %d", i)
		}
		if cShape != nil {
			allocations = append(allocations, cShape)
		}
		if cStrides != nil {
			allocations = append(allocations, cStrides)
		}
		if cData != nil {
			allocations = append(allocations, cData)
		}
		if len(shape) > 0 {
			shapeArray := (*[1 << 20]C.int64_t)(cShape)[:len(shape):len(shape)]
			for j, value := range shape {
				shapeArray[j] = C.int64_t(value)
			}
		}
		if len(strides) > 0 {
			strideArray := (*[1 << 20]C.int64_t)(cStrides)[:len(strides):len(strides)]
			for j, value := range strides {
				strideArray[j] = C.int64_t(value)
			}
		}
		inputArray[i].dtype = C.uint32_t(dtypeCode(input.DType()))
		inputArray[i].rank = C.uint32_t(len(shape))
		inputArray[i].shape = (*C.int64_t)(cShape)
		inputArray[i].strides = (*C.int64_t)(cStrides)
		inputArray[i].data = cData
		inputArray[i].bytes = C.size_t(len(data))
	}
	outputMemory := C.calloc(1, C.size_t(C.sizeof_weft_accel_tensor_output))
	if outputMemory == nil {
		return nil, fmt.Errorf("allocate tensor output descriptor")
	}
	defer C.free(outputMemory)
	output := (*C.weft_accel_tensor_output)(outputMemory)
	var cErr *C.char
	if rc := C.weft_accel_run_tensor(p.handle, cop, (*C.weft_accel_tensor_input)(inputMemory), C.size_t(len(inputs)), output, &cErr); rc != 0 {
		C.weft_accel_free_tensor(p.handle, output)
		if cErr != nil {
			defer C.free(unsafe.Pointer(cErr))
			return nil, fmt.Errorf("%s", C.GoString(cErr))
		}
		return nil, fmt.Errorf("native tensor provider returned status %d", int(rc))
	}
	if (output.rank != 0 && (output.shape == nil || output.strides == nil)) ||
		(output.bytes != 0 && output.data == nil) {
		C.weft_accel_free_tensor(p.handle, output)
		return nil, fmt.Errorf("native tensor provider returned incomplete output")
	}
	if output.rank > 32 || output.bytes > C.size_t(MaxResultBytes) {
		C.weft_accel_free_tensor(p.handle, output)
		return nil, fmt.Errorf("native tensor provider returned an oversized output")
	}
	dtypeName, err := dtypeName(uint32(output.dtype))
	if err != nil {
		C.weft_accel_free_tensor(p.handle, output)
		return nil, err
	}
	shapeArray := (*[32]C.int64_t)(unsafe.Pointer(output.shape))[:output.rank:output.rank]
	strideArray := (*[32]C.int64_t)(unsafe.Pointer(output.strides))[:output.rank:output.rank]
	shape := make([]int, output.rank)
	wantStrides := make([]int64, output.rank)
	for i := range shape {
		if shapeArray[i] < 0 || shapeArray[i] > C.int64_t(int64(^uint(0)>>1)) {
			C.weft_accel_free_tensor(p.handle, output)
			return nil, fmt.Errorf("native tensor provider returned an invalid shape")
		}
		shape[i] = int(shapeArray[i])
		wantStrides[i] = int64(strideArray[i])
	}
	resultBytes := C.GoBytes(output.data, C.int(output.bytes))
	C.weft_accel_free_tensor(p.handle, output)
	result, err := tensor.FromBytes(dtypeName, shape, resultBytes)
	if err != nil {
		return nil, err
	}
	if !sameStrides(result.Strides(), wantStrides) {
		return nil, fmt.Errorf("native tensor provider returned non-contiguous output strides")
	}
	return result, nil
}

func dtypeCode(dtype tensor.DType) uint32 {
	switch dtype {
	case tensor.Bool:
		return 1
	case tensor.Int64:
		return 2
	case tensor.Float32:
		return 3
	case tensor.Float64:
		return 4
	case tensor.Int8:
		return 5
	case tensor.Int16:
		return 6
	case tensor.Int32:
		return 7
	case tensor.UInt8:
		return 8
	case tensor.UInt16:
		return 9
	case tensor.UInt32:
		return 10
	case tensor.UInt64:
		return 11
	default:
		return 0
	}
}

func dtypeName(code uint32) (tensor.DType, error) {
	switch code {
	case 1:
		return tensor.Bool, nil
	case 2:
		return tensor.Int64, nil
	case 3:
		return tensor.Float32, nil
	case 4:
		return tensor.Float64, nil
	case 5:
		return tensor.Int8, nil
	case 6:
		return tensor.Int16, nil
	case 7:
		return tensor.Int32, nil
	case 8:
		return tensor.UInt8, nil
	case 9:
		return tensor.UInt16, nil
	case 10:
		return tensor.UInt32, nil
	case 11:
		return tensor.UInt64, nil
	default:
		return "", fmt.Errorf("native tensor provider returned unsupported dtype code %d", code)
	}
}

func sameStrides(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (p *nativeSharedLibrary) close() error {
	if p.handle != nil {
		C.weft_accel_close(p.handle)
		p.handle = nil
	}
	return nil
}
