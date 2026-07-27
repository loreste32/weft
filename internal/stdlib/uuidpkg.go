package stdlib

import (
	"github.com/google/uuid"
	"github.com/loreste/weft/internal/runtime"
)

// packageUUID — UUID helpers (also available as crypto.uuid).
func packageUUID() runtime.Value {
	p := pkg()

	// uuid.v4() / uuid.new() -> str
	gen := func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str(uuid.NewString()), nil
	}
	set(p, "v4", gen, 0)
	set(p, "new", gen, 0)

	// uuid.parse(s) -> Result[str] normalized, or Err
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("uuid.parse(s)", "uuid"), nil
		}
		u, err := uuid.Parse(args[0].String())
		if err != nil {
			return errRes(err.Error(), "uuid"), nil
		}
		return runtime.Ok(runtime.Str(u.String())), nil
	}, 1)

	// uuid.is_valid(s) -> bool
	set(p, "is_valid", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		_, err := uuid.Parse(args[0].String())
		return runtime.Bool(err == nil), nil
	}, 1)

	return p
}
