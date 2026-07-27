package stdlib

import "github.com/loreste/weft/internal/runtime"

// packageCopy — deep/shallow copy (Python copy lite).
func packageCopy() runtime.Value {
	p := pkg()

	// copy.deepcopy(v) — structural deep copy of lists/maps/structs
	set(p, "deepcopy", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		return runtime.DeepCopy(args[0]), nil
	}, 1)

	// copy.copy(v) — shallow: new list/map shell, shared elements
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
			return nm, nil
		default:
			return v, nil
		}
	}, 1)

	return p
}
