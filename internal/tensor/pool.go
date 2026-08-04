package tensor

import (
	"fmt"
	"sync"
)

// Pool is a simple free-list allocator for contiguous tensor storage buffers.
// Buffers are keyed by exact byte size and reused by Acquire.
type Pool struct {
	mu   sync.Mutex
	free map[int][][]byte
}

// globalPool backs package-level Acquire/Release.
var globalPool = &Pool{free: make(map[int][][]byte)}

// Acquire returns a zeroed contiguous tensor, reusing a pooled buffer of the
// same byte size when available.
func Acquire(dtype DType, shape []int) (*Tensor, error) {
	return globalPool.Acquire(dtype, shape)
}

// Release returns a tensor's owned storage to the global pool and clears the
// handle. Double-release is a no-op. Views (non-owned storage) are cleared
// without pooling so they cannot free shared buffers.
func Release(t *Tensor) {
	globalPool.Release(t)
}

// Acquire implements Pool-scoped allocation.
func (p *Pool) Acquire(dtype DType, shape []int) (*Tensor, error) {
	if p == nil {
		return New(dtype, shape)
	}
	if !dtype.Valid() {
		return nil, fmt.Errorf("unsupported tensor dtype %q", dtype)
	}
	count, err := elementCount(shape)
	if err != nil {
		return nil, err
	}
	nbytes, ok := checkedBytes(count, dtype.ItemSize())
	if !ok {
		return nil, fmt.Errorf("tensor allocation is too large")
	}

	var buf []byte
	p.mu.Lock()
	if slots := p.free[nbytes]; len(slots) > 0 {
		buf = slots[len(slots)-1]
		p.free[nbytes] = slots[:len(slots)-1]
	}
	p.mu.Unlock()

	if buf == nil {
		buf = make([]byte, nbytes)
	} else {
		// Reused buffers must start zeroed so callers observe New-like state.
		clear(buf)
	}

	return &Tensor{
		dtype:           dtype,
		shape:           cloneInts(shape),
		strides:         contiguousStrides(shape),
		storage:         buf,
		storageElements: count,
		owned:           true,
	}, nil
}

// Release implements Pool-scoped recycling. Safe to call multiple times.
func (p *Pool) Release(t *Tensor) {
	if t == nil || t.storage == nil {
		return
	}
	buf := t.storage
	owned := t.owned
	// Clear handle first so double-release is a no-op even if pool work panics.
	t.storage = nil
	t.storageElements = 0
	t.owned = false
	t.offset = 0
	t.shape = nil
	t.strides = nil

	if !owned || p == nil {
		return
	}
	// Only recycle exact capacity slices (no sub-slices / views of larger arrays).
	if cap(buf) != len(buf) || len(buf) == 0 {
		return
	}
	p.mu.Lock()
	if p.free == nil {
		p.free = make(map[int][][]byte)
	}
	p.free[len(buf)] = append(p.free[len(buf)], buf[:len(buf)])
	p.mu.Unlock()
}

// pooledLen reports how many buffers of size nbytes sit in the global free list.
// Intended for tests.
func pooledLen(nbytes int) int {
	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()
	return len(globalPool.free[nbytes])
}
