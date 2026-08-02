//go:build js

package stdlib

import (
	"path"
	"syscall/js"

	"github.com/loreste/weft/internal/runtime"
)

// packageOS exposes the browser-safe portion of os. Environment values are
// read from process.env in Node and from globalThis.__WEFT_ENV__ in browsers.
// Host identity and process-control operations intentionally return neutral
// values because a browser has no process, user, or host filesystem.
func packageOS() runtime.Value {
	p := pkg()
	set(p, "getenv", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("os.getenv(key)", "os"), nil
		}
		key := args[0].String()
		if value, ok := wasmGetEnv(key); ok {
			return runtime.Str(value), nil
		}
		return runtime.Null(), nil
	}, 1)
	set(p, "setenv", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("os.setenv(key, value)", "os"), nil
		}
		wasmSetEnv(args[0].String(), args[1].String())
		return runtime.Ok(runtime.Null()), nil
	}, 2)
	set(p, "unsetenv", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("os.unsetenv(key)", "os"), nil
		}
		wasmDeleteEnv(args[0].String())
		return runtime.Ok(runtime.Null()), nil
	}, 1)
	set(p, "environ", func(args []runtime.Value) (runtime.Value, error) {
		return wasmEnvironment(), nil
	}, 0)
	set(p, "cwd", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str("."), nil }, 0)
	set(p, "chdir", func(args []runtime.Value) (runtime.Value, error) {
		return errRes("os.chdir is not available in browser Wasm", "os"), nil
	}, 1)
	set(p, "hostname", func(args []runtime.Value) (runtime.Value, error) {
		location := js.Global().Get("location")
		if location.Type() != js.TypeUndefined && location.Type() != js.TypeNull {
			return runtime.Str(location.Get("hostname").String()), nil
		}
		return runtime.Str("browser"), nil
	}, 0)
	set(p, "pid", func(args []runtime.Value) (runtime.Value, error) { return runtime.Int(0), nil }, 0)
	set(p, "ppid", func(args []runtime.Value) (runtime.Value, error) { return runtime.Int(0), nil }, 0)
	set(p, "uid", func(args []runtime.Value) (runtime.Value, error) { return runtime.Int(0), nil }, 0)
	set(p, "gid", func(args []runtime.Value) (runtime.Value, error) { return runtime.Int(0), nil }, 0)
	set(p, "user", func(args []runtime.Value) (runtime.Value, error) { return runtime.Ok(runtime.NewMap()), nil }, 0)
	set(p, "home", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(""), nil }, 0)
	set(p, "temp_dir", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str("/tmp"), nil }, 0)
	set(p, "args", func(args []runtime.Value) (runtime.Value, error) { return runtime.List(), nil }, 0)
	set(p, "platform", func(args []runtime.Value) (runtime.Value, error) {
		return wasmPlatform(), nil
	}, 0)

	set(p, "path_join", func(args []runtime.Value) (runtime.Value, error) {
		parts := make([]string, len(args))
		for i, arg := range args {
			parts[i] = arg.String()
		}
		return runtime.Str(path.Clean(path.Join(parts...))), nil
	}, -1)
	set(p, "path_dir", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(path.Dir(wasmArg(args))), nil }, 1)
	set(p, "path_base", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(path.Base(wasmArg(args))), nil }, 1)
	set(p, "path_ext", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(path.Ext(wasmArg(args))), nil }, 1)
	set(p, "path_abs", func(args []runtime.Value) (runtime.Value, error) {
		name := path.Clean(wasmArg(args))
		if name == "." {
			name = ""
		}
		if name == "" || name[0] != '/' {
			name = "/" + name
		}
		return runtime.Ok(runtime.Str(name)), nil
	}, 1)
	return p
}

func wasmEnvValue() js.Value {
	process := js.Global().Get("process")
	if process.Type() != js.TypeUndefined && process.Type() != js.TypeNull {
		env := process.Get("env")
		if env.Type() == js.TypeObject {
			return env
		}
	}
	env := js.Global().Get("__WEFT_ENV__")
	if env.Type() == js.TypeObject {
		return env
	}
	env = js.Global().Get("Object").New()
	js.Global().Set("__WEFT_ENV__", env)
	return env
}

func wasmGetEnv(key string) (string, bool) {
	env := wasmEnvValue()
	value := env.Get(key)
	if value.Type() == js.TypeUndefined || value.Type() == js.TypeNull {
		return "", false
	}
	return value.String(), true
}

func wasmSetEnv(key, value string) { wasmEnvValue().Set(key, value) }
func wasmDeleteEnv(key string) {
	js.Global().Get("Reflect").Call("deleteProperty", wasmEnvValue(), key)
}

func wasmEnvironment() runtime.Value {
	env := wasmEnvValue()
	keys := js.Global().Get("Object").Call("keys", env)
	result := runtime.NewMap()
	mo := result.Obj.(*runtime.MapObj)
	for i := 0; i < keys.Length(); i++ {
		key := keys.Index(i).String()
		mo.Keys = append(mo.Keys, key)
		mo.Vals[key] = runtime.Str(env.Get(key).String())
	}
	return result
}

func wasmPlatform() runtime.Value {
	result := runtime.NewMap()
	mo := result.Obj.(*runtime.MapObj)
	mo.Keys = append(mo.Keys, "os", "arch", "num_cpu")
	mo.Vals["os"] = runtime.Str("js")
	mo.Vals["arch"] = runtime.Str("wasm")
	mo.Vals["num_cpu"] = runtime.Int(1)
	return result
}
