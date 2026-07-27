package stdlib

import (
	"github.com/loreste/weft/internal/runtime"
)

// packageBisect — binary search on sorted lists (Python bisect).
func packageBisect() runtime.Value {
	p := pkg()

	// bisect.left(list, x) -> insertion index to keep sorted (leftmost)
	set(p, "left", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("bisect.left(list, x)", "bisect"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		x := args[1]
		lo, hi := 0, len(items)
		for lo < hi {
			mid := (lo + hi) / 2
			if compareValues(items[mid], x) < 0 {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		return runtime.Int(int64(lo)), nil
	}, 2)

	// bisect.right(list, x) -> insertion index (rightmost)
	set(p, "right", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("bisect.right(list, x)", "bisect"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		x := args[1]
		lo, hi := 0, len(items)
		for lo < hi {
			mid := (lo + hi) / 2
			if compareValues(items[mid], x) <= 0 {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		return runtime.Int(int64(lo)), nil
	}, 2)

	// bisect.insort(list, x) -> new list with x inserted sorted (stable right)
	set(p, "insort", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("bisect.insort(list, x)", "bisect"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		x := args[1]
		lo, hi := 0, len(items)
		for lo < hi {
			mid := (lo + hi) / 2
			if compareValues(items[mid], x) <= 0 {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		out := make([]runtime.Value, 0, len(items)+1)
		out = append(out, items[:lo]...)
		out = append(out, x)
		out = append(out, items[lo:]...)
		return runtime.List(out...), nil
	}, 2)

	return p
}

// compareValues: <0 if a<b, 0 equal, >0 if a>b (numbers/strings/bools).
func compareValues(a, b runtime.Value) int {
	if a.Kind == runtime.KindInt && b.Kind == runtime.KindInt {
		switch {
		case a.I < b.I:
			return -1
		case a.I > b.I:
			return 1
		default:
			return 0
		}
	}
	if af, aok := asFloat64(a); aok {
		if bf, bok := asFloat64(b); bok {
			switch {
			case af < bf:
				return -1
			case af > bf:
				return 1
			default:
				return 0
			}
		}
	}
	as, bs := a.String(), b.String()
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	default:
		return 0
	}
}
