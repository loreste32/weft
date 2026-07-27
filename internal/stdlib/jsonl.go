package stdlib

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageJSONL — line-delimited JSON for real data pipelines.
func packageJSONL(env *runtime.Env) runtime.Value {
	p := pkg()

	// jsonl.read(path) -> Result[[any]]
	set(p, "read", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("jsonl.read(path)", "jsonl"), nil
		}
		f, err := os.Open(args[0].String())
		if err != nil {
			return errRes(err.Error(), "jsonl"), nil
		}
		defer f.Close()
		var rows []runtime.Value
		sc := bufio.NewScanner(f)
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 16*1024*1024)
		lineNo := 0
		for sc.Scan() {
			lineNo++
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			v, err := parseJSONLine(line)
			if err != nil {
				return errRes(fmt.Sprintf("%s (line %d)", err.Error(), lineNo), "jsonl"), nil
			}
			rows = append(rows, v)
		}
		if err := sc.Err(); err != nil {
			return errRes(err.Error(), "jsonl"), nil
		}
		return runtime.Ok(runtime.List(rows...)), nil
	}, 1)

	// jsonl.write(path, rows) -> Result
	set(p, "write", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[1].Kind != runtime.KindList {
			return errRes("jsonl.write(path, rows)", "jsonl"), nil
		}
		path := args[0].String()
		var b strings.Builder
		for _, it := range args[1].Obj.(*runtime.ListObj).Items {
			s, err := jsonMarshal(it)
			if err != nil {
				return errRes(err.Error(), "jsonl"), nil
			}
			b.WriteString(s)
			b.WriteByte('\n')
		}
		if dir := filepathDir(path); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			return errRes(err.Error(), "jsonl"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// jsonl.append(path, row) -> Result
	set(p, "append", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("jsonl.append(path, row)", "jsonl"), nil
		}
		s, err := jsonMarshal(args[1])
		if err != nil {
			return errRes(err.Error(), "jsonl"), nil
		}
		f, err := os.OpenFile(args[0].String(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return errRes(err.Error(), "jsonl"), nil
		}
		_, err = f.WriteString(s + "\n")
		_ = f.Close()
		if err != nil {
			return errRes(err.Error(), "jsonl"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// jsonl.parse(text) -> Result[[any]]
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("jsonl.parse(text)", "jsonl"), nil
		}
		var rows []runtime.Value
		for i, line := range strings.Split(args[0].String(), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			v, err := parseJSONLine(line)
			if err != nil {
				return errRes(fmt.Sprintf("%s (line %d)", err.Error(), i+1), "jsonl"), nil
			}
			rows = append(rows, v)
		}
		return runtime.Ok(runtime.List(rows...)), nil
	}, 1)

	// jsonl.stringify(rows) -> str
	set(p, "stringify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.Str(""), nil
		}
		var b strings.Builder
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			s, err := jsonMarshal(it)
			if err != nil {
				return errRes(err.Error(), "jsonl"), nil
			}
			b.WriteString(s)
			b.WriteByte('\n')
		}
		return runtime.Str(b.String()), nil
	}, 1)

	return p
}

func parseJSONLine(line string) (runtime.Value, error) {
	var raw any
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return runtime.Null(), err
	}
	return goToValue(raw), nil
}

func filepathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
