package stdlib

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
	_ "modernc.org/sqlite"
)

func row(kvs ...any) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	for i := 0; i+1 < len(kvs); i += 2 {
		k := kvs[i].(string)
		var v runtime.Value
		switch x := kvs[i+1].(type) {
		case string:
			v = runtime.Str(x)
		case int:
			v = runtime.Int(int64(x))
		case bool:
			v = runtime.Bool(x)
		case runtime.Value:
			v = x
		default:
			v = runtime.Str("")
		}
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	return m
}

func TestArchive_ZipTarGzipRoundtrip(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "sub", "b.txt")
	_ = os.MkdirAll(filepath.Dir(b), 0o755)
	_ = os.WriteFile(a, []byte("hello"), 0o644)
	_ = os.WriteFile(b, []byte("world"), 0o644)

	p := packageArchive()

	// zip single file
	zpath := filepath.Join(dir, "out.zip")
	mustOk(t, callPkg(t, p, "zip", runtime.Str(zpath), runtime.Str(a)))
	// zip list + map entries + dir
	z2 := filepath.Join(dir, "out2.zip")
	files := runtime.List(
		runtime.Str(a),
		row("path", b, "name", "nested/b.txt"),
		row("src", "", "name", "skip"), // empty path skipped
	)
	mustOk(t, callPkg(t, p, "zip", runtime.Str(z2), files))
	// zip directory
	zdir := filepath.Join(dir, "dir.zip")
	mustOk(t, callPkg(t, p, "zip", runtime.Str(zdir), runtime.Str(dir)))

	mustErr(t, callPkg(t, p, "zip", runtime.Str(zpath)))
	mustErr(t, callPkg(t, p, "zip", runtime.Str(zpath), runtime.Int(1)))

	// list / unzip
	lst := mustOk(t, callPkg(t, p, "list", runtime.Str(zpath)))
	if lst.Kind != runtime.KindList {
		t.Fatal(lst)
	}
	mustErr(t, callPkg(t, p, "list"))
	dest := filepath.Join(dir, "uz")
	names := mustOk(t, callPkg(t, p, "unzip", runtime.Str(zpath), runtime.Str(dest)))
	if names.Kind != runtime.KindList {
		t.Fatal(names)
	}
	mustErr(t, callPkg(t, p, "unzip", runtime.Str(zpath)))
	mustErr(t, callPkg(t, p, "unzip", runtime.Str(filepath.Join(dir, "nope.zip")), runtime.Str(dest)))

	// gzip / gunzip
	gz := mustOk(t, callPkg(t, p, "gzip", runtime.Str(a)))
	gz2 := filepath.Join(dir, "a.custom.gz")
	mustOk(t, callPkg(t, p, "gzip", runtime.Str(a), runtime.Str(gz2)))
	out := mustOk(t, callPkg(t, p, "gunzip", gz))
	_, _ = os.ReadFile(out.S)
	mustOk(t, callPkg(t, p, "gunzip", runtime.Str(gz2), runtime.Str(filepath.Join(dir, "a.out"))))
	// gunzip non-.gz default dest
	rawgz := filepath.Join(dir, "plain")
	_ = os.WriteFile(rawgz, []byte{0x1f, 0x8b}, 0o644) // may fail gunzip
	_ = callPkg(t, p, "gunzip", runtime.Str(rawgz))
	mustErr(t, callPkg(t, p, "gzip"))
	mustErr(t, callPkg(t, p, "gunzip"))
	mustErr(t, callPkg(t, p, "gzip", runtime.Str(filepath.Join(dir, "missing"))))

	// tar + untar
	tpath := filepath.Join(dir, "out.tar")
	mustOk(t, callPkg(t, p, "tar", runtime.Str(tpath), runtime.Str(a)))
	mustOk(t, callPkg(t, p, "tar", runtime.Str(filepath.Join(dir, "list.tar")), files))
	mustOk(t, callPkg(t, p, "tar", runtime.Str(filepath.Join(dir, "subdir.tar")), runtime.Str(filepath.Dir(b))))
	mustErr(t, callPkg(t, p, "tar", runtime.Str(tpath)))
	tdest := filepath.Join(dir, "ut")
	mustOk(t, callPkg(t, p, "untar", runtime.Str(tpath), runtime.Str(tdest)))
	mustErr(t, callPkg(t, p, "untar", runtime.Str(tpath)))

	// archiveAddArgs edges
	if err := archiveAddArgs(runtime.Int(1), func(src, name string) error { return nil }); err == nil {
		t.Fatal("expected type err")
	}
	_ = archiveAddArgs(runtime.Str(a), func(src, name string) error { return nil })
	_ = archiveAddArgs(runtime.List(row("path", a, "name", "x.txt")), func(src, name string) error { return nil })
}

func TestCSV_AllPaths(t *testing.T) {
	p := packageCSV()
	mustErr(t, callPkg(t, p, "parse"))
	r := mustOk(t, callPkg(t, p, "parse", runtime.Str("a,b\n1,2")))
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	opts := row("header", true, "comma", ";")
	r2 := mustOk(t, callPkg(t, p, "parse", runtime.Str("a;b\n1;2"), opts))
	if r2.Kind != runtime.KindMap {
		t.Fatal(r2)
	}
	// stringify list rows
	s := callPkg(t, p, "stringify", runtime.List(
		runtime.List(runtime.Str("x"), runtime.Str("y")),
		runtime.List(runtime.Str("1"), runtime.Str("2")),
	))
	if !strings.Contains(s.S, "x") {
		t.Fatal(s)
	}
	// stringify map rows + header opts
	s2 := callPkg(t, p, "stringify",
		runtime.List(row("name", "Ada", "n", 1)),
		row("header", runtime.List(runtime.Str("name"), runtime.Str("n")), "comma", ","),
	)
	if !strings.Contains(s2.S, "Ada") {
		t.Fatal(s2)
	}
	// empty stringify
	if callPkg(t, p, "stringify").S != "" {
		t.Fatal()
	}
	if callPkg(t, p, "stringify", runtime.List()).S != "" {
		t.Fatal()
	}
	// bad row type
	bad := callPkg(t, p, "stringify", runtime.List(runtime.Str("nope")))
	if bad.Kind == runtime.KindStr && bad.S != "" {
		// may return err Result depending on path
	}
	if ro, ok := bad.Obj.(*runtime.ResultObj); ok && ro.Ok {
		t.Fatal("expected stringify err")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "t.csv")
	mustOk(t, callPkg(t, p, "write", runtime.Str(path), runtime.List(
		runtime.List(runtime.Str("a"), runtime.Str("b")),
	)))
	mustOk(t, callPkg(t, p, "read", runtime.Str(path)))
	mustErr(t, callPkg(t, p, "read"))
	mustErr(t, callPkg(t, p, "write", runtime.Str(path)))
	mustErr(t, callPkg(t, p, "read", runtime.Str(filepath.Join(dir, "missing.csv"))))

	// Error() on errString
	var e errString = "x"
	if e.Error() != "x" {
		t.Fatal()
	}
}

func TestEmail_ParseBuild(t *testing.T) {
	env := runtime.NewEnv()
	p := packageEmail(env)
	mustErr(t, callPkg(t, p, "parse"))
	raw := "From: a@x\r\nTo: b@y\r\nCc: c@z\r\nSubject: hi\r\nDate: Mon, 02 Jan 2006 15:04:05 -0700\r\n\r\nbody"
	m := mustOk(t, callPkg(t, p, "parse", runtime.Str(raw)))
	if m.Obj.(*runtime.MapObj).Vals["subject"].S != "hi" {
		t.Fatal(m)
	}
	mustErr(t, callPkg(t, p, "parse", runtime.Str("not an email without headers properly")))

	built := callPkg(t, p, "build", row(
		"from", "a@x",
		"to", "b@y",
		"subject", "s",
		"body", "hello",
		"cc", "c@z",
		"headers", row("X-Test", "1"),
	))
	if !strings.Contains(built.S, "Subject: s") || !strings.Contains(built.S, "hello") {
		t.Fatal(built)
	}
	mustErr(t, callPkg(t, p, "build"))
	mustErr(t, callPkg(t, p, "build", runtime.Str("x")))

	// send missing host
	mustErr(t, callPkg(t, p, "send"))
	mustErr(t, callPkg(t, p, "send", row("subject", "s")))
	// send with host but will fail dial (not full mail server — just error path)
	mustErr(t, callPkg(t, p, "send", row(
		"host", "127.0.0.1",
		"port", "1",
		"from", "a@x",
		"to", "b@y",
		"subject", "s",
		"body", "b",
	)))
	// to as list
	mustErr(t, callPkg(t, p, "send", row(
		"host", "127.0.0.1",
		"port", "1",
		"from", "a@x",
		"to", runtime.List(runtime.Str("b@y"), runtime.Str(" c@z ")),
		"subject", "s",
		"body", "b",
		"user", "u",
		"password", "p",
	)))
	// envOr
	env.Environ = map[string]string{"SMTP_HOST": "smtp.example"}
	if envOr(env, "SMTP_HOST", "") != "smtp.example" {
		t.Fatal()
	}
	if envOr(env, "NOPE", "def") != "def" {
		t.Fatal()
	}
}

func TestJSON_PathGetSetHas(t *testing.T) {
	p := packageJSON()
	doc := mustOk(t, callPkg(t, p, "parse", runtime.Str(`{"user":{"name":"Ada","tags":["a","b"]},"n":1}`)))
	g := mustOk(t, callPkg(t, p, "get", doc, runtime.Str("user.name")))
	if g.S != "Ada" {
		t.Fatal(g)
	}
	// list path
	g2 := mustOk(t, callPkg(t, p, "get", doc, runtime.List(runtime.Str("user"), runtime.Str("tags"), runtime.Str("0"))))
	if g2.S != "a" {
		t.Fatal(g2)
	}
	// default on miss
	d := mustOk(t, callPkg(t, p, "get", doc, runtime.Str("missing"), runtime.Str("fb")))
	if d.S != "fb" {
		t.Fatal(d)
	}
	mustErr(t, callPkg(t, p, "get", doc, runtime.Str("missing")))
	mustErr(t, callPkg(t, p, "get", doc))
	// has
	if !callPkg(t, p, "has", doc, runtime.Str("user.name")).B {
		t.Fatal()
	}
	if callPkg(t, p, "has", doc, runtime.Str("nope")).B {
		t.Fatal()
	}
	if callPkg(t, p, "has", doc).B {
		t.Fatal()
	}
	// set
	set := mustOk(t, callPkg(t, p, "set", doc, runtime.Str("user.name"), runtime.Str("Bob")))
	g3 := mustOk(t, callPkg(t, p, "get", set, runtime.Str("user.name")))
	if g3.S != "Bob" {
		t.Fatal(g3)
	}
	// set nested list index
	mustOk(t, callPkg(t, p, "set", doc, runtime.Str("user.tags.1"), runtime.Str("z")))
	mustErr(t, callPkg(t, p, "set", doc, runtime.Str("user")))
	// path edges
	_, err := jsonPathParts(runtime.Str(""))
	if err == nil {
		t.Fatal()
	}
	_, err = jsonPathParts(runtime.List())
	if err == nil {
		t.Fatal()
	}
	_, err = jsonPathParts(runtime.Int(1))
	if err == nil {
		t.Fatal()
	}
	// index errors
	_, err = jsonPathGet(doc, runtime.Str("user.tags.x"))
	if err == nil {
		t.Fatal()
	}
	_, err = jsonPathGet(doc, runtime.Str("user.tags.99"))
	if err == nil {
		t.Fatal()
	}
	_, err = jsonPathGet(doc, runtime.Str("n.x"))
	if err == nil {
		t.Fatal()
	}
	// merge / clone / pretty
	a := row("a", 1)
	b := row("b", 2)
	merged := callPkg(t, p, "merge", a, b)
	if merged.Kind != runtime.KindMap {
		t.Fatal(merged)
	}
	mustErr(t, callPkg(t, p, "merge", a))
	mustErr(t, callPkg(t, p, "merge", runtime.Str("x"), runtime.Str("y")))
	_ = callPkg(t, p, "clone", doc)
	_ = callPkg(t, p, "clone")
	_ = callPkg(t, p, "pretty", doc)
	_ = callPkg(t, p, "pretty")
	_ = callPkg(t, p, "stringify", doc, runtime.Int(2))
	_ = callPkg(t, p, "stringify", doc, runtime.Str("\t"))
	_ = callPkg(t, p, "stringify")
}

func TestCircuit_StateMachine(t *testing.T) {
	b := &circuitBook{hosts: make(map[string]*hostCircuit)}
	host := "ex.test"
	if !b.allow(host, time.Second) {
		t.Fatal("nil host allow")
	}
	// fail until open
	for i := 0; i < 3; i++ {
		b.failure(host, 3, time.Second)
	}
	if b.allow(host, time.Hour) {
		t.Fatal("should be open")
	}
	// cooldown → half-open allows one probe
	b.hosts[host].openedAt = time.Now().Add(-2 * time.Second)
	if !b.allow(host, time.Second) {
		t.Fatal("half open probe")
	}
	if b.allow(host, time.Second) {
		t.Fatal("second probe blocked")
	}
	// failure in half-open reopens
	b.failure(host, 3, time.Second)
	if b.allow(host, time.Hour) {
		t.Fatal("reopened")
	}
	// success closes
	b.hosts[host].openedAt = time.Now().Add(-2 * time.Second)
	_ = b.allow(host, time.Second) // half-open
	b.success(host)
	if !b.allow(host, time.Second) {
		t.Fatal("closed")
	}
	b.success("unknown") // no-op
	if hostKey("http://ex.com/path") != "ex.com" {
		t.Fatal(hostKey("http://ex.com/path"))
	}
	if hostKey("::not-url") == "" {
		t.Fatal()
	}
}

func TestHelpers_GoToValueAndValueKey(t *testing.T) {
	// goToValue type matrix
	for _, x := range []any{
		nil, true, int(1), int8(2), int16(3), int32(4), int64(5),
		uint(6), uint8(7), uint16(8), uint32(9), uint64(10),
		float32(1.5), float64(2.5), float64(3), // int-ish float
		json.Number("42"), json.Number("1.5"), "s",
		[]any{1, "a"}, map[string]any{"k": 1},
		map[any]any{"x": 1}, struct{ A int }{1},
	} {
		_ = goToValue(x)
	}
	// valueToGo
	_ = valueToGo(runtime.Null())
	_ = valueToGo(runtime.Bool(true))
	_ = valueToGo(runtime.Int(1))
	_ = valueToGo(runtime.Float(1.5))
	_ = valueToGo(runtime.Str("s"))
	_ = valueToGo(runtime.List(runtime.Int(1)))
	_ = valueToGo(row("a", 1))
	_ = valueToGo(runtime.Ok(runtime.Str("x")))
	_ = valueToGo(runtime.Err(runtime.NewError("e", "k")))
	// Secret struct
	sec := runtime.Value{Kind: runtime.KindStruct, Obj: &runtime.StructObj{
		TypeName: "Secret",
		Fields:   map[string]runtime.Value{"v": runtime.Str("secret")},
	}}
	if valueToGo(sec) != "***" {
		t.Fatal(valueToGo(sec))
	}
	// valueKey
	for _, v := range []runtime.Value{
		runtime.Int(1), runtime.Float(1.2), runtime.Bool(true), runtime.Bool(false),
		runtime.Str("s"), runtime.Null(), runtime.List(),
	} {
		if valueKey(v) == "" {
			t.Fatal(v)
		}
	}
	// sortValues / valueLess
	items := []runtime.Value{runtime.Int(3), runtime.Int(1), runtime.Int(2)}
	sortValues(items)
	if items[0].I != 1 {
		t.Fatal(items)
	}
	// defaultMapWorkers
	_ = defaultMapWorkers()
	t.Setenv("WEFT_WORKERS", "4")
	if defaultMapWorkers() != 4 {
		t.Fatal(defaultMapWorkers())
	}
	t.Setenv("WEFT_WORKERS", "bad")
	_ = defaultMapWorkers()
}

func TestTable_FullOps(t *testing.T) {
	env := runtime.NewEnv()
	p := packageTable(env)
	rows := runtime.List(
		row("name", "Ada", "age", 30, "ok", true),
		row("name", "Bob", "age", 20, "ok", false),
		row("name", "Ada", "age", 40, "ok", 1),
		row("name", "Cy", "age", 25, "ok", ""),
	)
	pl := callPkg(t, p, "pluck", rows, runtime.Str("name"))
	if pl.Kind != runtime.KindList {
		t.Fatal(pl)
	}
	mustErr(t, callPkg(t, p, "pluck", rows))
	pr := callPkg(t, p, "project", rows, runtime.List(runtime.Str("name"), runtime.Str("age")))
	_ = callPkg(t, p, "pick", rows, runtime.List(runtime.Str("name")))
	_ = pr
	_ = callPkg(t, p, "where_eq", rows, runtime.Str("name"), runtime.Str("Ada"))
	_ = callPkg(t, p, "where_ne", rows, runtime.Str("name"), runtime.Str("Ada"))
	_ = callPkg(t, p, "where_truthy", rows, runtime.Str("ok"))
	// truthy edges via isTruthy
	for _, v := range []runtime.Value{
		runtime.Null(), runtime.Unit(), runtime.Bool(false), runtime.Int(0),
		runtime.Float(0), runtime.Str(""), runtime.Str("false"), runtime.Str("0"),
		runtime.Str("yes"), runtime.List(),
	} {
		_ = isTruthy(v)
	}
	_ = callPkg(t, p, "sort", rows, runtime.Str("age"))
	_ = callPkg(t, p, "sort", rows, runtime.Str("age"), runtime.Bool(true))
	_ = callPkg(t, p, "sort", rows, runtime.Str("name"))
	_ = callPkg(t, p, "unique", rows, runtime.Str("name"))
	_ = callPkg(t, p, "group_by", rows, runtime.Str("name"))
	_ = callPkg(t, p, "count", rows)
	_ = callPkg(t, p, "count")
	_ = callPkg(t, p, "take", rows, runtime.Int(2))
	_ = callPkg(t, p, "take", rows, runtime.Int(99))
	_ = callPkg(t, p, "drop", rows, runtime.Int(1))
	_ = callPkg(t, p, "drop", rows, runtime.Int(0))
	_ = callPkg(t, p, "drop", rows, runtime.Int(99))
	_ = callPkg(t, p, "set", rows, runtime.Str("role"), runtime.Str("eng"))
	_ = callPkg(t, p, "rename", rows, runtime.Str("name"), runtime.Str("nm"))
	right := runtime.List(row("name", "Ada", "city", "LDN"), row("name", "Bob", "city", "NYC"))
	_ = callPkg(t, p, "merge", rows, right, runtime.Str("name"))
	_ = callPkg(t, p, "to_records", rows)
	_ = callPkg(t, p, "to_records")
	// cmpValues string path
	_ = cmpValues(runtime.Str("a"), runtime.Str("b"))
	_, _ = asFloatVal(runtime.Str("3.5"))
	_, _ = asFloatVal(runtime.Bool(true))
	// arity errs
	mustErr(t, callPkg(t, p, "where_eq", rows))
	mustErr(t, callPkg(t, p, "set", rows))
	mustErr(t, callPkg(t, p, "merge", rows))
}

func TestIO_EprintAndTTY(t *testing.T) {
	var stderr bytes.Buffer
	env := runtime.NewEnv()
	env.Stderr = &stderr
	p := packageIO(env)
	_ = callPkg(t, p, "eprint", runtime.Str("a"), runtime.Str("b"))
	_ = callPkg(t, p, "eprintln", runtime.Str("c"))
	if !strings.Contains(stderr.String(), "a b") || !strings.Contains(stderr.String(), "c") {
		t.Fatal(stderr.String())
	}
	_ = callPkg(t, p, "is_tty")
}

func TestSocket_ListenDialLoopback(t *testing.T) {
	env := runtime.NewEnv()
	p := packageSocket(env)
	mustErr(t, callPkg(t, p, "listen"))
	mustErr(t, callPkg(t, p, "dial", runtime.Str("tcp")))
	mustErr(t, callPkg(t, p, "resolve"))

	ln := mustOk(t, callPkg(t, p, "listen", runtime.Str("tcp"), runtime.Str("127.0.0.1:0")))
	addr := ln.Obj.(*runtime.MapObj).Vals["addr"].S

	done := make(chan struct{})
	go func() {
		defer close(done)
		c := mustOk(t, callMap(t, ln, "accept"))
		_ = callMap(t, c, "write", runtime.Str("pong"))
		_ = callMap(t, c, "set_deadline", runtime.Int(5))
		_ = callMap(t, c, "close")
	}()

	conn := mustOk(t, callPkg(t, p, "dial", runtime.Str("tcp"), runtime.Str(addr), runtime.Int(5)))
	// write then read
	_ = callMap(t, conn, "write", runtime.Str("ping"))
	mustErr(t, callMap(t, conn, "write"))
	_ = callMap(t, conn, "read", runtime.Int(64))
	_ = callMap(t, conn, "set_deadline", runtime.Int(2))
	_ = callMap(t, conn, "close")
	<-done
	_ = callMap(t, ln, "close")

	// resolve localhost
	r := callPkg(t, p, "resolve", runtime.Str("localhost"))
	if ro, ok := r.Obj.(*runtime.ResultObj); ok && !ro.Ok {
		t.Log("resolve failed offline", r)
	}
	// dial fail
	mustErr(t, callPkg(t, p, "dial", runtime.Str("tcp"), runtime.Str("127.0.0.1:1"), runtime.Int(1)))
}

func TestDB_TxBeginPing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.db")
	// open via sql directly then wrap — also exercise packageDB
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		if fn.Kind == runtime.KindBuiltin {
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		}
		return runtime.Unit(), nil
	}
	p := packageDB(env)
	c := mustOk(t, callPkg(t, p, "open", runtime.Str("sqlite:"+path)))
	mustOk(t, callMap(t, c, "ping"))
	mustOk(t, callMap(t, c, "exec", runtime.Str("CREATE TABLE t(id INTEGER PRIMARY KEY, name TEXT, blob BLOB, f REAL, b INT)")))
	mustOk(t, callMap(t, c, "exec", runtime.Str("INSERT INTO t(name, blob, f, b) VALUES (?,?,?,?)"),
		runtime.List(runtime.Str("Ada"), runtime.Str("\x00\x01"), runtime.Float(1.5), runtime.Bool(true))))
	// null insert via no rows query_one miss
	miss := mustOk(t, callMap(t, c, "query_one", runtime.Str("SELECT * FROM t WHERE id = ?"), runtime.List(runtime.Int(999))))
	if miss.Kind != runtime.KindNull {
		t.Fatal(miss)
	}
	rows := mustOk(t, callMap(t, c, "query", runtime.Str("SELECT * FROM t")))
	_ = rows
	// arity
	mustErr(t, callMap(t, c, "query"))
	mustErr(t, callMap(t, c, "exec"))
	mustErr(t, callMap(t, c, "query_one"))

	// begin / commit
	tx := mustOk(t, callMap(t, c, "begin"))
	mustOk(t, callMap(t, tx, "exec", runtime.Str("INSERT INTO t(name) VALUES (?)"), runtime.List(runtime.Str("Bob"))))
	_ = callMap(t, tx, "query", runtime.Str("SELECT name FROM t"))
	_ = callMap(t, tx, "query_one", runtime.Str("SELECT name FROM t WHERE name = ?"), runtime.List(runtime.Str("Bob")))
	mustOk(t, callMap(t, tx, "commit"))

	// begin / rollback
	tx2 := mustOk(t, callMap(t, c, "begin"))
	mustOk(t, callMap(t, tx2, "exec", runtime.Str("INSERT INTO t(name) VALUES ('X')")))
	mustOk(t, callMap(t, tx2, "rollback"))

	// tx(fn) success
	fnOK := runtime.MakeBuiltin("ok", 1, func(args []runtime.Value) (runtime.Value, error) {
		tx := args[0]
		return callMap(t, tx, "exec", runtime.Str("INSERT INTO t(name) VALUES ('Y')")), nil
	})
	mustOk(t, callMap(t, c, "tx", fnOK))
	// tx(fn) returns Err → rollback
	fnErr := runtime.MakeBuiltin("err", 1, func(args []runtime.Value) (runtime.Value, error) {
		return errRes("nope", "db"), nil
	})
	r := callMap(t, c, "tx", fnErr)
	if ro, ok := r.Obj.(*runtime.ResultObj); !ok || ro.Ok {
		t.Fatal(r)
	}
	// tx non-function
	mustErr(t, callMap(t, c, "tx", runtime.Str("no")))
	mustErr(t, callMap(t, c, "tx"))
	// tx without Call
	env2 := runtime.NewEnv()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	w := wrapSQLConn(env2, db)
	mustErr(t, callMap(t, w, "tx", fnOK))
	_ = db.Close()

	mustOk(t, callMap(t, c, "close"))

	// sqlVal / bytesVal / stringVal helpers via insert of various types already
	_ = sqlVal(nil)
	_ = sqlVal(int64(1))
	_ = sqlVal(float64(1.2))
	_ = sqlVal(true)
	_ = sqlVal("hi")
	_ = sqlVal(time.Now())
	_ = sqlVal([]byte(`{"a":1}`))
	_ = sqlVal([]byte("not-json-bytes"))
	_ = sqlVal(struct{}{})
	// tryParseJSON
	if _, ok := tryParseJSON(`{"a":1}`); !ok {
		t.Fatal()
	}
	if _, ok := tryParseJSON(`[1,2]`); !ok {
		t.Fatal()
	}
	if _, ok := tryParseJSON("x"); ok {
		t.Fatal()
	}
	if _, ok := tryParseJSON(""); ok {
		t.Fatal()
	}

	// open fail
	mustErr(t, callPkg(t, p, "open"))
}

func TestDB_sqlArgs(t *testing.T) {
	if sqlArgs(runtime.Null()) != nil {
		t.Fatal()
	}
	if len(sqlArgs(runtime.Str("x"))) != 1 {
		t.Fatal()
	}
	if len(sqlArgs(runtime.List(runtime.Int(1), runtime.Str("a")))) != 2 {
		t.Fatal()
	}
}
