package stdlib

import (
	"os"
	"os/user"
	"sort"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

func packageEnv(env *runtime.Env) runtime.Value {
	p := pkg()
	// env.get(key, default?) -> str?   (null if missing and no default)
	set(p, "get", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		v, ok := getenv(env, args[0].String())
		if !ok {
			if len(args) >= 2 {
				return args[1], nil // keep caller's type (str/int/…)
			}
			return runtime.Null(), nil
		}
		return runtime.Str(v), nil
	}, 2)
	// env.require(key) -> Result[str]
	set(p, "require", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("env.require needs a key", "env"), nil
		}
		v, ok := getenv(env, args[0].String())
		if !ok || v == "" {
			return errRes("missing env "+args[0].String(), "env"), nil
		}
		return runtime.Ok(runtime.Str(v)), nil
	}, 1)

	// env.set(key, value) — process env (and Environ override map if present)
	set(p, "set", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("env.set(key, value)", "env"), nil
		}
		k, v := args[0].String(), args[1].String()
		if env.Environ != nil {
			env.Environ[k] = v
		}
		_ = os.Setenv(k, v)
		return runtime.Unit(), nil
	}, 2)

	// env.unset(key)
	set(p, "unset", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("env.unset(key)", "env"), nil
		}
		k := args[0].String()
		if env.Environ != nil {
			delete(env.Environ, k)
		}
		_ = os.Unsetenv(k)
		return runtime.Unit(), nil
	}, 1)

	// env.keys() -> [str]
	set(p, "keys", func(args []runtime.Value) (runtime.Value, error) {
		var keys []string
		if env.Environ != nil {
			for k := range env.Environ {
				keys = append(keys, k)
			}
		} else {
			for _, e := range os.Environ() {
				if i := strings.IndexByte(e, '='); i > 0 {
					keys = append(keys, e[:i])
				}
			}
		}
		sort.Strings(keys)
		items := make([]runtime.Value, len(keys))
		for i, k := range keys {
			items[i] = runtime.Str(k)
		}
		return runtime.List(items...), nil
	}, 0)

	// env.hostname() -> str
	set(p, "hostname", func(args []runtime.Value) (runtime.Value, error) {
		h, err := os.Hostname()
		if err != nil {
			return runtime.Str(""), nil
		}
		return runtime.Str(h), nil
	}, 0)

	// env.pid() -> int
	set(p, "pid", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Int(int64(os.Getpid())), nil
	}, 0)

	// env.home() -> str
	set(p, "home", func(args []runtime.Value) (runtime.Value, error) {
		if v, ok := getenv(env, "HOME"); ok && v != "" {
			return runtime.Str(v), nil
		}
		if u, err := user.Current(); err == nil && u.HomeDir != "" {
			return runtime.Str(u.HomeDir), nil
		}
		return runtime.Str(""), nil
	}, 0)

	// env.user() -> str
	set(p, "user", func(args []runtime.Value) (runtime.Value, error) {
		if v, ok := getenv(env, "USER"); ok && v != "" {
			return runtime.Str(v), nil
		}
		if v, ok := getenv(env, "USERNAME"); ok && v != "" {
			return runtime.Str(v), nil
		}
		if u, err := user.Current(); err == nil {
			return runtime.Str(u.Username), nil
		}
		return runtime.Str(""), nil
	}, 0)

	return p
}
