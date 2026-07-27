# Data visualization (Weft)

Build charts and dashboards **without a charting framework or a JS build step**.  
The `viz` package renders **SVG** in pure Go and can wrap charts in HTML for the browser or `web.app`.

## Quick start

```weft
fn main -> Result {
    c := viz.bar({"a": 10, "b": 20, "c": 15}, {"title": "Sales"})
    viz.save("sales.svg", c)?
    viz.save("sales.html", c)?          // full HTML page
    say(viz.spark([1, 3, 2, 5, 4]))    // ▁▃▂█▄
}
```

```bash
weft run examples/viz_charts.weft
open examples/viz_out/dashboard.html

weft run examples/viz_dashboard.weft   # http://127.0.0.1:8080
```

## Chart types

| Call | Data | Notes |
|------|------|--------|
| `viz.bar(data, opts?)` | map labels→values, or list of numbers | categorical bars |
| `viz.line(data, opts?)` | list of y, or `[[x,y],…]` | line + points |
| `viz.area(data, opts?)` | same as line | filled under line |
| `viz.scatter(data, opts?)` | `[[x,y],…]` or `{x,y}` maps | points |
| `viz.pie(data, opts?)` | map or labeled values | legend + % |
| `viz.hist(values, opts?)` | numeric list | `bins` option |

### Data shapes

```weft
viz.bar({"north": 3, "south": 7})
viz.bar([3, 7, 2])
viz.line([1, 3, 2, 5])
viz.scatter([[0, 1], [1, 3], [2, 2]])
viz.scatter([{"x": 0, "y": 1}, {"x": 1, "y": 3}])
viz.pie({"ok": 80, "err": 20})
```

### Options map

| Key | |
|-----|--|
| `title` | chart title |
| `width` `height` | pixels (default 640×400) |
| `color` | line/area stroke |
| `colors` | list of fill colors (bar/pie/scatter) |
| `xlabel` `ylabel` | axis labels |
| `bins` | histogram bins |

## Chart value

```weft
c := viz.bar([1, 2, 3])
c.kind    // "bar"
c.title
c.svg     // raw SVG markup
c.html    // <figure>…</figure> fragment
```

## Save & pages

```weft
viz.save("out.svg", chart)?
viz.save("out.html", chart)?           // wraps chart in a page
viz.html(chart)                        // HTML string, single chart
viz.page("Dashboard", [c1, c2, c3])    // multi-chart grid HTML
viz.svg(chart)                         // just the SVG string
```

## Tables & sparklines

```weft
// Unicode sparkline (terminal / logs)
say(viz.spark(metrics))

// Table → text + HTML
t := viz.table([
    ["name", "value"],
    ["cpu", 0.4],
    ["mem", 0.7],
])
say(t.text)
viz.save("table.html", viz.html(t))?

// list of maps also works
viz.table([{"name": "a", "n": 1}, {"name": "b", "n": 2}])
```

## With `web` (live dashboard)

```weft
fn main {
    app := web.app()
    app.get("/", fn(req) {
        c := viz.line(load_metrics(), {"title": "RPS"})
        web.html(viz.page("Ops", [c]))
    })
    app.listen(":8080")
}
```

## With agents / LLM pipelines

```weft
fn main -> Result {
    rows := json.parse(fs.read("stats.json")?)?
    // transform rows → series, then:
    c := viz.bar(rows, {"title": "From data"})
    viz.save("report.html", viz.page("Report", [c]))?
}
```

## Design notes

| | |
|--|--|
| Engine | Pure Go SVG — **no** npm |
| Output | `.svg`, `.html`, embed in `web.html(...)` |
| Style | Clean defaults (indigo palette); override with `color` / `colors` |
| Not v1 | Interactive zoom, 3D, WebGL, Excel export |

For heavy BI you can still emit data as JSON and point a front-end at it; `viz` covers the “script → chart file / dashboard” path without a separate charting toolchain.
