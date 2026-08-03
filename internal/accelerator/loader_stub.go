//go:build !cgo || (!darwin && !linux && !freebsd)

package accelerator

import "fmt"

func nativeSupported() bool { return false }

func loadNative(path string) (nativePlugin, error) {
	return nil, fmt.Errorf("%w: %s", ErrUnavailable, path)
}
