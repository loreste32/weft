//go:build !js

package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestDNSPackage(t *testing.T) {
	p := packageDNS()
	mo := p.Obj.(*runtime.MapObj)

	expected := []string{"lookup", "srv", "cname", "ns", "mx", "txt", "reverse"}
	for _, name := range expected {
		if _, ok := mo.Vals[name]; !ok {
			t.Errorf("dns.%s not registered", name)
		}
	}
}
