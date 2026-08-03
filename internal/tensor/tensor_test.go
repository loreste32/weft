package tensor

import (
	"errors"
	"testing"
)

func TestTypedStorageRoundTrip(t *testing.T) {
	tensor, err := FromFloat64([]int{2, 2}, []float64{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if tensor.DType() != Float64 || tensor.Numel() != 4 || !tensor.IsContiguous() {
		t.Fatalf("unexpected tensor metadata: dtype=%s shape=%v strides=%v", tensor.DType(), tensor.Shape(), tensor.Strides())
	}
	value, err := tensor.Value(1, -1)
	if err != nil || value != float64(4) {
		t.Fatalf("Value(1,-1) = %v, %v", value, err)
	}
	if err := tensor.Set(7.5, 0, 1); err != nil {
		t.Fatal(err)
	}
	values, err := tensor.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{1, 7.5, 3, 4}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("values = %v, want %v", values, want)
		}
	}
}

func TestTypedDTypes(t *testing.T) {
	bools, err := New(Bool, []int{2})
	if err != nil {
		t.Fatal(err)
	}
	if err := bools.Set(true, 0); err != nil {
		t.Fatal(err)
	}
	value, err := bools.Value(0)
	if err != nil || value != true {
		t.Fatalf("bool round trip = %v, %v", value, err)
	}
	ints, err := New(Int64, []int{2})
	if err != nil {
		t.Fatal(err)
	}
	if err := ints.Set(int64(-3), 1); err != nil {
		t.Fatal(err)
	}
	value, err = ints.Value(1)
	if err != nil || value != int64(-3) {
		t.Fatalf("int round trip = %v, %v", value, err)
	}
	if _, err := ParseDType("complex128"); err == nil {
		t.Fatal("unsupported dtype accepted")
	}
}

func TestTransposeIsZeroCopyAndReshapeRequiresContiguous(t *testing.T) {
	original, err := FromFloat64([]int{2, 3}, []float64{1, 2, 3, 4, 5, 6})
	if err != nil {
		t.Fatal(err)
	}
	transposed, err := original.Transpose([]int{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if transposed.IsContiguous() || transposed.Numel() != 6 {
		t.Fatalf("transpose metadata = contiguous:%v shape:%v", transposed.IsContiguous(), transposed.Shape())
	}
	value, err := transposed.Value(2, 1)
	if err != nil || value != float64(6) {
		t.Fatalf("transposed value = %v, %v", value, err)
	}
	if _, err := transposed.Reshape([]int{6}); !errors.Is(err, ErrNotContiguous) {
		t.Fatalf("reshape error = %v, want ErrNotContiguous", err)
	}
	contiguous, err := transposed.Contiguous()
	if err != nil {
		t.Fatal(err)
	}
	values, err := contiguous.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{1, 4, 2, 5, 3, 6}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("contiguous values = %v, want %v", values, want)
		}
	}
}

func TestContiguousCopiesOffsetView(t *testing.T) {
	base, err := FromFloat64([]int{5}, []float64{99, 1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	view, err := base.View([]int{2, 2}, []int64{2, 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	contiguous, err := view.Contiguous()
	if err != nil {
		t.Fatal(err)
	}
	values, err := contiguous.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{1, 2, 3, 4}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("offset view values = %v, want %v", values, want)
		}
	}
	if contiguous.Offset() != 0 || !contiguous.IsContiguous() {
		t.Fatalf("contiguous offset view metadata: offset=%d strides=%v", contiguous.Offset(), contiguous.Strides())
	}
}

func TestBroadcastViewAndBounds(t *testing.T) {
	base, err := FromFloat64([]int{1, 3}, []float64{2, 4, 6})
	if err != nil {
		t.Fatal(err)
	}
	broadcast, err := base.BroadcastTo([]int{2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if value, err := broadcast.Value(1, 2); err != nil || value != float64(6) {
		t.Fatalf("broadcast value = %v, %v", value, err)
	}
	if err := broadcast.Set(9, 1, 0); err == nil {
		t.Fatal("write through broadcast view accepted")
	}
	if _, err := base.Value(0, 3); !errors.Is(err, ErrOutOfBounds) {
		t.Fatalf("out of bounds error = %v", err)
	}
}

func TestInvalidViewsAreRejected(t *testing.T) {
	base, err := New(Float32, []int{4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base.View([]int{2}, []int64{2}, 1); err != nil {
		t.Fatal("valid strided view rejected:", err)
	}
	if _, err := base.View([]int{3}, []int64{2}, 2); !errors.Is(err, ErrOutOfBounds) {
		t.Fatalf("invalid view error = %v", err)
	}
	if _, err := base.View([]int{2}, []int64{int64(^uint64(0) >> 1)}, 0); !errors.Is(err, ErrOutOfBounds) {
		t.Fatalf("overflowing view error = %v", err)
	}
	if _, err := base.View([]int{0}, []int64{1}, 5); !errors.Is(err, ErrOutOfBounds) {
		t.Fatalf("empty view with invalid offset error = %v", err)
	}
	if _, err := New(Float64, []int{-1}); !errors.Is(err, ErrInvalidShape) {
		t.Fatalf("invalid shape error = %v", err)
	}
}
