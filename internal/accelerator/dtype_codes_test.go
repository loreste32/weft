//go:build cgo && (darwin || linux || freebsd)

package accelerator

import (
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

func TestNativeDTypeCodesRoundTrip(t *testing.T) {
	dtypes := []tensor.DType{
		tensor.Bool,
		tensor.Int8,
		tensor.Int16,
		tensor.Int32,
		tensor.Int64,
		tensor.UInt8,
		tensor.UInt16,
		tensor.UInt32,
		tensor.UInt64,
		tensor.Float32,
		tensor.Float64,
	}
	for _, dtype := range dtypes {
		code := dtypeCode(dtype)
		if code == 0 {
			t.Fatalf("dtype %q has no native ABI code", dtype)
		}
		got, err := dtypeName(code)
		if err != nil {
			t.Fatalf("decode dtype %q code %d: %v", dtype, code, err)
		}
		if got != dtype {
			t.Fatalf("dtype code %d decoded as %q, want %q", code, got, dtype)
		}
	}
}
