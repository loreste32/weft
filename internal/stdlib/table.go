package stdlib

import (
	"sort"
	"strconv"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageTable — row-oriented ops on list-of-maps (no callbacks required).
// Ideal for ETL when you cannot capture locals in fn literals.
func packageTable(env *runtime.Env) runtime.Value {
	p := pkg()

	// table.pluck(rows, field) -> list of values
	set(p, "pluck", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("table.pluck(rows, field)", "table"), nil
		}
		field := args[1].String()
		var out []runtime.Value
		for _, row := range args[0].Obj.(*runtime.ListObj).Items {
			if v, ok := mapGet(row, field); ok {
				out = append(out, v)
			} else {
				out = append(out, runtime.Null())
			}
		}
		return runtime.List(out...), nil
	}, 2)

	// table.project / table.pick(rows, [fields…]) — "select" is a reserved keyword
	projectFn := func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList || args[1].Kind != runtime.KindList {
			return errRes("table.project(rows, [fields])", "table"), nil
		}
		fields := stringList(args[1])
		var out []runtime.Value
		for _, row := range args[0].Obj.(*runtime.ListObj).Items {
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			for _, f := range fields {
				mo.Keys = append(mo.Keys, f)
				if v, ok := mapGet(row, f); ok {
					mo.Vals[f] = v
				} else {
					mo.Vals[f] = runtime.Null()
				}
			}
			out = append(out, m)
		}
		return runtime.List(out...), nil
	}
	set(p, "project", projectFn, 2)
	set(p, "pick", projectFn, 2)

	// table.where_eq(rows, field, value) -> filtered rows
	set(p, "where_eq", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 || args[0].Kind != runtime.KindList {
			return errRes("table.where_eq(rows, field, value)", "table"), nil
		}
		field := args[1].String()
		want := args[2]
		var out []runtime.Value
		for _, row := range args[0].Obj.(*runtime.ListObj).Items {
			if v, ok := mapGet(row, field); ok && runtime.Equal(v, want) {
				out = append(out, row)
			}
		}
		return runtime.List(out...), nil
	}, 3)

	// table.where_truthy(rows, field) -> rows where field is truthy
	set(p, "where_truthy", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("table.where_truthy(rows, field)", "table"), nil
		}
		field := args[1].String()
		var out []runtime.Value
		for _, row := range args[0].Obj.(*runtime.ListObj).Items {
			v, ok := mapGet(row, field)
			if !ok {
				continue
			}
			if isTruthy(v) {
				out = append(out, row)
			}
		}
		return runtime.List(out...), nil
	}, 2)

	// table.where_ne(rows, field, value)
	set(p, "where_ne", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 || args[0].Kind != runtime.KindList {
			return errRes("table.where_ne(rows, field, value)", "table"), nil
		}
		field := args[1].String()
		want := args[2]
		var out []runtime.Value
		for _, row := range args[0].Obj.(*runtime.ListObj).Items {
			v, ok := mapGet(row, field)
			if !ok || !runtime.Equal(v, want) {
				out = append(out, row)
			}
		}
		return runtime.List(out...), nil
	}, 3)

	// table.sort(rows, field, desc?) -> new list sorted by field (string/number)
	set(p, "sort", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("table.sort(rows, field, desc?)", "table"), nil
		}
		field := args[1].String()
		desc := false
		if len(args) >= 3 && args[2].Kind == runtime.KindBool {
			desc = args[2].B
		}
		items := append([]runtime.Value{}, args[0].Obj.(*runtime.ListObj).Items...)
		sort.SliceStable(items, func(i, j int) bool {
			vi, _ := mapGet(items[i], field)
			vj, _ := mapGet(items[j], field)
			cmp := cmpValues(vi, vj)
			if desc {
				return cmp > 0
			}
			return cmp < 0
		})
		return runtime.List(items...), nil
	}, 3)

	// table.unique(rows, field) -> first row per field value
	set(p, "unique", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("table.unique(rows, field)", "table"), nil
		}
		field := args[1].String()
		seen := map[string]bool{}
		var out []runtime.Value
		for _, row := range args[0].Obj.(*runtime.ListObj).Items {
			v, _ := mapGet(row, field)
			k := v.String()
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, row)
		}
		return runtime.List(out...), nil
	}, 2)

	// table.group_by(rows, field) -> map field -> list of rows
	set(p, "group_by", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return errRes("table.group_by(rows, field)", "table"), nil
		}
		field := args[1].String()
		groups := map[string][]runtime.Value{}
		var order []string
		for _, row := range args[0].Obj.(*runtime.ListObj).Items {
			v, _ := mapGet(row, field)
			k := v.String()
			if _, ok := groups[k]; !ok {
				order = append(order, k)
			}
			groups[k] = append(groups[k], row)
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		for _, k := range order {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = runtime.List(groups[k]...)
		}
		return m, nil
	}, 2)

	// table.count(rows) -> int
	set(p, "count", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.Int(0), nil
		}
		return runtime.Int(int64(len(args[0].Obj.(*runtime.ListObj).Items))), nil
	}, 1)

	// table.take(rows, n) / table.drop(rows, n)
	set(p, "take", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return runtime.List(), nil
		}
		n64, _ := runtime.AsInt(args[1])
		items := args[0].Obj.(*runtime.ListObj).Items
		if n64 < 0 {
			n64 = 0
		}
		if n64 > int64(len(items)) {
			n64 = int64(len(items))
		}
		return runtime.List(items[:n64]...), nil
	}, 2)
	set(p, "drop", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return runtime.List(), nil
		}
		n64, _ := runtime.AsInt(args[1])
		items := args[0].Obj.(*runtime.ListObj).Items
		if n64 <= 0 {
			return runtime.List(items...), nil
		}
		if n64 >= int64(len(items)) {
			return runtime.List(), nil
		}
		return runtime.List(items[n64:]...), nil
	}, 2)

	// table.set(rows, field, value) -> copy rows with field set (constant)
	set(p, "set", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 || args[0].Kind != runtime.KindList {
			return errRes("table.set(rows, field, value)", "table"), nil
		}
		field := args[1].String()
		val := args[2]
		var out []runtime.Value
		for _, row := range args[0].Obj.(*runtime.ListObj).Items {
			out = append(out, mapWith(row, field, val))
		}
		return runtime.List(out...), nil
	}, 3)

	// table.rename(rows, from, to)
	set(p, "rename", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 || args[0].Kind != runtime.KindList {
			return errRes("table.rename(rows, from, to)", "table"), nil
		}
		from, to := args[1].String(), args[2].String()
		var out []runtime.Value
		for _, row := range args[0].Obj.(*runtime.ListObj).Items {
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			if row.Kind == runtime.KindMap {
				src := row.Obj.(*runtime.MapObj)
				for _, k := range src.Keys {
					nk := k
					if k == from {
						nk = to
					}
					mo.Keys = append(mo.Keys, nk)
					mo.Vals[nk] = src.Vals[k]
				}
			}
			out = append(out, m)
		}
		return runtime.List(out...), nil
	}, 3)

	// table.merge(left, right, key) — left join on key field (inner for matching)
	set(p, "merge", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 || args[0].Kind != runtime.KindList || args[1].Kind != runtime.KindList {
			return errRes("table.merge(left, right, key)", "table"), nil
		}
		key := args[2].String()
		index := map[string]runtime.Value{}
		for _, r := range args[1].Obj.(*runtime.ListObj).Items {
			if v, ok := mapGet(r, key); ok {
				index[v.String()] = r
			}
		}
		var out []runtime.Value
		for _, l := range args[0].Obj.(*runtime.ListObj).Items {
			m := copyMap(l)
			if v, ok := mapGet(l, key); ok {
				if r, ok := index[v.String()]; ok {
					m = mergeMaps(m, r)
				}
			}
			out = append(out, m)
		}
		return runtime.List(out...), nil
	}, 3)

	// table.to_records — identity for chaining clarity
	set(p, "to_records", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.List(), nil
		}
		return args[0], nil
	}, 1)

	return p
}

func stringList(v runtime.Value) []string {
	if v.Kind != runtime.KindList {
		return nil
	}
	var out []string
	for _, it := range v.Obj.(*runtime.ListObj).Items {
		out = append(out, it.String())
	}
	return out
}

func isTruthy(v runtime.Value) bool {
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
		return v.S != "" && v.S != "false" && v.S != "0"
	default:
		return true
	}
}

func cmpValues(a, b runtime.Value) int {
	// try numbers
	af, aok := asFloatVal(a)
	bf, bok := asFloatVal(b)
	if aok && bok {
		if af < bf {
			return -1
		}
		if af > bf {
			return 1
		}
		return 0
	}
	return strings.Compare(a.String(), b.String())
}

func asFloatVal(v runtime.Value) (float64, bool) {
	switch v.Kind {
	case runtime.KindInt:
		return float64(v.I), true
	case runtime.KindFloat:
		return v.F, true
	case runtime.KindStr:
		f, err := strconv.ParseFloat(v.S, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func mapWith(row runtime.Value, field string, val runtime.Value) runtime.Value {
	m := copyMap(row)
	mo := m.Obj.(*runtime.MapObj)
	if _, ok := mo.Vals[field]; !ok {
		mo.Keys = append(mo.Keys, field)
	}
	mo.Vals[field] = val
	return m
}

func copyMap(row runtime.Value) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	if row.Kind == runtime.KindMap {
		src := row.Obj.(*runtime.MapObj)
		for _, k := range src.Keys {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = src.Vals[k]
		}
	}
	return m
}

func mergeMaps(left, right runtime.Value) runtime.Value {
	m := copyMap(left)
	mo := m.Obj.(*runtime.MapObj)
	if right.Kind == runtime.KindMap {
		src := right.Obj.(*runtime.MapObj)
		for _, k := range src.Keys {
			if _, ok := mo.Vals[k]; !ok {
				mo.Keys = append(mo.Keys, k)
			}
			mo.Vals[k] = src.Vals[k]
		}
	}
	return m
}
