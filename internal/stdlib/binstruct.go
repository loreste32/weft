package stdlib

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/loreste/weft/internal/runtime"
)

// packageBinstruct — binary pack/unpack (layout string, endian prefix).
// Codes: x pad, c/b/B 8-bit, h/H 16, i/I/l/L 32, q/Q 64, f/d float, Ns string.
func packageBinstruct() runtime.Value {
	p := pkg()
	set(p, "pack", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("binstruct.pack(layout, ...)", "binstruct"), nil
		}
		b, err := bsPack(args[0].String(), args[1:])
		if err != nil {
			return errRes(err.Error(), "binstruct"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, -1)
	set(p, "unpack", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("binstruct.unpack(layout, data)", "binstruct"), nil
		}
		vals, err := bsUnpack(args[0].String(), []byte(args[1].String()))
		if err != nil {
			return errRes(err.Error(), "binstruct"), nil
		}
		return runtime.Ok(runtime.List(vals...)), nil
	}, 2)
	set(p, "size", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("binstruct.size(layout)", "binstruct"), nil
		}
		n, err := bsSize(args[0].String())
		if err != nil {
			return errRes(err.Error(), "binstruct"), nil
		}
		return runtime.Ok(runtime.Int(int64(n))), nil
	}, 1)
	return p
}

type bsField struct {
	kind byte
	n    int
}

func bsOrder(layout string) (binary.ByteOrder, string) {
	if layout == "" {
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

func bsParse(layout string) (binary.ByteOrder, []bsField, error) {
	order, body := bsOrder(layout)
	var fields []bsField
	i := 0
	for i < len(body) {
		n, hasN := 0, false
		for i < len(body) && body[i] >= '0' && body[i] <= '9' {
			hasN = true
			n = n*10 + int(body[i]-'0')
			i++
		}
		if i >= len(body) {
			return order, nil, fmt.Errorf("binstruct: trailing count")
		}
		k := body[i]
		i++
		if !hasN {
			n = 1
		}
		switch k {
		case 'x', 'c', 'b', 'B', 'h', 'H', 'i', 'I', 'l', 'L', 'q', 'Q', 'f', 'd':
			for j := 0; j < n; j++ {
				fields = append(fields, bsField{kind: k, n: 1})
			}
		case 's':
			fields = append(fields, bsField{kind: 's', n: n})
		default:
			return order, nil, fmt.Errorf("binstruct: bad code %q", string(k))
		}
	}
	return order, fields, nil
}

func bsSize(layout string) (int, error) {
	_, fields, err := bsParse(layout)
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

func bsPack(layout string, vals []runtime.Value) ([]byte, error) {
	order, fields, err := bsParse(layout)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	vi := 0
	need := func() error {
		if vi >= len(vals) {
			return fmt.Errorf("binstruct.pack: not enough values")
		}
		return nil
	}
	for _, f := range fields {
		switch f.kind {
		case 'x':
			b.WriteByte(0)
		case 'c', 'b':
			if err := need(); err != nil {
				return nil, err
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			b.WriteByte(byte(int8(n)))
		case 'B':
			if err := need(); err != nil {
				return nil, err
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			b.WriteByte(byte(n))
		case 'h':
			if err := need(); err != nil {
				return nil, err
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [2]byte
			order.PutUint16(buf[:], uint16(int16(n)))
			b.Write(buf[:])
		case 'H':
			if err := need(); err != nil {
				return nil, err
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [2]byte
			order.PutUint16(buf[:], uint16(n))
			b.Write(buf[:])
		case 'i', 'l':
			if err := need(); err != nil {
				return nil, err
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [4]byte
			order.PutUint32(buf[:], uint32(int32(n)))
			b.Write(buf[:])
		case 'I', 'L':
			if err := need(); err != nil {
				return nil, err
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [4]byte
			order.PutUint32(buf[:], uint32(n))
			b.Write(buf[:])
		case 'q', 'Q':
			if err := need(); err != nil {
				return nil, err
			}
			n, _ := runtime.AsInt(vals[vi])
			vi++
			var buf [8]byte
			order.PutUint64(buf[:], uint64(n))
			b.Write(buf[:])
		case 'f':
			if err := need(); err != nil {
				return nil, err
			}
			x, _ := asFloat64(vals[vi])
			vi++
			var buf [4]byte
			order.PutUint32(buf[:], math.Float32bits(float32(x)))
			b.Write(buf[:])
		case 'd':
			if err := need(); err != nil {
				return nil, err
			}
			x, _ := asFloat64(vals[vi])
			vi++
			var buf [8]byte
			order.PutUint64(buf[:], math.Float64bits(x))
			b.Write(buf[:])
		case 's':
			if err := need(); err != nil {
				return nil, err
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

func bsUnpack(layout string, data []byte) ([]runtime.Value, error) {
	order, fields, err := bsParse(layout)
	if err != nil {
		return nil, err
	}
	need, _ := bsSize(layout)
	if len(data) < need {
		return nil, fmt.Errorf("binstruct.unpack: need %d bytes, got %d", need, len(data))
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
		case 'q', 'Q':
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
