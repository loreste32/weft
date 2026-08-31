package dap

// eval.go implements debug-console expression evaluation against a paused
// frame. The expression is parsed with the real Weft parser
// (parse.ParseExpr) and then walked by a small recursive evaluator — the
// compiler/VM cannot be safely re-entered from the debug pause hook, so
// bytecode execution is not used.
//
// Supported subset:
//   - literals: int, float, string, raw string, bool, null, unit
//   - identifiers: frame locals first, then VM globals (read-only)
//   - field access x.y (structs, maps, Result ok/value/err-style fields)
//   - index access x[i] (lists, maps, strings)
//   - unary: -x, !x
//   - binary: + - * / % == != < <= > >= && || ?? (VM semantics:
//     &&/|| keep the deciding operand's value, ?? yields the right side
//     only when the left is null)
//   - list/map literals, f-strings, parenthesized expressions
//
// NOT supported: function calls of any kind (they would require re-entering
// the VM from the pause hook), match/if expressions, struct literals,
// closures, and the ? unwrap operator.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/token"
)

type evaluator struct {
	locals map[string]runtime.Value
	env    *runtime.Env
}

// evalExpression parses and evaluates src against the given frame locals.
func evalExpression(src string, locals map[string]runtime.Value, env *runtime.Env) (runtime.Value, error) {
	ex, errs := parse.ParseExpr("<debug>", src)
	if errs.HasErrors() {
		return runtime.Null(), fmt.Errorf("parse: %v", errs)
	}
	ev := &evaluator{locals: locals, env: env}
	return ev.eval(ex)
}

// assignExpression evaluates valueSrc and writes the result into target,
// which must be an identifier (written through setLocal into the VM frame)
// or a field/index chain rooted at a frame local (mutated in place — the
// runtime objects are shared with the paused frame).
func assignExpression(target, valueSrc string, locals map[string]runtime.Value, env *runtime.Env, setLocal func(string, runtime.Value) bool) (runtime.Value, error) {
	val, err := evalExpression(valueSrc, locals, env)
	if err != nil {
		return runtime.Null(), err
	}
	ex, errs := parse.ParseExpr("<debug>", target)
	if errs.HasErrors() {
		return runtime.Null(), fmt.Errorf("parse target: %v", errs)
	}
	ev := &evaluator{locals: locals, env: env}
	switch t := ex.(type) {
	case *ast.Ident:
		if setLocal == nil || !setLocal(t.Name, val) {
			return runtime.Null(), fmt.Errorf("unknown or unwritable local %q", t.Name)
		}
		return val, nil
	case *ast.FieldExpr:
		container, err := ev.eval(t.X)
		if err != nil {
			return runtime.Null(), err
		}
		if err := setFieldValue(container, t.Name, val); err != nil {
			return runtime.Null(), err
		}
		return val, nil
	case *ast.IndexExpr:
		container, err := ev.eval(t.X)
		if err != nil {
			return runtime.Null(), err
		}
		idx, err := ev.eval(t.Index)
		if err != nil {
			return runtime.Null(), err
		}
		if err := setIndexValue(container, idx, val); err != nil {
			return runtime.Null(), err
		}
		return val, nil
	default:
		return runtime.Null(), fmt.Errorf("not assignable: %s", target)
	}
}

func (ev *evaluator) eval(ex ast.Expr) (runtime.Value, error) {
	switch e := ex.(type) {
	case *ast.BasicLit:
		return litValue(e)
	case *ast.Ident:
		if e.Name == "unit" {
			return runtime.Unit(), nil
		}
		if v, ok := ev.locals[e.Name]; ok {
			return v, nil
		}
		if ev.env != nil {
			if v, ok := ev.env.Get(e.Name); ok {
				return v, nil
			}
		}
		return runtime.Null(), fmt.Errorf("undefined: %s", e.Name)
	case *ast.UnaryExpr:
		return ev.evalUnary(e)
	case *ast.BinaryExpr:
		return ev.evalBinary(e)
	case *ast.FieldExpr:
		x, err := ev.eval(e.X)
		if err != nil {
			return runtime.Null(), err
		}
		return getFieldValue(x, e.Name)
	case *ast.IndexExpr:
		x, err := ev.eval(e.X)
		if err != nil {
			return runtime.Null(), err
		}
		idx, err := ev.eval(e.Index)
		if err != nil {
			return runtime.Null(), err
		}
		return getIndexValue(x, idx)
	case *ast.ListLit:
		items := make([]runtime.Value, 0, len(e.Elts))
		for _, el := range e.Elts {
			v, err := ev.eval(el)
			if err != nil {
				return runtime.Null(), err
			}
			items = append(items, v)
		}
		return runtime.List(items...), nil
	case *ast.MapLit:
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		for i, ke := range e.Keys {
			kv, err := ev.eval(ke)
			if err != nil {
				return runtime.Null(), err
			}
			vv, err := ev.eval(e.Vals[i])
			if err != nil {
				return runtime.Null(), err
			}
			ks := kv.String()
			if _, exists := mo.Vals[ks]; !exists {
				mo.Keys = append(mo.Keys, ks)
			}
			mo.Vals[ks] = vv
		}
		return m, nil
	case *ast.FStringExpr:
		var b strings.Builder
		for _, p := range e.Parts {
			if p.Expr == nil {
				b.WriteString(p.Text)
				continue
			}
			v, err := ev.eval(p.Expr)
			if err != nil {
				return runtime.Null(), err
			}
			b.WriteString(v.String())
		}
		return runtime.Str(b.String()), nil
	case *ast.CallExpr:
		return runtime.Null(), fmt.Errorf("function calls are not supported in debug evaluate")
	default:
		return runtime.Null(), fmt.Errorf("unsupported expression: %T", ex)
	}
}

func (ev *evaluator) evalUnary(e *ast.UnaryExpr) (runtime.Value, error) {
	x, err := ev.eval(e.X)
	if err != nil {
		return runtime.Null(), err
	}
	switch e.Op {
	case token.Minus:
		switch x.Kind {
		case runtime.KindInt:
			return runtime.Int(-x.I), nil
		case runtime.KindFloat:
			return runtime.Float(-x.F), nil
		default:
			return runtime.Null(), fmt.Errorf("unary - on %s", x.KindName())
		}
	case token.Bang:
		return runtime.Bool(!x.IsTruthy()), nil
	default:
		return runtime.Null(), fmt.Errorf("unsupported unary operator %v", e.Op)
	}
}

func (ev *evaluator) evalBinary(e *ast.BinaryExpr) (runtime.Value, error) {
	// Short-circuiting operators mirror the VM's keep-value semantics.
	switch e.Op {
	case token.And:
		x, err := ev.eval(e.X)
		if err != nil {
			return runtime.Null(), err
		}
		if !x.IsTruthy() {
			return x, nil
		}
		return ev.eval(e.Y)
	case token.Or:
		x, err := ev.eval(e.X)
		if err != nil {
			return runtime.Null(), err
		}
		if x.IsTruthy() {
			return x, nil
		}
		return ev.eval(e.Y)
	case token.NullCoalesce:
		x, err := ev.eval(e.X)
		if err != nil {
			return runtime.Null(), err
		}
		if x.Kind == runtime.KindNull {
			return ev.eval(e.Y)
		}
		return x, nil
	}
	a, err := ev.eval(e.X)
	if err != nil {
		return runtime.Null(), err
	}
	b, err := ev.eval(e.Y)
	if err != nil {
		return runtime.Null(), err
	}
	switch e.Op {
	case token.Plus:
		return binAddValues(a, b)
	case token.Minus:
		return binNumValues(a, b, func(x, y float64) float64 { return x - y })
	case token.Star:
		return binNumValues(a, b, func(x, y float64) float64 { return x * y })
	case token.Slash:
		return divValues(a, b)
	case token.Percent:
		if a.Kind != runtime.KindInt || b.Kind != runtime.KindInt {
			return runtime.Null(), fmt.Errorf("%% requires ints")
		}
		if b.I == 0 {
			return runtime.Null(), fmt.Errorf("division by zero")
		}
		return runtime.Int(a.I % b.I), nil
	case token.Eq:
		return runtime.Bool(runtime.Equal(a, b)), nil
	case token.Neq:
		return runtime.Bool(!runtime.Equal(a, b)), nil
	case token.Lt, token.Lte, token.Gt, token.Gte:
		cmp, err := compareValues(a, b)
		if err != nil {
			return runtime.Null(), err
		}
		switch e.Op {
		case token.Lt:
			return runtime.Bool(cmp < 0), nil
		case token.Lte:
			return runtime.Bool(cmp <= 0), nil
		case token.Gt:
			return runtime.Bool(cmp > 0), nil
		default:
			return runtime.Bool(cmp >= 0), nil
		}
	default:
		return runtime.Null(), fmt.Errorf("unsupported operator %v", e.Op)
	}
}

// The helpers below mirror the VM's operator semantics (internal/vm/ops.go);
// they are unexported there, so the evaluator keeps its own copies.

func litValue(e *ast.BasicLit) (runtime.Value, error) {
	switch e.Kind {
	case token.Int:
		n, err := strconv.ParseInt(strings.ReplaceAll(e.Value, "_", ""), 0, 64)
		return runtime.Int(n), err
	case token.Float:
		f, err := strconv.ParseFloat(strings.ReplaceAll(e.Value, "_", ""), 64)
		return runtime.Float(f), err
	case token.String, token.RawString:
		return runtime.Str(e.Value), nil
	case token.True:
		return runtime.Bool(true), nil
	case token.False:
		return runtime.Bool(false), nil
	case token.Null:
		return runtime.Null(), nil
	default:
		return runtime.Null(), fmt.Errorf("bad literal kind")
	}
}

func binAddValues(a, b runtime.Value) (runtime.Value, error) {
	if a.Kind == runtime.KindStr || b.Kind == runtime.KindStr {
		return runtime.Str(a.String() + b.String()), nil
	}
	if a.Kind == runtime.KindInt && b.Kind == runtime.KindInt {
		return runtime.Int(a.I + b.I), nil
	}
	return binNumValues(a, b, func(x, y float64) float64 { return x + y })
}

func binNumValues(a, b runtime.Value, op func(x, y float64) float64) (runtime.Value, error) {
	if a.Kind == runtime.KindInt && b.Kind == runtime.KindInt {
		return runtime.Int(int64(op(float64(a.I), float64(b.I)))), nil
	}
	af, aok := asFloatValue(a)
	bf, bok := asFloatValue(b)
	if !aok || !bok {
		return runtime.Null(), fmt.Errorf("numeric op on %s and %s", a.KindName(), b.KindName())
	}
	return runtime.Float(op(af, bf)), nil
}

func divValues(a, b runtime.Value) (runtime.Value, error) {
	if b.Kind == runtime.KindInt && b.I == 0 {
		return runtime.Null(), fmt.Errorf("division by zero")
	}
	if b.Kind == runtime.KindFloat && b.F == 0 {
		return runtime.Null(), fmt.Errorf("division by zero")
	}
	if a.Kind == runtime.KindInt && b.Kind == runtime.KindInt {
		return runtime.Int(a.I / b.I), nil
	}
	af, aok := asFloatValue(a)
	bf, bok := asFloatValue(b)
	if !aok || !bok {
		return runtime.Null(), fmt.Errorf("numeric op on %s and %s", a.KindName(), b.KindName())
	}
	return runtime.Float(af / bf), nil
}

func asFloatValue(v runtime.Value) (float64, bool) {
	switch v.Kind {
	case runtime.KindInt:
		return float64(v.I), true
	case runtime.KindFloat:
		return v.F, true
	default:
		return 0, false
	}
}

func compareValues(a, b runtime.Value) (int, error) {
	if a.Kind == runtime.KindStr && b.Kind == runtime.KindStr {
		return strings.Compare(a.S, b.S), nil
	}
	af, aok := asFloatValue(a)
	bf, bok := asFloatValue(b)
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

func getIndexValue(x, idx runtime.Value) (runtime.Value, error) {
	switch x.Kind {
	case runtime.KindList:
		lo := x.Obj.(*runtime.ListObj)
		i, err := runtime.AsInt(idx)
		if err != nil {
			return runtime.Null(), err
		}
		if i < 0 || int(i) >= len(lo.Items) {
			return runtime.Null(), fmt.Errorf("index %d out of range (list has %d elements)", i, len(lo.Items))
		}
		return lo.Items[i], nil
	case runtime.KindMap:
		mo := x.Obj.(*runtime.MapObj)
		v, ok := mo.Vals[idx.String()]
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
			return runtime.Null(), fmt.Errorf("index %d out of range (string has %d characters)", i, len(runes))
		}
		return runtime.Str(string(runes[i])), nil
	default:
		return runtime.Null(), fmt.Errorf("cannot index %s", x.KindName())
	}
}

func setIndexValue(x, idx, val runtime.Value) error {
	switch x.Kind {
	case runtime.KindList:
		lo := x.Obj.(*runtime.ListObj)
		i, err := runtime.AsInt(idx)
		if err != nil {
			return err
		}
		if i < 0 || int(i) >= len(lo.Items) {
			return fmt.Errorf("index %d out of range (list has %d elements)", i, len(lo.Items))
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

func getFieldValue(x runtime.Value, name string) (runtime.Value, error) {
	switch x.Kind {
	case runtime.KindStruct:
		so := x.Obj.(*runtime.StructObj)
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
		}
		return runtime.Null(), fmt.Errorf("no field %q on Result", name)
	default:
		return runtime.Null(), fmt.Errorf("cannot get field on %s", x.KindName())
	}
}

func setFieldValue(x runtime.Value, name string, val runtime.Value) error {
	switch x.Kind {
	case runtime.KindStruct:
		so := x.Obj.(*runtime.StructObj)
		if so.TypeName == "Secret" {
			return fmt.Errorf("Secret fields are sealed; use secrets.unwrap")
		}
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
