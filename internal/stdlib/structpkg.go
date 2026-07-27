package stdlib

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/loreste/weft/internal/runtime"
)

// packageStruct — binary pack/unpack (Python struct lite, big/little endian).
// Formats: x pad, c char, b/B int8, h/H int16, i/I/l/L int32, q/Q int64, f float32, d float64, s string (length prefix via Ns)
func packageStruct() runtime.Value {
	p := pkg()

	// struct.pack(layout, ...values) -> Result[str]  (bytes as latin-1 string)
	set(p, "pack", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("struct.pack(layout, ...)", "struct"), nil
		}
		b, err := structPack(args[0].String(), args[1:])
		if err != nil {
			return errRes(err.Error(), "struct"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, -1)

	// struct.unpack(layout, data) -> Result[list]
	set(p, "unpack", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("struct.unpack(layout, data)", "struct"), nil
		}
		vals, err := structUnpack(args[0].String(), []byte(args[1].String()))
		if err != nil {
			return errRes(err.Error(), "struct"), nil
		}
		return runtime.Ok(runtime.List(vals...)), nil
	}, 2)

	// struct.size(layout) -> Result[int]
	set(p, "size", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Int(0), nil
		}
		n, err := structSize(args[0].String())
		if err != nil {
			return errRes(err.Error(), "struct"), nil
		}
		return runtime.Ok(runtime.Int(int64(n))), nil
	}, 1)

	return p
}

func structOrder(layout string) (binary.ByteOrder, string) {
	if len(layout) == 0 {
		return binary.BigEndian, layout
	}
	switch layout[0] {
	case '<':
		return binary.LittleEndian, layout[1:]
	case '>', '!':
		return binary.BigEndian, layout[1:]
	case '=':
		return binary.NativeEndian, layout[1:]
	default:
		return binary.BigEndian, layout
	}
}

type structField struct {
	kind byte
	n    int // for s: length; for x: pad count
}

func parseStructFmt(layout string) (binary.ByteOrder, []structField, error) {
	order, body := structOrder(layout)
	var fields []structField
	i := 0
	for i < len(body) {
		// optional count
		n := 0
		hasN := false
		for i < len(body) && body[i] >= '0' && body[i] <= '9' {
			hasN = true
			n = n*10 + int(body[i]-'0')
			i++
		}
		if i >= len(body) {
			return order, nil, fmt.Errorf("struct: trailing count")
		}
		k := body[i]
		i++
		if !hasN {
			n = 1
		}
		switch k {
		case 'x', 'c', 'b', 'B', 'h', 'H', 'i', 'I', 'l', 'L', 'q', 'Q', 'f', 'd':
			for j := 0; j < n; j++ {
				fields = append(fields, structField{kind: k, n: 1})
			}
		case 's':
			fields = append(fields, structField{kind: 's', n: n})
		default:
			return order, nil, fmt.Errorf("struct: unknown format %q", string(k))
		}
	}
	return order, fields, nil
}

func structSize(layout string) (int, error) {
	_, fields, err := parseStructFmt(layout)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range fields {
		switch f.kind {
		case 'x', 'c', 'b', 'B':
			n++
		case 'h', 'H':
			n += 2
		case 'i', 'I', 'l', 'L', 'f':
			n += 4
		case 'q', 'Q', 'd':
			n += 8
		case 's':
			n += f.n
		}
	}
	return n, nil
}

func structPack(layout string, vals []runtime.Value) ([]byte, error) {
	order, fields, err := parseStructFmt(layout)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	vi := 0
	for _, f := range fields {
		switch f.kind {
		case 'x':
			b.WriteByte(0)
		case 'c', 'b':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			b.WriteByte(byte(int8(n)))
		case 'B':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			b.WriteByte(byte(n))
		case 'h':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [2]byte
			order.PutUint16(buf[:], uint16(int16(n)))
			b.Write(buf[:])
		case 'H':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [2]byte
			order.PutUint16(buf[:], uint16(n))
			b.Write(buf[:])
		case 'i', 'l':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [4]byte
			order.PutUint32(buf[:], uint32(int32(n)))
			b.Write(buf[:])
		case 'I', 'L':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [4]byte
			order.PutUint32(buf[:], uint32(n))
			b.Write(buf[:])
		case 'q':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [8]byte
			order.PutUint64(buf[:], uint64(n))
			b.Write(buf[:])
		case 'Q':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [8]byte
			order.PutUint64(buf[:], uint64(n))
			b.Write(buf[:])
		case 'f':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			x, _ := asFloat64(vals[vi])
			vi++
			var buf [4]byte
			order.PutUint32(buf[:], math.Float32bits(float32(x)))
			b.Write(buf[:])
		case 'd':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			x, _ := asFloat64(vals[vi])
			vi++
			var buf [8]byte
			order.PutUint64(buf[:], math.Float64bits(x))
			b.Write(buf[:])
		case 's':
			if vi >= len(vals) {
				return nil, fmt.Errorf("struct.pack: not enough values")
			}
			s := vals[vi].String()
			vi++
			buf := make([]byte, f.n)
			copy(buf, []byte(s))
			b.Write(buf)
		}
	}
	return b.Bytes(), nil
}

func structUnpack(layout string, data []byte) ([]runtime.Value, error) {
	order, fields, err := parseStructFmt(layout)
	if err != nil {
		return nil, err
	}
	need, _ := structSize(layout)
	if len(data) < need {
		return nil, fmt.Errorf("struct.unpack: need %d bytes, got %d", need, len(data))
	}
	var out []runtime.Value
	off := 0
	for _, f := range fields {
		switch f.kind {
		case 'x':
			off++
		case 'c', 'b':
			out = append(out, runtime.Int(int64(int8(data[off]))))
			off++
		case 'B':
			out = append(out, runtime.Int(int64(data[off])))
			off++
		case 'h':
			out = append(out, runtime.Int(int64(int16(order.Uint16(data[off:])))))
			off += 2
		case 'H':
			out = append(out, runtime.Int(int64(order.Uint16(data[off:]))))
			off += 2
		case 'i', 'l':
			out = append(out, runtime.Int(int64(int32(order.Uint32(data[off:])))))
			off += 4
		case 'I', 'L':
			out = append(out, runtime.Int(int64(order.Uint32(data[off:]))))
			off += 4
		case 'q':
			out = append(out, runtime.Int(int64(order.Uint64(data[off:]))))
			off += 8
		case 'Q':
			out = append(out, runtime.Int(int64(order.Uint64(data[off:]))))
			off += 8
		case 'f':
			out = append(out, runtime.Float(float64(math.Float32frombits(order.Uint32(data[off:])))))
			off += 4
		case 'd':
			out = append(out, runtime.Float(math.Float64frombits(order.Uint64(data[off:]))))
			off += 8
		case 's':
			out = append(out, runtime.Str(string(data[off:off+f.n])))
			off += f.n
		}
	}
	return out, nil
}
