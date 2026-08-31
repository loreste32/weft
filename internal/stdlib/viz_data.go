package stdlib

import (
	"fmt"
	"math"
	"strconv"

	"github.com/loreste/weft/internal/runtime"
)

// chartSeries is normalized plot data.
type chartSeries struct {
	Labels []string
	X      []float64
	Y      []float64
}

type chartOpts struct {
	Title  string
	Width  float64
	Height float64
	Color  string
	Colors []string
	XLabel string
	YLabel string
	Bins   int
	// Stacked unused in v1
}

func defaultOpts() chartOpts {
	return chartOpts{
		Width:  640,
		Height: 400,
		Color:  "#4f46e5",
		Colors: defaultPalette(),
		Bins:   10,
	}
}

func defaultPalette() []string {
	return []string{
		"#4f46e5", "#06b6d4", "#10b981", "#f59e0b", "#ef4444",
		"#8b5cf6", "#ec4899", "#14b8a6", "#f97316", "#6366f1",
	}
}

func parseOpts(v runtime.Value) chartOpts {
	o := defaultOpts()
	if v.Kind != runtime.KindMap && v.Kind != runtime.KindStruct {
		return o
	}
	if s := mapGetStr(v, "title", ""); s != "" {
		o.Title = s
	}
	if n := mapGetInt(v, "width", 0); n > 0 {
		o.Width = float64(n)
	}
	if n := mapGetInt(v, "height", 0); n > 0 {
		o.Height = float64(n)
	}
	if s := mapGetStr(v, "color", ""); s != "" {
		o.Color = s
	}
	if s := mapGetStr(v, "xlabel", ""); s != "" {
		o.XLabel = s
	}
	if s := mapGetStr(v, "x_label", ""); s != "" {
		o.XLabel = s
	}
	if s := mapGetStr(v, "ylabel", ""); s != "" {
		o.YLabel = s
	}
	if s := mapGetStr(v, "y_label", ""); s != "" {
		o.YLabel = s
	}
	if n := mapGetInt(v, "bins", 0); n > 0 && n <= math.MaxInt32 {
		o.Bins = int(n)
	}
	if c, ok := mapGet(v, "colors"); ok && c.Kind == runtime.KindList {
		lo := c.Obj.(*runtime.ListObj)
		var cols []string
		for _, it := range lo.Items {
			cols = append(cols, it.String())
		}
		if len(cols) > 0 {
			o.Colors = cols
		}
	}
	return o
}

// parseSeries accepts:
//   - [1,2,3]  → labels 0..n, y=values
//   - [[x,y],...] or list of maps {x,y} / {label,value}
//   - map { "a": 1, "b": 2 }
func parseSeries(v runtime.Value) (chartSeries, error) {
	var s chartSeries
	switch v.Kind {
	case runtime.KindMap:
		mo := v.Obj.(*runtime.MapObj)
		// preserve key order
		for _, k := range mo.Keys {
			y, err := asFloat(mo.Vals[k])
			if err != nil {
				return s, fmt.Errorf("map value for %q: %w", k, err)
			}
			s.Labels = append(s.Labels, k)
			s.X = append(s.X, float64(len(s.X)))
			s.Y = append(s.Y, y)
		}
		if len(s.Y) == 0 {
			return s, fmt.Errorf("empty series")
		}
		return s, nil
	case runtime.KindList:
		lo := v.Obj.(*runtime.ListObj)
		if len(lo.Items) == 0 {
			return s, fmt.Errorf("empty series")
		}
		// detect pair / point
		first := lo.Items[0]
		if first.Kind == runtime.KindList {
			for i, it := range lo.Items {
				pair, ok := it.Obj.(*runtime.ListObj)
				if !ok || len(pair.Items) < 2 {
					return s, fmt.Errorf("point %d: want [x,y]", i)
				}
				x, err := asFloat(pair.Items[0])
				if err != nil {
					return s, err
				}
				y, err := asFloat(pair.Items[1])
				if err != nil {
					return s, err
				}
				s.X = append(s.X, x)
				s.Y = append(s.Y, y)
				s.Labels = append(s.Labels, fmt.Sprintf("%g", x))
			}
			return s, nil
		}
		if first.Kind == runtime.KindMap || first.Kind == runtime.KindStruct {
			for i, it := range lo.Items {
				x, hasX := mapFloat(it, "x")
				y, hasY := mapFloat(it, "y")
				if !hasY {
					if v, ok := mapFloat(it, "value"); ok {
						y, hasY = v, true
					}
				}
				label := mapGetStr(it, "label", "")
				if label == "" {
					label = mapGetStr(it, "name", "")
				}
				if hasX && hasY {
					s.X = append(s.X, x)
					s.Y = append(s.Y, y)
					if label == "" {
						label = fmt.Sprintf("%g", x)
					}
					s.Labels = append(s.Labels, label)
					continue
				}
				if hasY && label != "" {
					s.X = append(s.X, float64(i))
					s.Y = append(s.Y, y)
					s.Labels = append(s.Labels, label)
					continue
				}
				return s, fmt.Errorf("point %d: need {x,y} or {label,value}", i)
			}
			return s, nil
		}
		// plain numbers
		for i, it := range lo.Items {
			y, err := asFloat(it)
			if err != nil {
				return s, fmt.Errorf("index %d: %w", i, err)
			}
			s.X = append(s.X, float64(i))
			s.Y = append(s.Y, y)
			s.Labels = append(s.Labels, strconv.Itoa(i))
		}
		return s, nil
	default:
		return s, fmt.Errorf("series must be list or map, got %s", v.KindName())
	}
}

func mapFloat(m runtime.Value, key string) (float64, bool) {
	v, ok := mapGet(m, key)
	if !ok {
		return 0, false
	}
	f, err := asFloat(v)
	if err != nil {
		return 0, false
	}
	return f, true
}

func asFloat(v runtime.Value) (float64, error) {
	switch v.Kind {
	case runtime.KindInt:
		return float64(v.I), nil
	case runtime.KindFloat:
		return v.F, nil
	case runtime.KindStr:
		f, err := strconv.ParseFloat(v.S, 64)
		if err != nil {
			return 0, err
		}
		return f, nil
	default:
		return 0, fmt.Errorf("not a number")
	}
}

func minMax(ys []float64) (min, max float64) {
	if len(ys) == 0 {
		return 0, 1
	}
	min, max = ys[0], ys[0]
	for _, y := range ys[1:] {
		if y < min {
			min = y
		}
		if y > max {
			max = y
		}
	}
	if min == max {
		if min == 0 {
			max = 1
		} else if min > 0 {
			min = 0
		} else {
			max = 0
		}
	}
	return min, max
}

func nicePad(min, max float64) (float64, float64) {
	if min > 0 {
		min = 0 // bar charts baseline at 0 when all positive
	}
	span := max - min
	if span == 0 {
		span = 1
	}
	pad := span * 0.05
	return min, max + pad
}

func histogram(ys []float64, bins int) (labels []string, counts []float64) {
	if bins < 1 {
		bins = 10
	}
	if len(ys) == 0 {
		return nil, nil
	}
	min, max := minMax(ys)
	if min == max {
		max = min + 1
	}
	width := (max - min) / float64(bins)
	counts = make([]float64, bins)
	labels = make([]string, bins)
	for i := 0; i < bins; i++ {
		lo := min + float64(i)*width
		hi := lo + width
		labels[i] = fmt.Sprintf("%.2g", lo)
		_ = hi
	}
	for _, y := range ys {
		idx := int(math.Floor((y - min) / width))
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		counts[idx]++
	}
	return labels, counts
}

func chartValue(kind, title, svg string, opts chartOpts) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(k string, v runtime.Value) {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	put("kind", runtime.Str(kind))
	put("title", runtime.Str(title))
	put("svg", runtime.Str(svg))
	put("width", runtime.Int(int64(opts.Width)))
	put("height", runtime.Int(int64(opts.Height)))
	// html fragment for embedding
	frag := fmt.Sprintf(`<figure class="weft-chart" data-kind="%s">%s</figure>`, kind, svg)
	put("html", runtime.Str(frag))
	return m
}

func chartSVGOf(v runtime.Value) (string, error) {
	if v.Kind == runtime.KindStr {
		return v.S, nil
	}
	if s, ok := mapGet(v, "svg"); ok {
		return s.String(), nil
	}
	return "", fmt.Errorf("not a chart (need .svg)")
}
