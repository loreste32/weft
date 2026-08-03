// Package tensor provides the typed, strided storage primitive used by the
// numerical compatibility layers. It is deliberately independent of the
// Weft VM so native providers and future runtime intrinsics can share the same
// shape, dtype, view, and bounds contracts.
package tensor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

type DType string

const (
	Bool    DType = "bool"
	Int64   DType = "int64"
	Float32 DType = "float32"
	Float64 DType = "float64"
)

var (
	ErrInvalidShape  = errors.New("tensor shape must contain non-negative dimensions")
	ErrOutOfBounds   = errors.New("tensor index is out of bounds")
	ErrNotContiguous = errors.New("tensor operation requires C-contiguous storage")
)

func (d DType) ItemSize() int {
	switch d {
	case Bool:
		return 1
	case Int64, Float64:
		return 8
	case Float32:
		return 4
	default:
		return 0
	}
}

func (d DType) Valid() bool { return d.ItemSize() != 0 }

func ParseDType(name string) (DType, error) {
	d := DType(name)
	if !d.Valid() {
		return "", fmt.Errorf("unsupported tensor dtype %q", name)
	}
	return d, nil
}

type Tensor struct {
	dtype           DType
	shape           []int
	strides         []int64 // strides are measured in elements, not bytes
	offset          int64
	storage         []byte
	storageElements int64
}

func New(dtype DType, shape []int) (*Tensor, error) {
	if !dtype.Valid() {
		return nil, fmt.Errorf("unsupported tensor dtype %q", dtype)
	}
	count, err := elementCount(shape)
	if err != nil {
		return nil, err
	}
	bytes, ok := checkedBytes(count, dtype.ItemSize())
	if !ok {
		return nil, errors.New("tensor allocation is too large")
	}
	return &Tensor{
		dtype:           dtype,
		shape:           cloneInts(shape),
		strides:         contiguousStrides(shape),
		storage:         make([]byte, bytes),
		storageElements: count,
	}, nil
}

func FromBytes(dtype DType, shape []int, data []byte) (*Tensor, error) {
	t, err := New(dtype, shape)
	if err != nil {
		return nil, err
	}
	if len(data) != len(t.storage) {
		return nil, fmt.Errorf("tensor byte length %d does not match shape/dtype storage %d", len(data), len(t.storage))
	}
	copy(t.storage, data)
	return t, nil
}

func FromFloat64(shape []int, values []float64) (*Tensor, error) {
	t, err := New(Float64, shape)
	if err != nil {
		return nil, err
	}
	if len(values) != int(t.storageElements) {
		return nil, fmt.Errorf("tensor value count %d does not match shape element count %d", len(values), t.storageElements)
	}
	for i, value := range values {
		if err := t.setElement(int64(i), value); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (t *Tensor) DType() DType { return t.dtype }

func (t *Tensor) Shape() []int { return cloneInts(t.shape) }

func (t *Tensor) Strides() []int64 { return append([]int64(nil), t.strides...) }

func (t *Tensor) Offset() int64 { return t.offset }

func (t *Tensor) Numel() int64 {
	if t == nil {
		return 0
	}
	count, _ := elementCount(t.shape)
	return count
}

func (t *Tensor) IsContiguous() bool {
	if t == nil || t.offset < 0 {
		return false
	}
	want := contiguousStrides(t.shape)
	if len(want) != len(t.strides) {
		return false
	}
	for i := range want {
		if want[i] != t.strides[i] {
			return false
		}
	}
	return true
}

// View creates a zero-copy strided view over the tensor's storage. Strides
// are expressed in elements and may be negative. The addressed region is
// checked before the view is returned.
func (t *Tensor) View(shape []int, strides []int64, offset int64) (*Tensor, error) {
	if t == nil {
		return nil, errors.New("cannot view a nil tensor")
	}
	if _, err := elementCount(shape); err != nil {
		return nil, err
	}
	if len(shape) != len(strides) {
		return nil, errors.New("tensor view shape and strides must have equal rank")
	}
	minIndex, maxIndex, hasElements, ok := addressedBounds(shape, strides, offset)
	if !ok || (!hasElements && (offset < 0 || offset > t.storageElements)) ||
		(hasElements && (minIndex < 0 || maxIndex >= t.storageElements)) {
		return nil, ErrOutOfBounds
	}
	return &Tensor{
		dtype:           t.dtype,
		shape:           cloneInts(shape),
		strides:         append([]int64(nil), strides...),
		offset:          offset,
		storage:         t.storage,
		storageElements: t.storageElements,
	}, nil
}

func (t *Tensor) Reshape(shape []int) (*Tensor, error) {
	if t == nil {
		return nil, errors.New("cannot reshape a nil tensor")
	}
	count, err := elementCount(shape)
	if err != nil {
		return nil, err
	}
	if count != t.Numel() {
		return nil, errors.New("tensor reshape must preserve element count")
	}
	if !t.IsContiguous() {
		return nil, ErrNotContiguous
	}
	return t.View(shape, contiguousStrides(shape), t.offset)
}

func (t *Tensor) Transpose(axes []int) (*Tensor, error) {
	if t == nil {
		return nil, errors.New("cannot transpose a nil tensor")
	}
	if len(axes) != len(t.shape) {
		return nil, errors.New("transpose axes must contain one entry per dimension")
	}
	shape := make([]int, len(axes))
	strides := make([]int64, len(axes))
	seen := make([]bool, len(axes))
	for i, axis := range axes {
		if axis < 0 {
			axis += len(axes)
		}
		if axis < 0 || axis >= len(axes) || seen[axis] {
			return nil, errors.New("transpose axes must be a permutation")
		}
		seen[axis] = true
		shape[i] = t.shape[axis]
		strides[i] = t.strides[axis]
	}
	return t.View(shape, strides, t.offset)
}

func (t *Tensor) BroadcastTo(shape []int) (*Tensor, error) {
	if t == nil {
		return nil, errors.New("cannot broadcast a nil tensor")
	}
	if _, err := elementCount(shape); err != nil {
		return nil, err
	}
	if len(shape) < len(t.shape) {
		return nil, errors.New("broadcast target rank is smaller than source rank")
	}
	strides := make([]int64, len(shape))
	offset := len(shape) - len(t.shape)
	for i, target := range shape {
		if i < offset {
			if target < 0 {
				return nil, ErrInvalidShape
			}
			strides[i] = 0
			continue
		}
		sourceDim := t.shape[i-offset]
		if sourceDim != target && sourceDim != 1 {
			return nil, fmt.Errorf("cannot broadcast dimension %d to %d", sourceDim, target)
		}
		strides[i] = t.strides[i-offset]
		if sourceDim == 1 && target != 1 {
			strides[i] = 0
		}
	}
	return t.View(shape, strides, t.offset)
}

func (t *Tensor) Value(indices ...int) (any, error) {
	position, err := t.position(indices)
	if err != nil {
		return nil, err
	}
	offset := int(position) * t.dtype.ItemSize()
	switch t.dtype {
	case Bool:
		return t.storage[offset] != 0, nil
	case Int64:
		return int64(binary.LittleEndian.Uint64(t.storage[offset : offset+8])), nil
	case Float32:
		return math.Float32frombits(binary.LittleEndian.Uint32(t.storage[offset : offset+4])), nil
	case Float64:
		return math.Float64frombits(binary.LittleEndian.Uint64(t.storage[offset : offset+8])), nil
	default:
		return nil, fmt.Errorf("unsupported tensor dtype %q", t.dtype)
	}
}

func (t *Tensor) Set(value any, indices ...int) error {
	position, err := t.position(indices)
	if err != nil {
		return err
	}
	for i, stride := range t.strides {
		if stride == 0 && t.shape[i] > 1 {
			return errors.New("cannot write through a broadcast tensor view")
		}
	}
	return t.setElement(position, value)
}

func (t *Tensor) Bytes() []byte { return append([]byte(nil), t.storage...) }

func (t *Tensor) Contiguous() (*Tensor, error) {
	if t == nil {
		return nil, errors.New("cannot copy a nil tensor")
	}
	if t.IsContiguous() && t.offset == 0 {
		return t, nil
	}
	result, err := New(t.dtype, t.shape)
	if err != nil {
		return nil, err
	}
	for flat := int64(0); flat < t.Numel(); flat++ {
		indices := unravel(flat, t.shape)
		value, err := t.Value(indices...)
		if err != nil {
			return nil, err
		}
		if err := result.setElement(flat, value); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (t *Tensor) Float64Values() ([]float64, error) {
	values := make([]float64, 0, t.Numel())
	for flat := int64(0); flat < t.Numel(); flat++ {
		value, err := t.Value(unravel(flat, t.shape)...)
		if err != nil {
			return nil, err
		}
		switch n := value.(type) {
		case bool:
			if n {
				values = append(values, 1)
			} else {
				values = append(values, 0)
			}
		case int64:
			values = append(values, float64(n))
		case float32:
			values = append(values, float64(n))
		case float64:
			values = append(values, n)
		default:
			return nil, fmt.Errorf("tensor value %T is not numeric", value)
		}
	}
	return values, nil
}

func (t *Tensor) position(indices []int) (int64, error) {
	if t == nil {
		return 0, errors.New("cannot index a nil tensor")
	}
	if len(indices) != len(t.shape) {
		return 0, fmt.Errorf("tensor index rank %d does not match tensor rank %d", len(indices), len(t.shape))
	}
	position := t.offset
	for i, index := range indices {
		if index < 0 {
			index += t.shape[i]
		}
		if index < 0 || index >= t.shape[i] {
			return 0, ErrOutOfBounds
		}
		term, ok := safeMulInt64(int64(index), t.strides[i])
		if !ok {
			return 0, ErrOutOfBounds
		}
		position, ok = safeAddInt64(position, term)
		if !ok {
			return 0, ErrOutOfBounds
		}
	}
	if position < 0 || position >= t.storageElements {
		return 0, ErrOutOfBounds
	}
	return position, nil
}

func (t *Tensor) setElement(position int64, value any) error {
	if position < 0 || position >= t.storageElements {
		return ErrOutOfBounds
	}
	offset := int(position) * t.dtype.ItemSize()
	switch t.dtype {
	case Bool:
		v, ok := value.(bool)
		if !ok {
			return fmt.Errorf("cannot store %T in bool tensor", value)
		}
		if v {
			t.storage[offset] = 1
		} else {
			t.storage[offset] = 0
		}
	case Int64:
		v, ok := numericInt64(value)
		if !ok {
			return fmt.Errorf("cannot store %T in int64 tensor", value)
		}
		binary.LittleEndian.PutUint64(t.storage[offset:offset+8], uint64(v))
	case Float32:
		v, ok := numericFloat64(value)
		if !ok {
			return fmt.Errorf("cannot store %T in float32 tensor", value)
		}
		binary.LittleEndian.PutUint32(t.storage[offset:offset+4], math.Float32bits(float32(v)))
	case Float64:
		v, ok := numericFloat64(value)
		if !ok {
			return fmt.Errorf("cannot store %T in float64 tensor", value)
		}
		binary.LittleEndian.PutUint64(t.storage[offset:offset+8], math.Float64bits(v))
	default:
		return fmt.Errorf("unsupported tensor dtype %q", t.dtype)
	}
	return nil
}

func numericInt64(value any) (int64, bool) {
	switch n := value.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		return int64(n), uint64(n) <= math.MaxInt64
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		return int64(n), n <= math.MaxInt64
	default:
		return 0, false
	}
}

func numericFloat64(value any) (float64, bool) {
	switch n := value.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func elementCount(shape []int) (int64, error) {
	count := int64(1)
	zero := false
	for _, dim := range shape {
		if dim < 0 {
			return 0, ErrInvalidShape
		}
		if dim == 0 {
			zero = true
			continue
		}
		if zero {
			continue
		}
		if count > math.MaxInt64/int64(dim) {
			return 0, errors.New("tensor shape element count overflows int64")
		}
		count *= int64(dim)
	}
	if zero {
		return 0, nil
	}
	return count, nil
}

func checkedBytes(elements int64, itemSize int) (int, bool) {
	if elements < 0 || itemSize <= 0 || elements > math.MaxInt64/int64(itemSize) {
		return 0, false
	}
	bytes := elements * int64(itemSize)
	if bytes > int64(maxInt()) {
		return 0, false
	}
	return int(bytes), true
}

func addressedBounds(shape []int, strides []int64, offset int64) (int64, int64, bool, bool) {
	minIndex, maxIndex := offset, offset
	count, err := elementCount(shape)
	if err != nil {
		return 0, 0, false, false
	}
	if count == 0 {
		return offset, offset, false, true
	}
	for i, dim := range shape {
		if dim <= 1 {
			continue
		}
		span, ok := safeMulInt64(int64(dim-1), strides[i])
		if !ok {
			return 0, 0, false, false
		}
		if span < 0 {
			minIndex, ok = safeAddInt64(minIndex, span)
		} else {
			maxIndex, ok = safeAddInt64(maxIndex, span)
		}
		if !ok {
			return 0, 0, false, false
		}
	}
	return minIndex, maxIndex, true, true
}

func contiguousStrides(shape []int) []int64 {
	strides := make([]int64, len(shape))
	stride := int64(1)
	for i := len(shape) - 1; i >= 0; i-- {
		strides[i] = stride
		if shape[i] > 0 {
			stride *= int64(shape[i])
		}
	}
	return strides
}

func unravel(flat int64, shape []int) []int {
	result := make([]int, len(shape))
	remaining := flat
	for i := range shape {
		stride := int64(1)
		for j := i + 1; j < len(shape); j++ {
			stride *= int64(shape[j])
		}
		if stride != 0 {
			result[i] = int(remaining / stride)
			remaining %= stride
		}
	}
	return result
}

func cloneInts(values []int) []int { return append([]int(nil), values...) }

func maxInt() int {
	return int(^uint(0) >> 1)
}

func safeMulInt64(left, right int64) (int64, bool) {
	if left == 0 || right == 0 {
		return 0, true
	}
	if left == math.MinInt64 && right == -1 || right == math.MinInt64 && left == -1 {
		return 0, false
	}
	result := left * right
	if result/right != left {
		return 0, false
	}
	return result, true
}

func safeAddInt64(left, right int64) (int64, bool) {
	result := left + right
	if (right > 0 && result < left) || (right < 0 && result > left) {
		return 0, false
	}
	return result, true
}
