package stdlib

import (
	"os"

	"github.com/loreste/weft/internal/runtime"
	"gopkg.in/yaml.v3"
)

// packageYAML — YAML parse/stringify/load/save (ops configs; pairs with toml/ini).
func packageYAML() runtime.Value {
	p := pkg()

	// yaml.parse(text) -> Result[map|list]
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("yaml.parse(text)", "yaml"), nil
		}
		var raw any
		if err := yaml.Unmarshal([]byte(args[0].String()), &raw); err != nil {
			return errRes(err.Error(), "yaml"), nil
		}
		return runtime.Ok(goToValue(raw)), nil
	}, 1)

	// yaml.load(path) -> Result
	set(p, "load", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("yaml.load(path)", "yaml"), nil
		}
		b, err := os.ReadFile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "yaml"), nil
		}
		var raw any
		if err := yaml.Unmarshal(b, &raw); err != nil {
			return errRes(err.Error(), "yaml"), nil
		}
		return runtime.Ok(goToValue(raw)), nil
	}, 1)

	// yaml.stringify(value) -> Result[str]
	set(p, "stringify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Ok(runtime.Str("")), nil
		}
		b, err := yaml.Marshal(valueToGo(args[0]))
		if err != nil {
			return errRes(err.Error(), "yaml"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	// yaml.save(path, value) -> Result
	set(p, "save", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("yaml.save(path, value)", "yaml"), nil
		}
		b, err := yaml.Marshal(valueToGo(args[1]))
		if err != nil {
			return errRes(err.Error(), "yaml"), nil
		}
		if err := os.WriteFile(args[0].String(), b, 0o644); err != nil {
			return errRes(err.Error(), "yaml"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	return p
}
