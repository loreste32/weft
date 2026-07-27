package stdlib

import (
	"fmt"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageCLI builds devops-style command-line tools (flags, usage, exit).
func packageCLI(env *runtime.Env) runtime.Value {
	p := pkg()

	// cli.args() -> list[str]  (full argv including script name)
	set(p, "args", func(args []runtime.Value) (runtime.Value, error) {
		return argvList(env), nil
	}, 0)

	// cli.argv() -> list[str]  (positional/flags after script name)
	set(p, "argv", func(args []runtime.Value) (runtime.Value, error) {
		all := argvStrings(env)
		if len(all) <= 1 {
			return runtime.List(), nil
		}
		items := make([]runtime.Value, len(all)-1)
		for i, s := range all[1:] {
			items[i] = runtime.Str(s)
		}
		return runtime.List(items...), nil
	}, 0)

	// cli.prog() -> script / binary name
	set(p, "prog", func(args []runtime.Value) (runtime.Value, error) {
		all := argvStrings(env)
		if len(all) == 0 {
			return runtime.Str("weft"), nil
		}
		return runtime.Str(all[0]), nil
	}, 0)

	// cli.parse(spec) -> Result[map]
	// spec: {
	//   about?: str,
	//   flags?: { name: { short?, default?, help?, bool? } | default_value },
	//   args?: [name, ...]  // required positionals names (for usage)
	// }
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		spec := runtime.NewMap()
		if len(args) >= 1 {
			spec = args[0]
		}
		return cliParse(env, spec)
	}, 1)

	// cli.flag(argv_or_null, name, default) — quick single flag lookup
	// Supports --name, --name=val, -n val (if short given as "name|n")
	set(p, "flag", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("cli.flag(name, default) or cli.flag(argv, name, default)", "cli"), nil
		}
		var argv []string
		var name string
		var def runtime.Value
		if len(args) >= 3 {
			argv = valueToStringSlice(args[0])
			name = args[1].String()
			def = args[2]
		} else {
			all := argvStrings(env)
			if len(all) > 1 {
				argv = all[1:]
			}
			name = args[0].String()
			def = args[1]
		}
		v, _ := lookupFlag(argv, name, def)
		return v, nil
	}, 3)

	// cli.has(name) -> bool  whether --name or -x present
	set(p, "has", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		all := argvStrings(env)
		argv := []string{}
		if len(all) > 1 {
			argv = all[1:]
		}
		return runtime.Bool(flagPresent(argv, args[0].String())), nil
	}, 1)

	// cli.usage(spec|string) -> str
	set(p, "usage", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(defaultUsage(env, runtime.NewMap())), nil
		}
		if args[0].Kind == runtime.KindStr {
			return runtime.Str(args[0].S), nil
		}
		return runtime.Str(defaultUsage(env, args[0])), nil
	}, 1)

	// cli.exit(code) — ends process
	set(p, "exit", func(args []runtime.Value) (runtime.Value, error) {
		code := 0
		if len(args) >= 1 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				code = int(n)
			}
		}
		return runtime.Null(), &runtime.ExitSignal{Code: code}
	}, 1)

	// cli.die(msg) — stderr + exit 1
	set(p, "die", func(args []runtime.Value) (runtime.Value, error) {
		msg := "error"
		if len(args) >= 1 {
			msg = args[0].String()
		}
		fmt.Fprintln(env.Stderr, msg)
		return runtime.Null(), &runtime.ExitSignal{Code: 1, Message: msg}
	}, 1)

	// cli.ok(msg?) — print and exit 0
	set(p, "ok", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) >= 1 && args[0].String() != "" {
			fmt.Fprintln(env.Stdout, args[0].String())
		}
		return runtime.Null(), &runtime.ExitSignal{Code: 0}
	}, 1)

	return p
}

func argvList(env *runtime.Env) runtime.Value {
	ss := argvStrings(env)
	items := make([]runtime.Value, len(ss))
	for i, s := range ss {
		items[i] = runtime.Str(s)
	}
	return runtime.List(items...)
}

func argvStrings(env *runtime.Env) []string {
	if v, ok := env.Get("args"); ok && v.Kind == runtime.KindList {
		lo := v.Obj.(*runtime.ListObj)
		out := make([]string, len(lo.Items))
		for i, it := range lo.Items {
			out[i] = it.String()
		}
		return out
	}
	return nil
}

func valueToStringSlice(v runtime.Value) []string {
	if v.Kind != runtime.KindList {
		return nil
	}
	lo := v.Obj.(*runtime.ListObj)
	out := make([]string, len(lo.Items))
	for i, it := range lo.Items {
		out[i] = it.String()
	}
	return out
}

type flagDef struct {
	name    string
	short   string
	help    string
	boolish bool
	def     runtime.Value
}

func cliParse(env *runtime.Env, spec runtime.Value) (runtime.Value, error) {
	about := mapGetStr(spec, "about", "")
	if about == "" {
		about = mapGetStr(spec, "description", "")
	}
	var flags []flagDef
	if fm, ok := mapGet(spec, "flags"); ok {
		if mo, ok := fm.Obj.(*runtime.MapObj); ok {
			for _, name := range mo.Keys {
				fd := flagDef{name: name, def: runtime.Str("")}
				raw := mo.Vals[name]
				if raw.Kind == runtime.KindMap || raw.Kind == runtime.KindStruct {
					fd.short = mapGetStr(raw, "short", "")
					fd.help = mapGetStr(raw, "help", "")
					if b, ok := mapGet(raw, "bool"); ok && b.Kind == runtime.KindBool {
						fd.boolish = b.B
					}
					if d, ok := mapGet(raw, "default"); ok {
						fd.def = d
						if d.Kind == runtime.KindBool {
							fd.boolish = true
						}
					}
					// type: "bool"
					if mapGetStr(raw, "type", "") == "bool" {
						fd.boolish = true
						if fd.def.Kind == runtime.KindNull || fd.def.Kind == runtime.KindStr && fd.def.S == "" {
							fd.def = runtime.Bool(false)
						}
					}
				} else {
					fd.def = raw
					if raw.Kind == runtime.KindBool {
						fd.boolish = true
					}
				}
				flags = append(flags, fd)
			}
		}
	}

	all := argvStrings(env)
	argv := []string{}
	if len(all) > 1 {
		argv = all[1:]
	}

	// help?
	if flagPresent(argv, "help") || flagPresent(argv, "h") {
		usage := defaultUsage(env, spec)
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("help", runtime.Bool(true))
		put("usage", runtime.Str(usage))
		put("about", runtime.Str(about))
		put("args", runtime.List())
		put("flags", runtime.NewMap())
		// also flat empty flags with defaults
		for _, fd := range flags {
			put(fd.name, fd.def)
		}
		return runtime.Ok(m), nil
	}

	rest := make([]string, 0, len(argv))
	vals := map[string]runtime.Value{}
	for _, fd := range flags {
		vals[fd.name] = fd.def
	}

	i := 0
	for i < len(argv) {
		a := argv[i]
		if a == "--" {
			rest = append(rest, argv[i+1:]...)
			break
		}
		if a == "-h" || a == "--help" {
			i++
			continue
		}
		if strings.HasPrefix(a, "--") {
			body := strings.TrimPrefix(a, "--")
			name, val, hasVal := body, "", false
			if eq := strings.IndexByte(body, '='); eq >= 0 {
				name = body[:eq]
				val = body[eq+1:]
				hasVal = true
			}
			fd := findFlag(flags, name, "")
			if fd == nil {
				return errRes("unknown flag --"+name, "cli"), nil
			}
			if fd.boolish {
				if hasVal {
					vals[fd.name] = runtime.Bool(val != "false" && val != "0")
				} else {
					vals[fd.name] = runtime.Bool(true)
				}
				i++
				continue
			}
			if !hasVal {
				if i+1 >= len(argv) {
					return errRes("flag --"+name+" needs a value", "cli"), nil
				}
				val = argv[i+1]
				i += 2
			} else {
				i++
			}
			vals[fd.name] = coerceFlag(val, fd.def)
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 && a != "-" {
			// short cluster -abc or -e value
			cluster := a[1:]
			// -e=val
			if eq := strings.IndexByte(cluster, '='); eq >= 0 {
				short := cluster[:eq]
				val := cluster[eq+1:]
				fd := findFlag(flags, "", short)
				if fd == nil {
					return errRes("unknown flag -"+short, "cli"), nil
				}
				if fd.boolish {
					vals[fd.name] = runtime.Bool(val != "false" && val != "0")
				} else {
					vals[fd.name] = coerceFlag(val, fd.def)
				}
				i++
				continue
			}
			// multi bool shorts or single with value
			if len(cluster) > 1 {
				// all bools?
				allBool := true
				for _, ch := range cluster {
					fd := findFlag(flags, "", string(ch))
					if fd == nil || !fd.boolish {
						allBool = false
						break
					}
				}
				if allBool {
					for _, ch := range cluster {
						fd := findFlag(flags, "", string(ch))
						vals[fd.name] = runtime.Bool(true)
					}
					i++
					continue
				}
			}
			fd := findFlag(flags, "", string(cluster[0]))
			if fd == nil {
				return errRes("unknown flag -"+string(cluster[0]), "cli"), nil
			}
			if fd.boolish {
				vals[fd.name] = runtime.Bool(true)
				// remaining as more shorts if bool
				for _, ch := range cluster[1:] {
					f2 := findFlag(flags, "", string(ch))
					if f2 != nil && f2.boolish {
						vals[f2.name] = runtime.Bool(true)
					}
				}
				i++
				continue
			}
			if len(cluster) > 1 {
				vals[fd.name] = coerceFlag(cluster[1:], fd.def)
				i++
				continue
			}
			if i+1 >= len(argv) {
				return errRes("flag -"+fd.short+" needs a value", "cli"), nil
			}
			vals[fd.name] = coerceFlag(argv[i+1], fd.def)
			i += 2
			continue
		}
		rest = append(rest, a)
		i++
	}

	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(k string, v runtime.Value) {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	put("help", runtime.Bool(false))
	put("usage", runtime.Str(defaultUsage(env, spec)))
	put("about", runtime.Str(about))
	pos := make([]runtime.Value, len(rest))
	for i, s := range rest {
		pos[i] = runtime.Str(s)
	}
	put("args", runtime.List(pos...))
	// flat flags
	for _, fd := range flags {
		put(fd.name, vals[fd.name])
	}
	// nested flags map too
	fm := runtime.NewMap()
	fmo := fm.Obj.(*runtime.MapObj)
	for _, fd := range flags {
		fmo.Keys = append(fmo.Keys, fd.name)
		fmo.Vals[fd.name] = vals[fd.name]
	}
	put("flags", fm)
	return runtime.Ok(m), nil
}

func findFlag(flags []flagDef, name, short string) *flagDef {
	for i := range flags {
		if name != "" && flags[i].name == name {
			return &flags[i]
		}
		if short != "" && flags[i].short == short {
			return &flags[i]
		}
	}
	return nil
}

func coerceFlag(s string, def runtime.Value) runtime.Value {
	switch def.Kind {
	case runtime.KindBool:
		return runtime.Bool(s != "false" && s != "0" && s != "")
	case runtime.KindInt:
		if n, err := runtime.AsInt(runtime.Str(s)); err == nil {
			return runtime.Int(n)
		}
		return def
	case runtime.KindFloat:
		// leave as str if not parseable via AsInt path
		return runtime.Str(s)
	default:
		return runtime.Str(s)
	}
}

func flagPresent(argv []string, name string) bool {
	// name can be long or short
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--"+name || a == "-"+name {
			return true
		}
		if strings.HasPrefix(a, "--"+name+"=") {
			return true
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && len(name) == 1 {
			if strings.Contains(a[1:], name) {
				return true
			}
		}
	}
	return false
}

func lookupFlag(argv []string, name string, def runtime.Value) (runtime.Value, bool) {
	long := name
	short := ""
	if i := strings.IndexByte(name, '|'); i >= 0 {
		long = name[:i]
		short = name[i+1:]
	} else if len(name) == 1 {
		short = name
	}
	boolish := def.Kind == runtime.KindBool
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--"+long || (short != "" && a == "-"+short) {
			if boolish {
				return runtime.Bool(true), true
			}
			if i+1 < len(argv) {
				return coerceFlag(argv[i+1], def), true
			}
			return def, true
		}
		if strings.HasPrefix(a, "--"+long+"=") {
			return coerceFlag(strings.TrimPrefix(a, "--"+long+"="), def), true
		}
	}
	return def, false
}

func defaultUsage(env *runtime.Env, spec runtime.Value) string {
	prog := "tool"
	all := argvStrings(env)
	if len(all) > 0 {
		prog = all[0]
	}
	about := mapGetStr(spec, "about", "")
	if about == "" {
		about = mapGetStr(spec, "description", "")
	}
	var b strings.Builder
	if about != "" {
		fmt.Fprintf(&b, "%s\n\n", about)
	}
	fmt.Fprintf(&b, "Usage: %s [flags] [args]\n", prog)
	if fm, ok := mapGet(spec, "flags"); ok {
		if mo, ok := fm.Obj.(*runtime.MapObj); ok && len(mo.Keys) > 0 {
			b.WriteString("\nFlags:\n")
			for _, name := range mo.Keys {
				raw := mo.Vals[name]
				short, help, def := "", "", ""
				if raw.Kind == runtime.KindMap || raw.Kind == runtime.KindStruct {
					short = mapGetStr(raw, "short", "")
					help = mapGetStr(raw, "help", "")
					if d, ok := mapGet(raw, "default"); ok {
						def = d.String()
					}
				} else {
					def = raw.String()
				}
				flag := "  --" + name
				if short != "" {
					flag = fmt.Sprintf("  -%s, --%s", short, name)
				}
				if def != "" {
					fmt.Fprintf(&b, "%-24s %s (default %s)\n", flag, help, def)
				} else {
					fmt.Fprintf(&b, "%-24s %s\n", flag, help)
				}
			}
		}
	}
	b.WriteString("\n  -h, --help             show help\n")
	return b.String()
}
