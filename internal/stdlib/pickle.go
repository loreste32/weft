package stdlib

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"os"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packagePickle — binary marshal via gob (Python pickle analog).
// Loads require WEFT_ALLOW_PICKLE=1 (arbitrary object decode is unsafe).
func packagePickle() runtime.Value {
	p := pkg()

	// pickle.dumps(v) -> Result[str]  base64-encoded gob
	set(p, "dumps", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("pickle.dumps(v)", "pickle"), nil
		}
		raw, err := gobEncodeValue(args[0])
		if err != nil {
			return errRes(err.Error(), "pickle"), nil
		}
		return runtime.Ok(runtime.Str(base64.StdEncoding.EncodeToString(raw))), nil
	}, 1)

	// pickle.loads(s) -> Result[any]  requires WEFT_ALLOW_PICKLE=1
	set(p, "loads", func(args []runtime.Value) (runtime.Value, error) {
		if !pickleAllowed() {
			return errRes("pickle.loads disabled — set WEFT_ALLOW_PICKLE=1 (untrusted data is unsafe)", "pickle"), nil
		}
		if len(args) < 1 {
			return errRes("pickle.loads(s)", "pickle"), nil
		}
		raw, err := base64.StdEncoding.DecodeString(args[0].String())
		if err != nil {
			return errRes(err.Error(), "pickle"), nil
		}
		v, err := gobDecodeValue(raw)
		if err != nil {
			return errRes(err.Error(), "pickle"), nil
		}
		return runtime.Ok(v), nil
	}, 1)

	// pickle.dump(path, v) / load(path)
	set(p, "dump", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("pickle.dump(path, v)", "pickle"), nil
		}
		raw, err := gobEncodeValue(args[1])
		if err != nil {
			return errRes(err.Error(), "pickle"), nil
		}
		if err := os.WriteFile(args[0].String(), raw, 0o644); err != nil {
			return errRes(err.Error(), "pickle"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	set(p, "load", func(args []runtime.Value) (runtime.Value, error) {
		if !pickleAllowed() {
			return errRes("pickle.load disabled — set WEFT_ALLOW_PICKLE=1", "pickle"), nil
		}
		if len(args) < 1 {
			return errRes("pickle.load(path)", "pickle"), nil
		}
		raw, err := os.ReadFile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "pickle"), nil
		}
		v, err := gobDecodeValue(raw)
		if err != nil {
			return errRes(err.Error(), "pickle"), nil
		}
		return runtime.Ok(v), nil
	}, 1)

	return p
}

func pickleAllowed() bool {
	v := strings.TrimSpace(os.Getenv("WEFT_ALLOW_PICKLE"))
	return v == "1" || strings.EqualFold(v, "true") || v == "yes"
}

// gob wire uses a simplified JSON-like tree of Go types.
type gobTree struct {
	Kind    string
	Str     string
	Int     int64
	Float   float64
	Bool    bool
	List    []gobTree
	MapKeys []string
	MapVals []gobTree
}

func gobEncodeValue(v runtime.Value) ([]byte, error) {
	t := valueToGobTree(v)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(t); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gobDecodeValue(raw []byte) (runtime.Value, error) {
	var t gobTree
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&t); err != nil {
		return runtime.Null(), err
	}
	return gobTreeToValue(t), nil
}

func valueToGobTree(v runtime.Value) gobTree {
	switch v.Kind {
	case runtime.KindNull, runtime.KindUnit:
		return gobTree{Kind: "null"}
	case runtime.KindBool:
		return gobTree{Kind: "bool", Bool: v.B}
	case runtime.KindInt:
		return gobTree{Kind: "int", Int: v.I}
	case runtime.KindFloat:
		return gobTree{Kind: "float", Float: v.F}
	case runtime.KindStr:
		return gobTree{Kind: "str", Str: v.S}
	case runtime.KindList:
		items := v.Obj.(*runtime.ListObj).Items
		list := make([]gobTree, len(items))
		for i, it := range items {
			list[i] = valueToGobTree(it)
		}
		return gobTree{Kind: "list", List: list}
	case runtime.KindMap:
		mo := v.Obj.(*runtime.MapObj)
		keys := append([]string{}, mo.Keys...)
		// include any keys only in Vals
		seen := map[string]bool{}
		for _, k := range keys {
			seen[k] = true
		}
		for k := range mo.Vals {
			if !seen[k] {
				keys = append(keys, k)
			}
		}
		vals := make([]gobTree, len(keys))
		for i, k := range keys {
			vals[i] = valueToGobTree(mo.Vals[k])
		}
		return gobTree{Kind: "map", MapKeys: keys, MapVals: vals}
	default:
		return gobTree{Kind: "str", Str: v.String()}
	}
}

func gobTreeToValue(t gobTree) runtime.Value {
	switch t.Kind {
	case "null", "":
		return runtime.Null()
	case "bool":
		return runtime.Bool(t.Bool)
	case "int":
		return runtime.Int(t.Int)
	case "float":
		return runtime.Float(t.Float)
	case "str":
		return runtime.Str(t.Str)
	case "list":
		items := make([]runtime.Value, len(t.List))
		for i, it := range t.List {
			items[i] = gobTreeToValue(it)
		}
		return runtime.List(items...)
	case "map":
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		for i, k := range t.MapKeys {
			mo.Keys = append(mo.Keys, k)
			if i < len(t.MapVals) {
				mo.Vals[k] = gobTreeToValue(t.MapVals[i])
			} else {
				mo.Vals[k] = runtime.Null()
			}
		}
		return m
	default:
		return runtime.Str(fmt.Sprintf("unknown pickle kind %q", t.Kind))
	}
}
