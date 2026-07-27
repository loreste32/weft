package stdlib

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

func packageJSON() runtime.Value {
	p := pkg()
	// json.parse(s) -> Result[any]
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("json.parse needs a string", "json"), nil
		}
		var raw any
		dec := json.NewDecoder(strings.NewReader(args[0].String()))
		dec.UseNumber()
		if err := dec.Decode(&raw); err != nil {
			return errRes(err.Error(), "json"), nil
		}
		return runtime.Ok(goToValue(raw)), nil
	}, 1)
	// json.stringify(v, indent?) -> str
	// indent: int spaces or string; omit for compact
	set(p, "stringify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str("null"), nil
		}
		goV := valueToGo(args[0])
		if len(args) >= 2 && args[1].IsTruthy() {
			prefix := ""
			indent := "  "
			switch args[1].Kind {
			case runtime.KindInt:
				n := int(args[1].I)
				if n < 0 {
					n = 0
				}
				if n > 8 {
					n = 8
				}
				indent = strings.Repeat(" ", n)
			case runtime.KindStr:
				indent = args[1].String()
			}
			b, err := json.MarshalIndent(goV, prefix, indent)
			if err != nil {
				return errRes(err.Error(), "json"), nil
			}
			return runtime.Str(string(b)), nil
		}
		b, err := json.Marshal(goV)
		if err != nil {
			return errRes(err.Error(), "json"), nil
		}
		return runtime.Str(string(b)), nil
	}, 2)

	// json.pretty(v) -> str  (indent 2)
	set(p, "pretty", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str("null"), nil
		}
		b, err := json.MarshalIndent(valueToGo(args[0]), "", "  ")
		if err != nil {
			return errRes(err.Error(), "json"), nil
		}
		return runtime.Str(string(b)), nil
	}, 1)

	// json.get(obj, path, default?) -> Result
	// path: "a.b.0.c" or ["a","b",0,"c"]. With default, missing path → Ok(default).
	set(p, "get", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("json.get(obj, path, default?)", "json"), nil
		}
		v, err := jsonPathGet(args[0], args[1])
		if err != nil {
			if len(args) >= 3 {
				return runtime.Ok(args[2]), nil
			}
			return errRes(err.Error(), "json"), nil
		}
		return runtime.Ok(v), nil
	}, 3)

	// json.has(obj, path) -> bool
	set(p, "has", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		_, err := jsonPathGet(args[0], args[1])
		return runtime.Bool(err == nil), nil
	}, 2)

	// json.set(obj, path, value) -> new obj (deep copy path parents)
	set(p, "set", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("json.set(obj, path, value)", "json"), nil
		}
		out, err := jsonPathSet(args[0], args[1], args[2])
		if err != nil {
			return errRes(err.Error(), "json"), nil
		}
		return runtime.Ok(out), nil
	}, 3)

	// json.merge(a, b) -> map  shallow merge b over a (maps only)
	set(p, "merge", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("json.merge(a, b)", "json"), nil
		}
		if args[0].Kind != runtime.KindMap || args[1].Kind != runtime.KindMap {
			return errRes("json.merge: need maps", "json"), nil
		}
		a := args[0].Obj.(*runtime.MapObj)
		b := args[1].Obj.(*runtime.MapObj)
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		seen := map[string]bool{}
		for _, k := range a.Keys {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = a.Vals[k]
			seen[k] = true
		}
		for _, k := range b.Keys {
			if !seen[k] {
				mo.Keys = append(mo.Keys, k)
			}
			mo.Vals[k] = b.Vals[k]
		}
		return m, nil
	}, 2)

	// json.clone(v) -> deep copy
	set(p, "clone", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		return runtime.DeepCopy(args[0]), nil
	}, 1)

	return p
}

func jsonPathParts(path runtime.Value) ([]string, error) {
	switch path.Kind {
	case runtime.KindStr:
		s := path.S
		if s == "" {
			return nil, fmt.Errorf("empty path")
		}
		return strings.Split(s, "."), nil
	case runtime.KindList:
		var parts []string
		for _, it := range path.Obj.(*runtime.ListObj).Items {
			parts = append(parts, it.String())
		}
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty path")
		}
		return parts, nil
	default:
		return nil, fmt.Errorf("path must be string or list")
	}
}

func jsonPathGet(root runtime.Value, path runtime.Value) (runtime.Value, error) {
	parts, err := jsonPathParts(path)
	if err != nil {
		return runtime.Null(), err
	}
	cur := root
	for _, p := range parts {
		switch cur.Kind {
		case runtime.KindMap:
			mo := cur.Obj.(*runtime.MapObj)
			v, ok := mo.Vals[p]
			if !ok {
				return runtime.Null(), fmt.Errorf("missing key %q", p)
			}
			cur = v
		case runtime.KindList:
			idx, err := strconv.Atoi(p)
			if err != nil {
				return runtime.Null(), fmt.Errorf("list index %q", p)
			}
			items := cur.Obj.(*runtime.ListObj).Items
			if idx < 0 || idx >= len(items) {
				return runtime.Null(), fmt.Errorf("index %d out of range", idx)
			}
			cur = items[idx]
		case runtime.KindStruct:
			so := cur.Obj.(*runtime.StructObj)
			v, ok := so.Fields[p]
			if !ok {
				return runtime.Null(), fmt.Errorf("missing field %q", p)
			}
			cur = v
		default:
			return runtime.Null(), fmt.Errorf("cannot index %s", cur.KindName())
		}
	}
	return cur, nil
}

func jsonPathSet(root runtime.Value, path runtime.Value, val runtime.Value) (runtime.Value, error) {
	parts, err := jsonPathParts(path)
	if err != nil {
		return runtime.Null(), err
	}
	// deep copy root then set
	root = runtime.DeepCopy(root)
	if len(parts) == 0 {
		return val, nil
	}
	var setAt func(cur runtime.Value, parts []string) (runtime.Value, error)
	setAt = func(cur runtime.Value, parts []string) (runtime.Value, error) {
		if len(parts) == 1 {
			key := parts[0]
			switch cur.Kind {
			case runtime.KindMap:
				mo := cur.Obj.(*runtime.MapObj)
				if _, ok := mo.Vals[key]; !ok {
					mo.Keys = append(mo.Keys, key)
				}
				mo.Vals[key] = val
				return cur, nil
			case runtime.KindList:
				idx, err := strconv.Atoi(key)
				if err != nil {
					return runtime.Null(), err
				}
				items := cur.Obj.(*runtime.ListObj).Items
				if idx < 0 || idx >= len(items) {
					return runtime.Null(), fmt.Errorf("index %d out of range", idx)
				}
				items[idx] = val
				return cur, nil
			default:
				return runtime.Null(), fmt.Errorf("cannot set on %s", cur.KindName())
			}
		}
		key := parts[0]
		var child runtime.Value
		switch cur.Kind {
		case runtime.KindMap:
			mo := cur.Obj.(*runtime.MapObj)
			var ok bool
			child, ok = mo.Vals[key]
			if !ok {
				// create map container
				child = runtime.NewMap()
				mo.Keys = append(mo.Keys, key)
				mo.Vals[key] = child
			}
			updated, err := setAt(child, parts[1:])
			if err != nil {
				return runtime.Null(), err
			}
			mo.Vals[key] = updated
			return cur, nil
		case runtime.KindList:
			idx, err := strconv.Atoi(key)
			if err != nil {
				return runtime.Null(), err
			}
			items := cur.Obj.(*runtime.ListObj).Items
			if idx < 0 || idx >= len(items) {
				return runtime.Null(), fmt.Errorf("index %d out of range", idx)
			}
			updated, err := setAt(items[idx], parts[1:])
			if err != nil {
				return runtime.Null(), err
			}
			items[idx] = updated
			return cur, nil
		default:
			return runtime.Null(), fmt.Errorf("cannot set on %s", cur.KindName())
		}
	}
	return setAt(root, parts)
}
