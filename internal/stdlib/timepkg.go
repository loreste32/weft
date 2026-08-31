package stdlib

import (
	"math"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// safeTimeInt converts int64 to int for time.Date args, clamped to safe range.
func safeTimeInt(n int64) int {
	if n < math.MinInt32 {
		return math.MinInt32
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(n)
}

// packageTime — clocks and sleeps for devops CLIs.
func packageTime() runtime.Value {
	p := pkg()

	// time.now() -> unix seconds (int)
	set(p, "now", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Int(time.Now().Unix()), nil
	}, 0)

	// time.unix(sec) -> same (identity / document alias of int as epoch)
	set(p, "unix", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Int(time.Now().Unix()), nil
		}
		n, err := runtime.AsInt(args[0])
		if err != nil {
			return runtime.Int(0), nil
		}
		return runtime.Int(n), nil
	}, 1)

	// time.now_ms() -> unix millis
	set(p, "now_ms", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Int(time.Now().UnixMilli()), nil
	}, 0)

	// time.iso() -> RFC3339 string UTC
	set(p, "iso", func(args []runtime.Value) (runtime.Value, error) {
		t := time.Now().UTC()
		if len(args) >= 1 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				t = time.Unix(n, 0).UTC()
			}
		}
		return runtime.Str(t.Format(time.RFC3339)), nil
	}, 1)

	// time.format(layout?, unix?) — Go layout; default RFC3339 local
	set(p, "format", func(args []runtime.Value) (runtime.Value, error) {
		layout := time.RFC3339
		t := time.Now()
		if len(args) >= 1 && args[0].String() != "" {
			layout = args[0].String()
			// friendly aliases
			switch layout {
			case "date":
				layout = "2006-01-02"
			case "time":
				layout = "15:04:05"
			case "datetime":
				layout = "2006-01-02 15:04:05"
			case "iso":
				layout = time.RFC3339
			}
		}
		if len(args) >= 2 {
			if n, err := runtime.AsInt(args[1]); err == nil {
				t = time.Unix(n, 0)
			}
		}
		return runtime.Str(t.Format(layout)), nil
	}, 2)

	// time.sleep(seconds) — float or int; blocks
	set(p, "sleep", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Unit(), nil
		}
		var d time.Duration
		switch args[0].Kind {
		case runtime.KindInt:
			d = time.Duration(args[0].I) * time.Second
		case runtime.KindFloat:
			d = time.Duration(args[0].F * float64(time.Second))
		default:
			if n, err := runtime.AsInt(args[0]); err == nil {
				d = time.Duration(n) * time.Second
			}
		}
		if d > 0 {
			time.Sleep(d)
		}
		return runtime.Unit(), nil
	}, 1)

	// time.since(unix_sec) -> seconds elapsed
	set(p, "since", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Int(0), nil
		}
		n, err := runtime.AsInt(args[0])
		if err != nil {
			return runtime.Int(0), nil
		}
		return runtime.Int(time.Now().Unix() - n), nil
	}, 1)

	// time.parse(s, layout?) -> Result[unix_sec]
	// layout aliases: iso, date, datetime; default RFC3339
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("time.parse(s, layout?)", "time"), nil
		}
		s := args[0].String()
		layout := time.RFC3339
		if len(args) >= 2 && args[1].String() != "" {
			layout = args[1].String()
			switch layout {
			case "date":
				layout = "2006-01-02"
			case "time":
				layout = "15:04:05"
			case "datetime":
				layout = "2006-01-02 15:04:05"
			case "iso":
				layout = time.RFC3339
			}
		}
		t, err := time.Parse(layout, s)
		if err != nil {
			// try RFC3339Nano
			t, err = time.Parse(time.RFC3339Nano, s)
			if err != nil {
				return errRes(err.Error(), "time"), nil
			}
		}
		return runtime.Ok(runtime.Int(t.Unix())), nil
	}, 2)

	// time.add(unix_sec, seconds) -> unix_sec
	set(p, "add", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Int(0), nil
		}
		base, err1 := runtime.AsInt(args[0])
		delta, err2 := runtime.AsInt(args[1])
		if err1 != nil || err2 != nil {
			// float delta seconds
			if err1 == nil {
				if f, ok := asFloat64(args[1]); ok {
					return runtime.Int(base + int64(f)), nil
				}
			}
			return runtime.Int(0), nil
		}
		return runtime.Int(base + delta), nil
	}, 2)

	// time.diff(unix_a, unix_b) -> a-b seconds
	set(p, "diff", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Int(0), nil
		}
		a, e1 := runtime.AsInt(args[0])
		b, e2 := runtime.AsInt(args[1])
		if e1 != nil || e2 != nil {
			return runtime.Int(0), nil
		}
		return runtime.Int(a - b), nil
	}, 2)

	// time.sleep_ms(ms)
	set(p, "sleep_ms", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Unit(), nil
		}
		n, err := runtime.AsInt(args[0])
		if err != nil || n <= 0 {
			return runtime.Unit(), nil
		}
		time.Sleep(time.Duration(n) * time.Millisecond)
		return runtime.Unit(), nil
	}, 1)

	// time.date(unix?) -> "YYYY-MM-DD" UTC
	set(p, "date", func(args []runtime.Value) (runtime.Value, error) {
		t := time.Now().UTC()
		if len(args) >= 1 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				t = time.Unix(n, 0).UTC()
			}
		}
		return runtime.Str(t.Format("2006-01-02")), nil
	}, 1)

	// --- full timezones ---

	// time.zone(name) -> Result[str]  validates IANA name (e.g. "America/New_York")
	set(p, "zone", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("time.zone(name)", "time"), nil
		}
		loc, err := time.LoadLocation(args[0].String())
		if err != nil {
			return errRes(err.Error(), "time"), nil
		}
		return runtime.Ok(runtime.Str(loc.String())), nil
	}, 1)

	// time.format_in(unix, zone, layout?) -> Result[str]
	set(p, "format_in", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("time.format_in(unix, zone, layout?)", "time"), nil
		}
		sec, err := runtime.AsInt(args[0])
		if err != nil {
			return errRes("unix seconds required", "time"), nil
		}
		loc, err := time.LoadLocation(args[1].String())
		if err != nil {
			return errRes(err.Error(), "time"), nil
		}
		layout := time.RFC3339
		if len(args) >= 3 && args[2].String() != "" {
			layout = args[2].String()
			switch layout {
			case "date":
				layout = "2006-01-02"
			case "time":
				layout = "15:04:05"
			case "datetime":
				layout = "2006-01-02 15:04:05"
			case "iso":
				layout = time.RFC3339
			}
		}
		t := time.Unix(sec, 0).In(loc)
		return runtime.Ok(runtime.Str(t.Format(layout))), nil
	}, 3)

	// time.parse_in(s, zone, layout?) -> Result[unix]
	set(p, "parse_in", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("time.parse_in(s, zone, layout?)", "time"), nil
		}
		loc, err := time.LoadLocation(args[1].String())
		if err != nil {
			return errRes(err.Error(), "time"), nil
		}
		layout := time.RFC3339
		if len(args) >= 3 && args[2].String() != "" {
			layout = args[2].String()
			switch layout {
			case "date":
				layout = "2006-01-02"
			case "datetime":
				layout = "2006-01-02 15:04:05"
			case "iso":
				layout = time.RFC3339
			}
		}
		t, err := time.ParseInLocation(layout, args[0].String(), loc)
		if err != nil {
			return errRes(err.Error(), "time"), nil
		}
		return runtime.Ok(runtime.Int(t.Unix())), nil
	}, 3)

	// time.convert(unix, from_zone, to_zone) -> Result[{unix, local}]
	// unix is absolute; returns formatted local in to_zone for convenience
	set(p, "convert", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("time.convert(unix, from_zone, to_zone)", "time"), nil
		}
		sec, err := runtime.AsInt(args[0])
		if err != nil {
			return errRes("unix required", "time"), nil
		}
		toLoc, err := time.LoadLocation(args[2].String())
		if err != nil {
			return errRes(err.Error(), "time"), nil
		}
		// from_zone is informational for API symmetry; instant is absolute
		_ = args[1]
		t := time.Unix(sec, 0).In(toLoc)
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"unix", "local", "zone", "offset"}
		mo.Vals["unix"] = runtime.Int(t.Unix())
		mo.Vals["local"] = runtime.Str(t.Format(time.RFC3339))
		mo.Vals["zone"] = runtime.Str(toLoc.String())
		_, off := t.Zone()
		mo.Vals["offset"] = runtime.Int(int64(off))
		return runtime.Ok(m), nil
	}, 3)

	// time.offset(zone, unix?) -> Result[int] seconds east of UTC
	set(p, "offset", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("time.offset(zone, unix?)", "time"), nil
		}
		loc, err := time.LoadLocation(args[0].String())
		if err != nil {
			return errRes(err.Error(), "time"), nil
		}
		t := time.Now().In(loc)
		if len(args) >= 2 {
			if n, e := runtime.AsInt(args[1]); e == nil {
				t = time.Unix(n, 0).In(loc)
			}
		}
		_, off := t.Zone()
		return runtime.Ok(runtime.Int(int64(off))), nil
	}, 2)

	// time.parts(unix?) -> {year, month, day, hour, min, sec, weekday, yday}
	set(p, "parts", func(args []runtime.Value) (runtime.Value, error) {
		t := time.Now().UTC()
		if len(args) >= 1 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				t = time.Unix(n, 0).UTC()
			}
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("year", runtime.Int(int64(t.Year())))
		put("month", runtime.Int(int64(t.Month())))
		put("day", runtime.Int(int64(t.Day())))
		put("hour", runtime.Int(int64(t.Hour())))
		put("min", runtime.Int(int64(t.Minute())))
		put("sec", runtime.Int(int64(t.Second())))
		put("weekday", runtime.Int(int64(t.Weekday()))) // 0=Sunday
		put("weekday_name", runtime.Str(t.Weekday().String()))
		put("yday", runtime.Int(int64(t.YearDay())))
		put("unix", runtime.Int(t.Unix()))
		return m, nil
	}, 1)

	// time.weekday(unix?) -> 0..6 Sunday=0
	set(p, "weekday", func(args []runtime.Value) (runtime.Value, error) {
		t := time.Now().UTC()
		if len(args) >= 1 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				t = time.Unix(n, 0).UTC()
			}
		}
		return runtime.Int(int64(t.Weekday())), nil
	}, 1)

	// time.from_parts(year, month, day, hour?, min?, sec?) -> unix
	set(p, "from_parts", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("time.from_parts(year, month, day, hour?, min?, sec?)", "time"), nil
		}
		y, _ := runtime.AsInt(args[0])
		mo, _ := runtime.AsInt(args[1])
		d, _ := runtime.AsInt(args[2])
		h, mi, s := int64(0), int64(0), int64(0)
		if len(args) >= 4 {
			h, _ = runtime.AsInt(args[3])
		}
		if len(args) >= 5 {
			mi, _ = runtime.AsInt(args[4])
		}
		if len(args) >= 6 {
			s, _ = runtime.AsInt(args[5])
		}
		t := time.Date(safeTimeInt(y), time.Month(safeTimeInt(mo)), safeTimeInt(d), safeTimeInt(h), safeTimeInt(mi), safeTimeInt(s), 0, time.UTC)
		return runtime.Int(t.Unix()), nil
	}, -1)

	// time.start_of_day(unix?) -> unix at 00:00:00 UTC
	set(p, "start_of_day", func(args []runtime.Value) (runtime.Value, error) {
		t := time.Now().UTC()
		if len(args) >= 1 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				t = time.Unix(n, 0).UTC()
			}
		}
		sod := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		return runtime.Int(sod.Unix()), nil
	}, 1)

	return p
}
