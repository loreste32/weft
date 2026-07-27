package stdlib

import (
	"os"
	goruntime "runtime"

	"github.com/loreste/weft/internal/runtime"
)

// packagePlatform — OS/arch/runtime facts (Python platform lite).
func packagePlatform() runtime.Value {
	p := pkg()

	set(p, "os", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str(goruntime.GOOS), nil
	}, 0)

	set(p, "arch", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str(goruntime.GOARCH), nil
	}, 0)

	set(p, "cpus", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Int(int64(goruntime.NumCPU())), nil
	}, 0)

	set(p, "hostname", func(args []runtime.Value) (runtime.Value, error) {
		h, err := os.Hostname()
		if err != nil {
			return errRes(err.Error(), "platform"), nil
		}
		return runtime.Ok(runtime.Str(h)), nil
	}, 0)

	set(p, "go_version", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str(goruntime.Version()), nil
	}, 0)

	// platform.uname() -> map  Python-like summary
	set(p, "uname", func(args []runtime.Value) (runtime.Value, error) {
		h, _ := os.Hostname()
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k, v string) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = runtime.Str(v)
		}
		put("system", goruntime.GOOS)
		put("node", h)
		put("machine", goruntime.GOARCH)
		put("release", goruntime.Version())
		put("version", goruntime.Version())
		return m, nil
	}, 0)

	set(p, "is_windows", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(goruntime.GOOS == "windows"), nil
	}, 0)

	set(p, "is_linux", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(goruntime.GOOS == "linux"), nil
	}, 0)

	set(p, "is_darwin", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(goruntime.GOOS == "darwin"), nil
	}, 0)

	return p
}
