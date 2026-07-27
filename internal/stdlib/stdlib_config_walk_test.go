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

func TestFSWalkStrWrapINIToml(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(sub, "b.txt"), []byte("y"), 0o644)
	iniPath := filepath.Join(dir, "c.ini")
	tomlPath := filepath.Join(dir, "c.toml")
	_ = os.WriteFile(iniPath, []byte("[db]\nhost = localhost\nport = 5432\n"), 0o644)
	_ = os.WriteFile(tomlPath, []byte("name = \"weft\"\n[server]\nport = 8080\n"), 0o644)

	root := filepath.ToSlash(dir)
	iniW := filepath.ToSlash(iniPath)
	tomlW := filepath.ToSlash(tomlPath)

	code := `
fn main -> Result {
    nodes := fs.walk("` + root + `")?
    say(len(nodes) >= 3)
    w := str.wrap("one two three four five six seven", 10)
    say(str.contains(w, "\n"))
    f := str.fill("hello   world\n\nnext para here", 12)
    say(str.contains(f, "hello"))
    ind := str.indent("a\nb", ">>")
    say(ind)
    d := str.dedent("    x\n    y")
    say(d)
    cfg := ini.load("` + iniW + `")?
    say(ini.get(cfg, "db", "host"))
    say(ini.get(cfg, "db", "missing", "def"))
    t := toml.load("` + tomlW + `")?
    say(t.name)
    say(t.server.port)
    s := toml.stringify({"a": 1, "b": "x"})?
    say(str.contains(s, "a"))
    Ok(1)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "cfg.weft", code); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	for _, need := range []string{"true", ">>a", "localhost", "def", "weft", "8080"} {
		if !strings.Contains(s, need) {
			t.Fatalf("missing %q in %q", need, s)
		}
	}
}
