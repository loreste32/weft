package types

import (
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/ast"
)

// Kind classifies a Weft type for inference.
type Kind int

const (
	TyAny Kind = iota
	TyUnit
	TyNull
	TyBool
	TyInt
	TyFloat
	TyStr
	TyList
	TyMap
	TyResult
	TyOptional
	TyFn
	TyNamed // user type / package / opaque
	TyChannel
)

// Type is an inferred or declared Weft type.
type Type struct {
	Kind   Kind
	Name   string // for named / package
	Elem   *Type  // list/result/optional
	Key    *Type  // map key
	Params []*Type
	Ret    *Type
}

func (t *Type) String() string {
	if t == nil {
		return "any"
	}
	switch t.Kind {
	case TyAny:
		return "any"
	case TyUnit:
		return "unit"
	case TyNull:
		return "null"
	case TyBool:
		return "bool"
	case TyInt:
		return "int"
	case TyFloat:
		return "float"
	case TyStr:
		return "str"
	case TyList:
		return "[" + t.Elem.String() + "]"
	case TyMap:
		return "Map[" + t.Key.String() + "," + t.Elem.String() + "]"
	case TyResult:
		return "Result[" + t.Elem.String() + "]"
	case TyOptional:
		return t.Elem.String() + "?"
	case TyChannel:
		return "channel"
	case TyFn:
		ps := make([]string, len(t.Params))
		for i, p := range t.Params {
			ps[i] = p.String()
		}
		ret := "unit"
		if t.Ret != nil {
			ret = t.Ret.String()
		}
		return "fn(" + strings.Join(ps, ", ") + ") -> " + ret
	case TyNamed:
		if t.Name != "" {
			return t.Name
		}
		return "named"
	default:
		return "?"
	}
}

func tyAny() *Type     { return &Type{Kind: TyAny} }
func tyUnit() *Type    { return &Type{Kind: TyUnit} }
func tyNull() *Type    { return &Type{Kind: TyNull} }
func tyBool() *Type    { return &Type{Kind: TyBool} }
func tyInt() *Type     { return &Type{Kind: TyInt} }
func tyFloat() *Type   { return &Type{Kind: TyFloat} }
func tyStr() *Type     { return &Type{Kind: TyStr} }
func tyChannel() *Type { return &Type{Kind: TyChannel} }

func tyList(e *Type) *Type {
	if e == nil {
		e = tyAny()
	}
	return &Type{Kind: TyList, Elem: e}
}
func tyMap(k, v *Type) *Type {
	if k == nil {
		k = tyStr()
	}
	if v == nil {
		v = tyAny()
	}
	return &Type{Kind: TyMap, Key: k, Elem: v}
}
func tyResult(e *Type) *Type {
	if e == nil {
		e = tyAny()
	}
	return &Type{Kind: TyResult, Elem: e}
}
func tyOpt(e *Type) *Type {
	if e == nil {
		e = tyAny()
	}
	return &Type{Kind: TyOptional, Elem: e}
}
func tyNamed(name string) *Type { return &Type{Kind: TyNamed, Name: name} }
func tyFn(params []*Type, ret *Type) *Type {
	if ret == nil {
		ret = tyUnit()
	}
	return &Type{Kind: TyFn, Params: params, Ret: ret}
}

// Equal reports structural equality (any matches anything for equality of shape checks).
func (t *Type) Equal(o *Type) bool {
	if t == nil || o == nil {
		return true
	}
	if t.Kind == TyAny || o.Kind == TyAny {
		return true
	}
	if t.Kind != o.Kind {
		// int/float soft equal for numbers?
		if (t.Kind == TyInt && o.Kind == TyFloat) || (t.Kind == TyFloat && o.Kind == TyInt) {
			return true
		}
		return false
	}
	switch t.Kind {
	case TyList, TyResult, TyOptional:
		return t.Elem.Equal(o.Elem)
	case TyMap:
		return t.Key.Equal(o.Key) && t.Elem.Equal(o.Elem)
	case TyFn:
		if len(t.Params) != len(o.Params) {
			return false
		}
		for i := range t.Params {
			if !t.Params[i].Equal(o.Params[i]) {
				return false
			}
		}
		return t.Ret.Equal(o.Ret)
	case TyNamed:
		return t.Name == o.Name || t.Name == "" || o.Name == ""
	default:
		return true
	}
}

// Assignable: can value of `from` be used where `to` is expected?
func Assignable(to, from *Type) bool {
	if to == nil || from == nil {
		return true
	}
	if to.Kind == TyAny || from.Kind == TyAny {
		return true
	}
	if from.Kind == TyNull {
		return to.Kind == TyOptional || to.Kind == TyNull || to.Kind == TyAny
	}
	if to.Kind == TyOptional {
		return Assignable(to.Elem, from) || from.Kind == TyNull
	}
	if to.Kind == TyFloat && from.Kind == TyInt {
		return true
	}
	if to.Kind == TyResult && from.Kind != TyResult {
		// bare T can wrap to Result[T] on return
		return Assignable(to.Elem, from)
	}
	return to.Equal(from)
}

// FromAST converts an AST type expression to a Type.
func FromAST(t ast.TypeExpr) *Type {
	if t == nil {
		return tyAny()
	}
	switch t := t.(type) {
	case *ast.NamedType:
		switch t.Name {
		case "int":
			return tyInt()
		case "float":
			return tyFloat()
		case "str":
			return tyStr()
		case "bool":
			return tyBool()
		case "unit":
			return tyUnit()
		case "any":
			return tyAny()
		case "channel":
			return tyChannel()
		default:
			return tyNamed(t.Name)
		}
	case *ast.ListType:
		return tyList(FromAST(t.Element))
	case *ast.MapType:
		return tyMap(FromAST(t.Key), FromAST(t.Value))
	case *ast.ResultType:
		return tyResult(FromAST(t.Ok))
	case *ast.OptionalType:
		return tyOpt(FromAST(t.Element))
	case *ast.StructType:
		return tyNamed("struct")
	default:
		return tyAny()
	}
}

// Unify returns a type that can represent both (lub), or any.
func Unify(a, b *Type) *Type {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Kind == TyAny {
		return b
	}
	if b.Kind == TyAny {
		return a
	}
	if a.Equal(b) {
		return a
	}
	if a.Kind == TyInt && b.Kind == TyFloat {
		return tyFloat()
	}
	if a.Kind == TyFloat && b.Kind == TyInt {
		return tyFloat()
	}
	if a.Kind == TyList && b.Kind == TyList {
		return tyList(Unify(a.Elem, b.Elem))
	}
	return tyAny()
}

func fmtType(t *Type) string {
	if t == nil {
		return "any"
	}
	return t.String()
}

// ensure unused import safety for fmt in errors elsewhere
var _ = fmt.Sprintf
