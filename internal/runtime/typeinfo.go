package runtime

// TypeInfo is a runtime type descriptor used for schema/tool reflection.
type TypeInfo struct {
	Name   string
	Kind   string // "struct", "alias", "builtin", "list", "map", "result", "optional"
	Alias  *TypeInfo
	Fields []FieldInfo
	// for list/map/result/optional
	Elem *TypeInfo
	Key  *TypeInfo
}

// FieldInfo describes a struct field.
type FieldInfo struct {
	Name     string
	Type     *TypeInfo
	Optional bool
	Default  *Value // optional default
}

// Builtin type infos.
var (
	TypeStr   = &TypeInfo{Name: "str", Kind: "builtin"}
	TypeInt   = &TypeInfo{Name: "int", Kind: "builtin"}
	TypeFloat = &TypeInfo{Name: "float", Kind: "builtin"}
	TypeBool  = &TypeInfo{Name: "bool", Kind: "builtin"}
	TypeUnit  = &TypeInfo{Name: "unit", Kind: "struct", Fields: nil}
	TypeAny   = &TypeInfo{Name: "any", Kind: "builtin"}
)

// TypeInfoValue wraps TypeInfo as a Value.
func TypeInfoValue(t *TypeInfo) Value {
	return Value{Kind: KindTypeInfo, Obj: t}
}
