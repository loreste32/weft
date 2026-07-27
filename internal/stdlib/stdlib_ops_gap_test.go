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

func TestStdlibOpsGaps(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	_ = os.WriteFile(f, []byte("hello weft"), 0o644)
	fw := filepath.ToSlash(f)
	rw := filepath.ToSlash(filepath.Join(dir, "b.txt"))
	gzw := filepath.ToSlash(filepath.Join(dir, "a.txt.gz"))
	tarw := filepath.ToSlash(filepath.Join(dir, "out.tar"))
	utw := filepath.ToSlash(filepath.Join(dir, "ut"))

	code := `
fn main -> Result {
    st := fs.stat("` + fw + `")?
    say(st.size)
    say(st.is_file)
    fs.rename("` + fw + `", "` + rw + `")?
    say(fs.exists("` + rw + `"))
    say(len(crypto.md5("hi")) == 32)
    say(len(crypto.sha1("hi")) == 40)
    say(len(crypto.sha512("hi")) == 128)
    say(html.escape("<a>"))
    say(html.strip_tags("<b>x</b>"))
    say(str.slice("abcdef", 1, 4))
    say(str.pad_left("7", 3, "0"))
    archive.gzip("` + rw + `", "` + gzw + `")?
    say(fs.exists("` + gzw + `"))
    archive.tar("` + tarw + `", ["` + rw + `"])?
    archive.untar("` + tarw + `", "` + utw + `")?
    say(fs.exists("` + utw + `/b.txt"))
    Ok(1)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "gaps.weft", code); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "true") || !strings.Contains(s, "bcd") || !strings.Contains(s, "007") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "&lt;") {
		t.Fatal("escape", s)
	}
	if !strings.Contains(s, "x") {
		t.Fatal("strip_tags", s)
	}
}
