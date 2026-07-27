package stdlib

import "github.com/loreste/weft/internal/runtime"

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
