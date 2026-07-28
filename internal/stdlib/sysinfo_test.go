package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestSysinfoPackage(t *testing.T) {
	p := packageSysinfo()
	mo := p.Obj.(*runtime.MapObj)

	expected := []string{"uptime", "loadavg", "memory", "disk", "cpu_count", "net_interfaces", "env_summary"}
	for _, name := range expected {
		if _, ok := mo.Vals[name]; !ok {
			t.Errorf("sysinfo.%s not registered", name)
		}
	}
}

func TestSysinfoCPUCount(t *testing.T) {
	p := packageSysinfo()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["cpu_count"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindInt || r.I < 1 {
		t.Fatalf("cpu_count returned %v", r)
	}
}

func TestSysinfoEnvSummary(t *testing.T) {
	p := packageSysinfo()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["env_summary"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindMap {
		t.Fatalf("env_summary should return map, got %v", r.Kind)
	}
	rmo := r.Obj.(*runtime.MapObj)
	for _, key := range []string{"os", "arch", "cpus", "hostname", "pid"} {
		if _, ok := rmo.Vals[key]; !ok {
			t.Errorf("env_summary missing key %s", key)
		}
	}
}

func TestSysinfoUptime(t *testing.T) {
	p := packageSysinfo()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["uptime"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Result — unwrap Ok
	if r.Kind != runtime.KindResult {
		t.Fatalf("uptime should return Result, got %v", r.Kind)
	}
}

func TestSysinfoMemory(t *testing.T) {
	p := packageSysinfo()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["memory"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindResult {
		t.Fatalf("memory should return Result, got %v", r.Kind)
	}
}

func TestSysinfoDisk(t *testing.T) {
	p := packageSysinfo()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["disk"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("/")})
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindResult {
		t.Fatalf("disk should return Result, got %v", r.Kind)
	}
}

func TestHumanDuration(t *testing.T) {
	cases := []struct {
		sec  float64
		want string
	}{
		{90061, "1d 1h 1m"},
		{3661, "1h 1m"},
		{300, "5m"},
	}
	for _, c := range cases {
		got := humanDuration(c.sec)
		if got != c.want {
			t.Errorf("humanDuration(%v) = %q, want %q", c.sec, got, c.want)
		}
	}
}
