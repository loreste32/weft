package tensor

import (
	"testing"
)

func TestPoolAcquireReleaseReuse(t *testing.T) {
	shape := []int{4, 4}
	first, err := Acquire(Float32, shape)
	if err != nil {
		t.Fatal(err)
	}
	nbytes := len(first.storage)
	if nbytes == 0 {
		t.Fatal("expected non-empty storage")
	}
	if err := first.Set(float32(3.5), 0, 0); err != nil {
		t.Fatal(err)
	}

	before := pooledLen(nbytes)
	Release(first)
	if first.storage != nil {
		t.Fatal("Release did not clear handle")
	}
	if pooledLen(nbytes) != before+1 {
		t.Fatalf("pooled count = %d, want %d", pooledLen(nbytes), before+1)
	}

	// Double-release must not panic and must not grow the free list again.
	Release(first)
	if pooledLen(nbytes) != before+1 {
		t.Fatalf("double-release changed pool size to %d", pooledLen(nbytes))
	}

	second, err := Acquire(Float32, shape)
	if err != nil {
		t.Fatal(err)
	}
	if pooledLen(nbytes) != before {
		t.Fatalf("Acquire did not reuse pooled buffer; pooled=%d want %d", pooledLen(nbytes), before)
	}
	// Reused buffer must be zeroed.
	value, err := second.Value(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if value.(float32) != 0 {
		t.Fatalf("reused buffer not zeroed: %v", value)
	}
	Release(second)
}

func TestPoolDoesNotFreeViews(t *testing.T) {
	base, err := Acquire(Int16, []int{8})
	if err != nil {
		t.Fatal(err)
	}
	nbytes := len(base.storage)
	view, err := base.View([]int{4}, []int64{1}, 0)
	if err != nil {
		t.Fatal(err)
	}
	before := pooledLen(nbytes)
	Release(view)
	if pooledLen(nbytes) != before {
		t.Fatal("view Release incorrectly pooled shared storage")
	}
	// Base still usable.
	if err := base.Set(int16(7), 3); err != nil {
		t.Fatal(err)
	}
	got, err := base.Value(3)
	if err != nil || got != int16(7) {
		t.Fatalf("base after view release = %v, %v", got, err)
	}
	Release(base)
	if pooledLen(nbytes) != before+1 {
		t.Fatalf("owned Release did not pool: %d", pooledLen(nbytes))
	}
}

func TestPoolDifferentSizes(t *testing.T) {
	a, err := Acquire(UInt8, []int{16})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Acquire(UInt8, []int{32})
	if err != nil {
		t.Fatal(err)
	}
	na, nb := len(a.storage), len(b.storage)
	Release(a)
	Release(b)
	if pooledLen(na) < 1 || pooledLen(nb) < 1 {
		t.Fatalf("expected both sizes pooled: %d and %d", pooledLen(na), pooledLen(nb))
	}
	// Wrong size must not pull the other bucket.
	c, err := Acquire(UInt8, []int{16})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.storage) != na {
		t.Fatalf("size mismatch: got %d want %d", len(c.storage), na)
	}
	Release(c)
}

func TestConstructorsReleasePooledStorageOnError(t *testing.T) {
	shape := []int{17}
	nbytes := shape[0] * Float64.ItemSize()
	before := pooledLen(nbytes)

	if _, err := FromFloat64(shape, make([]float64, 16)); err == nil {
		t.Fatal("FromFloat64 accepted the wrong number of values")
	}
	if pooledLen(nbytes) != before+1 {
		t.Fatalf("FromFloat64 error leaked storage: pooled=%d want=%d", pooledLen(nbytes), before+1)
	}

	shape2 := []int{18}
	nbytes2 := shape2[0] * Float64.ItemSize()
	before2 := pooledLen(nbytes2)
	if _, err := FromList(Float64, shape2, make([]any, 17)); err == nil {
		t.Fatal("FromList accepted the wrong number of values")
	}
	if pooledLen(nbytes2) != before2+1 {
		t.Fatalf("FromList error leaked storage: pooled=%d want=%d", pooledLen(nbytes2), before2+1)
	}

	first, err := Acquire(Float64, shape)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(Float64, shape)
	if err != nil {
		Release(first)
		t.Fatal(err)
	}
	Release(first)
	Release(second)
}
