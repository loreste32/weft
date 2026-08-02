package stdlib

import (
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func getCryptoFn(name string) runtime.Builtin {
	p := packageCrypto()
	mo := p.Obj.(*runtime.MapObj)
	return mo.Vals[name].Obj.(*runtime.BuiltinObj).Fn
}

func TestArgon2idBasic(t *testing.T) {
	fn := getCryptoFn("argon2id")
	r, err := fn([]runtime.Value{runtime.Str("password"), runtime.Str("salt1234")})
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindStr || len(r.String()) != 64 {
		t.Fatalf("expected 64-char hex, got %v (%d chars)", r.Kind, len(r.String()))
	}
}

func TestArgon2idTimeTooHigh(t *testing.T) {
	fn := getCryptoFn("argon2id")
	r, _ := fn([]runtime.Value{
		runtime.Str("password"), runtime.Str("salt"),
		runtime.Int(99), // time > 10
	})
	if r.Kind != runtime.KindResult {
		t.Fatalf("expected Result error, got %v", r.Kind)
	}
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected error for time=99")
	}
}

func TestArgon2idMemoryTooHigh(t *testing.T) {
	fn := getCryptoFn("argon2id")
	r, _ := fn([]runtime.Value{
		runtime.Str("password"), runtime.Str("salt"),
		runtime.Int(1),       // time
		runtime.Int(1 << 20), // memory > 256 MiB
	})
	if r.Kind != runtime.KindResult {
		t.Fatalf("expected Result error, got %v", r.Kind)
	}
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected error for memory too high")
	}
}

func TestArgon2idMemoryTooLow(t *testing.T) {
	fn := getCryptoFn("argon2id")
	r, _ := fn([]runtime.Value{
		runtime.Str("password"), runtime.Str("salt"),
		runtime.Int(1),
		runtime.Int(1024), // memory < 8192
	})
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected error for memory too low")
	}
}

func TestArgon2idThreadsTooHigh(t *testing.T) {
	fn := getCryptoFn("argon2id")
	r, _ := fn([]runtime.Value{
		runtime.Str("password"), runtime.Str("salt"),
		runtime.Int(1), runtime.Int(65536),
		runtime.Int(32), // threads > 16
	})
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected error for threads=32")
	}
}

func TestArgon2idKeyLenTooSmall(t *testing.T) {
	fn := getCryptoFn("argon2id")
	r, _ := fn([]runtime.Value{
		runtime.Str("password"), runtime.Str("salt"),
		runtime.Int(1), runtime.Int(65536), runtime.Int(4),
		runtime.Int(8), // keyLen < 16
	})
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected error for keyLen=8")
	}
}

func TestPBKDF2Basic(t *testing.T) {
	fn := getCryptoFn("pbkdf2")
	r, err := fn([]runtime.Value{runtime.Str("password"), runtime.Str("salt")})
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindStr || len(r.String()) != 64 {
		t.Fatalf("expected 64-char hex, got %v (%d chars)", r.Kind, len(r.String()))
	}
}

func TestPBKDF2IterationsTooLow(t *testing.T) {
	fn := getCryptoFn("pbkdf2")
	r, _ := fn([]runtime.Value{
		runtime.Str("password"), runtime.Str("salt"),
		runtime.Int(100), // iterations < 10000
	})
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected error for iterations=100")
	}
}

func TestPBKDF2IterationsTooHigh(t *testing.T) {
	fn := getCryptoFn("pbkdf2")
	r, _ := fn([]runtime.Value{
		runtime.Str("password"), runtime.Str("salt"),
		runtime.Int(99_000_000), // > 10M
	})
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected error for iterations too high")
	}
}

func TestPBKDF2Deterministic(t *testing.T) {
	fn := getCryptoFn("pbkdf2")
	r1, _ := fn([]runtime.Value{runtime.Str("pass"), runtime.Str("salt"), runtime.Int(10000)})
	r2, _ := fn([]runtime.Value{runtime.Str("pass"), runtime.Str("salt"), runtime.Int(10000)})
	if r1.String() != r2.String() {
		t.Fatal("PBKDF2 not deterministic")
	}
}

func TestArgon2idDeterministic(t *testing.T) {
	fn := getCryptoFn("argon2id")
	r1, _ := fn([]runtime.Value{runtime.Str("pass"), runtime.Str("salt")})
	r2, _ := fn([]runtime.Value{runtime.Str("pass"), runtime.Str("salt")})
	if r1.String() != r2.String() {
		t.Fatal("Argon2id not deterministic for same inputs")
	}
}

func TestArgon2idDifferentSalts(t *testing.T) {
	fn := getCryptoFn("argon2id")
	r1, _ := fn([]runtime.Value{runtime.Str("pass"), runtime.Str("salt1")})
	r2, _ := fn([]runtime.Value{runtime.Str("pass"), runtime.Str("salt2")})
	if r1.String() == r2.String() {
		t.Fatal("different salts should produce different hashes")
	}
}

func TestPBKDF2KeyLenLimits(t *testing.T) {
	fn := getCryptoFn("pbkdf2")
	// keyLen too small
	r, _ := fn([]runtime.Value{
		runtime.Str("pass"), runtime.Str("salt"),
		runtime.Int(10000), runtime.Int(8),
	})
	s := r.String()
	if !strings.Contains(s, "keyLen") {
		t.Fatalf("expected keyLen error, got: %s", s)
	}
}
