//go:build !js

package stdlib

import (
	"os"
	"runtime"
	"testing"

	rt "github.com/loreste/weft/internal/runtime"
)

func TestOSPackage(t *testing.T) {
	p := packageOS()
	mo := p.Obj.(*rt.MapObj)

	expected := []string{"getenv", "setenv", "unsetenv", "environ", "cwd", "chdir",
		"hostname", "pid", "ppid", "uid", "gid", "user", "home", "temp_dir",
		"args", "platform", "path_join", "path_dir", "path_base", "path_ext",
		"path_abs", "path_exists", "mkdir", "remove", "rename", "stat", "chmod", "separator"}
	for _, name := range expected {
		if _, ok := mo.Vals[name]; !ok {
			t.Errorf("os.%s not registered", name)
		}
	}
}

func TestOSHostname(t *testing.T) {
	p := packageOS()
	mo := p.Obj.(*rt.MapObj)
	fn := mo.Vals["hostname"]
	r, err := fn.Obj.(*rt.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := os.Hostname()
	if r.String() != expected {
		t.Fatalf("hostname = %q, want %q", r.String(), expected)
	}
}

func TestOSPid(t *testing.T) {
	p := packageOS()
	mo := p.Obj.(*rt.MapObj)
	fn := mo.Vals["pid"]
	r, err := fn.Obj.(*rt.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.I != int64(os.Getpid()) {
		t.Fatalf("pid = %d, want %d", r.I, os.Getpid())
	}
}

func TestOSPlatform(t *testing.T) {
	p := packageOS()
	mo := p.Obj.(*rt.MapObj)
	fn := mo.Vals["platform"]
	r, err := fn.Obj.(*rt.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != rt.KindMap {
		t.Fatalf("platform should return map, got %v", r.Kind)
	}
	rmo := r.Obj.(*rt.MapObj)
	if rmo.Vals["os"].String() != runtime.GOOS {
		t.Errorf("platform.os = %q, want %q", rmo.Vals["os"].String(), runtime.GOOS)
	}
}

func TestOSPathJoin(t *testing.T) {
	p := packageOS()
	mo := p.Obj.(*rt.MapObj)
	fn := mo.Vals["path_join"]
	r, err := fn.Obj.(*rt.BuiltinObj).Fn([]rt.Value{rt.Str("/tmp"), rt.Str("foo"), rt.Str("bar.txt")})
	if err != nil {
		t.Fatal(err)
	}
	if r.String() != "/tmp/foo/bar.txt" {
		t.Fatalf("path_join = %q, want /tmp/foo/bar.txt", r.String())
	}
}

func TestOSCwd(t *testing.T) {
	p := packageOS()
	mo := p.Obj.(*rt.MapObj)
	fn := mo.Vals["cwd"]
	r, err := fn.Obj.(*rt.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := os.Getwd()
	if r.String() != expected {
		t.Fatalf("cwd = %q, want %q", r.String(), expected)
	}
}
