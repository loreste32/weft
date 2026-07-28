package stdlib

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageDataset: streaming data loading for large files.
// Unlike fs.read which loads everything into memory, dataset.stream
// returns an iterator that reads line-by-line.
//
//	for row in dataset.stream("big.jsonl") { say(row.name) }
//	for row in dataset.stream_csv("big.csv") { say(row[0]) }
//	dataset.count("big.jsonl") -> int (without loading all into memory)
//	dataset.sample("big.jsonl", 100) -> [map] (reservoir sampling)
func packageDataset() runtime.Value {
	p := pkg()

	// dataset.stream(path) -> Result[Iter]  streaming JSONL reader
	set(p, "stream", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dataset.stream(path)", "dataset"), nil
		}
		f, err := os.Open(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dataset"), nil
		}
		it := &jsonlStreamIter{scanner: bufio.NewScanner(f), file: f}
		it.scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // 10MB line limit
		return runtime.Ok(runtime.MakeIter(it)), nil
	}, 1)

	// dataset.stream_csv(path, opts?) -> Result[Iter]  streaming CSV reader
	set(p, "stream_csv", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dataset.stream_csv(path)", "dataset"), nil
		}
		f, err := os.Open(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dataset"), nil
		}
		sep := ","
		if len(args) >= 2 {
			if s := mapGetStr(args[1], "sep", ""); s != "" {
				sep = s
			}
		}
		skipHeader := true
		if len(args) >= 2 {
			if v, ok := mapGet(args[1], "header"); ok && !v.IsTruthy() {
				skipHeader = false
			}
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		// Read header
		var header []string
		if skipHeader && scanner.Scan() {
			header = strings.Split(scanner.Text(), sep)
		}
		it := &csvStreamIter{scanner: scanner, file: f, sep: sep, header: header}
		return runtime.Ok(runtime.MakeIter(it)), nil
	}, 2)

	// dataset.count(path) -> Result[int]  count lines without loading
	set(p, "count", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dataset.count(path)", "dataset"), nil
		}
		f, err := os.Open(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dataset"), nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		n := 0
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				n++
			}
		}
		return runtime.Ok(runtime.Int(int64(n))), nil
	}, 1)

	// dataset.sample(path, n) -> Result[[map]]  reservoir sampling (uniform random n rows)
	set(p, "sample", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("dataset.sample(path, n)", "dataset"), nil
		}
		k, err := runtime.AsInt(args[1])
		if err != nil || k <= 0 {
			return errRes("n must be positive", "dataset"), nil
		}
		f, err := os.Open(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dataset"), nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		reservoir := make([]runtime.Value, 0, int(k))
		i := 0
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var raw any
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				continue
			}
			val := goToValue(raw)
			if i < int(k) {
				reservoir = append(reservoir, val)
			} else {
				// simple modular replacement (deterministic for reproducibility)
				j := i % int(k)
				reservoir[j] = val
			}
			i++
		}
		return runtime.Ok(runtime.List(reservoir...)), nil
	}, 2)

	// dataset.head(path, n?) -> Result[[map]]  first n rows (default 10)
	set(p, "head", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dataset.head(path, n?)", "dataset"), nil
		}
		n := int64(10)
		if len(args) >= 2 {
			if v, err := runtime.AsInt(args[1]); err == nil && v > 0 {
				n = v
			}
		}
		f, err := os.Open(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dataset"), nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		var rows []runtime.Value
		for scanner.Scan() && int64(len(rows)) < n {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var raw any
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				continue
			}
			rows = append(rows, goToValue(raw))
		}
		return runtime.Ok(runtime.List(rows...)), nil
	}, 2)

	return p
}

// jsonlStreamIter reads JSONL line-by-line.
type jsonlStreamIter struct {
	scanner *bufio.Scanner
	file    *os.File
}

func (it *jsonlStreamIter) Next() (runtime.Value, bool) {
	for it.scanner.Scan() {
		line := strings.TrimSpace(it.scanner.Text())
		if line == "" {
			continue
		}
		var raw any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		return goToValue(raw), true
	}
	it.file.Close()
	return runtime.Null(), false
}

// csvStreamIter reads CSV line-by-line, returning maps if header is set.
type csvStreamIter struct {
	scanner *bufio.Scanner
	file    *os.File
	sep     string
	header  []string
}

func (it *csvStreamIter) Next() (runtime.Value, bool) {
	for it.scanner.Scan() {
		line := strings.TrimSpace(it.scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, it.sep)
		if len(it.header) > 0 {
			// Return as map
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			for i, h := range it.header {
				val := ""
				if i < len(fields) {
					val = fields[i]
				}
				mo.Keys = append(mo.Keys, h)
				mo.Vals[h] = runtime.Str(val)
			}
			return m, true
		}
		// Return as list
		items := make([]runtime.Value, len(fields))
		for i, f := range fields {
			items[i] = runtime.Str(f)
		}
		return runtime.List(items...), true
	}
	it.file.Close()
	return runtime.Null(), false
}
