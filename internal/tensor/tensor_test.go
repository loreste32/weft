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

func TestAllIntegerDTypesRoundTrip(t *testing.T) {
	cases := []struct {
		dtype DType
		size  int
		set   any
		want  any
	}{
		{Int8, 1, int8(-8), int8(-8)},
		{Int16, 2, int16(-300), int16(-300)},
		{Int32, 4, int32(-100000), int32(-100000)},
		{Int64, 8, int64(-9), int64(-9)},
		{UInt8, 1, uint8(200), uint8(200)},
		{UInt16, 2, uint16(60000), uint16(60000)},
		{UInt32, 4, uint32(4000000000), uint32(4000000000)},
		{UInt64, 8, uint64(1 << 40), uint64(1 << 40)},
	}
	for _, tc := range cases {
		t.Run(string(tc.dtype), func(t *testing.T) {
			if got := tc.dtype.ItemSize(); got != tc.size {
				t.Fatalf("ItemSize = %d, want %d", got, tc.size)
			}
			parsed, err := ParseDType(string(tc.dtype))
			if err != nil || parsed != tc.dtype {
				t.Fatalf("ParseDType = %v, %v", parsed, err)
			}
			ten, err := New(tc.dtype, []int{2})
			if err != nil {
				t.Fatal(err)
			}
			if err := ten.Set(tc.set, 1); err != nil {
				t.Fatal(err)
			}
			got, err := ten.Value(1)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Value = %v (%T), want %v (%T)", got, got, tc.want, tc.want)
			}
			// FromList/ToList path used by stdlib
			list, err := FromList(tc.dtype, []int{1}, []any{tc.set})
			if err != nil {
				t.Fatal(err)
			}
			out, err := ToList(list)
			if err != nil {
				t.Fatal(err)
			}
			if len(out) != 1 || out[0] != tc.want {
				t.Fatalf("FromList/ToList = %#v, want %#v", out, tc.want)
			}
			// Float64Values must accept every numeric dtype
			vals, err := list.Float64Values()
			if err != nil || len(vals) != 1 {
				t.Fatalf("Float64Values = %v, %v", vals, err)
			}
		})
	}
}

func TestIntegerRangeRejected(t *testing.T) {
	ten, err := New(Int8, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	if err := ten.Set(int64(200), 0); err == nil {
		t.Fatal("expected out-of-range int8 store to fail")
	}
	u8, err := New(UInt8, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	if err := u8.Set(int64(-1), 0); err == nil {
		t.Fatal("expected negative uint8 store to fail")
	}
}

func TestPromoteIntegerDTypes(t *testing.T) {
	a, err := FromList(Int8, []int{2}, []any{int8(1), int8(2)})
	if err != nil {
		t.Fatal(err)
	}
	b, err := FromList(Int32, []int{2}, []any{int32(10), int32(20)})
	if err != nil {
		t.Fatal(err)
	}
	sum, err := Add(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if sum.DType() != Int32 {
		t.Fatalf("promoted dtype = %s, want int32", sum.DType())
	}
	values, err := sum.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	if values[0] != 11 || values[1] != 22 {
		t.Fatalf("promoted add = %v", values)
	}
}

func TestIntegerBinaryOpsPreserveUInt64Precision(t *testing.T) {
	left, err := FromList(UInt64, []int{1}, []any{uint64(^uint64(0) - 1)})
	if err != nil {
		t.Fatal(err)
	}
	right, err := FromList(UInt64, []int{1}, []any{uint64(1)})
	if err != nil {
		t.Fatal(err)
	}
	sum, err := Add(left, right)
	if err != nil {
		t.Fatal(err)
	}
	value, err := sum.Value(0)
	if err != nil {
		t.Fatal(err)
	}
	if value != uint64(^uint64(0)) {
		t.Fatalf("uint64 addition = %v, want %d", value, ^uint64(0))
	}
	Release(left)
	Release(right)
	Release(sum)
}

func TestIntegerMatMulPreservesUInt64Precision(t *testing.T) {
	left, err := FromList(UInt64, []int{1, 1}, []any{uint64(1 << 40)})
	if err != nil {
		t.Fatal(err)
	}
	right, err := FromList(UInt64, []int{1, 1}, []any{uint64(3)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := MatMul(left, right)
	if err != nil {
		t.Fatal(err)
	}
	value, err := result.Value(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if value != uint64(3<<40) {
		t.Fatalf("uint64 matmul = %v, want %d", value, uint64(3<<40))
	}
	Release(left)
	Release(right)
	Release(result)
}

func TestIntegerSumPreservesUInt64Precision(t *testing.T) {
	value := uint64(1 << 52)
	input, err := FromList(UInt64, []int{2}, []any{value, uint64(3)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Sum(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.DType() != UInt64 {
		t.Fatalf("sum dtype = %s, want uint64", result.DType())
	}
	got, err := result.Value()
	if err != nil {
		t.Fatal(err)
	}
	if got != value+3 {
		t.Fatalf("uint64 sum = %v, want %d", got, value+3)
	}
	Release(input)
	Release(result)
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

func TestContiguousReleasesOutputWhenCopyFails(t *testing.T) {
	base, err := Acquire(Float64, []int{4})
	if err != nil {
		t.Fatal(err)
	}
	defer Release(base)
	view, err := base.View([]int{2}, []int64{2}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt only the view shape to force the copy loop to fail after it has
	// acquired its destination buffer.
	view.shape[0] = 3
	nbytes := 3 * Float64.ItemSize()
	before := pooledLen(nbytes)
	if _, err := view.Contiguous(); err == nil {
		t.Fatal("Contiguous accepted an invalid view")
	}
	if pooledLen(nbytes) != before+1 {
		t.Fatalf("failed Contiguous leaked output: pooled=%d want=%d", pooledLen(nbytes), before+1)
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
