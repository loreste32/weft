package stdlib

import (
	"fmt"
	"os"
	goruntime "runtime"
	"strconv"
	"sync"

	"github.com/loreste/weft/internal/runtime"
)

// installListOps registers map/filter/reduce/each/flat_map as globals (pipeline core).
// Concurrent by default: map/filter fan out (order preserved). Use seq_map/seq_filter when order of
// side effects matters. Callbacks may be closures (by-value capture) or named fns.
func installListOps(env *runtime.Env) {
	call := func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		if env.Call == nil {
			return runtime.Null(), fmt.Errorf("runtime Call not configured")
		}
		return env.Call(fn, args)
	}
	truthy := func(v runtime.Value) bool {
		switch v.Kind {
		case runtime.KindNull, runtime.KindUnit:
			return false
		case runtime.KindBool:
			return v.B
		case runtime.KindInt:
			return v.I != 0
		case runtime.KindFloat:
			return v.F != 0
		case runtime.KindStr:
			return v.S != ""
		case runtime.KindList:
			return len(v.Obj.(*runtime.ListObj).Items) > 0
		case runtime.KindResult:
			return v.Obj.(*runtime.ResultObj).Ok
		default:
			return true
		}
	}
	unwrap := func(v runtime.Value) (runtime.Value, error) {
		if v.Kind == runtime.KindResult {
			ro := v.Obj.(*runtime.ResultObj)
			if !ro.Ok {
				return runtime.Null(), fmt.Errorf("%s", ro.Err.String())
			}
			return ro.Val, nil
		}
		return v, nil
	}

	// map(list, fn, workers?) -> list
	// Concurrent by default (order preserved). Use seq_map for sequential.
	env.SetShared("map", runtime.MakeBuiltin("map", 3, func(args []runtime.Value) (runtime.Value, error) {
		return mapParallel(call, unwrap, args, defaultMapWorkers)
	}))

	// seq_map(list, fn) -> list  sequential (side-effect order, debugging)
	env.SetShared("seq_map", runtime.MakeBuiltin("seq_map", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("seq_map(list, fn)", "pipe"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		fn := args[1]
		out := make([]runtime.Value, 0, len(items))
		for _, it := range items {
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			v, err = unwrap(v)
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			out = append(out, v)
		}
		return runtime.List(out...), nil
	}))

	// filter(list, fn, workers?) -> list  concurrent by default; order of kept items preserved
	env.SetShared("filter", runtime.MakeBuiltin("filter", 3, func(args []runtime.Value) (runtime.Value, error) {
		return filterParallel(call, unwrap, truthy, args, defaultMapWorkers)
	}))

	// seq_filter(list, fn) -> sequential filter
	env.SetShared("seq_filter", runtime.MakeBuiltin("seq_filter", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("seq_filter(list, fn)", "pipe"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		fn := args[1]
		var out []runtime.Value
		for _, it := range items {
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			v, err = unwrap(v)
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			if truthy(v) {
				out = append(out, it)
			}
		}
		return runtime.List(out...), nil
	}))

	// reduce(list, init, fn) -> value  fn(acc, x)
	env.SetShared("reduce", runtime.MakeBuiltin("reduce", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 || args[0].Kind != runtime.KindList {
			return errRes("reduce(list, init, fn)", "pipe"), nil
		}
		acc := args[1]
		fn := args[2]
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			v, err := call(fn, []runtime.Value{acc, it})
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			v, err = unwrap(v)
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			acc = v
		}
		return acc, nil
	}))

	// each(list, fn) -> unit  side effects
	env.SetShared("each", runtime.MakeBuiltin("each", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("each(list, fn)", "pipe"), nil
		}
		fn := args[1]
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			if v.Kind == runtime.KindResult && !v.Obj.(*runtime.ResultObj).Ok {
				return v, nil
			}
		}
		return runtime.Unit(), nil
	}))

	// flat_map(list, fn) -> list  fn returns list
	env.SetShared("flat_map", runtime.MakeBuiltin("flat_map", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("flat_map(list, fn)", "pipe"), nil
		}
		fn := args[1]
		var out []runtime.Value
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			v, err = unwrap(v)
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			if v.Kind != runtime.KindList {
				return errRes("flat_map fn must return list", "pipe"), nil
			}
			out = append(out, v.Obj.(*runtime.ListObj).Items...)
		}
		return runtime.List(out...), nil
	}))

	// par_map — explicit concurrent map (same as map; workers optional)
	env.SetShared("par_map", runtime.MakeBuiltin("par_map", 3, func(args []runtime.Value) (runtime.Value, error) {
		return mapParallel(call, unwrap, args, defaultMapWorkers)
	}))

	// find(list, fn) -> item|null
	env.SetShared("find", runtime.MakeBuiltin("find", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return runtime.Null(), nil
		}
		fn := args[1]
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			v, _ = unwrap(v)
			if truthy(v) {
				return it, nil
			}
		}
		return runtime.Null(), nil
	}))

	// any(list, fn) / all(list, fn)
	env.SetShared("any", runtime.MakeBuiltin("any", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return runtime.Bool(false), nil
		}
		fn := args[1]
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			v, _ = unwrap(v)
			if truthy(v) {
				return runtime.Bool(true), nil
			}
		}
		return runtime.Bool(false), nil
	}))
	env.SetShared("all", runtime.MakeBuiltin("all", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return runtime.Bool(true), nil
		}
		fn := args[1]
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			v, _ = unwrap(v)
			if !truthy(v) {
				return runtime.Bool(false), nil
			}
		}
		return runtime.Bool(true), nil
	}))

	// sort(list) -> list  numbers/strings ascending (stable copy)
	env.SetShared("sort", runtime.MakeBuiltin("sort", -1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("sort(list, key_fn?)", "pipe"), nil
		}
		items := append([]runtime.Value(nil), args[0].Obj.(*runtime.ListObj).Items...)
		if len(args) >= 2 && (args[1].Kind == runtime.KindFunc || args[1].Kind == runtime.KindBuiltin) {
			// Sort by key function: compute keys, sort by key values
			keyFn := args[1]
			type keyed struct {
				key runtime.Value
				val runtime.Value
			}
			pairs := make([]keyed, len(items))
			for i, it := range items {
				k, err := env.Call(keyFn, []runtime.Value{it})
				if err != nil {
					return errRes("sort key function error: "+err.Error(), "pipe"), nil
				}
				pairs[i] = keyed{key: k, val: it}
			}
			// stable insertion sort by key
			for i := 1; i < len(pairs); i++ {
				j := i
				for j > 0 && compareValues(pairs[j].key, pairs[j-1].key) < 0 {
					pairs[j], pairs[j-1] = pairs[j-1], pairs[j]
					j--
				}
			}
			result := make([]runtime.Value, len(pairs))
			for i, p := range pairs {
				result[i] = p.val
			}
			return runtime.List(result...), nil
		}
		sortValues(items)
		return runtime.List(items...), nil
	}))

	// min_of(list) / max_of(list) — min/max over a list (prelude min/max are for scalars via math)
	env.SetShared("min_of", runtime.MakeBuiltin("min_of", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("min_of(list)", "pipe"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		if len(items) == 0 {
			return runtime.Null(), nil
		}
		best := items[0]
		for _, it := range items[1:] {
			if compareValues(it, best) < 0 {
				best = it
			}
		}
		return best, nil
	}))
	env.SetShared("max_of", runtime.MakeBuiltin("max_of", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("max_of(list)", "pipe"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		if len(items) == 0 {
			return runtime.Null(), nil
		}
		best := items[0]
		for _, it := range items[1:] {
			if compareValues(it, best) > 0 {
				best = it
			}
		}
		return best, nil
	}))
	// first(list) / last(list)
	env.SetShared("first", runtime.MakeBuiltin("first", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.Null(), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		if len(items) == 0 {
			return runtime.Null(), nil
		}
		return items[0], nil
	}))
	env.SetShared("last", runtime.MakeBuiltin("last", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.Null(), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		if len(items) == 0 {
			return runtime.Null(), nil
		}
		return items[len(items)-1], nil
	}))

	// sum_of(list) — numeric sum (math.sum also works)
	env.SetShared("sum_of", runtime.MakeBuiltin("sum_of", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("sum_of(list)", "pipe"), nil
		}
		var si int64
		var sf float64
		allInt := true
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			switch it.Kind {
			case runtime.KindInt:
				si += it.I
				sf += float64(it.I)
			case runtime.KindFloat:
				allInt = false
				sf += it.F
			default:
				if f, ok := asFloat64(it); ok {
					allInt = false
					sf += f
				}
			}
		}
		if allInt {
			return runtime.Int(si), nil
		}
		return runtime.Float(sf), nil
	}))

	// reverse(list) -> list  copy reversed
	env.SetShared("reverse", runtime.MakeBuiltin("reverse", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("reverse(list)", "pipe"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		out := make([]runtime.Value, len(items))
		for i, it := range items {
			out[len(items)-1-i] = it
		}
		return runtime.List(out...), nil
	}))

	// unique(list) -> list  first-seen order
	env.SetShared("unique", runtime.MakeBuiltin("unique", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("unique(list)", "pipe"), nil
		}
		seen := map[string]bool{}
		var out []runtime.Value
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			k := valueKey(it)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, it)
		}
		return runtime.List(out...), nil
	}))

	// zip(a, b, ...) -> [[a0,b0,...], ...]  length of shortest
	env.SetShared("zip", runtime.MakeBuiltin("zip", -1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("zip(list, list, ...)", "pipe"), nil
		}
		lists := make([][]runtime.Value, len(args))
		n := -1
		for i, a := range args {
			if a.Kind != runtime.KindList {
				return errRes("zip: all args must be lists", "pipe"), nil
			}
			lists[i] = a.Obj.(*runtime.ListObj).Items
			if n < 0 || len(lists[i]) < n {
				n = len(lists[i])
			}
		}
		if n < 0 {
			n = 0
		}
		out := make([]runtime.Value, n)
		for i := 0; i < n; i++ {
			row := make([]runtime.Value, len(lists))
			for j := range lists {
				row[j] = lists[j][i]
			}
			out[i] = runtime.List(row...)
		}
		return runtime.List(out...), nil
	}))

	// flatten(list) -> list  one level
	env.SetShared("flatten", runtime.MakeBuiltin("flatten", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("flatten(list)", "pipe"), nil
		}
		var out []runtime.Value
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			if it.Kind == runtime.KindList {
				out = append(out, it.Obj.(*runtime.ListObj).Items...)
			} else {
				out = append(out, it)
			}
		}
		return runtime.List(out...), nil
	}))

	// enumerate(list) -> [[i, x], ...]
	env.SetShared("enumerate", runtime.MakeBuiltin("enumerate", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("enumerate(list)", "pipe"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		out := make([]runtime.Value, len(items))
		for i, it := range items {
			out[i] = runtime.List(runtime.Int(int64(i)), it)
		}
		return runtime.List(out...), nil
	}))

	// count(list) -> len
	// count(list, pred_fn) -> count where pred is truthy
	// count(list, value) -> occurrence count (Python list.count)
	env.SetShared("count", runtime.MakeBuiltin("count", -1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.Int(0), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		if len(args) < 2 {
			return runtime.Int(int64(len(items))), nil
		}
		needle := args[1]
		// callable → predicate mode
		if needle.Kind == runtime.KindFunc || needle.Kind == runtime.KindBuiltin {
			var n int64
			for _, it := range items {
				v, err := call(needle, []runtime.Value{it})
				if err != nil {
					return errRes(err.Error(), "pipe"), nil
				}
				v, _ = unwrap(v)
				if truthy(v) {
					n++
				}
			}
			return runtime.Int(n), nil
		}
		// value → equality count
		var n int64
		for _, it := range items {
			if runtime.Equal(it, needle) {
				n++
			}
		}
		return runtime.Int(n), nil
	}))
}

func valueKey(v runtime.Value) string {
	// structural key for unique — good enough for numbers/strings/bools
	switch v.Kind {
	case runtime.KindInt:
		return "i:" + strconv.FormatInt(v.I, 10)
	case runtime.KindFloat:
		return "f:" + strconv.FormatFloat(v.F, 'g', -1, 64)
	case runtime.KindBool:
		if v.B {
			return "b:1"
		}
		return "b:0"
	case runtime.KindStr:
		return "s:" + v.S
	case runtime.KindNull:
		return "null"
	default:
		return "x:" + v.String()
	}
}

func sortValues(items []runtime.Value) {
	// simple insertion sort with comparable key
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && valueLess(items[j], items[j-1]) {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
}

func valueLess(a, b runtime.Value) bool {
	// prefer numeric compare when both numbers
	af, aok := asFloat64(a)
	bf, bok := asFloat64(b)
	if aok && bok {
		return af < bf
	}
	return a.String() < b.String()
}

// defaultMapWorkers: WEFT_WORKERS env, else GOMAXPROCS (at least 2 for I/O fan-out).
func defaultMapWorkers() int {
	if v := os.Getenv("WEFT_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	n := goruntime.GOMAXPROCS(0)
	if n < 2 {
		n = 2
	}
	return n
}

func mapParallel(
	call func(runtime.Value, []runtime.Value) (runtime.Value, error),
	unwrap func(runtime.Value) (runtime.Value, error),
	args []runtime.Value,
	defaultWorkers func() int,
) (runtime.Value, error) {
	if len(args) < 2 || args[0].Kind != runtime.KindList {
		return errRes("map(list, fn, workers?)", "pipe"), nil
	}
	items := args[0].Obj.(*runtime.ListObj).Items
	fn := args[1]
	workers := defaultWorkers()
	if len(args) >= 3 {
		if n, err := runtime.AsInt(args[2]); err == nil && n > 0 {
			workers = int(n)
		}
	}
	if workers > len(items) {
		workers = len(items)
	}
	if workers < 1 {
		workers = 1
	}
	if workers == 1 || len(items) <= 1 {
		out := make([]runtime.Value, 0, len(items))
		for _, it := range items {
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			v, err = unwrap(v)
			if err != nil {
				return errRes(err.Error(), "pipe"), nil
			}
			out = append(out, v)
		}
		return runtime.List(out...), nil
	}
	out := make([]runtime.Value, len(items))
	errs := make([]error, len(items))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i, it := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, it runtime.Value) {
			defer wg.Done()
			defer func() { <-sem }()
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				errs[i] = err
				return
			}
			v, err = unwrap(v)
			if err != nil {
				errs[i] = err
				return
			}
			out[i] = v
		}(i, it)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return errRes(e.Error(), "pipe"), nil
		}
	}
	return runtime.List(out...), nil
}

func filterParallel(
	call func(runtime.Value, []runtime.Value) (runtime.Value, error),
	unwrap func(runtime.Value) (runtime.Value, error),
	truthy func(runtime.Value) bool,
	args []runtime.Value,
	defaultWorkers func() int,
) (runtime.Value, error) {
	if len(args) < 2 || args[0].Kind != runtime.KindList {
		return errRes("filter(list, fn, workers?)", "pipe"), nil
	}
	items := args[0].Obj.(*runtime.ListObj).Items
	fn := args[1]
	workers := defaultWorkers()
	if len(args) >= 3 {
		if n, err := runtime.AsInt(args[2]); err == nil && n > 0 {
			workers = int(n)
		}
	}
	if workers > len(items) {
		workers = len(items)
	}
	if workers < 1 {
		workers = 1
	}
	// compute keep flags concurrently, then gather in order
	keep := make([]bool, len(items))
	errs := make([]error, len(items))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i, it := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, it runtime.Value) {
			defer wg.Done()
			defer func() { <-sem }()
			v, err := call(fn, []runtime.Value{it})
			if err != nil {
				errs[i] = err
				return
			}
			v, err = unwrap(v)
			if err != nil {
				errs[i] = err
				return
			}
			keep[i] = truthy(v)
		}(i, it)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return errRes(e.Error(), "pipe"), nil
		}
	}
	var out []runtime.Value
	for i, it := range items {
		if keep[i] {
			out = append(out, it)
		}
	}
	return runtime.List(out...), nil
}
