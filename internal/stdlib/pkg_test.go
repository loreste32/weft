package stdlib_test

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
)

func TestNamesMatchRegistryAndRegister(t *testing.T) {
	names := stdlib.Names()
	if len(names) == 0 {
		t.Fatal("no packages")
	}
	env := runtime.NewEnv()
	stdlib.Register(env, stdlib.Options{})
	for _, n := range names {
		if !stdlib.IsPackage(n) {
			t.Fatalf("IsPackage(%q) false", n)
		}
		if _, ok := env.Get(n); !ok {
			t.Fatalf("Register did not install %q", n)
		}
		if !env.IsHostShared(n) {
			t.Fatalf("%q not host-shared for modules", n)
		}
	}
	// unknown package
	if stdlib.IsPackage("not_a_real_pkg") {
		t.Fatal("bogus package")
	}
}

func TestHostSharedPreludeAndListOps(t *testing.T) {
	env := runtime.NewEnv()
	stdlib.Register(env, stdlib.Options{})
	for _, n := range []string{"println", "len", "map", "filter", "spawn", "Ok"} {
		if !env.IsHostShared(n) {
			t.Fatalf("expected host-shared %q", n)
		}
	}
	// user binding is not shared unless SetShared
	env.Set("user_only", runtime.Int(1))
	if env.IsHostShared("user_only") {
		t.Fatal("user Set should not auto-share")
	}
	dst := runtime.NewEnv()
	env.CopyHostSharedInto(dst)
	if _, ok := dst.Get("user_only"); ok {
		t.Fatal("user binding leaked into module env")
	}
	if _, ok := dst.Get("map"); !ok {
		t.Fatal("map should copy into module env")
	}
	if _, ok := dst.Get("json"); !ok {
		t.Fatal("json package should copy into module env")
	}
}
