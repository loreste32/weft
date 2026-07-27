package stdlib

import (
	"encoding/csv"
	"os"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageCSV — simple CSV for data-processing CLIs (no Python/pandas).
func packageCSV() runtime.Value {
	p := pkg()

	// csv.parse(text, opts?) -> Result
	// opts: {header: true} → {header: [...], rows: [{col: val}, ...]}
	// else → [[cell, ...], ...]
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("csv.parse(text)", "csv"), nil
		}
		return csvParse(args[0].String(), optArg(args, 1))
	}, 2)

	// csv.stringify(rows, opts?) -> str
	set(p, "stringify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		s, err := csvStringify(args[0], optArg(args, 1))
		if err != nil {
			return errRes(err.Error(), "csv"), nil
		}
		return runtime.Str(s), nil
	}, 2)

	// csv.read(path, opts?) -> Result
	set(p, "read", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("csv.read(path)", "csv"), nil
		}
		b, err := os.ReadFile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "csv"), nil
		}
		return csvParse(string(b), optArg(args, 1))
	}, 2)

	// csv.write(path, rows, opts?) -> Result
	set(p, "write", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("csv.write(path, rows)", "csv"), nil
		}
		s, err := csvStringify(args[1], optArg(args, 2))
		if err != nil {
			return errRes(err.Error(), "csv"), nil
		}
		if err := os.WriteFile(args[0].String(), []byte(s), 0o644); err != nil {
			return errRes(err.Error(), "csv"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 3)

	return p
}

func optArg(args []runtime.Value, i int) runtime.Value {
	if len(args) > i {
		return args[i]
	}
	return runtime.Null()
}

func csvParse(text string, opts runtime.Value) (runtime.Value, error) {
	header := false
	comma := ','
	if opts.Kind == runtime.KindMap || opts.Kind == runtime.KindStruct {
		if b, ok := mapGet(opts, "header"); ok && b.Kind == runtime.KindBool {
			header = b.B
		}
		if s := mapGetStr(opts, "comma", ""); s != "" {
			comma = rune(s[0])
		}
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = comma
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return errRes(err.Error(), "csv"), nil
	}
	if header && len(records) > 0 {
		hdr := records[0]
		var rowMaps []runtime.Value
		for _, rec := range records[1:] {
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			for i, h := range hdr {
				mo.Keys = append(mo.Keys, h)
				cell := ""
				if i < len(rec) {
					cell = rec[i]
				}
				mo.Vals[h] = runtime.Str(cell)
			}
			rowMaps = append(rowMaps, m)
		}
		out := runtime.NewMap()
		omo := out.Obj.(*runtime.MapObj)
		hitems := make([]runtime.Value, len(hdr))
		for i, h := range hdr {
			hitems[i] = runtime.Str(h)
		}
		omo.Keys = []string{"header", "rows"}
		omo.Vals["header"] = runtime.List(hitems...)
		omo.Vals["rows"] = runtime.List(rowMaps...)
		return runtime.Ok(out), nil
	}
	rows := make([]runtime.Value, len(records))
	for i, rec := range records {
		cells := make([]runtime.Value, len(rec))
		for j, c := range rec {
			cells[j] = runtime.Str(c)
		}
		rows[i] = runtime.List(cells...)
	}
	return runtime.Ok(runtime.List(rows...)), nil
}

func csvStringify(v, opts runtime.Value) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	if opts.Kind == runtime.KindMap || opts.Kind == runtime.KindStruct {
		if s := mapGetStr(opts, "comma", ""); s != "" {
			w.Comma = rune(s[0])
		}
	}
	if v.Kind != runtime.KindList {
		return "", errString("csv.stringify needs list of rows")
	}
	lo := v.Obj.(*runtime.ListObj)
	if len(lo.Items) == 0 {
		return "", nil
	}
	if lo.Items[0].Kind == runtime.KindMap || lo.Items[0].Kind == runtime.KindStruct {
		var keys []string
		if opts.Kind == runtime.KindMap || opts.Kind == runtime.KindStruct {
			if h, ok := mapGet(opts, "header"); ok && h.Kind == runtime.KindList {
				for _, it := range h.Obj.(*runtime.ListObj).Items {
					keys = append(keys, it.String())
				}
			}
		}
		if len(keys) == 0 {
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
		}
		if err := w.Write(keys); err != nil {
			return "", err
		}
		for _, it := range lo.Items {
			row := make([]string, len(keys))
			for i, k := range keys {
				if val, ok := mapGet(it, k); ok {
					row[i] = val.String()
				}
			}
			if err := w.Write(row); err != nil {
				return "", err
			}
		}
	} else {
		for _, it := range lo.Items {
			if it.Kind != runtime.KindList {
				return "", errString("csv row must be list")
			}
			cells := it.Obj.(*runtime.ListObj)
			row := make([]string, len(cells.Items))
			for i, c := range cells.Items {
				row[i] = c.String()
			}
			if err := w.Write(row); err != nil {
				return "", err
			}
		}
	}
	w.Flush()
	return b.String(), w.Error()
}

type errString string

func (e errString) Error() string { return string(e) }
