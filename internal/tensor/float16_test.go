package tensor

import (
	"math"
	"testing"
)

// float16Cases pairs float64 inputs with the binary16 bit pattern and the
// widened value NumPy 2.4.3 produces for np.float16(x) (generated from the
// pinned conformance venv). It covers normals, subnormals, ties-to-even,
// overflow to Inf, underflow to zero, Inf, and NaN.
var float16Cases = []struct {
	in   float64
	bits uint16
	back float64
}{
	{0.0, 0x0000, 0.0},
	{math.Copysign(0, -1), 0x8000, math.Copysign(0, -1)},
	{1.0, 0x3c00, 1.0},
	{-1.0, 0xbc00, -1.0},
	{0.5, 0x3800, 0.5},
	{0.1, 0x2e66, 0.0999755859375},
	{65504.0, 0x7bff, 65504.0},
	{65519.0, 0x7bff, 65504.0},
	{65512.0, 0x7bff, 65504.0},
	{65520.0, 0x7c00, math.Inf(1)},
	{1e10, 0x7c00, math.Inf(1)},
	{-1e10, 0xfc00, math.Inf(-1)},
	{6.103515625e-05, 0x0400, 6.103515625e-05},
	{5.960464477539063e-08, 0x0001, 5.960464477539063e-08},
	{2.9802322387695312e-08, 0x0000, 0.0},
	{8.940696716308594e-08, 0x0002, 1.1920928955078125e-07},
	{1e-10, 0x0000, 0.0},
	{1.5e-08, 0x0000, 0.0},
	{5.9604645e-08, 0x0001, 5.960464477539063e-08},
	{2048.0001, 0x6800, 2048.0},
	{1.00048828125, 0x3c00, 1.0},         // exact midpoint, ties to even
	{1.00146484375, 0x3c02, 1.001953125}, // exact midpoint, ties to even
	{1.0009765625, 0x3c01, 1.0009765625},
	{0.999755859375, 0x3c00, 1.0}, // midpoint below 1.0, ties to even
	{math.Inf(1), 0x7c00, math.Inf(1)},
	{math.Inf(-1), 0xfc00, math.Inf(-1)},
	{math.NaN(), 0x7e00, math.NaN()},
}

func TestFloat16FromFloat64MatchesNumPy(t *testing.T) {
	for _, tc := range float16Cases {
		if got := Float16FromFloat64(tc.in); got != tc.bits {
			t.Errorf("Float16FromFloat64(%v) = %#04x, NumPy says %#04x", tc.in, got, tc.bits)
		}
	}
}

func TestFloat16ToFloat64MatchesNumPy(t *testing.T) {
	for _, tc := range float16Cases {
		got := Float16ToFloat64(tc.bits)
		if math.IsNaN(tc.back) {
			if !math.IsNaN(got) {
				t.Errorf("Float16ToFloat64(%#04x) = %v, want NaN", tc.bits, got)
			}
			continue
		}
		if got != tc.back || math.Signbit(got) != math.Signbit(tc.back) {
			t.Errorf("Float16ToFloat64(%#04x) = %v, NumPy says %v", tc.bits, got, tc.back)
		}
	}
}

func TestFloat16TensorStorage(t *testing.T) {
	if Float16.ItemSize() != 2 {
		t.Fatalf("Float16 ItemSize = %d, want 2", Float16.ItemSize())
	}
	parsed, err := ParseDType("float16")
	if err != nil || parsed != Float16 {
		t.Fatalf("ParseDType(float16) = %v, %v", parsed, err)
	}
	tensor, err := FromList(Float16, []int{4}, []any{0.1, 65504.0, 1e10, 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ToList(tensor)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{float32(0.0999755859375), float32(65504.0), float32(math.Inf(1)), float32(0)}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ToList[%d] = %v (%T), want %v", i, got[i], got[i], want[i])
		}
	}
	if err := tensor.Set(2048.0001, 0); err != nil {
		t.Fatal(err)
	}
	value, err := tensor.Value(0)
	if err != nil || value != float32(2048.0) {
		t.Fatalf("Value(0) = %v, %v; want 2048", value, err)
	}
}

func TestFloat16PromotionAndOps(t *testing.T) {
	if got := promoteDType(Float16, Int16); got != Float32 {
		t.Fatalf("promoteDType(float16, int16) = %s, want float32", got)
	}
	if got := promoteDType(Float16, UInt32); got != Float64 {
		t.Fatalf("promoteDType(float16, uint32) = %s, want float64", got)
	}
	left, err := FromList(Float16, []int{2}, []any{0.1, 1.0})
	if err != nil {
		t.Fatal(err)
	}
	right, err := FromList(Float16, []int{2}, []any{0.2, 2.0})
	if err != nil {
		t.Fatal(err)
	}
	sum, err := Add(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if sum.DType() != Float16 {
		t.Fatalf("Add dtype = %s, want float16", sum.DType())
	}
	got, err := ToList(sum)
	if err != nil {
		t.Fatal(err)
	}
	// Half arithmetic rounds each result back to binary16.
	want := []any{float32(0.2998046875), float32(3.0)}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sum[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
