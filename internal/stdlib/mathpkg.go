package stdlib

import (
	"math"

	"github.com/loreste/weft/internal/runtime"
)

// packageMath — numeric helpers (Python math lite).
func packageMath() runtime.Value {
	p := pkg()

	// constants
	mo := p.Obj.(*runtime.MapObj)
	for k, v := range map[string]runtime.Value{
		"pi":  runtime.Float(math.Pi),
		"e":   runtime.Float(math.E),
		"inf": runtime.Float(math.Inf(1)),
		"nan": runtime.Float(math.NaN()),
	} {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}

	num := func(args []runtime.Value, i int) (float64, bool) {
		if i >= len(args) {
			return 0, false
		}
		return asFloat64(args[i])
	}

	set(p, "abs", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Float(0), nil
		}
		if args[0].Kind == runtime.KindInt {
			n := args[0].I
			if n < 0 {
				n = -n
			}
			return runtime.Int(n), nil
		}
		return runtime.Float(math.Abs(x)), nil
	}, 1)

	set(p, "sqrt", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok || x < 0 {
			return runtime.Float(math.NaN()), nil
		}
		return runtime.Float(math.Sqrt(x)), nil
	}, 1)

	set(p, "pow", func(args []runtime.Value) (runtime.Value, error) {
		a, ok1 := num(args, 0)
		b, ok2 := num(args, 1)
		if !ok1 || !ok2 {
			return runtime.Float(0), nil
		}
		return runtime.Float(math.Pow(a, b)), nil
	}, 2)

	set(p, "floor", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Int(0), nil
		}
		return runtime.Int(int64(math.Floor(x))), nil
	}, 1)

	set(p, "ceil", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Int(0), nil
		}
		return runtime.Int(int64(math.Ceil(x))), nil
	}, 1)

	set(p, "round", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Float(0), nil
		}
		return runtime.Float(math.Round(x)), nil
	}, 1)

	set(p, "min", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) == 0 {
			return runtime.Null(), nil
		}
		// min(a, b, ...) or min([list])
		if len(args) == 1 && args[0].Kind == runtime.KindList {
			items := args[0].Obj.(*runtime.ListObj).Items
			if len(items) == 0 {
				return runtime.Null(), nil
			}
			best, _ := asFloat64(items[0])
			out := items[0]
			for _, it := range items[1:] {
				f, ok := asFloat64(it)
				if ok && f < best {
					best, out = f, it
				}
			}
			return out, nil
		}
		best, _ := asFloat64(args[0])
		out := args[0]
		for _, a := range args[1:] {
			f, ok := asFloat64(a)
			if ok && f < best {
				best, out = f, a
			}
		}
		return out, nil
	}, -1)

	set(p, "max", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) == 0 {
			return runtime.Null(), nil
		}
		if len(args) == 1 && args[0].Kind == runtime.KindList {
			items := args[0].Obj.(*runtime.ListObj).Items
			if len(items) == 0 {
				return runtime.Null(), nil
			}
			best, _ := asFloat64(items[0])
			out := items[0]
			for _, it := range items[1:] {
				f, ok := asFloat64(it)
				if ok && f > best {
					best, out = f, it
				}
			}
			return out, nil
		}
		best, _ := asFloat64(args[0])
		out := args[0]
		for _, a := range args[1:] {
			f, ok := asFloat64(a)
			if ok && f > best {
				best, out = f, a
			}
		}
		return out, nil
	}, -1)

	set(p, "clamp", func(args []runtime.Value) (runtime.Value, error) {
		x, ok0 := num(args, 0)
		lo, ok1 := num(args, 1)
		hi, ok2 := num(args, 2)
		if !ok0 || !ok1 || !ok2 {
			return runtime.Float(0), nil
		}
		if x < lo {
			x = lo
		}
		if x > hi {
			x = hi
		}
		if args[0].Kind == runtime.KindInt && args[1].Kind == runtime.KindInt && args[2].Kind == runtime.KindInt {
			return runtime.Int(int64(x)), nil
		}
		return runtime.Float(x), nil
	}, 3)

	set(p, "sin", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Float(0), nil
		}
		return runtime.Float(math.Sin(x)), nil
	}, 1)
	set(p, "cos", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Float(0), nil
		}
		return runtime.Float(math.Cos(x)), nil
	}, 1)
	set(p, "tan", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Float(0), nil
		}
		return runtime.Float(math.Tan(x)), nil
	}, 1)
	set(p, "log", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok || x <= 0 {
			return runtime.Float(math.NaN()), nil
		}
		return runtime.Float(math.Log(x)), nil
	}, 1)
	set(p, "log10", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok || x <= 0 {
			return runtime.Float(math.NaN()), nil
		}
		return runtime.Float(math.Log10(x)), nil
	}, 1)
	set(p, "exp", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Float(0), nil
		}
		return runtime.Float(math.Exp(x)), nil
	}, 1)
	set(p, "hypot", func(args []runtime.Value) (runtime.Value, error) {
		a, ok1 := num(args, 0)
		b, ok2 := num(args, 1)
		if !ok1 || !ok2 {
			return runtime.Float(0), nil
		}
		return runtime.Float(math.Hypot(a, b)), nil
	}, 2)
	set(p, "isnan", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		return runtime.Bool(ok && math.IsNaN(x)), nil
	}, 1)
	set(p, "isinf", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		return runtime.Bool(ok && math.IsInf(x, 0)), nil
	}, 1)

	// statistics lite (Python statistics cousin) — list of numbers
	floats := func(args []runtime.Value) []float64 {
		var xs []float64
		if len(args) == 1 && args[0].Kind == runtime.KindList {
			for _, it := range args[0].Obj.(*runtime.ListObj).Items {
				if f, ok := asFloat64(it); ok {
					xs = append(xs, f)
				}
			}
			return xs
		}
		for _, a := range args {
			if f, ok := asFloat64(a); ok {
				xs = append(xs, f)
			}
		}
		return xs
	}

	set(p, "sum", func(args []runtime.Value) (runtime.Value, error) {
		xs := floats(args)
		var s float64
		allInt := true
		if len(args) == 1 && args[0].Kind == runtime.KindList {
			for _, it := range args[0].Obj.(*runtime.ListObj).Items {
				if it.Kind != runtime.KindInt {
					allInt = false
					break
				}
			}
		} else {
			for _, a := range args {
				if a.Kind != runtime.KindInt {
					allInt = false
					break
				}
			}
		}
		for _, x := range xs {
			s += x
		}
		if allInt && len(xs) > 0 {
			return runtime.Int(int64(s)), nil
		}
		return runtime.Float(s), nil
	}, -1)

	set(p, "mean", func(args []runtime.Value) (runtime.Value, error) {
		xs := floats(args)
		if len(xs) == 0 {
			return runtime.Float(math.NaN()), nil
		}
		var s float64
		for _, x := range xs {
			s += x
		}
		return runtime.Float(s / float64(len(xs))), nil
	}, -1)

	set(p, "median", func(args []runtime.Value) (runtime.Value, error) {
		xs := floats(args)
		n := len(xs)
		if n == 0 {
			return runtime.Float(math.NaN()), nil
		}
		// insertion sort copy
		ys := append([]float64(nil), xs...)
		for i := 1; i < n; i++ {
			j := i
			for j > 0 && ys[j-1] > ys[j] {
				ys[j-1], ys[j] = ys[j], ys[j-1]
				j--
			}
		}
		if n%2 == 1 {
			return runtime.Float(ys[n/2]), nil
		}
		return runtime.Float((ys[n/2-1] + ys[n/2]) / 2), nil
	}, -1)

	set(p, "stdev", func(args []runtime.Value) (runtime.Value, error) {
		xs := floats(args)
		n := len(xs)
		if n < 2 {
			return runtime.Float(math.NaN()), nil
		}
		var s float64
		for _, x := range xs {
			s += x
		}
		mean := s / float64(n)
		var v float64
		for _, x := range xs {
			d := x - mean
			v += d * d
		}
		// sample stdev
		return runtime.Float(math.Sqrt(v / float64(n-1))), nil
	}, -1)

	// math.variance(list) — sample variance
	set(p, "variance", func(args []runtime.Value) (runtime.Value, error) {
		xs := floats(args)
		n := len(xs)
		if n < 2 {
			return runtime.Float(math.NaN()), nil
		}
		var s float64
		for _, x := range xs {
			s += x
		}
		mean := s / float64(n)
		var v float64
		for _, x := range xs {
			d := x - mean
			v += d * d
		}
		return runtime.Float(v / float64(n-1)), nil
	}, -1)

	set(p, "degrees", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Float(0), nil
		}
		return runtime.Float(x * 180 / math.Pi), nil
	}, 1)

	set(p, "radians", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Float(0), nil
		}
		return runtime.Float(x * math.Pi / 180), nil
	}, 1)

	set(p, "atan2", func(args []runtime.Value) (runtime.Value, error) {
		y, ok1 := num(args, 0)
		x, ok2 := num(args, 1)
		if !ok1 || !ok2 {
			return runtime.Float(0), nil
		}
		return runtime.Float(math.Atan2(y, x)), nil
	}, 2)

	set(p, "trunc", func(args []runtime.Value) (runtime.Value, error) {
		x, ok := num(args, 0)
		if !ok {
			return runtime.Int(0), nil
		}
		return runtime.Int(int64(math.Trunc(x))), nil
	}, 1)

	// math.gcd(a, b) / lcm(a, b)
	set(p, "gcd", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Int(0), nil
		}
		a, err1 := runtime.AsInt(args[0])
		b, err2 := runtime.AsInt(args[1])
		if err1 != nil || err2 != nil {
			return runtime.Int(0), nil
		}
		if a < 0 {
			a = -a
		}
		if b < 0 {
			b = -b
		}
		for b != 0 {
			a, b = b, a%b
		}
		return runtime.Int(a), nil
	}, 2)

	set(p, "lcm", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Int(0), nil
		}
		a, err1 := runtime.AsInt(args[0])
		b, err2 := runtime.AsInt(args[1])
		if err1 != nil || err2 != nil {
			return runtime.Int(0), nil
		}
		if a == 0 || b == 0 {
			return runtime.Int(0), nil
		}
		aa, bb := a, b
		if aa < 0 {
			aa = -aa
		}
		if bb < 0 {
			bb = -bb
		}
		g, btmp := aa, bb
		for btmp != 0 {
			g, btmp = btmp, g%btmp
		}
		return runtime.Int(aa / g * bb), nil
	}, 2)

	// math.quantile(list, q) -> float  q in [0,1], linear interpolation
	set(p, "quantile", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[0].Kind != runtime.KindList {
			return runtime.Float(math.NaN()), nil
		}
		q, ok := num(args, 1)
		if !ok || q < 0 || q > 1 {
			return runtime.Float(math.NaN()), nil
		}
		var xs []float64
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			if x, ok := asFloat64(it); ok {
				xs = append(xs, x)
			}
		}
		if len(xs) == 0 {
			return runtime.Float(math.NaN()), nil
		}
		// insertion sort
		for i := 1; i < len(xs); i++ {
			j := i
			for j > 0 && xs[j-1] > xs[j] {
				xs[j-1], xs[j] = xs[j], xs[j-1]
				j--
			}
		}
		if len(xs) == 1 {
			return runtime.Float(xs[0]), nil
		}
		pos := q * float64(len(xs)-1)
		lo := int(math.Floor(pos))
		hi := int(math.Ceil(pos))
		if lo == hi {
			return runtime.Float(xs[lo]), nil
		}
		w := pos - float64(lo)
		return runtime.Float(xs[lo]*(1-w) + xs[hi]*w), nil
	}, 2)

	// math.mode(list) -> most common number|str (first on ties)
	set(p, "mode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.Null(), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		if len(items) == 0 {
			return runtime.Null(), nil
		}
		counts := map[string]int{}
		first := map[string]runtime.Value{}
		order := []string{}
		for _, it := range items {
			k := it.String()
			if _, ok := counts[k]; !ok {
				order = append(order, k)
				first[k] = it
			}
			counts[k]++
		}
		bestK, bestN := order[0], counts[order[0]]
		for _, k := range order[1:] {
			if counts[k] > bestN {
				bestK, bestN = k, counts[k]
			}
		}
		return first[bestK], nil
	}, 1)

	return p
}

func asFloat64(v runtime.Value) (float64, bool) {
	switch v.Kind {
	case runtime.KindInt:
		return float64(v.I), true
	case runtime.KindFloat:
		return v.F, true
	default:
		n, err := runtime.AsInt(v)
		if err == nil {
			return float64(n), true
		}
		return 0, false
	}
}
