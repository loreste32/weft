package stdlib

import (
	"fmt"
	"html"
	"math"
	"strings"
)

func esc(s string) string { return html.EscapeString(s) }

func renderBar(s chartSeries, o chartOpts) string {
	w, h := o.Width, o.Height
	padL, padR, padT, padB := 48.0, 16.0, 36.0, 48.0
	if o.Title == "" {
		padT = 16
	}
	plotW := w - padL - padR
	plotH := h - padT - padB
	ymin, ymax := nicePad(minMax(s.Y))
	n := len(s.Y)
	if n == 0 {
		return emptySVG(w, h, "no data")
	}
	gap := plotW * 0.08 / float64(n)
	bw := (plotW - gap*float64(n+1)) / float64(n)
	if bw < 2 {
		bw = 2
	}

	var b strings.Builder
	svgOpen(&b, w, h, o.Title)
	// axes
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#94a3b8" stroke-width="1"/>`,
		padL, padT, padL, padT+plotH)
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#94a3b8" stroke-width="1"/>`,
		padL, padT+plotH, padL+plotW, padT+plotH)
	// y ticks
	for i := 0; i <= 4; i++ {
		t := float64(i) / 4
		y := padT + plotH*(1-t)
		val := ymin + (ymax-ymin)*t
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#e2e8f0" stroke-width="1"/>`,
			padL, y, padL+plotW, y)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="end" font-size="11" fill="#64748b" font-family="system-ui,sans-serif">%s</text>`,
			padL-6, y+4, formatNum(val))
	}
	for i, yv := range s.Y {
		x := padL + gap + float64(i)*(bw+gap)
		frac := (yv - ymin) / (ymax - ymin)
		bh := plotH * frac
		y := padT + plotH - bh
		col := o.Colors[i%len(o.Colors)]
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" rx="3"/>`,
			x, y, bw, math.Max(bh, 0.5), col)
		lab := s.Labels[i]
		if len(lab) > 12 {
			lab = lab[:11] + "…"
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="10" fill="#475569" font-family="system-ui,sans-serif">%s</text>`,
			x+bw/2, padT+plotH+14, esc(lab))
	}
	axisLabels(&b, o, padL, padT, plotW, plotH, w, h)
	svgClose(&b)
	return b.String()
}

func renderLine(s chartSeries, o chartOpts, fill bool) string {
	w, h := o.Width, o.Height
	padL, padR, padT, padB := 48.0, 16.0, 36.0, 40.0
	if o.Title == "" {
		padT = 16
	}
	plotW := w - padL - padR
	plotH := h - padT - padB
	n := len(s.Y)
	if n == 0 {
		return emptySVG(w, h, "no data")
	}
	xmin, xmax := minMax(s.X)
	ymin, ymax := minMax(s.Y)
	_, ymax = nicePad(ymin, ymax)
	if ymin > 0 && fill {
		ymin = 0
	}
	if xmax == xmin {
		xmax = xmin + 1
	}
	if ymax == ymin {
		ymax = ymin + 1
	}

	var b strings.Builder
	svgOpen(&b, w, h, o.Title)
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#94a3b8" stroke-width="1"/>`,
		padL, padT, padL, padT+plotH)
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#94a3b8" stroke-width="1"/>`,
		padL, padT+plotH, padL+plotW, padT+plotH)
	for i := 0; i <= 4; i++ {
		t := float64(i) / 4
		y := padT + plotH*(1-t)
		val := ymin + (ymax-ymin)*t
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#e2e8f0"/>`,
			padL, y, padL+plotW, y)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="end" font-size="11" fill="#64748b" font-family="system-ui,sans-serif">%s</text>`,
			padL-6, y+4, formatNum(val))
	}

	px := func(x float64) float64 {
		return padL + (x-xmin)/(xmax-xmin)*plotW
	}
	py := func(y float64) float64 {
		return padT + plotH - (y-ymin)/(ymax-ymin)*plotH
	}

	var pts strings.Builder
	for i := range s.Y {
		if i > 0 {
			pts.WriteByte(' ')
		}
		fmt.Fprintf(&pts, "%.1f,%.1f", px(s.X[i]), py(s.Y[i]))
	}
	if fill {
		// area path
		var path strings.Builder
		fmt.Fprintf(&path, "M %.1f %.1f", px(s.X[0]), py(ymin))
		for i := range s.Y {
			fmt.Fprintf(&path, " L %.1f %.1f", px(s.X[i]), py(s.Y[i]))
		}
		fmt.Fprintf(&path, " L %.1f %.1f Z", px(s.X[n-1]), py(ymin))
		fmt.Fprintf(&b, `<path d="%s" fill="%s" fill-opacity="0.25" stroke="none"/>`, path.String(), o.Color)
	}
	fmt.Fprintf(&b, `<polyline fill="none" stroke="%s" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round" points="%s"/>`,
		o.Color, pts.String())
	for i := range s.Y {
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="3.5" fill="%s" stroke="#fff" stroke-width="1"/>`,
			px(s.X[i]), py(s.Y[i]), o.Color)
	}
	axisLabels(&b, o, padL, padT, plotW, plotH, w, h)
	svgClose(&b)
	return b.String()
}

func renderScatter(s chartSeries, o chartOpts) string {
	w, h := o.Width, o.Height
	padL, padR, padT, padB := 48.0, 16.0, 36.0, 40.0
	if o.Title == "" {
		padT = 16
	}
	plotW := w - padL - padR
	plotH := h - padT - padB
	if len(s.Y) == 0 {
		return emptySVG(w, h, "no data")
	}
	xmin, xmax := minMax(s.X)
	ymin, ymax := minMax(s.Y)
	if xmax == xmin {
		xmax = xmin + 1
	}
	if ymax == ymin {
		ymax = ymin + 1
	}
	// pad scatter
	dx, dy := (xmax-xmin)*0.08, (ymax-ymin)*0.08
	xmin -= dx
	xmax += dx
	ymin -= dy
	ymax += dy

	var b strings.Builder
	svgOpen(&b, w, h, o.Title)
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#94a3b8"/>`, padL, padT, padL, padT+plotH)
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#94a3b8"/>`, padL, padT+plotH, padL+plotW, padT+plotH)
	px := func(x float64) float64 { return padL + (x-xmin)/(xmax-xmin)*plotW }
	py := func(y float64) float64 { return padT + plotH - (y-ymin)/(ymax-ymin)*plotH }
	for i := range s.Y {
		col := o.Colors[i%len(o.Colors)]
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="5" fill="%s" fill-opacity="0.85"/>`,
			px(s.X[i]), py(s.Y[i]), col)
	}
	axisLabels(&b, o, padL, padT, plotW, plotH, w, h)
	svgClose(&b)
	return b.String()
}

func renderPie(s chartSeries, o chartOpts) string {
	w, h := o.Width, o.Height
	if h < w*0.75 {
		// ok
	}
	cx, cy := w*0.38, h*0.52
	r := math.Min(w, h) * 0.32
	sum := 0.0
	for _, y := range s.Y {
		if y > 0 {
			sum += y
		}
	}
	if sum <= 0 {
		return emptySVG(w, h, "no positive values")
	}
	var b strings.Builder
	svgOpen(&b, w, h, o.Title)
	angle := -math.Pi / 2
	for i, yv := range s.Y {
		if yv <= 0 {
			continue
		}
		sweep := yv / sum * 2 * math.Pi
		x1 := cx + r*math.Cos(angle)
		y1 := cy + r*math.Sin(angle)
		angle2 := angle + sweep
		x2 := cx + r*math.Cos(angle2)
		y2 := cy + r*math.Sin(angle2)
		large := 0
		if sweep > math.Pi {
			large = 1
		}
		col := o.Colors[i%len(o.Colors)]
		fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f A %.1f %.1f 0 %d 1 %.1f %.1f Z" fill="%s" stroke="#fff" stroke-width="1.5"/>`,
			cx, cy, x1, y1, r, r, large, x2, y2, col)
		angle = angle2
	}
	// legend
	lx, ly := w*0.68, h*0.28
	for i, lab := range s.Labels {
		if i >= len(s.Y) || s.Y[i] <= 0 {
			continue
		}
		col := o.Colors[i%len(o.Colors)]
		pct := s.Y[i] / sum * 100
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="12" height="12" rx="2" fill="%s"/>`, lx, ly+float64(i)*22, col)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="12" fill="#334155" font-family="system-ui,sans-serif">%s (%.0f%%)</text>`,
			lx+18, ly+float64(i)*22+10, esc(lab), pct)
	}
	svgClose(&b)
	return b.String()
}

func svgOpen(b *strings.Builder, w, h float64, title string) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img">`, w, h, w, h)
	fmt.Fprintf(b, `<rect width="100%%" height="100%%" fill="#fafafa" rx="8"/>`)
	if title != "" {
		fmt.Fprintf(b, `<text x="%.1f" y="22" text-anchor="middle" font-size="15" font-weight="600" fill="#0f172a" font-family="system-ui,sans-serif">%s</text>`,
			w/2, esc(title))
	}
}

func svgClose(b *strings.Builder) {
	b.WriteString(`</svg>`)
}

func emptySVG(w, h float64, msg string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f"><rect width="100%%" height="100%%" fill="#f8fafc"/><text x="50%%" y="50%%" text-anchor="middle" fill="#94a3b8" font-family="system-ui">%s</text></svg>`,
		w, h, w, h, esc(msg))
}

func axisLabels(b *strings.Builder, o chartOpts, padL, padT, plotW, plotH, w, h float64) {
	if o.XLabel != "" {
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="11" fill="#64748b" font-family="system-ui,sans-serif">%s</text>`,
			padL+plotW/2, h-8, esc(o.XLabel))
	}
	if o.YLabel != "" {
		fmt.Fprintf(b, `<text x="14" y="%.1f" text-anchor="middle" font-size="11" fill="#64748b" font-family="system-ui,sans-serif" transform="rotate(-90 14 %.1f)">%s</text>`,
			padT+plotH/2, padT+plotH/2, esc(o.YLabel))
	}
}

func formatNum(v float64) string {
	if math.Abs(v) >= 1000 {
		return fmt.Sprintf("%.2g", v)
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e9 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2g", v)
}

func sparkline(ys []float64) string {
	if len(ys) == 0 {
		return ""
	}
	blocks := []rune("▁▂▃▄▅▆▇█")
	min, max := minMax(ys)
	span := max - min
	if span == 0 {
		span = 1
	}
	var b strings.Builder
	for _, y := range ys {
		t := (y - min) / span
		idx := int(math.Round(t * float64(len(blocks)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}

func renderTableHTML(rows [][]string) string {
	if len(rows) == 0 {
		return "<table></table>"
	}
	var b strings.Builder
	b.WriteString(`<table class="weft-table" style="border-collapse:collapse;font-family:system-ui,sans-serif;font-size:14px">`)
	for i, row := range rows {
		b.WriteString("<tr>")
		tag := "td"
		if i == 0 {
			tag = "th"
		}
		for _, cell := range row {
			fmt.Fprintf(&b, `<%s style="border:1px solid #e2e8f0;padding:6px 10px;text-align:left">%s</%s>`,
				tag, esc(cell), tag)
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</table>")
	return b.String()
}

func renderTableText(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	// column widths
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	widths := make([]int, cols)
	for _, r := range rows {
		for i, c := range r {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}
	var b strings.Builder
	for ri, r := range rows {
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(r) {
				cell = r[i]
			}
			fmt.Fprintf(&b, "%-*s", widths[i]+2, cell)
		}
		b.WriteByte('\n')
		if ri == 0 {
			for i := 0; i < cols; i++ {
				b.WriteString(strings.Repeat("-", widths[i]+2))
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func fullHTMLPage(title string, body string) string {
	if title == "" {
		title = "Weft chart"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
  body{font-family:system-ui,sans-serif;margin:0;padding:1.5rem;background:#f1f5f9;color:#0f172a}
  h1{font-size:1.25rem;margin:0 0 1rem}
  .grid{display:grid;gap:1rem;grid-template-columns:repeat(auto-fit,minmax(280px,1fr))}
  .card{background:#fff;border-radius:12px;padding:1rem;box-shadow:0 1px 3px rgb(0 0 0/0.08)}
  .card svg{max-width:100%%;height:auto;display:block}
  footer{margin-top:1.5rem;font-size:12px;color:#94a3b8}
</style>
</head>
<body>
<h1>%s</h1>
%s
<footer>made with Weft viz</footer>
</body>
</html>
`, esc(title), esc(title), body)
}
