package stdlib

import (
	"github.com/loreste/weft/internal/runtime"
)

// packageIter — itertools lite (lists in, lists out; no lazy iterators for v1 simplicity).
func packageIter() runtime.Value {
	p := pkg()

	// iter.chain(list, ...) -> list
	set(p, "chain", func(args []runtime.Value) (runtime.Value, error) {
		var out []runtime.Value
		for _, a := range args {
			if a.Kind != runtime.KindList {
				return errRes("iter.chain: all args must be lists", "iter"), nil
			}
			out = append(out, a.Obj.(*runtime.ListObj).Items...)
		}
		return runtime.List(out...), nil
	}, -1)

	// iter.islice(list, start, end?) -> list  [start:end]
	set(p, "islice", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("iter.islice(list, start, end?)", "iter"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		n := int64(len(items))
		start, _ := runtime.AsInt(args[1])
		end := n
		if len(args) >= 3 {
			if e, err := runtime.AsInt(args[2]); err == nil {
				end = e
			}
		}
		if start < 0 {
			start = 0
		}
		if end > n {
			end = n
		}
		if start > end {
			return runtime.List(), nil
		}
		return runtime.List(items[start:end]...), nil
	}, 3)

	// iter.take(list, n) / iter.drop(list, n)
	set(p, "take", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("iter.take(list, n)", "iter"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		n, _ := runtime.AsInt(args[1])
		if n < 0 {
			n = 0
		}
		if n > int64(len(items)) {
			n = int64(len(items))
		}
		return runtime.List(items[:n]...), nil
	}, 2)

	set(p, "drop", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("iter.drop(list, n)", "iter"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		n, _ := runtime.AsInt(args[1])
		if n < 0 {
			n = 0
		}
		if n >= int64(len(items)) {
			return runtime.List(), nil
		}
		return runtime.List(items[n:]...), nil
	}, 2)

	// iter.repeat(x, n) -> [x]*n
	set(p, "repeat", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("iter.repeat(x, n)", "iter"), nil
		}
		n, _ := runtime.AsInt(args[1])
		if n < 0 {
			n = 0
		}
		if n > 1_000_000 {
			return errRes("iter.repeat: n too large", "iter"), nil
		}
		out := make([]runtime.Value, n)
		for i := range out {
			out[i] = args[0]
		}
		return runtime.List(out...), nil
	}, 2)

	// iter.cycle(list, n) -> list  repeat list n times (total len = len*n)
	set(p, "cycle", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("iter.cycle(list, times)", "iter"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		times, _ := runtime.AsInt(args[1])
		if times < 0 {
			times = 0
		}
		if int64(len(items))*times > 1_000_000 {
			return errRes("iter.cycle: result too large", "iter"), nil
		}
		out := make([]runtime.Value, 0, len(items)*int(times))
		for i := int64(0); i < times; i++ {
			out = append(out, items...)
		}
		return runtime.List(out...), nil
	}, 2)

	// iter.chunk(list, size) -> [[...], ...]
	set(p, "chunk", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("iter.chunk(list, size)", "iter"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		size, _ := runtime.AsInt(args[1])
		if size < 1 {
			return errRes("iter.chunk: size >= 1", "iter"), nil
		}
		var out []runtime.Value
		for i := 0; i < len(items); i += int(size) {
			j := i + int(size)
			if j > len(items) {
				j = len(items)
			}
			out = append(out, runtime.List(items[i:j]...))
		}
		return runtime.List(out...), nil
	}, 2)

	// iter.windows(list, size) -> sliding windows
	set(p, "windows", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("iter.windows(list, size)", "iter"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		size, _ := runtime.AsInt(args[1])
		if size < 1 {
			return errRes("iter.windows: size >= 1", "iter"), nil
		}
		if int(size) > len(items) {
			return runtime.List(), nil
		}
		out := make([]runtime.Value, 0, len(items)-int(size)+1)
		for i := 0; i+int(size) <= len(items); i++ {
			out = append(out, runtime.List(items[i:i+int(size)]...))
		}
		return runtime.List(out...), nil
	}, 2)

	// iter.product(a, b) -> cartesian pairs [[a0,b0], ...]
	set(p, "product", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList || args[1].Kind != runtime.KindList {
			return errRes("iter.product(list, list)", "iter"), nil
		}
		a := args[0].Obj.(*runtime.ListObj).Items
		b := args[1].Obj.(*runtime.ListObj).Items
		if int64(len(a))*int64(len(b)) > 1_000_000 {
			return errRes("iter.product: result too large", "iter"), nil
		}
		out := make([]runtime.Value, 0, len(a)*len(b))
		for _, x := range a {
			for _, y := range b {
				out = append(out, runtime.List(x, y))
			}
		}
		return runtime.List(out...), nil
	}, 2)

	// iter.combinations(list, k) -> k-length combinations (order preserved, no repeats)
	set(p, "combinations", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("iter.combinations(list, k)", "iter"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		k, _ := runtime.AsInt(args[1])
		if k < 0 {
			return runtime.List(), nil
		}
		if k == 0 {
			return runtime.List(runtime.List()), nil
		}
		if int(k) > len(items) {
			return runtime.List(), nil
		}
		var out []runtime.Value
		var comb func(start int, cur []runtime.Value)
		comb = func(start int, cur []runtime.Value) {
			if len(cur) == int(k) {
				cp := make([]runtime.Value, len(cur))
				copy(cp, cur)
				out = append(out, runtime.List(cp...))
				return
			}
			for i := start; i < len(items); i++ {
				comb(i+1, append(cur, items[i]))
			}
		}
		comb(0, nil)
		return runtime.List(out...), nil
	}, 2)

	// iter.enumerate(list, start?) -> [[i, x], ...]  (also prelude enumerate)
	set(p, "enumerate", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("iter.enumerate(list, start?)", "iter"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		start := int64(0)
		if len(args) >= 2 {
			start, _ = runtime.AsInt(args[1])
		}
		out := make([]runtime.Value, len(items))
		for i, it := range items {
			out[i] = runtime.List(runtime.Int(start+int64(i)), it)
		}
		return runtime.List(out...), nil
	}, 2)

	return p
}
