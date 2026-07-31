package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestCompressPackage(t *testing.T) {
	p := packageCompress()
	mo := p.Obj.(*runtime.MapObj)

	expected := []string{"gzip", "gunzip", "deflate", "inflate"}
	for _, name := range expected {
		if _, ok := mo.Vals[name]; !ok {
			t.Errorf("compress.%s not registered", name)
		}
	}
}

func TestCompressGzipRoundtrip(t *testing.T) {
	p := packageCompress()
	mo := p.Obj.(*runtime.MapObj)

	gzFn := mo.Vals["gzip"]
	compressed, err := gzFn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("hello world")})
	if err != nil {
		t.Fatal(err)
	}
	if compressed.Kind != runtime.KindResult {
		t.Fatalf("gzip should return Result, got %v", compressed.Kind)
	}

	gunzipFn := mo.Vals["gunzip"]
	result, ok := compressed.Obj.(*runtime.ResultObj)
	if !ok || !result.Ok {
		t.Fatal("gzip returned error")
	}
	decompressed, err := gunzipFn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{result.Val})
	if err != nil {
		t.Fatal(err)
	}
	if decompressed.Kind != runtime.KindResult {
		t.Fatalf("gunzip should return Result, got %v", decompressed.Kind)
	}
	result2, ok := decompressed.Obj.(*runtime.ResultObj)
	if !ok || !result2.Ok {
		t.Fatal("gunzip returned error")
	}
	if result2.Val.String() != "hello world" {
		t.Fatalf("roundtrip = %q, want hello world", result2.Val.String())
	}
}
