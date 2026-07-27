package stdlib

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageDecimal — arbitrary-precision decimal via math/big.Rat (string-based API).
func packageDecimal() runtime.Value {
	p := pkg()

	// decimal.new(s|n) -> Result[str]
	set(p, "new", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("decimal.new(value)", "decimal"), nil
		}
		r, err := parseRat(args[0])
		if err != nil {
			return errRes(err.Error(), "decimal"), nil
		}
		return runtime.Ok(runtime.Str(formatRat(r))), nil
	}, 1)

	// decimal.add(a, b) -> str
	set(p, "add", binRat(func(a, b *big.Rat) *big.Rat {
		return new(big.Rat).Add(a, b)
	}), 2)
	set(p, "sub", binRat(func(a, b *big.Rat) *big.Rat {
		return new(big.Rat).Sub(a, b)
	}), 2)
	set(p, "mul", binRat(func(a, b *big.Rat) *big.Rat {
		return new(big.Rat).Mul(a, b)
	}), 2)
	set(p, "div", binRat(func(a, b *big.Rat) *big.Rat {
		if b.Sign() == 0 {
			return nil
		}
		return new(big.Rat).Quo(a, b)
	}), 2)

	// decimal.cmp(a, b) -> int  -1/0/1
	set(p, "cmp", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Int(0), nil
		}
		a, e1 := parseRat(args[0])
		b, e2 := parseRat(args[1])
		if e1 != nil || e2 != nil {
			return runtime.Int(0), nil
		}
		return runtime.Int(int64(a.Cmp(b))), nil
	}, 2)

	set(p, "eq", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		a, e1 := parseRat(args[0])
		b, e2 := parseRat(args[1])
		if e1 != nil || e2 != nil {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(a.Cmp(b) == 0), nil
	}, 2)

	// decimal.string(a, places?) -> Result[str]
	set(p, "string", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Ok(runtime.Str("0")), nil
		}
		r, err := parseRat(args[0])
		if err != nil {
			return errRes(err.Error(), "decimal"), nil
		}
		places := 10
		if len(args) >= 2 {
			if n, e := runtime.AsInt(args[1]); e == nil && n >= 0 && n < 100 {
				places = int(n)
			}
		}
		return runtime.Ok(runtime.Str(r.FloatString(places))), nil
	}, 2)

	// decimal.float(a) -> Result[float]
	set(p, "float", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Ok(runtime.Float(0)), nil
		}
		r, err := parseRat(args[0])
		if err != nil {
			return errRes(err.Error(), "decimal"), nil
		}
		f, _ := r.Float64()
		return runtime.Ok(runtime.Float(f)), nil
	}, 1)

	// decimal.abs(a) -> Result[str]
	set(p, "abs", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Ok(runtime.Str("0")), nil
		}
		r, err := parseRat(args[0])
		if err != nil {
			return errRes(err.Error(), "decimal"), nil
		}
		return runtime.Ok(runtime.Str(formatRat(new(big.Rat).Abs(r)))), nil
	}, 1)

	// decimal.neg(a) -> Result[str]
	set(p, "neg", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Ok(runtime.Str("0")), nil
		}
		r, err := parseRat(args[0])
		if err != nil {
			return errRes(err.Error(), "decimal"), nil
		}
		return runtime.Ok(runtime.Str(formatRat(new(big.Rat).Neg(r)))), nil
	}, 1)

	return p
}

func parseRat(v runtime.Value) (*big.Rat, error) {
	switch v.Kind {
	case runtime.KindInt:
		return new(big.Rat).SetInt64(v.I), nil
	case runtime.KindFloat:
		r := new(big.Rat)
		if _, ok := r.SetString(fmt.Sprintf("%g", v.F)); !ok {
			r.SetFloat64(v.F)
		}
		return r, nil
	default:
		s := strings.TrimSpace(v.String())
		if s == "" {
			return nil, fmt.Errorf("empty decimal")
		}
		r := new(big.Rat)
		if _, ok := r.SetString(s); !ok {
			return nil, fmt.Errorf("invalid decimal %q", s)
		}
		return r, nil
	}
}

func formatRat(r *big.Rat) string {
	if r == nil {
		return "0"
	}
	// prefer decimal float string with enough precision
	return r.FloatString(16)
}

func binRat(op func(a, b *big.Rat) *big.Rat) runtime.Builtin {
	return func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("decimal op needs 2 args", "decimal"), nil
		}
		a, e1 := parseRat(args[0])
		b, e2 := parseRat(args[1])
		if e1 != nil {
			return errRes(e1.Error(), "decimal"), nil
		}
		if e2 != nil {
			return errRes(e2.Error(), "decimal"), nil
		}
		out := op(a, b)
		if out == nil {
			return errRes("division by zero", "decimal"), nil
		}
		return runtime.Ok(runtime.Str(formatRat(out))), nil
	}
}
