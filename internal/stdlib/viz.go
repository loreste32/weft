package stdlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageViz is pure-Go data visualization: SVG charts, HTML dashboards, sparklines.
func packageViz(env *runtime.Env) runtime.Value {
	p := pkg()

	// viz.bar(data, opts?) -> chart
	set(p, "bar", func(args []runtime.Value) (runtime.Value, error) {
		return makeChart("bar", args, func(s chartSeries, o chartOpts) string {
			return renderBar(s, o)
		})
	}, 2)
	set(p, "line", func(args []runtime.Value) (runtime.Value, error) {
		return makeChart("line", args, func(s chartSeries, o chartOpts) string {
			return renderLine(s, o, false)
		})
	}, 2)
	set(p, "area", func(args []runtime.Value) (runtime.Value, error) {
		return makeChart("area", args, func(s chartSeries, o chartOpts) string {
			return renderLine(s, o, true)
		})
	}, 2)
	set(p, "scatter", func(args []runtime.Value) (runtime.Value, error) {
		return makeChart("scatter", args, func(s chartSeries, o chartOpts) string {
			return renderScatter(s, o)
		})
	}, 2)
	set(p, "pie", func(args []runtime.Value) (runtime.Value, error) {
		return makeChart("pie", args, func(s chartSeries, o chartOpts) string {
			return renderPie(s, o)
		})
	}, 2)
	// viz.hist(values, opts?) — histogram from numeric list
	set(p, "hist", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("viz.hist(values, opts?)", "viz"), nil
		}
		o := defaultOpts()
		if len(args) >= 2 {
			o = parseOpts(args[1])
		}
		s, err := parseSeries(args[0])
		if err != nil {
			return errRes(err.Error(), "viz"), nil
		}
		labels, counts := histogram(s.Y, o.Bins)
		hs := chartSeries{Labels: labels, Y: counts}
		for i := range counts {
			hs.X = append(hs.X, float64(i))
		}
		if o.Title == "" {
			o.Title = "Histogram"
		}
		svg := renderBar(hs, o)
		return chartValue("hist", o.Title, svg, o), nil
	}, 2)

	// viz.svg(chart) -> str
	set(p, "svg", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("viz.svg(chart)", "viz"), nil
		}
		s, err := chartSVGOf(args[0])
		if err != nil {
			return errRes(err.Error(), "viz"), nil
		}
		return runtime.Str(s), nil
	}, 1)

	// viz.spark(values) -> unicode sparkline str
	set(p, "spark", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		s, err := parseSeries(args[0])
		if err != nil {
			return errRes(err.Error(), "viz"), nil
		}
		return runtime.Str(sparkline(s.Y)), nil
	}, 1)

	// viz.table(rows, opts?) -> {text, html}
	// rows: [[...], ...] or list of maps
	set(p, "table", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("viz.table(rows)", "viz"), nil
		}
		rows, err := parseTable(args[0])
		if err != nil {
			return errRes(err.Error(), "viz"), nil
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"text", "html", "kind"}
		mo.Vals["text"] = runtime.Str(renderTableText(rows))
		mo.Vals["html"] = runtime.Str(renderTableHTML(rows))
		mo.Vals["kind"] = runtime.Str("table")
		return m, nil
	}, 2)

	// viz.page(title, charts, opts?) -> html str
	// charts: list of chart maps
	set(p, "page", func(args []runtime.Value) (runtime.Value, error) {
		title := "Dashboard"
		var charts []runtime.Value
		if len(args) >= 1 {
			if args[0].Kind == runtime.KindList {
				charts = args[0].Obj.(*runtime.ListObj).Items
			} else {
				title = args[0].String()
			}
		}
		if len(args) >= 2 {
			if args[1].Kind == runtime.KindList {
				charts = args[1].Obj.(*runtime.ListObj).Items
			} else if args[0].Kind == runtime.KindList {
				title = args[1].String()
			}
		}
		if len(args) >= 3 && args[2].Kind == runtime.KindMap {
			if t := mapGetStr(args[2], "title", ""); t != "" {
				title = t
			}
		}
		var cards strings.Builder
		cards.WriteString(`<div class="grid">`)
		for _, c := range charts {
			svg, err := chartSVGOf(c)
			if err != nil {
				// table?
				if h, ok := mapGet(c, "html"); ok {
					fmt.Fprintf(&cards, `<div class="card">%s</div>`, h.String())
					continue
				}
				continue
			}
			ctitle := mapGetStr(c, "title", "")
			fmt.Fprintf(&cards, `<div class="card">`)
			if ctitle != "" {
				fmt.Fprintf(&cards, `<div style="font-weight:600;margin-bottom:.5rem;font-size:14px">%s</div>`, esc(ctitle))
			}
			cards.WriteString(svg)
			cards.WriteString(`</div>`)
		}
		cards.WriteString(`</div>`)
		return runtime.Str(fullHTMLPage(title, cards.String())), nil
	}, 3)

	// viz.save(path, chart|html|svg) -> Result
	set(p, "save", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("viz.save(path, chart|html)", "viz"), nil
		}
		path := args[0].String()
		content, err := materializeSave(args[1], path)
		if err != nil {
			return errRes(err.Error(), "viz"), nil
		}
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return errRes(err.Error(), "viz"), nil
			}
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return errRes(err.Error(), "viz"), nil
		}
		return runtime.Ok(runtime.Str(path)), nil
	}, 2)

	// viz.html(chart) -> full single-chart HTML page
	set(p, "html", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("viz.html(chart)", "viz"), nil
		}
		title := mapGetStr(args[0], "title", "Chart")
		svg, err := chartSVGOf(args[0])
		if err != nil {
			if h, ok := mapGet(args[0], "html"); ok {
				return runtime.Str(fullHTMLPage(title, `<div class="card">`+h.String()+`</div>`)), nil
			}
			return errRes(err.Error(), "viz"), nil
		}
		return runtime.Str(fullHTMLPage(title, `<div class="card">`+svg+`</div>`)), nil
	}, 1)

	return p
}

func makeChart(kind string, args []runtime.Value, render func(chartSeries, chartOpts) string) (runtime.Value, error) {
	if len(args) < 1 {
		return errRes("viz."+kind+"(data, opts?)", "viz"), nil
	}
	o := defaultOpts()
	if len(args) >= 2 {
		o = parseOpts(args[1])
	}
	s, err := parseSeries(args[0])
	if err != nil {
		return errRes(err.Error(), "viz"), nil
	}
	if o.Title == "" {
		o.Title = kindTitle(kind)
	}
	svg := render(s, o)
	return chartValue(kind, o.Title, svg, o), nil
}

func kindTitle(kind string) string {
	if kind == "" {
		return "Chart"
	}
	return strings.ToUpper(kind[:1]) + kind[1:]
}

func materializeSave(v runtime.Value, path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		if v.Kind == runtime.KindStr {
			s := v.S
			if strings.Contains(s, "<html") || strings.Contains(s, "<!doctype") {
				return s, nil
			}
			// raw svg string → wrap
			if strings.Contains(s, "<svg") {
				return fullHTMLPage(filepath.Base(path), `<div class="card">`+s+`</div>`), nil
			}
			return fullHTMLPage(filepath.Base(path), s), nil
		}
		if _, ok := mapGet(v, "svg"); ok {
			title := mapGetStr(v, "title", "Chart")
			svg, _ := chartSVGOf(v)
			return fullHTMLPage(title, `<div class="card">`+svg+`</div>`), nil
		}
		if h, ok := mapGet(v, "html"); ok && mapGetStr(v, "kind", "") == "table" {
			return fullHTMLPage("Table", `<div class="card">`+h.String()+`</div>`), nil
		}
		return "", fmt.Errorf("cannot save as html")
	case ".svg":
		return chartSVGOf(v)
	default:
		// prefer svg content
		if v.Kind == runtime.KindStr {
			return v.S, nil
		}
		if s, err := chartSVGOf(v); err == nil {
			return s, nil
		}
		if h, ok := mapGet(v, "html"); ok {
			return h.String(), nil
		}
		if t, ok := mapGet(v, "text"); ok {
			return t.String(), nil
		}
		return "", fmt.Errorf("unsupported save target")
	}
}

func parseTable(v runtime.Value) ([][]string, error) {
	if v.Kind != runtime.KindList {
		return nil, fmt.Errorf("table needs list of rows")
	}
	lo := v.Obj.(*runtime.ListObj)
	if len(lo.Items) == 0 {
		return nil, nil
	}
	// list of maps → header from keys of first
	if lo.Items[0].Kind == runtime.KindMap || lo.Items[0].Kind == runtime.KindStruct {
		var keys []string
		if lo.Items[0].Kind == runtime.KindMap {
			keys = append(keys, lo.Items[0].Obj.(*runtime.MapObj).Keys...)
		} else {
			so := lo.Items[0].Obj.(*runtime.StructObj)
			keys = append(keys, so.Order...)
			if len(keys) == 0 {
				for k := range so.Fields {
					keys = append(keys, k)
				}
			}
		}
		rows := [][]string{keys}
		for _, it := range lo.Items {
			row := make([]string, len(keys))
			for i, k := range keys {
				if val, ok := mapGet(it, k); ok {
					row[i] = val.String()
				}
			}
			rows = append(rows, row)
		}
		return rows, nil
	}
	var rows [][]string
	for _, it := range lo.Items {
		if it.Kind != runtime.KindList {
			return nil, fmt.Errorf("row must be list")
		}
		cells := it.Obj.(*runtime.ListObj)
		row := make([]string, len(cells.Items))
		for i, c := range cells.Items {
			row[i] = c.String()
		}
		rows = append(rows, row)
	}
	return rows, nil
}
