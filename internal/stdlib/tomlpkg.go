package stdlib

import (
	"os"

	"github.com/loreste/weft/internal/runtime"
	toml "github.com/pelletier/go-toml/v2"
)

// packageTOML — TOML parse/stringify/load/save (ops configs).
func packageTOML() runtime.Value {
	p := pkg()

	// toml.parse(text) -> Result[map]
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("toml.parse(text)", "toml"), nil
		}
		var raw any
		if err := toml.Unmarshal([]byte(args[0].String()), &raw); err != nil {
			return errRes(err.Error(), "toml"), nil
		}
		return runtime.Ok(goToValue(raw)), nil
	}, 1)

	// toml.load(path) -> Result[map]
	set(p, "load", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("toml.load(path)", "toml"), nil
		}
		b, err := os.ReadFile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "toml"), nil
		}
		var raw any
		if err := toml.Unmarshal(b, &raw); err != nil {
			return errRes(err.Error(), "toml"), nil
		}
		return runtime.Ok(goToValue(raw)), nil
	}, 1)

	// toml.stringify(value) -> Result[str]
	set(p, "stringify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Ok(runtime.Str("")), nil
		}
		goV := valueToGo(args[0])
		b, err := toml.Marshal(goV)
		if err != nil {
			return errRes(err.Error(), "toml"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	// toml.save(path, value) -> Result
	set(p, "save", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("toml.save(path, value)", "toml"), nil
		}
		b, err := toml.Marshal(valueToGo(args[1]))
		if err != nil {
			return errRes(err.Error(), "toml"), nil
		}
		if err := os.WriteFile(args[0].String(), b, 0o644); err != nil {
			return errRes(err.Error(), "toml"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	return p
}
