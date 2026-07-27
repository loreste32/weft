package stdlib

import (
	"sort"
	"strconv"

	"github.com/loreste/weft/internal/runtime"
)

// packageCollections — Counter / group_by (Python collections lite).
func packageCollections() runtime.Value {
	p := pkg()

	// collections.counter(list) -> map[str]int  counts by valueKey
	set(p, "counter", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("collections.counter(list)", "collections"), nil
		}
		counts := map[string]int64{}
		order := []string{}
		// also keep first-seen value for non-string keys via string form
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			k := counterKey(it)
			if _, ok := counts[k]; !ok {
				order = append(order, k)
			}
			counts[k]++
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		for _, k := range order {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = runtime.Int(counts[k])
		}
		return m, nil
	}, 1)

	// collections.most_common(list|counter_map, n?) -> [[key, count], ...] sorted by count desc
	set(p, "most_common", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("collections.most_common(list|map, n?)", "collections"), nil
		}
		var pairs [][2]runtime.Value
		switch args[0].Kind {
		case runtime.KindList:
			counts := map[string]int64{}
			vals := map[string]runtime.Value{}
			for _, it := range args[0].Obj.(*runtime.ListObj).Items {
				k := counterKey(it)
				counts[k]++
				if _, ok := vals[k]; !ok {
					vals[k] = it
				}
			}
			for k, c := range counts {
				pairs = append(pairs, [2]runtime.Value{vals[k], runtime.Int(c)})
			}
		case runtime.KindMap:
			mo := args[0].Obj.(*runtime.MapObj)
			for _, k := range mo.Keys {
				v := mo.Vals[k]
				n, _ := runtime.AsInt(v)
				pairs = append(pairs, [2]runtime.Value{runtime.Str(k), runtime.Int(n)})
			}
		default:
			return errRes("collections.most_common: list or map", "collections"), nil
		}
		sort.SliceStable(pairs, func(i, j int) bool {
			ci, _ := runtime.AsInt(pairs[i][1])
			cj, _ := runtime.AsInt(pairs[j][1])
			if ci != cj {
				return ci > cj
			}
			return pairs[i][0].String() < pairs[j][0].String()
		})
		n := int64(len(pairs))
		if len(args) >= 2 {
			if want, err := runtime.AsInt(args[1]); err == nil && want >= 0 && want < n {
				n = want
			}
		}
		out := make([]runtime.Value, n)
		for i := int64(0); i < n; i++ {
			out[i] = runtime.List(pairs[i][0], pairs[i][1])
		}
		return runtime.List(out...), nil
	}, 2)

	// collections.group_by(list, key_field) -> map[str]list
	// key_field: string field name on map/struct items, or if items are scalar, groups by value
	set(p, "group_by", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("collections.group_by(list, field?)", "collections"), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		field := ""
		if len(args) >= 2 {
			field = args[1].String()
		}
		groups := map[string][]runtime.Value{}
		order := []string{}
		for _, it := range items {
			k := groupKey(it, field)
			if _, ok := groups[k]; !ok {
				order = append(order, k)
			}
			groups[k] = append(groups[k], it)
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		for _, k := range order {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = runtime.List(groups[k]...)
		}
		return m, nil
	}, 2)

	// collections.defaultdict not needed — maps are dynamic
	return p
}

func counterKey(v runtime.Value) string {
	switch v.Kind {
	case runtime.KindStr:
		return v.S
	case runtime.KindInt:
		return strconv.FormatInt(v.I, 10)
	case runtime.KindFloat:
		return strconv.FormatFloat(v.F, 'g', -1, 64)
	case runtime.KindBool:
		if v.B {
			return "true"
		}
		return "false"
	case runtime.KindNull:
		return "null"
	default:
		return v.String()
	}
}

func groupKey(v runtime.Value, field string) string {
	if field == "" {
		return counterKey(v)
	}
	switch v.Kind {
	case runtime.KindMap:
		mo := v.Obj.(*runtime.MapObj)
		if x, ok := mo.Vals[field]; ok {
			return counterKey(x)
		}
	case runtime.KindStruct:
		so := v.Obj.(*runtime.StructObj)
		if x, ok := so.Fields[field]; ok {
			return counterKey(x)
		}
	}
	return ""
}
