package runtime

import "fmt"

// Iter is a pull iterator. for-in and streams use this.
type Iter interface {
	// Next returns the next value and true, or false when exhausted.
	Next() (Value, bool)
}

// SliceIter iterates a fixed slice.
type SliceIter struct {
	Items []Value
	i     int
}

func (it *SliceIter) Next() (Value, bool) {
	if it.i >= len(it.Items) {
		return Null(), false
	}
	v := it.Items[it.i]
	it.i++
	return v, true
}

// ChanIter pulls values until the channel is closed.
type ChanIter struct {
	Ch <-chan Value
}

func (it *ChanIter) Next() (Value, bool) {
	v, ok := <-it.Ch
	if !ok {
		return Null(), false
	}
	return v, true
}

// MakeIter wraps an Iter as a Value.
func MakeIter(it Iter) Value {
	return Value{Kind: KindIter, Obj: it}
}

// AsIter extracts an Iter from a Value, or adapts list/map/str.
func AsIter(v Value) (Iter, error) {
	switch v.Kind {
	case KindIter:
		if it, ok := v.Obj.(Iter); ok {
			return it, nil
		}
		return nil, fmt.Errorf("invalid iterator")
	case KindList:
		items := v.Obj.(*ListObj).Items
		cp := make([]Value, len(items))
		copy(cp, items)
		return &SliceIter{Items: cp}, nil
	case KindMap:
		mo := v.Obj.(*MapObj)
		items := make([]Value, len(mo.Keys))
		for i, k := range mo.Keys {
			items[i] = Str(k)
		}
		return &SliceIter{Items: items}, nil
	case KindStr:
		items := make([]Value, 0, len(v.S))
		for _, r := range v.S {
			items = append(items, Str(string(r)))
		}
		return &SliceIter{Items: items}, nil
	default:
		return nil, fmt.Errorf("cannot iterate %s", v.KindName())
	}
}
