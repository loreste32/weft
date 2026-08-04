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
