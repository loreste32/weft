package stdlib

import (
	"github.com/loreste/weft/internal/runtime"
)

// packageHeap — binary min-heap over lists (Python heapq lite).
// APIs return new lists (immutable style) except documented mutators.
func packageHeap() runtime.Value {
	p := pkg()

	// heap.heapify(list) -> min-heap list
	set(p, "heapify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("heap.heapify(list)", "heap"), nil
		}
		items := append([]runtime.Value(nil), args[0].Obj.(*runtime.ListObj).Items...)
		heapify(items)
		return runtime.List(items...), nil
	}, 1)

	// heap.push(heap_list, x) -> new heap
	set(p, "push", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("heap.push(heap, x)", "heap"), nil
		}
		items := append([]runtime.Value(nil), args[0].Obj.(*runtime.ListObj).Items...)
		items = append(items, args[1])
		siftUp(items, len(items)-1)
		return runtime.List(items...), nil
	}, 2)

	// heap.pop(heap_list) -> Result[{heap, value}]  or Err if empty
	set(p, "pop", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("heap.pop(heap)", "heap"), nil
		}
		items := append([]runtime.Value(nil), args[0].Obj.(*runtime.ListObj).Items...)
		if len(items) == 0 {
			return errRes("heap.pop: empty", "heap"), nil
		}
		val := items[0]
		last := items[len(items)-1]
		items = items[:len(items)-1]
		if len(items) > 0 {
			items[0] = last
			siftDown(items, 0)
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"heap", "value"}
		mo.Vals["heap"] = runtime.List(items...)
		mo.Vals["value"] = val
		return runtime.Ok(m), nil
	}, 1)

	// heap.nsmallest(n, list) / nlargest
	set(p, "nsmallest", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[1].Kind != runtime.KindList {
			return errRes("heap.nsmallest(n, list)", "heap"), nil
		}
		n, _ := runtime.AsInt(args[0])
		items := append([]runtime.Value(nil), args[1].Obj.(*runtime.ListObj).Items...)
		sortValues(items) // ascending
		if n < 0 {
			n = 0
		}
		if n > int64(len(items)) {
			n = int64(len(items))
		}
		return runtime.List(items[:n]...), nil
	}, 2)

	set(p, "nlargest", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[1].Kind != runtime.KindList {
			return errRes("heap.nlargest(n, list)", "heap"), nil
		}
		n, _ := runtime.AsInt(args[0])
		items := append([]runtime.Value(nil), args[1].Obj.(*runtime.ListObj).Items...)
		sortValues(items)
		// reverse
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
		if n < 0 {
			n = 0
		}
		if n > int64(len(items)) {
			n = int64(len(items))
		}
		return runtime.List(items[:n]...), nil
	}, 2)

	// heap.pushpop(heap, x) -> Result[{heap, value}]
	set(p, "pushpop", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("heap.pushpop(heap, x)", "heap"), nil
		}
		items := append([]runtime.Value(nil), args[0].Obj.(*runtime.ListObj).Items...)
		x := args[1]
		if len(items) == 0 {
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			mo.Keys = []string{"heap", "value"}
			mo.Vals["heap"] = runtime.List()
			mo.Vals["value"] = x
			return runtime.Ok(m), nil
		}
		if compareValues(x, items[0]) > 0 {
			// x larger than min → pop min, push x
			val := items[0]
			items[0] = x
			siftDown(items, 0)
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			mo.Keys = []string{"heap", "value"}
			mo.Vals["heap"] = runtime.List(items...)
			mo.Vals["value"] = val
			return runtime.Ok(m), nil
		}
		// x is new min → return x, heap unchanged
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"heap", "value"}
		mo.Vals["heap"] = runtime.List(items...)
		mo.Vals["value"] = x
		return runtime.Ok(m), nil
	}, 2)

	return p
}

func heapify(a []runtime.Value) {
	for i := len(a)/2 - 1; i >= 0; i-- {
		siftDown(a, i)
	}
}

func siftUp(a []runtime.Value, i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if compareValues(a[i], a[parent]) >= 0 {
			break
		}
		a[i], a[parent] = a[parent], a[i]
		i = parent
	}
}

func siftDown(a []runtime.Value, i int) {
	n := len(a)
	for {
		left := 2*i + 1
		if left >= n {
			break
		}
		j := left
		right := left + 1
		if right < n && compareValues(a[right], a[left]) < 0 {
			j = right
		}
		if compareValues(a[j], a[i]) >= 0 {
			break
		}
		a[i], a[j] = a[j], a[i]
		i = j
	}
}
