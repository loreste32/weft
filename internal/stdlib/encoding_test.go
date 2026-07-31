package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestEncodingPackage(t *testing.T) {
	p := packageEncoding()
	mo := p.Obj.(*runtime.MapObj)

	expected := []string{"hex_encode", "hex_decode", "base32_encode", "base32_decode",
		"url_encode", "url_decode", "path_encode", "path_decode", "to_hex"}
	for _, name := range expected {
		if _, ok := mo.Vals[name]; !ok {
			t.Errorf("encoding.%s not registered", name)
		}
	}
}

func TestEncodingHexRoundtrip(t *testing.T) {
	p := packageEncoding()
	mo := p.Obj.(*runtime.MapObj)

	enc := mo.Vals["hex_encode"]
	r, err := enc.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("hello")})
	if err != nil {
		t.Fatal(err)
	}
	if r.String() != "68656c6c6f" {
		t.Fatalf("hex_encode(hello) = %q, want 68656c6c6f", r.String())
	}

	dec := mo.Vals["hex_decode"]
	r2, err := dec.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("68656c6c6f")})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Kind != runtime.KindResult {
		t.Fatalf("hex_decode should return Result, got %v", r2.Kind)
	}
}

func TestEncodingURLEncode(t *testing.T) {
	p := packageEncoding()
	mo := p.Obj.(*runtime.MapObj)
	fn := mo.Vals["url_encode"]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("hello world")})
	if err != nil {
		t.Fatal(err)
	}
	if r.String() != "hello+world" {
		t.Fatalf("url_encode = %q, want hello+world", r.String())
	}
}
