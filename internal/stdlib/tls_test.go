//go:build !js

package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestTLSPackage(t *testing.T) {
	p := packageTLS()
	mo := p.Obj.(*runtime.MapObj)

	expected := []string{"cert_info", "verify", "chain", "expiry_check", "supported_versions", "system_roots"}
	for _, name := range expected {
		if _, ok := mo.Vals[name]; !ok {
			t.Errorf("tls.%s not registered", name)
		}
	}
}

func TestTLSSupportedVersions(t *testing.T) {
	p := packageTLS()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["supported_versions"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindList {
		t.Fatalf("supported_versions should return list, got %v", r.Kind)
	}
}
