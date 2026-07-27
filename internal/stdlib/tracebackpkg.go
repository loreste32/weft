package stdlib

import (
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageTraceback — format Result/errors for agents and CLIs.
func packageTraceback() runtime.Value {
	p := pkg()

	// traceback.format_err(err_or_result) -> str
	set(p, "format_err", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(formatErr(args[0])), nil
	}, 1)

	// traceback.is_err(v) -> bool
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

	// traceback.err_msg(result) -> str|null
	set(p, "err_msg", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindResult {
			return runtime.Null(), nil
		}
		ro := args[0].Obj.(*runtime.ResultObj)
		if ro.Ok {
			return runtime.Null(), nil
		}
		return runtime.Str(ro.Err.String()), nil
	}, 1)

	// traceback.print_err(err) -> unit  (returns formatted string; caller may say())
	set(p, "format", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(formatErr(args[0])), nil
	}, 1)

	return p
}

func formatErr(v runtime.Value) string {
	if v.Kind == runtime.KindResult {
		ro := v.Obj.(*runtime.ResultObj)
		if ro.Ok {
			return ""
		}
		return formatErr(ro.Err)
	}
	if v.Kind == runtime.KindStruct {
		so := v.Obj.(*runtime.StructObj)
		if so.TypeName == "Error" || so.TypeName == "Err" {
			msg := ""
			kind := ""
			if m, ok := so.Fields["msg"]; ok {
				msg = m.String()
			} else if m, ok := so.Fields["message"]; ok {
				msg = m.String()
			}
			if k, ok := so.Fields["kind"]; ok {
				kind = k.String()
			}
			if kind != "" {
				return fmt.Sprintf("Error(%s): %s", kind, msg)
			}
			if msg != "" {
				return "Error: " + msg
			}
		}
	}
	s := v.String()
	if strings.TrimSpace(s) == "" {
		return "Error"
	}
	return s
}
