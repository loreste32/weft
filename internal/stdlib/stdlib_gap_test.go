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

func runWeft(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "t.weft", src); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	return out.String()
}

func TestMathRandomUUIDBase64URL(t *testing.T) {
	out := runWeft(t, `
fn main -> Result {
    say(math.sqrt(9))
    say(math.min(3, 1, 2))
    say(math.clamp(15, 0, 10))
    random.seed(42)
    say(random.int(10))
    u := uuid.v4()
    say(len(u) > 30)
    say(uuid.is_valid(u))
    e := base64.encode("hi")
    d := base64.decode(e)?
    say(d)
    p := url.parse("https://ex.com/a?q=1#f")?
    say(p.host)
    say(p.params.q)
    b := url.build({"scheme": "https", "host": "ex.com", "path": "/x", "params": {"a": "b"}})
    say(b)
}
`)
	if !strings.Contains(out, "3") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "hi") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "ex.com") {
		t.Fatal(out)
	}
}

func TestFSTempAtomicAndArchive(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	_ = os.WriteFile(src, []byte("hello"), 0o644)
	zipPath := filepath.Join(dir, "out.zip")
	dest := filepath.Join(dir, "unz")
	// escape paths for weft string - use forward slashes
	srcW := filepath.ToSlash(src)
	zipW := filepath.ToSlash(zipPath)
	destW := filepath.ToSlash(dest)
	out := runWeft(t, `
fn main -> Result {
    td := fs.temp_dir("wtest")?
    tf := fs.temp_file("wtest")?
    fs.write_atomic(tf, "atomic")?
    say(fs.read(tf)?)
    archive.zip("`+zipW+`", ["`+srcW+`"])?
    names := archive.unzip("`+zipW+`", "`+destW+`")?
    say(len(names) >= 1)
    say(fs.exists("`+destW+`/a.txt"))
    // cleanup temp
    fs.remove(tf)?
    fs.remove_all(td)?
}
`)
	if !strings.Contains(out, "atomic") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "true") {
		t.Fatal(out)
	}
}

func TestTimeParseAdd(t *testing.T) {
	out := runWeft(t, `
fn main -> Result {
    u := time.parse("2020-01-02T00:00:00Z")?
    say(u > 0)
    u2 := time.add(u, 3600)
    say(time.diff(u2, u) == 3600)
    say(time.date(u))
}
`)
	if !strings.Contains(out, "2020-01-02") {
		t.Fatal(out)
	}
}
