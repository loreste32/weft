package stdlib

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"

	"github.com/loreste/weft/internal/runtime"
)

func packageSecrets(env *runtime.Env) runtime.Value {
	p := pkg()
	// secrets.require(name) -> Result[str]  (Secret redaction later; plain str for MVP-1)
	set(p, "require", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("secrets.require needs a name", "secrets"), nil
		}
		key := args[0].String()
		v, ok := getenv(env, key)
		if !ok || v == "" {
			return errRes("missing secret "+key, "secrets"), nil
		}
		// Mark as secret struct so println can redact later; for now str-compatible via String()
		sec := runtime.Struct("Secret", map[string]runtime.Value{
			"value": runtime.Str(v),
		}, []string{"value"})
		return runtime.Ok(sec), nil
	}, 1)
	// secrets.get(name) -> Secret|null  (never plain str — avoids accidental JSON leak)
	set(p, "get", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		v, ok := getenv(env, args[0].String())
		if !ok || v == "" {
			return runtime.Null(), nil
		}
		return runtime.Struct("Secret", map[string]runtime.Value{
			"value": runtime.Str(v),
		}, []string{"value"}), nil
	}, 1)
	// secrets.from(value) -> Secret  wrap a string as secret
	set(p, "from", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		return runtime.Struct("Secret", map[string]runtime.Value{
			"value": runtime.Str(args[0].String()),
		}, []string{"value"}), nil
	}, 1)
	// secrets.unwrap(s) -> str  explicit reveal (use sparingly)
	set(p, "unwrap", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(SecretString(args[0])), nil
	}, 1)

	// secrets.token_hex(n_bytes?) -> str  CSPRNG hex (default 32 bytes → 64 chars)
	set(p, "token_hex", func(args []runtime.Value) (runtime.Value, error) {
		n := 32
		if len(args) >= 1 {
			if x, err := runtime.AsInt(args[0]); err == nil && x > 0 && x <= 1024 {
				n = int(x)
			}
		}
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			return errRes(err.Error(), "secrets"), nil
		}
		return runtime.Str(hex.EncodeToString(b)), nil
	}, 1)

	// secrets.token_urlsafe(n_bytes?) -> str  base64url without padding
	set(p, "token_urlsafe", func(args []runtime.Value) (runtime.Value, error) {
		n := 32
		if len(args) >= 1 {
			if x, err := runtime.AsInt(args[0]); err == nil && x > 0 && x <= 1024 {
				n = int(x)
			}
		}
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			return errRes(err.Error(), "secrets"), nil
		}
		return runtime.Str(base64.RawURLEncoding.EncodeToString(b)), nil
	}, 1)

	// secrets.compare(a, b) -> bool  constant-time (Secret or str)
	set(p, "compare", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		a := []byte(SecretString(args[0]))
		b := []byte(SecretString(args[1]))
		if len(a) != len(b) {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(subtle.ConstantTimeCompare(a, b) == 1), nil
	}, 2)

	return p
}

// SecretString extracts API key from Secret struct or plain str.
func SecretString(v runtime.Value) string {
	if v.Kind == runtime.KindStruct {
		so := v.Obj.(*runtime.StructObj)
		if so.TypeName == "Secret" {
			if val, ok := so.Fields["value"]; ok {
				return val.String()
			}
		}
	}
	return v.String()
}
