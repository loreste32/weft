package stdlib

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageLog — structured-ish logging for production CLIs/workers.
func packageLog(env *runtime.Env) runtime.Value {
	p := pkg()
	level := "info"
	jsonMode := false

	write := func(lvl string, args []runtime.Value) {
		order := map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3}
		if order[lvl] < order[level] {
			return
		}
		fieldMap := map[string]any{}
		msgArgs := args
		if len(args) > 0 && args[len(args)-1].Kind == runtime.KindMap {
			mo := args[len(args)-1].Obj.(*runtime.MapObj)
			for _, k := range mo.Keys {
				fieldMap[k] = mo.Vals[k].String()
			}
			for k, v := range mo.Vals {
				if _, ok := fieldMap[k]; !ok {
					fieldMap[k] = v.String()
				}
			}
			msgArgs = args[:len(args)-1]
		}
		parts := make([]string, len(msgArgs))
		for i, a := range msgArgs {
			parts[i] = a.String()
		}
		msg := strings.Join(parts, " ")
		var line string
		if jsonMode {
			rec := map[string]any{
				"ts":    time.Now().UTC().Format(time.RFC3339),
				"level": lvl,
				"msg":   msg,
			}
			for k, v := range fieldMap {
				rec[k] = v
			}
			b, _ := json.Marshal(rec)
			line = string(b)
		} else {
			line = fmt.Sprintf("%s level=%s msg=%s",
				time.Now().UTC().Format(time.RFC3339),
				lvl,
				msg,
			)
			if len(fieldMap) > 0 {
				var kvs []string
				for k, v := range fieldMap {
					kvs = append(kvs, fmt.Sprintf("%s=%v", k, v))
				}
				line += " " + strings.Join(kvs, " ")
			}
		}
		if lvl == "error" || lvl == "warn" {
			fmt.Fprintln(env.Stderr, line)
		} else {
			fmt.Fprintln(env.Stdout, line)
		}
	}

	set(p, "set_level", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) >= 1 {
			level = strings.ToLower(args[0].String())
		}
		return runtime.Unit(), nil
	}, 1)
	// log.set_json(true|false) — JSON lines for agents/ops collectors
	set(p, "set_json", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) >= 1 {
			jsonMode = args[0].IsTruthy()
		} else {
			jsonMode = true
		}
		return runtime.Unit(), nil
	}, 1)
	set(p, "debug", func(args []runtime.Value) (runtime.Value, error) {
		write("debug", args)
		return runtime.Unit(), nil
	}, -1)
	set(p, "info", func(args []runtime.Value) (runtime.Value, error) {
		write("info", args)
		return runtime.Unit(), nil
	}, -1)
	set(p, "warn", func(args []runtime.Value) (runtime.Value, error) {
		write("warn", args)
		return runtime.Unit(), nil
	}, -1)
	set(p, "error", func(args []runtime.Value) (runtime.Value, error) {
		write("error", args)
		return runtime.Unit(), nil
	}, -1)
	return p
}
