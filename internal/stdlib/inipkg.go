package stdlib

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageINI — simple INI config (Python configparser lite).
// Sections map to nested maps: { "section": { "key": "value" }, "DEFAULT": {...} }
func packageINI() runtime.Value {
	p := pkg()

	// ini.parse(text) -> Result[map]
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("ini.parse(text)", "ini"), nil
		}
		m, err := parseINI(args[0].String())
		if err != nil {
			return errRes(err.Error(), "ini"), nil
		}
		return runtime.Ok(m), nil
	}, 1)

	// ini.load(path) -> Result[map]
	set(p, "load", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("ini.load(path)", "ini"), nil
		}
		b, err := os.ReadFile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "ini"), nil
		}
		m, err := parseINI(string(b))
		if err != nil {
			return errRes(err.Error(), "ini"), nil
		}
		return runtime.Ok(m), nil
	}, 1)

	// ini.stringify(map) -> str
	set(p, "stringify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(stringifyINI(args[0])), nil
	}, 1)

	// ini.save(path, map) -> Result
	set(p, "save", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("ini.save(path, map)", "ini"), nil
		}
		if err := os.WriteFile(args[0].String(), []byte(stringifyINI(args[1])), 0o644); err != nil {
			return errRes(err.Error(), "ini"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// ini.get(cfg, section, key, default?) -> str|null
	set(p, "get", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return runtime.Null(), nil
		}
		sec := sectionMap(args[0], args[1].String())
		if sec.Kind != runtime.KindMap {
			if len(args) >= 4 {
				return args[3], nil
			}
			return runtime.Null(), nil
		}
		if v, ok := mapGet(sec, args[2].String()); ok {
			return v, nil
		}
		// DEFAULT section fallback
		def := sectionMap(args[0], "DEFAULT")
		if def.Kind == runtime.KindMap {
			if v, ok := mapGet(def, args[2].String()); ok {
				return v, nil
			}
		}
		if len(args) >= 4 {
			return args[3], nil
		}
		return runtime.Null(), nil
	}, 4)

	return p
}

func sectionMap(cfg runtime.Value, section string) runtime.Value {
	if cfg.Kind != runtime.KindMap {
		return runtime.Null()
	}
	v, ok := mapGet(cfg, section)
	if !ok {
		return runtime.Null()
	}
	return v
}

func parseINI(text string) (runtime.Value, error) {
	root := runtime.NewMap()
	rmo := root.Obj.(*runtime.MapObj)
	ensure := func(sec string) *runtime.MapObj {
		if v, ok := rmo.Vals[sec]; ok && v.Kind == runtime.KindMap {
			return v.Obj.(*runtime.MapObj)
		}
		m := runtime.NewMap()
		rmo.Keys = append(rmo.Keys, sec)
		rmo.Vals[sec] = m
		return m.Obj.(*runtime.MapObj)
	}
	cur := "DEFAULT"
	smo := ensure(cur)
	sc := bufio.NewScanner(strings.NewReader(text))
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			cur = strings.TrimSpace(line[1 : len(line)-1])
			if cur == "" {
				return runtime.Null(), fmt.Errorf("line %d: empty section", lineNo)
			}
			smo = ensure(cur)
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			k, v, ok = strings.Cut(line, ":")
		}
		if !ok {
			return runtime.Null(), fmt.Errorf("line %d: expected key=value", lineNo)
		}
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(v)
		// strip optional quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if _, exists := smo.Vals[key]; !exists {
			smo.Keys = append(smo.Keys, key)
		}
		smo.Vals[key] = runtime.Str(val)
	}
	if err := sc.Err(); err != nil {
		return runtime.Null(), err
	}
	return root, nil
}

func stringifyINI(v runtime.Value) string {
	if v.Kind != runtime.KindMap {
		return ""
	}
	mo := v.Obj.(*runtime.MapObj)
	var b strings.Builder
	// DEFAULT first if present
	writeSec := func(name string, sec runtime.Value) {
		if sec.Kind != runtime.KindMap {
			return
		}
		if name != "DEFAULT" {
			fmt.Fprintf(&b, "[%s]\n", name)
		}
		smo := sec.Obj.(*runtime.MapObj)
		for _, k := range smo.Keys {
			fmt.Fprintf(&b, "%s = %s\n", k, smo.Vals[k].String())
		}
		b.WriteByte('\n')
	}
	if def, ok := mo.Vals["DEFAULT"]; ok {
		writeSec("DEFAULT", def)
	}
	for _, k := range mo.Keys {
		if k == "DEFAULT" {
			continue
		}
		writeSec(k, mo.Vals[k])
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
