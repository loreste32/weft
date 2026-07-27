package stdlib

import (
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageTraceback — format Result/Error values for CLIs and agents.
func packageTraceback() runtime.Value {
	p := pkg()
	set(p, "format", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(formatTrace(args[0])), nil
	}, 1)
	set(p, "is_err", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		v := args[0]
		if v.Kind == runtime.KindResult {
			return runtime.Bool(!v.Obj.(*runtime.ResultObj).Ok), nil
		}
		return runtime.Bool(false), nil
	}, 1)
	set(p, "err_msg", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindResult {
			return runtime.Null(), nil
		}
		ro := args[0].Obj.(*runtime.ResultObj)
		if ro.Ok {
			return runtime.Null(), nil
		}
		return runtime.Str(formatTrace(ro.Err)), nil
	}, 1)
	return p
}

func formatTrace(v runtime.Value) string {
	if v.Kind == runtime.KindResult {
		ro := v.Obj.(*runtime.ResultObj)
		if ro.Ok {
			return ""
		}
		return formatTrace(ro.Err)
	}
	if v.Kind == runtime.KindStruct {
		so := v.Obj.(*runtime.StructObj)
		msg, kind := "", ""
		if m, ok := so.Fields["msg"]; ok {
			msg = m.String()
		} else if m, ok := so.Fields["message"]; ok {
			msg = m.String()
		}
		if k, ok := so.Fields["kind"]; ok {
			kind = k.String()
		}
		if kind != "" && msg != "" {
			return fmt.Sprintf("Error(%s): %s", kind, msg)
		}
		if msg != "" {
			return "Error: " + msg
		}
	}
	s := strings.TrimSpace(v.String())
	if s == "" {
		return "Error"
	}
	return s
}
