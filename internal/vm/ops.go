package vm

import (
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

func binAdd(a, b runtime.Value) (runtime.Value, error) {
	if a.Kind == runtime.KindStr || b.Kind == runtime.KindStr {
		return runtime.Str(a.String() + b.String()), nil
	}
	if a.Kind == runtime.KindInt && b.Kind == runtime.KindInt {
		return runtime.Int(a.I + b.I), nil
	}
	return binNum(a, b, func(x, y float64) float64 { return x + y })
}

func binNum(a, b runtime.Value, op func(x, y float64) float64) (runtime.Value, error) {
	if a.Kind == runtime.KindInt && b.Kind == runtime.KindInt {
		// ponytail: float64 round-trip loses precision past 2^53; acceptable for a scripting VM
		return runtime.Int(int64(op(float64(a.I), float64(b.I)))), nil
	}
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if !aok || !bok {
		return runtime.Null(), fmt.Errorf("numeric op on %s and %s", a.KindName(), b.KindName())
	}
	return runtime.Float(op(af, bf)), nil
}

func div(a, b runtime.Value) (runtime.Value, error) {
	if b.Kind == runtime.KindInt && b.I == 0 {
		return runtime.Null(), fmt.Errorf("division by zero")
	}
	if b.Kind == runtime.KindFloat && b.F == 0 {
		return runtime.Null(), fmt.Errorf("division by zero")
	}
	if a.Kind == runtime.KindInt && b.Kind == runtime.KindInt {
		return runtime.Int(a.I / b.I), nil
	}
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if !aok || !bok {
		return runtime.Null(), fmt.Errorf("numeric op on %s and %s", a.KindName(), b.KindName())
	}
	if bf == 0 {
		return runtime.Null(), fmt.Errorf("division by zero")
	}
	return runtime.Float(af / bf), nil
}

func asFloat(v runtime.Value) (float64, bool) {
	switch v.Kind {
	case runtime.KindInt:
		return float64(v.I), true
	case runtime.KindFloat:
		return v.F, true
	default:
		return 0, false
	}
}

func compare(a, b runtime.Value) (int, error) {
	if a.Kind == runtime.KindStr && b.Kind == runtime.KindStr {
		return strings.Compare(a.S, b.S), nil
	}
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if !aok || !bok {
		return 0, fmt.Errorf("cannot compare %s and %s", a.KindName(), b.KindName())
	}
	switch {
	case af < bf:
		return -1, nil
	case af > bf:
		return 1, nil
	default:
		return 0, nil
	}
}

func getIndex(x, idx runtime.Value) (runtime.Value, error) {
	switch x.Kind {
	case runtime.KindList:
		lo := x.Obj.(*runtime.ListObj)
		i, err := runtime.AsInt(idx)
		if err != nil {
			return runtime.Null(), err
		}
		if i < 0 || int(i) >= len(lo.Items) {
			return runtime.Null(), fmt.Errorf("index out of range")
		}
		return lo.Items[i], nil
	case runtime.KindMap:
		mo := x.Obj.(*runtime.MapObj)
		k := idx.String()
		v, ok := mo.Vals[k]
		if !ok {
			return runtime.Null(), nil
		}
		return v, nil
	case runtime.KindStr:
		i, err := runtime.AsInt(idx)
		if err != nil {
			return runtime.Null(), err
		}
		runes := []rune(x.S)
		if i < 0 || int(i) >= len(runes) {
			return runtime.Null(), fmt.Errorf("index out of range")
		}
		return runtime.Str(string(runes[i])), nil
	default:
		return runtime.Null(), fmt.Errorf("cannot index %s", x.KindName())
	}
}

func setIndex(x, idx, val runtime.Value) error {
	switch x.Kind {
	case runtime.KindList:
		lo := x.Obj.(*runtime.ListObj)
		i, err := runtime.AsInt(idx)
		if err != nil {
			return err
		}
		if i < 0 || int(i) >= len(lo.Items) {
			return fmt.Errorf("index out of range")
		}
		lo.Items[i] = val
		return nil
	case runtime.KindMap:
		mo := x.Obj.(*runtime.MapObj)
		k := idx.String()
		if _, exists := mo.Vals[k]; !exists {
			mo.Keys = append(mo.Keys, k)
		}
		mo.Vals[k] = val
		return nil
	default:
		return fmt.Errorf("cannot set index on %s", x.KindName())
	}
}

func getField(x runtime.Value, name string) (runtime.Value, error) {
	switch x.Kind {
	case runtime.KindStruct:
		so := x.Obj.(*runtime.StructObj)
		// Secrets: never expose raw value via field access — use secrets.unwrap.
		if so.TypeName == "Secret" {
			return runtime.Null(), fmt.Errorf("Secret fields are sealed; use secrets.unwrap")
		}
		v, ok := so.Fields[name]
		if !ok {
			return runtime.Null(), fmt.Errorf("no field %q on %s", name, so.TypeName)
		}
		return v, nil
	case runtime.KindMap:
		mo := x.Obj.(*runtime.MapObj)
		v, ok := mo.Vals[name]
		if !ok {
			return runtime.Null(), fmt.Errorf("no key %q", name)
		}
		return v, nil
	case runtime.KindResult:
		ro := x.Obj.(*runtime.ResultObj)
		switch name {
		case "ok", "is_ok":
			return runtime.Bool(ro.Ok), nil
		case "is_err":
			return runtime.Bool(!ro.Ok), nil
		case "value", "val":
			if ro.Ok {
				return ro.Val, nil
			}
			return runtime.Null(), nil
		case "err", "error":
			if !ro.Ok {
				return ro.Err, nil
			}
			return runtime.Null(), nil
		// Methods (bound) — error management without try/catch
		case "unwrap_or":
			ok, val := ro.Ok, ro.Val
			return runtime.MakeBuiltin("Result.unwrap_or", 1, func(args []runtime.Value) (runtime.Value, error) {
				if ok {
					return val, nil
				}
				if len(args) < 1 {
					return runtime.Null(), nil
				}
				return args[0], nil
			}), nil
		case "context":
			// r.context("while loading") → Result; annotates Err with outer message
			ok, val, errV := ro.Ok, ro.Val, ro.Err
			return runtime.MakeBuiltin("Result.context", 1, func(args []runtime.Value) (runtime.Value, error) {
				if ok {
					return runtime.Ok(val), nil
				}
				ctx := "error"
				if len(args) >= 1 && args[0].String() != "" {
					ctx = args[0].String()
				}
				return runtime.Err(runtime.WrapError(errV, ctx)), nil
			}), nil
		case "expect":
			// r.expect("must exist") → Result; replaces message on Err
			ok, val, errV := ro.Ok, ro.Val, ro.Err
			return runtime.MakeBuiltin("Result.expect", 1, func(args []runtime.Value) (runtime.Value, error) {
				if ok {
					return runtime.Ok(val), nil
				}
				msg := "expectation failed"
				if len(args) >= 1 && args[0].String() != "" {
					msg = args[0].String()
				}
				return runtime.Err(runtime.WrapError(errV, msg)), nil
			}), nil
		case "or":
			// r.or(other_result) → r if Ok else other
			ok, val := ro.Ok, ro.Val
			return runtime.MakeBuiltin("Result.or", 1, func(args []runtime.Value) (runtime.Value, error) {
				if ok {
					return runtime.Ok(val), nil
				}
				if len(args) < 1 {
					return runtime.Err(runtime.NewError("Result.or(fallback)", "result")), nil
				}
				return args[0], nil
			}), nil
		case "unwrap":
			// hard unwrap — returns value or Err-as-Result for use with ? : prefer r?
			ok, val, errV := ro.Ok, ro.Val, ro.Err
			return runtime.MakeBuiltin("Result.unwrap", 0, func(args []runtime.Value) (runtime.Value, error) {
				if ok {
					return val, nil
				}
				// return as Result Err so callers can still use ? on methods chain carefully
				// but method call of unwrap with 0 arity used as: r.unwrap() without ?
				// Design: return Value on Ok; on Err return error to VM (like test.eq)
				return runtime.Null(), fmt.Errorf("%s", runtime.ErrorMessage(errV))
			}), nil
		}
		return runtime.Null(), fmt.Errorf("no field %q on Result", name)
	default:
		return runtime.Null(), fmt.Errorf("cannot get field on %s", x.KindName())
	}
}

func setField(x runtime.Value, name string, val runtime.Value) error {
	switch x.Kind {
	case runtime.KindStruct:
		so := x.Obj.(*runtime.StructObj)
		if _, ok := so.Fields[name]; !ok {
			so.Order = append(so.Order, name)
		}
		so.Fields[name] = val
		return nil
	case runtime.KindMap:
		mo := x.Obj.(*runtime.MapObj)
		if _, ok := mo.Vals[name]; !ok {
			mo.Keys = append(mo.Keys, name)
		}
		mo.Vals[name] = val
		return nil
	default:
		return fmt.Errorf("cannot set field on %s", x.KindName())
	}
}

func newIter(x runtime.Value) (runtime.Iter, error) {
	return runtime.AsIter(x)
}
