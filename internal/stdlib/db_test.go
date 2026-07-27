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
