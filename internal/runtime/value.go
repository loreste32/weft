// Package runtime defines Weft runtime values and builtins.
package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind tags a Value.
type Kind uint8

const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindStr
	KindList
	KindMap
	KindStruct
	KindFunc
	KindBuiltin
	KindClosure
	KindResult
	KindTypeInfo
	KindUnit
	KindIter
)

// Value is a tagged runtime value.
type Value struct {
	Kind Kind
	// payloads (only one used by Kind)
	B bool
	I int64
	F float64
	S string
	// heap objects
	Obj any
}

// Null, Unit, Bool helpers.
func Null() Value           { return Value{Kind: KindNull} }
func Unit() Value           { return Value{Kind: KindUnit} }
func Bool(b bool) Value     { return Value{Kind: KindBool, B: b} }
func Int(i int64) Value     { return Value{Kind: KindInt, I: i} }
func Float(f float64) Value { return Value{Kind: KindFloat, F: f} }
func Str(s string) Value    { return Value{Kind: KindStr, S: s} }

// ListObj is a mutable list.
type ListObj struct {
	Items []Value
}

func List(items ...Value) Value {
	cp := make([]Value, len(items))
	copy(cp, items)
	return Value{Kind: KindList, Obj: &ListObj{Items: cp}}
}

// MapObj is a string-keyed map for v1 simplicity (keys coerced to string for non-str).
type MapObj struct {
	// ordered for determinism
	Keys []string
	Vals map[string]Value
}

func NewMap() Value {
	return Value{Kind: KindMap, Obj: &MapObj{Vals: make(map[string]Value)}}
}

// StructObj is an instance of a named/anonymous struct.
type StructObj struct {
	TypeName string
	Fields   map[string]Value
	Order    []string
}

func Struct(name string, fields map[string]Value, order []string) Value {
	if fields == nil {
		fields = make(map[string]Value)
	}
	return Value{Kind: KindStruct, Obj: &StructObj{TypeName: name, Fields: fields, Order: order}}
}

// FuncObj is a compiled function.
type FuncObj struct {
	Name     string
	Arity    int
	Chunk    any // *compile.Chunk — avoid cycle; set by compiler
	TypeInfo *TypeInfo
	// Env is the module environment to run in (nil = caller's env).
	Env *Env
	// for closures
	Upvalues []Value
}

func Func(f *FuncObj) Value { return Value{Kind: KindFunc, Obj: f} }

// Builtin is a native function.
type Builtin func(args []Value) (Value, error)

type BuiltinObj struct {
	Name  string
	Fn    Builtin
	Arity int // -1 = variadic
}

func MakeBuiltin(name string, arity int, fn Builtin) Value {
	return Value{Kind: KindBuiltin, Obj: &BuiltinObj{Name: name, Fn: fn, Arity: arity}}
}

// ResultObj is Ok(T) or Err(Error).
type ResultObj struct {
	Ok  bool
	Val Value
	Err Value // Error struct when !Ok
}

func Ok(v Value) Value {
	return Value{Kind: KindResult, Obj: &ResultObj{Ok: true, Val: v}}
}

func Err(e Value) Value {
	return Value{Kind: KindResult, Obj: &ResultObj{Ok: false, Err: e}}
}

// NewError builds a user Error struct.
// Fields: message, kind, code, cause, at (source location when set by ?).
func NewError(message, kind string) Value {
	if kind == "" {
		kind = "user"
	}
	return Struct("Error", map[string]Value{
		"message": Str(message),
		"kind":    Str(kind),
		"code":    Null(),
		"cause":   Null(),
		"at":      Null(),
	}, []string{"message", "kind", "code", "cause", "at"})
}

// NewErrorCode builds an Error with a numeric/string code (HTTP status, errno, …).
func NewErrorCode(message, kind string, code Value) Value {
	e := NewError(message, kind)
	so := e.Obj.(*StructObj)
	so.Fields["code"] = code
	return e
}

// WrapError nests cause under a new context message (like fmt.Errorf("%w")).
// If cause is an Error, message becomes "context: original" and kind is preserved.
func WrapError(cause Value, context string) Value {
	kind := "wrap"
	msg := context
	if context == "" {
		msg = "error"
	}
	if cause.Kind == KindStruct {
		if so, ok := cause.Obj.(*StructObj); ok && so.TypeName == "Error" {
			if k, ok := so.Fields["kind"]; ok && k.Kind == KindStr && k.S != "" {
				kind = k.S
			}
			if m, ok := so.Fields["message"]; ok {
				if context != "" {
					msg = context + ": " + m.String()
				} else {
					msg = m.String()
				}
			}
		}
	}
	e := NewError(msg, kind)
	so := e.Obj.(*StructObj)
	so.Fields["cause"] = cause
	// preserve code from nested Error
	if cause.Kind == KindStruct {
		if cso, ok := cause.Obj.(*StructObj); ok && cso.TypeName == "Error" {
			if c, ok := cso.Fields["code"]; ok {
				so.Fields["code"] = c
			}
			if at, ok := cso.Fields["at"]; ok && at.Kind != KindNull {
				so.Fields["at"] = at
			}
		}
	}
	return e
}

// ErrorWithLocation sets Error.at if the value is an Error and at is empty.
func ErrorWithLocation(err Value, at string) Value {
	if at == "" || err.Kind != KindStruct {
		return err
	}
	so, ok := err.Obj.(*StructObj)
	if !ok || so.TypeName != "Error" {
		return err
	}
	if cur, ok := so.Fields["at"]; ok && cur.Kind != KindNull && cur.String() != "" {
		return err
	}
	// copy fields so we don't mutate shared Error values
	fields := make(map[string]Value, len(so.Fields)+1)
	for k, v := range so.Fields {
		fields[k] = v
	}
	fields["at"] = Str(at)
	order := append([]string(nil), so.Order...)
	if _, ok := fields["at"]; ok {
		has := false
		for _, k := range order {
			if k == "at" {
				has = true
				break
			}
		}
		if !has {
			order = append(order, "at")
		}
	}
	return Struct("Error", fields, order)
}

// IsError reports whether v is an Error struct.
func IsError(v Value) bool {
	if v.Kind != KindStruct {
		return false
	}
	so, ok := v.Obj.(*StructObj)
	return ok && so.TypeName == "Error"
}

// ErrorMessage extracts a readable message from Err payload or Error struct.
func ErrorMessage(v Value) string {
	if IsError(v) {
		if m, ok := v.Obj.(*StructObj).Fields["message"]; ok {
			return m.String()
		}
	}
	if v.Kind == KindResult {
		ro := v.Obj.(*ResultObj)
		if !ro.Ok {
			return ErrorMessage(ro.Err)
		}
	}
	return v.String()
}

func (v Value) IsTruthy() bool {
	switch v.Kind {
	case KindNull, KindUnit:
		return false
	case KindBool:
		return v.B
	case KindInt:
		return v.I != 0
	case KindFloat:
		return v.F != 0
	case KindStr:
		return v.S != ""
	case KindList:
		return len(v.Obj.(*ListObj).Items) > 0
	case KindMap:
		return len(v.Obj.(*MapObj).Vals) > 0
	case KindResult:
		return v.Obj.(*ResultObj).Ok
	default:
		return true
	}
}

func (v Value) String() string {
	switch v.Kind {
	case KindNull:
		return "null"
	case KindUnit:
		return "unit"
	case KindBool:
		if v.B {
			return "true"
		}
		return "false"
	case KindInt:
		return strconv.FormatInt(v.I, 10)
	case KindFloat:
		return strconv.FormatFloat(v.F, 'g', -1, 64)
	case KindStr:
		return v.S
	case KindList:
		lo := v.Obj.(*ListObj)
		parts := make([]string, len(lo.Items))
		for i, it := range lo.Items {
			parts[i] = it.String()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case KindMap:
		mo := v.Obj.(*MapObj)
		parts := make([]string, 0, len(mo.Keys))
		for _, k := range mo.Keys {
			parts = append(parts, fmt.Sprintf("%q: %s", k, mo.Vals[k].String()))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case KindStruct:
		so := v.Obj.(*StructObj)
		if so.TypeName == "Secret" {
			return "***"
		}
		parts := make([]string, 0, len(so.Order))
		for _, k := range so.Order {
			parts = append(parts, fmt.Sprintf("%s: %s", k, so.Fields[k].String()))
		}
		name := so.TypeName
		if name == "" {
			name = "struct"
		}
		return name + "{" + strings.Join(parts, ", ") + "}"
	case KindFunc:
		return "<fn " + v.Obj.(*FuncObj).Name + ">"
	case KindBuiltin:
		return "<builtin " + v.Obj.(*BuiltinObj).Name + ">"
	case KindResult:
		ro := v.Obj.(*ResultObj)
		if ro.Ok {
			return "Ok(" + ro.Val.String() + ")"
		}
		return "Err(" + ro.Err.String() + ")"
	case KindTypeInfo:
		ti := v.Obj.(*TypeInfo)
		return "<type " + ti.Name + ">"
	case KindIter:
		return "<iter>"
	default:
		return "<unknown>"
	}
}

// Equal is structural value equality.
func Equal(a, b Value) bool {
	if a.Kind != b.Kind {
		if a.Kind == KindInt && b.Kind == KindFloat {
			return float64(a.I) == b.F
		}
		if a.Kind == KindFloat && b.Kind == KindInt {
			return a.F == float64(b.I)
		}
		return false
	}
	switch a.Kind {
	case KindNull, KindUnit:
		return true
	case KindBool:
		return a.B == b.B
	case KindInt:
		return a.I == b.I
	case KindFloat:
		return a.F == b.F
	case KindStr:
		return a.S == b.S
	case KindList:
		la, lb := a.Obj.(*ListObj), b.Obj.(*ListObj)
		if len(la.Items) != len(lb.Items) {
			return false
		}
		for i := range la.Items {
			if !equal(la.Items[i], lb.Items[i]) {
				return false
			}
		}
		return true
	case KindMap:
		ma, mb := a.Obj.(*MapObj), b.Obj.(*MapObj)
		if len(ma.Vals) != len(mb.Vals) {
			return false
		}
		for k, va := range ma.Vals {
			vb, ok := mb.Vals[k]
			if !ok || !equal(va, vb) {
				return false
			}
		}
		return true
	case KindStruct:
		sa, sb := a.Obj.(*StructObj), b.Obj.(*StructObj)
		if sa.TypeName != sb.TypeName || len(sa.Fields) != len(sb.Fields) {
			return false
		}
		for k, va := range sa.Fields {
			vb, ok := sb.Fields[k]
			if !ok || !equal(va, vb) {
				return false
			}
		}
		return true
	case KindResult:
		ra, rb := a.Obj.(*ResultObj), b.Obj.(*ResultObj)
		if ra.Ok != rb.Ok {
			return false
		}
		if ra.Ok {
			return equal(ra.Val, rb.Val)
		}
		return equal(ra.Err, rb.Err)
	default:
		return a.Obj == b.Obj
	}
}

func equal(a, b Value) bool { return Equal(a, b) }

// AsInt coerces to int64.
func AsInt(v Value) (int64, error) {
	switch v.Kind {
	case KindInt:
		return v.I, nil
	case KindFloat:
		return int64(v.F), nil
	case KindStr:
		return strconv.ParseInt(v.S, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %s to int", v.KindName())
	}
}

// KindName returns a readable kind name.
func (v Value) KindName() string {
	switch v.Kind {
	case KindNull:
		return "null"
	case KindBool:
		return "bool"
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindStr:
		return "str"
	case KindList:
		return "list"
	case KindMap:
		return "map"
	case KindStruct:
		return "struct"
	case KindFunc:
		return "fn"
	case KindBuiltin:
		return "builtin"
	case KindResult:
		return "Result"
	case KindTypeInfo:
		return "TypeInfo"
	case KindUnit:
		return "unit"
	case KindIter:
		return "Iter"
	default:
		return "?"
	}
}

// DeepCopy copies lists/maps/structs (for spawn later).
func DeepCopy(v Value) Value {
	switch v.Kind {
	case KindList:
		lo := v.Obj.(*ListObj)
		items := make([]Value, len(lo.Items))
		for i, it := range lo.Items {
			items[i] = DeepCopy(it)
		}
		return List(items...)
	case KindMap:
		mo := v.Obj.(*MapObj)
		nm := &MapObj{Vals: make(map[string]Value, len(mo.Vals)), Keys: append([]string{}, mo.Keys...)}
		for k, val := range mo.Vals {
			nm.Vals[k] = DeepCopy(val)
		}
		return Value{Kind: KindMap, Obj: nm}
	case KindStruct:
		so := v.Obj.(*StructObj)
		fields := make(map[string]Value, len(so.Fields))
		for k, val := range so.Fields {
			fields[k] = DeepCopy(val)
		}
		return Struct(so.TypeName, fields, append([]string{}, so.Order...))
	default:
		return v
	}
}
