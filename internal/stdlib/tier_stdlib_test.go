package stdlib_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func runTier(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(t.Context(), "tier.weft", src); err != nil {
		t.Fatalf("%v\nsrc:\n%s", err, src)
	}
	return strings.TrimSpace(out.String())
}

// Only APIs we kept: shlex, signal, sh.lines, secrets tokens, xml.find, html.links, fs.stem, url.merge_query.
func TestKeptOpsSurface(t *testing.T) {
	out := runTier(t, `
fn main -> Result {
    parts := shlex.split("echo 'hello world'")?
    say(len(parts))
    say(shlex.quote("a b"))
    lines := sh.lines("printf", ["ok\\n"])?
    say(len(lines) >= 1)
    h := secrets.token_hex(8)
    say(len(h) == 16)
    say(secrets.compare("x", "x"))
    signal.listen()
    say(!signal.received())
    root := xml.parse("<r><item>hi</item></r>")?
    say(xml.find(root, "item").text)
    say(len(html.links("<a href=\"/a\">x</a>")))
    say(fs.stem("/tmp/foo.weft"))
    u := url.merge_query("https://ex.com/p?a=1", {"b": "2"})
    say(str.contains(u, "b=2"))
}
`)
	want := []string{"2", "'a b'", "true", "true", "true", "true", "hi", "1", "foo", "true"}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in:\n%s", w, out)
		}
	}
}
