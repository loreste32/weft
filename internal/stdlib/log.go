package stdlib

import (
	"fmt"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageLog — structured-ish logging for production CLIs/workers.
func packageLog(env *runtime.Env) runtime.Value {
	p := pkg()
	level := "info" // package-level default via closure on map field

	write := func(lvl string, args []runtime.Value) {
		// respect level: debug < info < warn < error
		order := map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3}
		if order[lvl] < order[level] {
			return
		}
		// last map arg → structured fields (k=v)
		var fields string
		msgArgs := args
		if len(args) > 0 && args[len(args)-1].Kind == runtime.KindMap {
			mo := args[len(args)-1].Obj.(*runtime.MapObj)
			var kvs []string
			for _, k := range mo.Keys {
				kvs = append(kvs, k+"="+mo.Vals[k].String())
			}
			fields = strings.Join(kvs, " ")
			msgArgs = args[:len(args)-1]
		}
		parts := make([]string, len(msgArgs))
		for i, a := range msgArgs {
			parts[i] = a.String()
		}
		msg := strings.Join(parts, " ")
		line := fmt.Sprintf("%s level=%s msg=%s",
			time.Now().UTC().Format(time.RFC3339),
			lvl,
			msg,
		)
		if fields != "" {
			line += " " + fields
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
	// log.with fields: log.info("msg", {"key": "val"}) already works as multi-arg
	return p
}
