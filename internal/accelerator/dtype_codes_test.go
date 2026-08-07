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

// The native ABI dtype codes are frozen at 1-11. float16 storage exists in
// the tensor package but must never cross run_tensor: it has no ABI code on
// the input side, and a provider reporting code 12 is rejected on output.
func TestFloat16RejectedFromNativeABI(t *testing.T) {
	if code := dtypeCode(tensor.Float16); code != 0 {
		t.Fatalf("float16 unexpectedly maps to ABI code %d", code)
	}
	for _, code := range []uint32{0, 12} {
		if _, err := dtypeName(code); err == nil {
			t.Fatalf("dtype code %d unexpectedly decoded", code)
		}
	}
	f16, err := tensor.FromList(tensor.Float16, []int{1}, []any{1.5})
	if err != nil {
		t.Fatal(err)
	}
	plugin := &nativeSharedLibrary{}
	if _, err := plugin.runTensor("tensor_add", []*tensor.Tensor{f16}); err == nil {
		t.Fatal("runTensor accepted a float16 input")
	}
}
