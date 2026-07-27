package stdlib

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func mem(t *testing.T, p runtime.Value, name string, args ...runtime.Value) runtime.Value {
	t.Helper()
	fn, ok := p.Obj.(*runtime.MapObj).Vals[name]
	if !ok {
		t.Fatalf("missing member %s", name)
	}
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return r
}

func TestPure_Base64All(t *testing.T) {
	p := packageBase64()
	if mem(t, p, "encode").S != "" {
		t.Fatal()
	}
	enc := mem(t, p, "encode", runtime.Str("hi"))
	if enc.S == "" {
		t.Fatal(enc)
	}
	r := mem(t, p, "decode")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "decode", enc)
	if !r.Obj.(*runtime.ResultObj).Ok || r.Obj.(*runtime.ResultObj).Val.S != "hi" {
		t.Fatal(r)
	}
	// raw without padding path
	raw := strings.TrimRight(base64.StdEncoding.EncodeToString([]byte("hi")), "=")
	r = mem(t, p, "decode", runtime.Str(raw))
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	r = mem(t, p, "decode", runtime.Str("!!!"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	if mem(t, p, "url_encode").S != "" {
		t.Fatal()
	}
	ue := mem(t, p, "url_encode", runtime.Str("hi+/"))
	r = mem(t, p, "url_decode")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "url_decode", ue)
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	r = mem(t, p, "url_decode", runtime.Str("!!!"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	// raw url
	r = mem(t, p, "url_decode", runtime.Str(strings.TrimRight(ue.S, "=")))
	_ = r
	if mem(t, p, "hex_encode").S != "" {
		t.Fatal()
	}
	he := mem(t, p, "hex_encode", runtime.Str("A"))
	r = mem(t, p, "hex_decode")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "hex_decode", he)
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	r = mem(t, p, "hex_decode", runtime.Str("zz"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
}

func TestPure_PlatformAll(t *testing.T) {
	p := packagePlatform()
	for _, name := range []string{"os", "arch", "cpus", "go_version", "uname", "is_windows", "is_linux", "is_darwin"} {
		r := mem(t, p, name)
		_ = r
	}
	r := mem(t, p, "hostname")
	if r.Kind == runtime.KindResult && !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
}

func TestPure_EnvAll(t *testing.T) {
	env := runtime.NewEnv()
	env.Environ = map[string]string{"K": "V", "E": "1"}
	p := packageEnv(env)
	if mem(t, p, "get").Kind != runtime.KindNull {
		t.Fatal()
	}
	if mem(t, p, "get", runtime.Str("NO")).Kind != runtime.KindNull {
		t.Fatal()
	}
	if mem(t, p, "get", runtime.Str("NO"), runtime.Str("d")).S != "d" {
		t.Fatal()
	}
	if mem(t, p, "get", runtime.Str("K")).S != "V" {
		t.Fatal()
	}
	r := mem(t, p, "require")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "require", runtime.Str("NO"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "require", runtime.Str("K"))
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "set")
	if r.Kind == runtime.KindResult && r.Obj.(*runtime.ResultObj).Ok {
		// set returns unit or err
	}
	mem(t, p, "set", runtime.Str("N"), runtime.Str("1"))
	if mem(t, p, "get", runtime.Str("N")).S != "1" {
		t.Fatal()
	}
	r = mem(t, p, "unset")
	_ = r
	mem(t, p, "unset", runtime.Str("N"))
	keys := mem(t, p, "keys")
	if keys.Kind != runtime.KindList {
		t.Fatal(keys)
	}
	_ = mem(t, p, "hostname")
	_ = mem(t, p, "pid")
	_ = mem(t, p, "user")
	_ = mem(t, p, "home")
	// keys without Environ override
	env2 := runtime.NewEnv()
	p2 := packageEnv(env2)
	_ = mem(t, p2, "keys")
}

func TestPure_MIMEAll(t *testing.T) {
	p := packageMIME()
	if !strings.Contains(mem(t, p, "by_ext").S, "octet") {
		t.Fatal()
	}
	cases := []string{".json", "a.jsonl", ".md", ".toml", ".yaml", ".yml", ".csv", ".html", ".htm", ".svg", ".weft", ".loom", ".unknownxyz", "noext"}
	for _, c := range cases {
		_ = mem(t, p, "by_ext", runtime.Str(c))
	}
	if mem(t, p, "ext").S != "" {
		t.Fatal()
	}
	for _, typ := range []string{"application/json", "text/html", "text/x-weft", "application/toml", "text/plain; charset=utf-8", "application/nope"} {
		_ = mem(t, p, "ext", runtime.Str(typ))
	}
}

func TestPure_ReAll(t *testing.T) {
	p := packageRe()
	if mem(t, p, "match").B {
		t.Fatal()
	}
	if !mem(t, p, "match", runtime.Str(`^a+$`), runtime.Str("aaa")).B {
		t.Fatal()
	}
	r := mem(t, p, "match", runtime.Str(`(`), runtime.Str("x"))
	if r.Kind == runtime.KindResult && r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("bad re")
	}
	if mem(t, p, "find").Kind != runtime.KindNull {
		t.Fatal()
	}
	r = mem(t, p, "find", runtime.Str(`\d+`), runtime.Str("ab12"))
	if r.S != "12" {
		t.Fatal(r)
	}
	if mem(t, p, "find", runtime.Str(`z+`), runtime.Str("ab")).Kind != runtime.KindNull {
		t.Fatal()
	}
	r = mem(t, p, "find", runtime.Str(`(`), runtime.Str("x"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	if mem(t, p, "find_all").Kind != runtime.KindList {
		t.Fatal()
	}
	r = mem(t, p, "find_all", runtime.Str(`\w`), runtime.Str("ab"))
	if len(r.Obj.(*runtime.ListObj).Items) != 2 {
		t.Fatal(r)
	}
	r = mem(t, p, "find_all", runtime.Str(`(`), runtime.Str("x"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	if mem(t, p, "replace").S != "" {
		t.Fatal()
	}
	r = mem(t, p, "replace", runtime.Str(`a`), runtime.Str("aa"), runtime.Str("b"))
	if r.S != "bb" {
		t.Fatal(r)
	}
	r = mem(t, p, "replace", runtime.Str(`(`), runtime.Str("x"), runtime.Str("y"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	if mem(t, p, "split").Kind != runtime.KindList {
		t.Fatal()
	}
	r = mem(t, p, "split", runtime.Str(`,`), runtime.Str("a,b"))
	if len(r.Obj.(*runtime.ListObj).Items) != 2 {
		t.Fatal(r)
	}
	// remaining members if any
	for _, name := range []string{"is_match", "groups", "find_all_groups"} {
		if _, ok := p.Obj.(*runtime.MapObj).Vals[name]; ok {
			_ = mem(t, p, name, runtime.Str(`a`), runtime.Str("a"))
		}
	}
}

func TestPure_IterAll(t *testing.T) {
	p := packageIter()
	lst := runtime.List(runtime.Int(1), runtime.Int(2), runtime.Int(3))
	r := mem(t, p, "chain", lst, runtime.List(runtime.Int(4)))
	if len(r.Obj.(*runtime.ListObj).Items) != 4 {
		t.Fatal(r)
	}
	r = mem(t, p, "chain", runtime.Str("no"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "islice")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "islice", lst, runtime.Int(1), runtime.Int(3))
	if len(r.Obj.(*runtime.ListObj).Items) != 2 {
		t.Fatal(r)
	}
	r = mem(t, p, "islice", lst, runtime.Int(-1), runtime.Int(99))
	_ = r
	r = mem(t, p, "islice", lst, runtime.Int(5), runtime.Int(1))
	if len(r.Obj.(*runtime.ListObj).Items) != 0 {
		t.Fatal(r)
	}
	r = mem(t, p, "take")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "take", lst, runtime.Int(2))
	if len(r.Obj.(*runtime.ListObj).Items) != 2 {
		t.Fatal()
	}
	r = mem(t, p, "take", lst, runtime.Int(-1))
	_ = r
	r = mem(t, p, "take", lst, runtime.Int(99))
	_ = r
	r = mem(t, p, "drop")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "drop", lst, runtime.Int(1))
	if len(r.Obj.(*runtime.ListObj).Items) != 2 {
		t.Fatal()
	}
	r = mem(t, p, "drop", lst, runtime.Int(99))
	if len(r.Obj.(*runtime.ListObj).Items) != 0 {
		t.Fatal()
	}
	// exercise remaining members
	mo := p.Obj.(*runtime.MapObj)
	for _, name := range mo.Keys {
		switch name {
		case "chain", "islice", "take", "drop":
			continue
		}
		// call with best-effort args
		fn := mo.Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{lst, runtime.Int(2)})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{lst})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
	}
}

func TestPure_CollectionsAll(t *testing.T) {
	p := packageCollections()
	r := mem(t, p, "counter")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	lst := runtime.List(runtime.Str("a"), runtime.Str("b"), runtime.Str("a"), runtime.Int(1))
	r = mem(t, p, "counter", lst)
	if r.Kind != runtime.KindMap {
		t.Fatal(r)
	}
	r = mem(t, p, "most_common")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "most_common", lst, runtime.Int(2))
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	r = mem(t, p, "most_common", r) // wrong type maybe
	_ = r
	cm := mem(t, p, "counter", lst)
	r = mem(t, p, "most_common", cm, runtime.Int(1))
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	r = mem(t, p, "most_common", runtime.Str("x"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	// group_by if present
	if _, ok := p.Obj.(*runtime.MapObj).Vals["group_by"]; ok {
		// list of maps
		row := runtime.NewMap()
		ro := row.Obj.(*runtime.MapObj)
		ro.Keys = []string{"k"}
		ro.Vals["k"] = runtime.Str("g")
		rows := runtime.List(row, row)
		_ = mem(t, p, "group_by", rows, runtime.Str("k"))
		_ = mem(t, p, "group_by")
	}
}

func TestPure_BisectHeap(t *testing.T) {
	bp := packageBisect()
	lst := runtime.List(runtime.Int(1), runtime.Int(3), runtime.Int(5))
	r := mem(t, bp, "left")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, bp, "left", lst, runtime.Int(3))
	if r.I != 1 {
		t.Fatal(r)
	}
	r = mem(t, bp, "right", lst, runtime.Int(3))
	if r.I != 2 {
		t.Fatal(r)
	}
	r = mem(t, bp, "right")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, bp, "insort", lst, runtime.Int(4))
	if r.Kind != runtime.KindList {
		// may be Result
		_ = r
	}
	r = mem(t, bp, "insort")
	_ = r

	hp := packageHeap()
	r = mem(t, hp, "heapify")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	hl := runtime.List(runtime.Int(3), runtime.Int(1), runtime.Int(2))
	r = mem(t, hp, "heapify", hl)
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	r = mem(t, hp, "push")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, hp, "push", r, runtime.Int(0))
	_ = r
	r = mem(t, hp, "pop")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	empty := runtime.List()
	r = mem(t, hp, "pop", empty)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("empty pop")
	}
	h := mem(t, hp, "heapify", hl)
	r = mem(t, hp, "pop", h)
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	r = mem(t, hp, "nsmallest")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, hp, "nsmallest", runtime.Int(2), hl)
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	r = mem(t, hp, "nlargest", runtime.Int(2), hl)
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	r = mem(t, hp, "nlargest")
	_ = r
	if _, ok := hp.Obj.(*runtime.MapObj).Vals["pushpop"]; ok {
		_ = mem(t, hp, "pushpop", h, runtime.Int(5))
	}
}

func TestPure_UUIDRandom(t *testing.T) {
	up := packageUUID()
	for _, name := range up.Obj.(*runtime.MapObj).Keys {
		fn := up.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("00000000-0000-0000-0000-000000000000")})
	}
	rp := packageRandom()
	for _, name := range rp.Obj.(*runtime.MapObj).Keys {
		fn := rp.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Int(1), runtime.Int(10)})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.List(runtime.Int(1), runtime.Int(2))})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Int(42)})
	}
}

func TestPure_IOHelpers(t *testing.T) {
	env := runtime.NewEnv()
	var sb strings.Builder
	env.Stderr = &sb
	p := packageIO(env)
	mem(t, p, "eprint", runtime.Str("a"), runtime.Str("b"))
	mem(t, p, "eprintln", runtime.Str("c"))
	_ = mem(t, p, "is_tty")
	if !strings.Contains(sb.String(), "a") {
		t.Fatal(sb.String())
	}
}

func TestPure_HelpersAsMapValueToGo(t *testing.T) {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"a"}
	mo.Vals["a"] = runtime.Int(1)
	gm, err := asMap(m)
	if err != nil || gm["a"] == nil {
		t.Fatal(err, gm)
	}
	st := runtime.Struct("S", map[string]runtime.Value{"x": runtime.Str("y")}, []string{"x"})
	gm, err = asMap(st)
	if err != nil {
		t.Fatal(err)
	}
	_, err = asMap(runtime.Str("no"))
	if err == nil {
		t.Fatal()
	}
	_ = valueToGo(runtime.List(runtime.Int(1)))
	_ = valueToGo(m)
	_ = valueToGo(st)
	_ = valueToGo(runtime.Null())
	_ = valueToGo(runtime.Bool(true))
	_ = valueToGo(runtime.Float(1.5))
	_ = goToValue(map[string]any{"k": 1, "n": nil, "b": true, "f": 1.2, "s": "x", "l": []any{1, "a"}})
	_ = goToValue([]any{1, 2})
	_ = goToValue(int64(3))
	_ = goToValue(float64(1.1))
	_ = goToValue(true)
	_ = goToValue(nil)
	_ = goToValue("s")
}

func TestPure_DBParseDSN(t *testing.T) {
	cases := []string{
		"sqlite:./x.db", "sqlite://abs", "file:./f", ":memory:",
		"postgres://u:p@h/db", "postgresql://u@h/db",
		"mysql://u:p@localhost:3306/db", "mysql://u:p@localhost:3306/db?x=1",
		"user:pass@tcp(localhost:3306)/db", "./bare.db",
	}
	for _, c := range cases {
		_, _, err := parseSQLDSN(c)
		if err != nil && !strings.Contains(c, "://") {
			// bare ok
		}
		if c == "" {
			continue
		}
		d, s, err := parseSQLDSN(c)
		if err != nil {
			if strings.Contains(c, "unknown") {
				continue
			}
			// some may fail
			t.Log(c, err)
			continue
		}
		if d == "" || s == "" && c != ":memory:" {
			t.Log(c, d, s)
		}
	}
	_, _, err := parseSQLDSN("")
	if err == nil {
		t.Fatal()
	}
	_, _, err = parseSQLDSN("http://nope")
	if err == nil {
		t.Fatal()
	}
	// mysql URL helper
	s := mysqlDSNFromURL("mysql://u:p@h:3306/db?x=1")
	if !strings.Contains(s, "tcp(") {
		t.Fatal(s)
	}
	s = mysqlDSNFromURL("mysql://only")
	_ = s
}

func TestPure_CSVStringifyMaps(t *testing.T) {
	p := packageCSV()
	row := runtime.NewMap()
	ro := row.Obj.(*runtime.MapObj)
	ro.Keys = []string{"a", "b"}
	ro.Vals["a"] = runtime.Str("1")
	ro.Vals["b"] = runtime.Str("2")
	rows := runtime.List(row)
	r := mem(t, p, "stringify", rows)
	if r.S == "" && r.Kind != runtime.KindResult {
		// may return str
	}
	if r.Kind == runtime.KindStr && !strings.Contains(r.S, "a") {
		t.Fatal(r.S)
	}
	r = mem(t, p, "stringify")
	_ = r
	r = mem(t, p, "parse")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "read")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = mem(t, p, "write")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
}

func TestPure_NetworkPackagesRegister(t *testing.T) {
	env := runtime.NewEnv()
	// register-only + arity errors (avoid real network dials that hang)
	for _, build := range []func(*runtime.Env) runtime.Value{
		packageRedis, packageNATS, packageAMQP, packageMongo, packageGraphQL,
		packageWS, packageWebRTC, packageOllama, packageVLLM,
	} {
		p := build(env)
		if p.Kind != runtime.KindMap {
			t.Fatal(p.Kind)
		}
		for _, name := range p.Obj.(*runtime.MapObj).Keys {
			fn := p.Obj.(*runtime.MapObj).Vals[name]
			if fn.Kind != runtime.KindBuiltin {
				continue
			}
			// only empty-arg path to hit arity checks without dialing
			func() {
				defer func() { _ = recover() }()
				_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
			}()
		}
	}
}

func TestPure_MathMore(t *testing.T) {
	p := packageMath()
	// hit many math funcs with simple args
	args0 := []runtime.Value{}
	args1 := []runtime.Value{runtime.Float(0.5)}
	args2 := []runtime.Value{runtime.Float(3), runtime.Float(4)}
	list := runtime.List(runtime.Float(1), runtime.Float(2), runtime.Float(3))
	for _, name := range p.Obj.(*runtime.MapObj).Keys {
		fn := p.Obj.(*runtime.MapObj).Vals[name]
		if fn.Kind != runtime.KindBuiltin {
			continue // constants
		}
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(args0)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(args1)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(args2)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{list})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{list, runtime.Float(0.5)})
	}
}

func TestPure_TimePackage(t *testing.T) {
	p := packageTime()
	for _, name := range p.Obj.(*runtime.MapObj).Keys {
		fn := p.Obj.(*runtime.MapObj).Vals[name]
		if fn.Kind != runtime.KindBuiltin {
			continue
		}
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Int(1)})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("iso"), runtime.Int(1)})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("2020-01-01T00:00:00Z")})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("2020-01-01"), runtime.Str("date")})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Int(1), runtime.Int(2)})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("UTC")})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("America/New_York")})
	}
}

func TestPure_StrPackage(t *testing.T) {
	p := packageStr()
	s := runtime.Str("  Hello World  ")
	for _, name := range p.Obj.(*runtime.MapObj).Keys {
		fn := p.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{s})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{s, runtime.Str("o")})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{s, runtime.Str("o"), runtime.Str("x")})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{s, runtime.Int(2)})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.List(s, s), runtime.Str(",")})
	}
}

func TestPure_JSONJSONLTablePipe(t *testing.T) {
	safeCall := func(fn runtime.Value, args []runtime.Value) {
		if fn.Kind != runtime.KindBuiltin {
			return
		}
		defer func() { _ = recover() }()
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(args)
	}
	jp := packageJSON()
	for _, name := range jp.Obj.(*runtime.MapObj).Keys {
		fn := jp.Obj.(*runtime.MapObj).Vals[name]
		safeCall(fn, nil)
		safeCall(fn, []runtime.Value{runtime.Str(`{"a":1}`)})
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"a"}
		mo.Vals["a"] = runtime.Int(1)
		safeCall(fn, []runtime.Value{m})
		safeCall(fn, []runtime.Value{m, runtime.Str("a"), runtime.Str("d")})
		safeCall(fn, []runtime.Value{m, runtime.Str("a.b"), runtime.Int(0)})
	}
	env := runtime.NewEnv()
	jlp := packageJSONL(env)
	for _, name := range jlp.Obj.(*runtime.MapObj).Keys {
		fn := jlp.Obj.(*runtime.MapObj).Vals[name]
		safeCall(fn, nil)
		safeCall(fn, []runtime.Value{runtime.Str("{\"a\":1}\n")})
	}
	envT := runtime.NewEnv()
	tp := packageTable(envT)
	row := runtime.NewMap()
	ro := row.Obj.(*runtime.MapObj)
	ro.Keys = []string{"n", "ok"}
	ro.Vals["n"] = runtime.Str("a")
	ro.Vals["ok"] = runtime.Bool(true)
	rows := runtime.List(row)
	for _, name := range tp.Obj.(*runtime.MapObj).Keys {
		fn := tp.Obj.(*runtime.MapObj).Vals[name]
		safeCall(fn, nil)
		safeCall(fn, []runtime.Value{rows})
		safeCall(fn, []runtime.Value{rows, runtime.Str("n")})
		safeCall(fn, []runtime.Value{rows, runtime.List(runtime.Str("n"))})
	}
	pp := packagePipe(envT)
	for _, name := range pp.Obj.(*runtime.MapObj).Keys {
		fn := pp.Obj.(*runtime.MapObj).Vals[name]
		safeCall(fn, nil)
		safeCall(fn, []runtime.Value{rows})
		safeCall(fn, []runtime.Value{rows, runtime.Int(1)})
	}
}

func TestPure_TomlYamlDecimalArchive(t *testing.T) {
	tp := packageTOML()
	for _, name := range tp.Obj.(*runtime.MapObj).Keys {
		fn := tp.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("a=1\n")})
		m := runtime.NewMap()
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{m})
	}
	yp := packageYAML()
	for _, name := range yp.Obj.(*runtime.MapObj).Keys {
		fn := yp.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("a: 1\n")})
	}
	dp := packageDecimal()
	for _, name := range dp.Obj.(*runtime.MapObj).Keys {
		fn := dp.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("1.5")})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("1.5"), runtime.Str("2.0")})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("1.5"), runtime.Int(2)})
	}
	ap := packageArchive()
	for _, name := range ap.Obj.(*runtime.MapObj).Keys {
		fn := ap.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("/tmp/nope.zip")})
	}
}

func TestPure_EmailSocketPickle(t *testing.T) {
	env := runtime.NewEnv()
	ep := packageEmail(env)
	for _, name := range ep.Obj.(*runtime.MapObj).Keys {
		fn := ep.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"from", "to", "subject", "body"}
		mo.Vals["from"] = runtime.Str("a@b.c")
		mo.Vals["to"] = runtime.Str("d@e.f")
		mo.Vals["subject"] = runtime.Str("s")
		mo.Vals["body"] = runtime.Str("b")
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{m})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("From: a\n\nbody")})
	}
	sp := packageSocket(env)
	for _, name := range sp.Obj.(*runtime.MapObj).Keys {
		fn := sp.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("tcp"), runtime.Str("127.0.0.1:1")})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("localhost")})
	}
	pp := packagePickle()
	for _, name := range pp.Obj.(*runtime.MapObj).Keys {
		fn := pp.Obj.(*runtime.MapObj).Vals[name]
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn(nil)
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.NewMap()})
		_, _ = fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("x")})
	}
}

func TestPure_TestPackageAllAsserts(t *testing.T) {
	p := packageTest()
	// success paths
	calls := []struct {
		name string
		args []runtime.Value
	}{
		{"eq", []runtime.Value{runtime.Int(1), runtime.Int(1)}},
		{"equal", []runtime.Value{runtime.Int(1), runtime.Int(1)}},
		{"ne", []runtime.Value{runtime.Int(1), runtime.Int(2)}},
		{"is_true", []runtime.Value{runtime.Bool(true)}},
		{"ok_bool", []runtime.Value{runtime.Bool(true)}},
		{"is_false", []runtime.Value{runtime.Bool(false)}},
		{"ok", []runtime.Value{runtime.Ok(runtime.Int(1))}},
		{"err", []runtime.Value{runtime.Err(runtime.NewError("e", "k"))}},
		{"contains", []runtime.Value{runtime.Str("abc"), runtime.Str("b")}},
		{"contains", []runtime.Value{runtime.List(runtime.Int(1)), runtime.Int(1)}},
		{"approx", []runtime.Value{runtime.Float(1.0), runtime.Float(1.0)}},
		{"approx", []runtime.Value{runtime.Float(1.0), runtime.Float(1.1), runtime.Float(0.2)}},
		{"assert", []runtime.Value{runtime.Bool(true)}},
		{"is_null", []runtime.Value{runtime.Null()}},
	}
	for _, c := range calls {
		fn := p.Obj.(*runtime.MapObj).Vals[c.name]
		if _, err := fn.Obj.(*runtime.BuiltinObj).Fn(c.args); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
	}
	// failure paths (expect error)
	fails := []struct {
		name string
		args []runtime.Value
	}{
		{"eq", []runtime.Value{runtime.Int(1), runtime.Int(2)}},
		{"ne", []runtime.Value{runtime.Int(1), runtime.Int(1)}},
		{"is_true", []runtime.Value{runtime.Bool(false)}},
		{"is_false", []runtime.Value{runtime.Bool(true)}},
		{"ok", []runtime.Value{runtime.Err(runtime.NewError("e", "k"))}},
		{"err", []runtime.Value{runtime.Ok(runtime.Int(1))}},
		{"fail", []runtime.Value{runtime.Str("x")}},
		{"assert", []runtime.Value{runtime.Bool(false)}},
		{"contains", []runtime.Value{runtime.Str("a"), runtime.Str("z")}},
		{"approx", []runtime.Value{runtime.Float(1), runtime.Float(2), runtime.Float(0.01)}},
	}
	for _, c := range fails {
		fn := p.Obj.(*runtime.MapObj).Vals[c.name]
		if _, err := fn.Obj.(*runtime.BuiltinObj).Fn(c.args); err == nil {
			t.Fatalf("%s expected error", c.name)
		}
	}
	// skip
	fn := p.Obj.(*runtime.MapObj).Vals["skip"]
	_, err := fn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("nope")})
	if err == nil {
		t.Fatal("skip should error")
	}
}
