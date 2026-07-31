//go:build !js

package stdlib

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	rt "github.com/loreste/weft/internal/runtime"
)

// packageOS — unified OS operations (env, paths, user, hostname, exit code).
func packageOS() rt.Value {
	p := pkg()

	set(p, "getenv", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.getenv(key)", "os"), nil
		}
		v, ok := os.LookupEnv(args[0].String())
		if !ok {
			return rt.Null(), nil
		}
		return rt.Str(v), nil
	}, 1)

	set(p, "setenv", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 2 {
			return errRes("os.setenv(key, val)", "os"), nil
		}
		if err := os.Setenv(args[0].String(), args[1].String()); err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Ok(rt.Null()), nil
	}, 2)

	set(p, "unsetenv", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.unsetenv(key)", "os"), nil
		}
		os.Unsetenv(args[0].String())
		return rt.Ok(rt.Null()), nil
	}, 1)

	set(p, "environ", func(args []rt.Value) (rt.Value, error) {
		env := os.Environ()
		m := rt.NewMap()
		mo := m.Obj.(*rt.MapObj)
		for _, e := range env {
			k, v, _ := strings.Cut(e, "=")
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = rt.Str(v)
		}
		return m, nil
	}, 0)

	set(p, "cwd", func(args []rt.Value) (rt.Value, error) {
		d, err := os.Getwd()
		if err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Str(d), nil
	}, 0)

	set(p, "chdir", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.chdir(path)", "os"), nil
		}
		if err := os.Chdir(args[0].String()); err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Ok(rt.Null()), nil
	}, 1)

	set(p, "hostname", func(args []rt.Value) (rt.Value, error) {
		h, err := os.Hostname()
		if err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Str(h), nil
	}, 0)

	set(p, "pid", func(args []rt.Value) (rt.Value, error) {
		return rt.Int(int64(os.Getpid())), nil
	}, 0)

	set(p, "ppid", func(args []rt.Value) (rt.Value, error) {
		return rt.Int(int64(os.Getppid())), nil
	}, 0)

	set(p, "uid", func(args []rt.Value) (rt.Value, error) {
		return rt.Int(int64(os.Getuid())), nil
	}, 0)

	set(p, "gid", func(args []rt.Value) (rt.Value, error) {
		return rt.Int(int64(os.Getgid())), nil
	}, 0)

	set(p, "user", func(args []rt.Value) (rt.Value, error) {
		u, err := user.Current()
		if err != nil {
			return errRes(err.Error(), "os"), nil
		}
		m := rt.NewMap()
		mo := m.Obj.(*rt.MapObj)
		mo.Keys = append(mo.Keys, "username", "name", "home", "uid", "gid")
		mo.Vals["username"] = rt.Str(u.Username)
		mo.Vals["name"] = rt.Str(u.Name)
		mo.Vals["home"] = rt.Str(u.HomeDir)
		mo.Vals["uid"] = rt.Str(u.Uid)
		mo.Vals["gid"] = rt.Str(u.Gid)
		return rt.Ok(m), nil
	}, 0)

	set(p, "home", func(args []rt.Value) (rt.Value, error) {
		h, err := os.UserHomeDir()
		if err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Str(h), nil
	}, 0)

	set(p, "temp_dir", func(args []rt.Value) (rt.Value, error) {
		return rt.Str(os.TempDir()), nil
	}, 0)

	set(p, "args", func(args []rt.Value) (rt.Value, error) {
		items := make([]rt.Value, 0, len(os.Args))
		for _, a := range os.Args {
			items = append(items, rt.Str(a))
		}
		return rt.List(items...), nil
	}, 0)

	set(p, "platform", func(args []rt.Value) (rt.Value, error) {
		m := rt.NewMap()
		mo := m.Obj.(*rt.MapObj)
		mo.Keys = append(mo.Keys, "os", "arch", "num_cpu")
		mo.Vals["os"] = rt.Str(runtime.GOOS)
		mo.Vals["arch"] = rt.Str(runtime.GOARCH)
		mo.Vals["num_cpu"] = rt.Int(int64(runtime.NumCPU()))
		return m, nil
	}, 0)

	set(p, "path_join", func(args []rt.Value) (rt.Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		return rt.Str(filepath.Join(parts...)), nil
	}, -1)

	set(p, "path_dir", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.path_dir(path)", "os"), nil
		}
		return rt.Str(filepath.Dir(args[0].String())), nil
	}, 1)

	set(p, "path_base", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.path_base(path)", "os"), nil
		}
		return rt.Str(filepath.Base(args[0].String())), nil
	}, 1)

	set(p, "path_ext", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.path_ext(path)", "os"), nil
		}
		return rt.Str(filepath.Ext(args[0].String())), nil
	}, 1)

	set(p, "path_abs", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.path_abs(path)", "os"), nil
		}
		p, err := filepath.Abs(args[0].String())
		if err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Ok(rt.Str(p)), nil
	}, 1)

	set(p, "path_exists", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.path_exists(path)", "os"), nil
		}
		_, err := os.Stat(args[0].String())
		return rt.Bool(err == nil), nil
	}, 1)

	set(p, "mkdir", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.mkdir(path, perm?)", "os"), nil
		}
		perm := os.FileMode(0o755)
		if len(args) > 1 {
			if p, err := rt.AsInt(args[1]); err == nil {
				perm = os.FileMode(p)
			}
		}
		if err := os.MkdirAll(args[0].String(), perm); err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Ok(rt.Null()), nil
	}, 1)

	set(p, "remove", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.remove(path)", "os"), nil
		}
		if err := os.RemoveAll(args[0].String()); err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Ok(rt.Null()), nil
	}, 1)

	set(p, "rename", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 2 {
			return errRes("os.rename(old, new)", "os"), nil
		}
		if err := os.Rename(args[0].String(), args[1].String()); err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Ok(rt.Null()), nil
	}, 2)

	set(p, "stat", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 1 {
			return errRes("os.stat(path)", "os"), nil
		}
		info, err := os.Stat(args[0].String())
		if err != nil {
			return errRes(err.Error(), "os"), nil
		}
		m := rt.NewMap()
		mo := m.Obj.(*rt.MapObj)
		mo.Keys = append(mo.Keys, "name", "size", "is_dir", "mode", "mod_time")
		mo.Vals["name"] = rt.Str(info.Name())
		mo.Vals["size"] = rt.Int(info.Size())
		mo.Vals["is_dir"] = rt.Bool(info.IsDir())
		mo.Vals["mode"] = rt.Str(info.Mode().String())
		mo.Vals["mod_time"] = rt.Str(info.ModTime().Format("2006-01-02T15:04:05Z07:00"))
		return rt.Ok(m), nil
	}, 1)

	set(p, "chmod", func(args []rt.Value) (rt.Value, error) {
		if len(args) < 2 {
			return errRes("os.chmod(path, mode)", "os"), nil
		}
		mode, err := rt.AsInt(args[1])
		if err != nil {
			// try parsing octal string like "0755"
			m, perr := strconv.ParseUint(args[1].String(), 8, 32)
			if perr != nil {
				return errRes("invalid mode", "os"), nil
			}
			mode = int64(m)
		}
		if err := os.Chmod(args[0].String(), os.FileMode(mode)); err != nil {
			return errRes(err.Error(), "os"), nil
		}
		return rt.Ok(rt.Null()), nil
	}, 2)

	set(p, "separator", func(args []rt.Value) (rt.Value, error) {
		return rt.Str(string(filepath.Separator)), nil
	}, 0)

	return p
}
