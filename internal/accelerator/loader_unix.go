//go:build cgo && (darwin || linux || freebsd)

package accelerator

/*
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>

typedef void *weft_accel_handle;
typedef const char *(*weft_accel_manifest_fn)(void);
typedef int (*weft_accel_run_fn)(const char *, const char *, char **);
typedef const char *(*weft_accel_error_fn)(void);
typedef void (*weft_accel_free_fn)(char *);

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
*/
import "C"

import (
	"fmt"
	"unsafe"
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

func (p *nativeSharedLibrary) close() error {
	if p.handle != nil {
		C.weft_accel_close(p.handle)
		p.handle = nil
	}
	return nil
}
