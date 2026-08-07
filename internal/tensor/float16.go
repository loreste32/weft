package tensor

import "math"

// IEEE 754 binary16 (half-precision) conversions. Float16FromFloat64 rounds
// to nearest, ties to even, and handles subnormals, infinities, and NaN,
// producing the same bit pattern as NumPy 2.x np.float16(x). Overflow yields
// ±Inf (NumPy emits a RuntimeWarning rather than an error in this case) and
// magnitudes below half of the smallest subnormal flush to zero.

// Float16FromFloat64 converts a float64 to its binary16 bit pattern.
func Float16FromFloat64(f float64) uint16 {
	bits := math.Float64bits(f)
	sign := uint16(bits>>63) << 15
	exp := int(bits>>52) & 0x7ff
	mant := bits & 0xfffffffffffff
	if exp == 0x7ff {
		if mant != 0 {
			return sign | 0x7e00 // NaN (quieted)
		}
		return sign | 0x7c00 // ±Inf
	}
	if exp == 0 {
		return sign // float64 subnormals are far below the half range
	}
	e := exp - 1023 // unbiased exponent
	sig := mant | (uint64(1) << 52)
	if e > 15 {
		return sign | 0x7c00 // magnitude >= 2^16 always overflows to Inf
	}
	if e >= -14 {
		// Normal half: keep the implicit bit plus 10 mantissa bits.
		keep := roundToNearestEven(sig>>42, sig&((uint64(1)<<42)-1), 42)
		if keep == 0x800 { // mantissa carry: 1.111...1 rounds up to 2.0
			keep = 0x400
			e++
			if e > 15 {
				return sign | 0x7c00
			}
		}
		return sign | uint16(e+15)<<10 | uint16(keep&0x3ff)
	}
	if e < -25 {
		return sign // below half of the smallest subnormal (2^-25 midpoint)
	}
	// Subnormal half: the quantum is 2^-24, so the mantissa is the value
	// scaled by 2^(24+e-52)... i.e. sig shifted right by 28-e bits.
	shift := uint(28 - e)
	keep := roundToNearestEven(sig>>shift, sig&((uint64(1)<<shift)-1), shift)
	return sign | uint16(keep)
}

// roundToNearestEven rounds the significand split into kept bits and a
// remainder of the given shift width, breaking ties toward the even keep.
func roundToNearestEven(keep, rem uint64, shift uint) uint64 {
	halfway := uint64(1) << (shift - 1)
	if rem > halfway || (rem == halfway && keep&1 == 1) {
		keep++
	}
	return keep
}

// Float16ToFloat64 widens a binary16 bit pattern to float64. The conversion
// is exact: every half value is representable as a float64.
func Float16ToFloat64(bits uint16) float64 {
	exp := (bits >> 10) & 0x1f
	mant := uint64(bits & 0x3ff)
	var value float64
	switch exp {
	case 0x1f:
		if mant != 0 {
			return math.Float64frombits(uint64(bits>>15)<<63 | 0x7ff<<52 | mant<<42)
		}
		return math.Float64frombits(uint64(bits>>15)<<63 | 0x7ff<<52)
	case 0:
		value = math.Ldexp(float64(mant), -24) // subnormal or zero
	default:
		value = math.Ldexp(float64(mant|0x400), int(exp)-25)
	}
	if bits>>15 == 1 {
		return -value
	}
	return value
}
