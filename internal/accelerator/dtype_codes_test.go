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
		tensor.Float16,
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

// The native ABI dtype codes are frozen at 1-12: existing codes never move,
// and float16 appended as code 12. Codes outside that range must stay
// unmapped so unknown provider output dtypes are rejected.
func TestFloat16NativeABICode(t *testing.T) {
	if code := dtypeCode(tensor.Float16); code != 12 {
		t.Fatalf("float16 maps to ABI code %d, want 12", code)
	}
	name, err := dtypeName(12)
	if err != nil || name != tensor.Float16 {
		t.Fatalf("dtype code 12 decoded as %q, %v; want float16", name, err)
	}
	for _, code := range []uint32{0, 13, 0xffffffff} {
		if _, err := dtypeName(code); err == nil {
			t.Fatalf("dtype code %d unexpectedly decoded", code)
		}
	}
}
