package stdlib_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestYAMLParseRoundTrip(t *testing.T) {
	src := `
use yaml

fn main {
    doc := yaml.parse("name: weft\nport: 8080\nflags:\n  - a\n  - b")?
    say(doc.name)
    say(doc.port + 1)   // must be int, not string concat
    say(len(doc.flags))
    out := yaml.stringify({"x": 1, "y": "z"})?
    say(contains(out, "x"))
}
`
	var buf bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &buf})
	if err := ctx.RunSource(context.Background(), "yaml.weft", src); err != nil {
		t.Fatal(err, buf.String())
	}
	s := buf.String()
	if !strings.Contains(s, "weft") || !strings.Contains(s, "8081") || !strings.Contains(s, "2") {
		t.Fatalf("%q", s)
	}
}

func TestYAMLLoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("env: prod\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.yaml")
	src := `
use yaml

fn main {
    c := yaml.load("` + path + `")?
    say(c.env)
    yaml.save("` + outPath + `", {"ok": true})?
    d := yaml.load("` + outPath + `")?
    say(d.ok)
}
`
	var buf bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &buf})
	if err := ctx.RunSource(context.Background(), "yaml2.weft", src); err != nil {
		t.Fatal(err, buf.String())
	}
	s := strings.TrimSpace(buf.String())
	if !strings.Contains(s, "prod") || !strings.Contains(s, "true") {
		t.Fatalf("%q", s)
	}
}
