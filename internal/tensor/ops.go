package tensor

import (
	"fmt"
	"math"
)

// Add returns element-wise a+b with NumPy-style right-aligned broadcasting.
func Add(a, b *Tensor) (*Tensor, error) {
	return binaryOp(a, b, "add", func(x, y float64) float64 { return x + y })
}

// Sub returns element-wise a-b with broadcasting.
func Sub(a, b *Tensor) (*Tensor, error) {
	return binaryOp(a, b, "sub", func(x, y float64) float64 { return x - y })
}

// Mul returns element-wise a*b with broadcasting.
func Mul(a, b *Tensor) (*Tensor, error) {
	return binaryOp(a, b, "mul", func(x, y float64) float64 { return x * y })
}

// Div returns element-wise a/b with broadcasting.
func Div(a, b *Tensor) (*Tensor, error) {
	return binaryOp(a, b, "div", func(x, y float64) float64 { return x / y })
}

// Sum reduces all elements to a scalar float64 tensor of shape [].
func Sum(t *Tensor) (*Tensor, error) {
	if t == nil {
		return nil, fmt.Errorf("cannot sum a nil tensor")
	}
	if isIntegerDType(t.dtype) {
		if t.dtype == Bool {
			count := int64(0)
			for flat := int64(0); flat < t.Numel(); flat++ {
				value, err := t.Value(unravel(flat, t.shape)...)
				if err != nil {
					return nil, err
				}
				if value.(bool) {
					count++
				}
			}
			return FromList(Int64, []int{}, []any{count})
		}
		var accumulator any = int64(0)
		if t.dtype == UInt64 {
			accumulator = uint64(0)
		}
		for flat := int64(0); flat < t.Numel(); flat++ {
			value, err := t.Value(unravel(flat, t.shape)...)
			if err != nil {
				return nil, err
			}
			var ok bool
			accumulator, ok = exactIntegerBinary(accumulator, value, t.dtype, "add")
			if !ok {
				return nil, fmt.Errorf("integer sum cannot represent dtype %s", t.dtype)
			}
		}
		return FromList(t.dtype, []int{}, []any{accumulator})
	}
	values, err := t.Float64Values()
	if err != nil {
		return nil, err
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return FromFloat64([]int{}, []float64{total})
}

// MatMul multiplies two rank-2 float tensors (M,K)·(K,N) -> (M,N).
func MatMul(a, b *Tensor) (*Tensor, error) {
	if a == nil || b == nil {
		return nil, fmt.Errorf("matmul requires two tensors")
	}
	if len(a.shape) != 2 || len(b.shape) != 2 {
		return nil, fmt.Errorf("matmul currently requires rank-2 tensors")
	}
	if a.shape[1] != b.shape[0] {
		return nil, fmt.Errorf("matmul inner dimensions do not match: %d vs %d", a.shape[1], b.shape[0])
	}
	left, err := a.Contiguous()
	if err != nil {
		return nil, err
	}
	if left != a {
		defer Release(left)
	}
	right, err := b.Contiguous()
	if err != nil {
		return nil, err
	}
	if right != b {
		defer Release(right)
	}
	m, k, n := left.shape[0], left.shape[1], right.shape[1]
	out, err := Acquire(promoteDType(left.dtype, right.dtype), []int{m, n})
	if err != nil {
		return nil, err
	}
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			if isIntegerDType(out.dtype) && out.dtype != Bool {
				var accumulator any = int64(0)
				if out.dtype == UInt64 {
					accumulator = uint64(0)
				}
				for t := 0; t < k; t++ {
					av, err := left.Value(i, t)
					if err != nil {
						Release(out)
						return nil, err
					}
					bv, err := right.Value(t, j)
					if err != nil {
						Release(out)
						return nil, err
					}
					product, ok := exactIntegerBinary(av, bv, out.dtype, "mul")
					if !ok {
						Release(out)
						return nil, fmt.Errorf("integer matmul cannot represent operand dtype %s", out.dtype)
					}
					accumulator, ok = exactIntegerBinary(accumulator, product, out.dtype, "add")
					if !ok {
						Release(out)
						return nil, fmt.Errorf("integer matmul cannot represent result dtype %s", out.dtype)
					}
				}
				if err := out.Set(accumulator, i, j); err != nil {
					Release(out)
					return nil, err
				}
				continue
			}
			acc := 0.0
			for t := 0; t < k; t++ {
				av, err := left.Value(i, t)
				if err != nil {
					return nil, err
				}
				bv, err := right.Value(t, j)
				if err != nil {
					return nil, err
				}
				acc += asFloat64(av) * asFloat64(bv)
			}
			if err := out.Set(castToDType(acc, out.dtype), i, j); err != nil {
				Release(out)
				return nil, err
			}
		}
	}
	return out, nil
}

// FromList builds a contiguous tensor from a flat list of Go values.
// Storage is obtained via Acquire so callers may Release it back to the pool.
func FromList(dtype DType, shape []int, values []any) (*Tensor, error) {
	t, err := Acquire(dtype, shape)
	if err != nil {
		return nil, err
	}
	expected := t.Numel()
	if int64(len(values)) != expected {
		Release(t)
		return nil, fmt.Errorf("tensor value count %d does not match shape element count %d", len(values), expected)
	}
	for i, value := range values {
		// Preserve fixed-width integer values such as uint64 without routing
		// them through float64, which would lose precision before storage.
		if err := t.setElement(int64(i), value); err != nil {
			Release(t)
			return nil, err
		}
	}
	return t, nil
}

// ToList materializes tensor values as a flat Go slice in C-order.
func ToList(t *Tensor) ([]any, error) {
	if t == nil {
		return nil, fmt.Errorf("cannot materialize a nil tensor")
	}
	out := make([]any, 0, t.Numel())
	for flat := int64(0); flat < t.Numel(); flat++ {
		value, err := t.Value(unravel(flat, t.shape)...)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func binaryOp(a, b *Tensor, operation string, op func(float64, float64) float64) (*Tensor, error) {
	if a == nil || b == nil {
		return nil, fmt.Errorf("binary tensor op requires two tensors")
	}
	shape, err := broadcastShape(a.shape, b.shape)
	if err != nil {
		return nil, err
	}
	dtype := promoteDType(a.dtype, b.dtype)
	out, err := Acquire(dtype, shape)
	if err != nil {
		return nil, err
	}
	count, err := elementCount(shape)
	if err != nil {
		return nil, err
	}
	for flat := int64(0); flat < count; flat++ {
		index := unravel(flat, shape)
		av, err := broadcastValue(a, index, shape)
		if err != nil {
			Release(out)
			return nil, err
		}
		bv, err := broadcastValue(b, index, shape)
		if err != nil {
			Release(out)
			return nil, err
		}
		var resultValue any
		if operation != "div" {
			if exact, ok := exactIntegerBinary(av, bv, dtype, operation); ok {
				resultValue = exact
			}
		}
		if resultValue == nil {
			resultValue = castToDType(op(asFloat64(av), asFloat64(bv)), dtype)
		}
		if err := out.setElement(flat, resultValue); err != nil {
			Release(out)
			return nil, err
		}
	}
	return out, nil
}

func exactIntegerBinary(left, right any, dtype DType, operation string) (any, bool) {
	if !isIntegerDType(dtype) {
		return nil, false
	}
	if dtype == UInt64 {
		a, aok := numericUint64(left)
		b, bok := numericUint64(right)
		if !aok || !bok {
			return nil, false
		}
		switch operation {
		case "add":
			return a + b, true
		case "sub":
			return a - b, true
		case "mul":
			return a * b, true
		}
		return nil, false
	}
	a, aok := numericInt64(left)
	b, bok := numericInt64(right)
	if !aok || !bok {
		return nil, false
	}
	var value int64
	switch operation {
	case "add":
		value = a + b
	case "sub":
		value = a - b
	case "mul":
		value = a * b
	default:
		return nil, false
	}
	switch dtype {
	case Bool:
		return value != 0, true
	case Int8:
		return int8(value), true
	case Int16:
		return int16(value), true
	case Int32:
		return int32(value), true
	case Int64:
		return value, true
	case UInt8:
		return uint8(value), true
	case UInt16:
		return uint16(value), true
	case UInt32:
		return uint32(value), true
	default:
		return nil, false
	}
}

func isIntegerDType(dtype DType) bool {
	switch dtype {
	case Bool, Int8, Int16, Int32, Int64, UInt8, UInt16, UInt32, UInt64:
		return true
	default:
		return false
	}
}

func broadcastShape(left, right []int) ([]int, error) {
	li, ri := len(left)-1, len(right)-1
	var rev []int
	for li >= 0 || ri >= 0 {
		ld, rd := 1, 1
		if li >= 0 {
			ld = left[li]
		}
		if ri >= 0 {
			rd = right[ri]
		}
		if ld != rd && ld != 1 && rd != 1 {
			return nil, fmt.Errorf("operands could not be broadcast together: %v vs %v", left, right)
		}
		if ld == rd || rd == 1 {
			rev = append(rev, ld)
		} else {
			rev = append(rev, rd)
		}
		li--
		ri--
	}
	out := make([]int, len(rev))
	for i := range rev {
		out[i] = rev[len(rev)-1-i]
	}
	return out, nil
}

func broadcastValue(t *Tensor, outIndex, outShape []int) (any, error) {
	src := make([]int, len(t.shape))
	offset := len(outShape) - len(t.shape)
	for i := range t.shape {
		oi := outIndex[i+offset]
		if t.shape[i] == 1 {
			src[i] = 0
		} else {
			src[i] = oi
		}
	}
	return t.Value(src...)
}

// promoteDType mirrors NumPy 2.x type promotion (np.promote_types) for the
// supported dtype set: bool, int8-64, uint8-64, float32/64. Keep it in sync
// with _promote_dtype in packages/warp/lib.weft; TestPromoteDTypeNumPyMatrix
// locks the full 11x11 matrix against the NumPy oracle.
func promoteDType(a, b DType) DType {
	if a == b {
		return a
	}
	// float64 dominates everything; float32 survives only against bool and
	// integers of 16 bits or fewer.
	if a == Float64 || b == Float64 {
		return Float64
	}
	if a == Float32 || b == Float32 {
		other := a
		if a == Float32 {
			other = b
		}
		switch other {
		case Int32, Int64, UInt32, UInt64:
			return Float64
		default:
			return Float32
		}
	}
	// bool mixed with a non-float dtype adopts the other operand's dtype.
	if a == Bool {
		return b
	}
	if b == Bool {
		return a
	}
	aSigned := isSignedIntegerDType(a)
	bSigned := isSignedIntegerDType(b)
	if aSigned == bSigned {
		if integerDTypeBits(a) >= integerDTypeBits(b) {
			return a
		}
		return b
	}
	// Mixed sign: the result is the smallest signed dtype wide enough to hold
	// both ranges. A 64-bit unsigned paired with any signed integer overflows
	// int64 and promotes to float64.
	signed, unsigned := a, b
	if !aSigned {
		signed, unsigned = b, a
	}
	need := integerDTypeBits(signed)
	if w := 2 * integerDTypeBits(unsigned); w > need {
		need = w
	}
	switch {
	case need <= 16:
		return Int16
	case need <= 32:
		return Int32
	case need <= 64:
		return Int64
	default:
		return Float64
	}
}

// integerDTypeBits reports the storage width in bits of an integer dtype.
func integerDTypeBits(d DType) int {
	switch d {
	case Int8, UInt8:
		return 8
	case Int16, UInt16:
		return 16
	case Int32, UInt32:
		return 32
	default:
		return 64
	}
}

func isSignedIntegerDType(dtype DType) bool {
	switch dtype {
	case Int8, Int16, Int32, Int64:
		return true
	default:
		return false
	}
}

func asFloat64(value any) float64 {
	if f, ok := numericToFloat64(value); ok {
		return f
	}
	return math.NaN()
}

func castToDType(value any, dtype DType) any {
	f := asFloat64(value)
	switch dtype {
	case Bool:
		return f != 0
	case Int8:
		return int8(truncTowardZero(f))
	case Int16:
		return int16(truncTowardZero(f))
	case Int32:
		return int32(truncTowardZero(f))
	case Int64:
		return truncTowardZero(f)
	case UInt8:
		if f < 0 {
			return uint8(0)
		}
		return uint8(truncTowardZero(f))
	case UInt16:
		if f < 0 {
			return uint16(0)
		}
		return uint16(truncTowardZero(f))
	case UInt32:
		if f < 0 {
			return uint32(0)
		}
		return uint32(truncTowardZero(f))
	case UInt64:
		if f < 0 {
			return uint64(0)
		}
		return uint64(truncTowardZero(f))
	case Float32:
		return float32(f)
	default:
		return f
	}
}

func truncTowardZero(f float64) int64 {
	if f < 0 {
		return int64(math.Ceil(f))
	}
	return int64(math.Floor(f))
}
