package stdlib_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestSQLiteCRUD(t *testing.T) {
	dbpath := filepath.Join(t.TempDir(), "t.db")
	src := `
fn main -> Result {
    c := db.open("sqlite:` + dbpath + `")?
    c.exec("CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT, ok INT)")?
    c.exec("INSERT INTO users(name, ok) VALUES (?, ?)", ["Ada", 1])?
    c.exec("INSERT INTO users(name, ok) VALUES (?, ?)", ["Bob", 0])?
    rows := c.query("SELECT name, ok FROM users WHERE ok = ?", [1])?
    println(len(rows))
    println(rows[0].name)
    one := c.query_one("SELECT name FROM users WHERE name = ?", ["Bob"])?
    println(one.name)
    c.close()?
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "db.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "1\n") || !strings.Contains(s, "Ada") || !strings.Contains(s, "Bob") {
		t.Fatal(s)
	}
}

func TestSQLiteJSONColumn(t *testing.T) {
	dbpath := filepath.Join(t.TempDir(), "j.db")
	src := `
fn main -> Result {
    c := db.open("sqlite:` + dbpath + `")?
    c.exec("CREATE TABLE docs(id INTEGER PRIMARY KEY, data TEXT)")?
    c.exec("INSERT INTO docs(data) VALUES (?)", ["{\"name\":\"Ada\",\"scores\":[1,2,3]}"])?
    rows := c.query("SELECT json(data) as data FROM docs")?
    doc := rows[0].data
    println(doc.name)
    println(doc.scores)
    c.close()?
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "json.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "Ada") {
		t.Fatalf("JSON column not parsed: %s", s)
	}
	if !strings.Contains(s, "[1, 2, 3]") {
		t.Fatalf("JSON array not parsed: %s", s)
	}
}

func TestSQLiteJSONBRoundTrip(t *testing.T) {
	dbpath := filepath.Join(t.TempDir(), "jb.db")
	src := `
fn main -> Result {
    c := db.open("sqlite:` + dbpath + `")?
    c.exec("CREATE TABLE kv(key TEXT PRIMARY KEY, val TEXT)")?
    c.exec("INSERT INTO kv VALUES (?, ?)", ["config", "{\"port\":8080,\"debug\":true}"])?
    row := c.query_one("SELECT json(val) as val FROM kv WHERE key = ?", ["config"])?
    println(row.val.port)
    println(row.val.debug)
    c.close()?
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "jsonb.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "8080") || !strings.Contains(s, "true") {
		t.Fatalf("JSONB round-trip: %s", s)
	}
}

func TestSQLiteJSONNestedAccess(t *testing.T) {
	dbpath := filepath.Join(t.TempDir(), "n.db")
	src := `
fn main -> Result {
    c := db.open("sqlite:` + dbpath + `")?
    c.exec("CREATE TABLE t(d TEXT)")?
    c.exec("INSERT INTO t VALUES (?)", ["{\"a\":{\"b\":{\"c\":42}}}"])?
    row := c.query_one("SELECT json(d) as d FROM t")?
    println(row.d.a.b.c)
    c.close()?
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "nest.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "42") {
		t.Fatalf("nested JSON: %s", out.String())
	}
}

func TestSQLiteJSONArray(t *testing.T) {
	dbpath := filepath.Join(t.TempDir(), "arr.db")
	src := `
fn main -> Result {
    c := db.open("sqlite:` + dbpath + `")?
    c.exec("CREATE TABLE t(d TEXT)")?
    c.exec("INSERT INTO t VALUES (?)", ["[1,2,3]"])?
    row := c.query_one("SELECT json(d) as d FROM t")?
    println(row.d[0])
    println(len(row.d))
    c.close()?
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "arr.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "1") || !strings.Contains(s, "3") {
		t.Fatalf("JSON array: %s", s)
	}
}

func TestSQLitePlainTextNotParsed(t *testing.T) {
	dbpath := filepath.Join(t.TempDir(), "plain.db")
	src := `
fn main -> Result {
    c := db.open("sqlite:` + dbpath + `")?
    c.exec("CREATE TABLE t(name TEXT)")?
    c.exec("INSERT INTO t VALUES (?)", ["hello world"])?
    row := c.query_one("SELECT name FROM t")?
    println(row.name)
    c.close()?
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "plain.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "hello world") {
		t.Fatalf("plain text: %s", out.String())
	}
}

func TestDBDrivers(t *testing.T) {
	src := `fn main { println(db.drivers()) }`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "d.weft", src); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "sqlite") {
		t.Fatal(out.String())
	}
}

func TestGraphQLClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"hello":"weft"}}`))
	}))
	defer srv.Close()

	src := `
fn main -> Result {
    res := graphql.query("` + srv.URL + `", "query { hello }", {})?
    println(res.data.hello)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out, HTTPClient: srv.Client()})
	// HTTPClient is on Options - need to check if weft.New passes it to stdlib
	if err := ctx.RunSource(context.Background(), "gql.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "weft") {
		t.Fatal(out.String())
	}
}
