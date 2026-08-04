package tensor

import (
	"math"
	"testing"
)

func TestBinaryOpsAndBroadcast(t *testing.T) {
	a, err := FromFloat64([]int{2, 1}, []float64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	b, err := FromFloat64([]int{1, 3}, []float64{10, 20, 30})
	if err != nil {
		t.Fatal(err)
	}
	sum, err := Add(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if got := sum.Shape(); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("shape = %v", got)
	}
	values, err := sum.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{11, 21, 31, 12, 22, 32}
	for i := range want {
		if math.Abs(values[i]-want[i]) > 1e-12 {
			t.Fatalf("values = %v, want %v", values, want)
		}
	}
}

func TestMatMul(t *testing.T) {
	a, err := FromFloat64([]int{2, 3}, []float64{1, 2, 3, 4, 5, 6})
	if err != nil {
		t.Fatal(err)
	}
	b, err := FromFloat64([]int{3, 2}, []float64{7, 8, 9, 10, 11, 12})
	if err != nil {
		t.Fatal(err)
	}
	out, err := MatMul(a, b)
	if err != nil {
		t.Fatal(err)
	}
	values, err := out.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	// [[58,64],[139,154]]
	want := []float64{58, 64, 139, 154}
	for i := range want {
		if math.Abs(values[i]-want[i]) > 1e-9 {
			t.Fatalf("matmul = %v, want %v", values, want)
		}
	}
}

func TestBroadcastFailure(t *testing.T) {
	a, _ := FromFloat64([]int{2}, []float64{1, 2})
	b, _ := FromFloat64([]int{3}, []float64{1, 2, 3})
	if _, err := Add(a, b); err == nil {
		t.Fatal("expected broadcast failure")
	}
}

func TestFromListToListRoundTrip(t *testing.T) {
	in := []any{int64(1), int64(2), int64(3), int64(4)}
	ten, err := FromList(Int64, []int{2, 2}, in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ToList(ten)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 || out[3].(int64) != 4 {
		t.Fatalf("roundtrip = %#v", out)
	}
}

// numpyPromotionMatrix is the NumPy 2.4.3 np.promote_types result for every
// ordered pair of the supported dtypes, indexed by
// [bool,int8,int16,int32,int64,uint8,uint16,uint32,uint64,float32,float64].
// Generated from the pinned conformance venv; regenerate with:
//
//	python3 -c 'import numpy as np; ds=["bool","int8","int16","int32","int64","uint8","uint16","uint32","uint64","float32","float64"]; print([[np.promote_types(a,b).name for b in ds] for a in ds])'
//
// The Weft-side _promote_dtype in packages/warp/lib.weft must agree with this
// table; scripts/conformance/run.py locks that path against NumPy directly.
var numpyPromotionMatrix = [11][11]DType{
	{Bool, Int8, Int16, Int32, Int64, UInt8, UInt16, UInt32, UInt64, Float32, Float64},
	{Int8, Int8, Int16, Int32, Int64, Int16, Int32, Int64, Float64, Float32, Float64},
	{Int16, Int16, Int16, Int32, Int64, Int16, Int32, Int64, Float64, Float32, Float64},
	{Int32, Int32, Int32, Int32, Int64, Int32, Int32, Int64, Float64, Float64, Float64},
	{Int64, Int64, Int64, Int64, Int64, Int64, Int64, Int64, Float64, Float64, Float64},
	{UInt8, Int16, Int16, Int32, Int64, UInt8, UInt16, UInt32, UInt64, Float32, Float64},
	{UInt16, Int32, Int32, Int32, Int64, UInt16, UInt16, UInt32, UInt64, Float32, Float64},
	{UInt32, Int64, Int64, Int64, Int64, UInt32, UInt32, UInt32, UInt64, Float64, Float64},
	{UInt64, Float64, Float64, Float64, Float64, UInt64, UInt64, UInt64, UInt64, Float64, Float64},
	{Float32, Float32, Float32, Float64, Float64, Float32, Float32, Float64, Float64, Float32, Float64},
	{Float64, Float64, Float64, Float64, Float64, Float64, Float64, Float64, Float64, Float64, Float64},
}

func TestPromoteDTypeNumPyMatrix(t *testing.T) {
	dtypes := []DType{Bool, Int8, Int16, Int32, Int64, UInt8, UInt16, UInt32, UInt64, Float32, Float64}
	for i, a := range dtypes {
		for j, b := range dtypes {
			if got := promoteDType(a, b); got != numpyPromotionMatrix[i][j] {
				t.Errorf("promoteDType(%s, %s) = %s, NumPy says %s", a, b, got, numpyPromotionMatrix[i][j])
			}
		}
	}
}
