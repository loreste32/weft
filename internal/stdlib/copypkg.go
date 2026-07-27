package stdlib

import "github.com/loreste/weft/internal/runtime"

// packageCopy — shallow/deep copy. deepcopy mirrors prelude deepcopy.
func packageCopy() runtime.Value {
	p := pkg()
	set(p, "deepcopy", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		return runtime.DeepCopy(args[0]), nil
	}, 1)
	set(p, "copy", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		v := args[0]
		switch v.Kind {
		case runtime.KindList:
			items := append([]runtime.Value(nil), v.Obj.(*runtime.ListObj).Items...)
			return runtime.List(items...), nil
		case runtime.KindMap:
			mo := v.Obj.(*runtime.MapObj)
			nm := runtime.NewMap()
			nmo := nm.Obj.(*runtime.MapObj)
			for _, k := range mo.Keys {
				nmo.Keys = append(nmo.Keys, k)
				nmo.Vals[k] = mo.Vals[k]
			}
			for k, val := range mo.Vals {
				if _, ok := nmo.Vals[k]; !ok {
					nmo.Keys = append(nmo.Keys, k)
					nmo.Vals[k] = val
				}
			}
			return nm, nil
		default:
			return v, nil
		}
	}, 1)
	return p
}
