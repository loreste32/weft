package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestNetutilPackage(t *testing.T) {
	env := runtime.NewEnv()
	p := packageNetutil(env)
	mo := p.Obj.(*runtime.MapObj)

	expected := []string{"port_open", "tcp_ping", "resolve", "lookup_host", "lookup_txt", "lookup_mx", "reverse_lookup", "scan_ports"}
	for _, name := range expected {
		if _, ok := mo.Vals[name]; !ok {
			t.Errorf("netutil.%s not registered", name)
		}
	}
}

func TestNetutilResolve(t *testing.T) {
	env := runtime.NewEnv()
	p := packageNetutil(env)
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["resolve"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("localhost")})
	if err != nil {
		t.Fatal(err)
	}
	// Should return Result (Ok or Err)
	if r.Kind != runtime.KindResult {
		t.Fatalf("resolve should return Result, got %v", r.Kind)
	}
}
