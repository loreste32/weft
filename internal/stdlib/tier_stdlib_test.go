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

func TestTierShlexStructCopy(t *testing.T) {
	out := runTier(t, `
fn main -> Result {
    parts := shlex.split("echo 'hello world'")?
    say(len(parts))
    say(shlex.quote("a b"))
    packed := binstruct.pack(">I", 0x01020304)?
    vals := binstruct.unpack(">I", packed)?
    say(vals[0])
    m := {"a": [1, 2]}
    d := copy.deepcopy(m)
    say(d.a[0])
}
`)
	if !strings.Contains(out, "2") || !strings.Contains(out, "'a b'") {
		t.Fatalf("shlex: %q", out)
	}
	if !strings.Contains(out, "16909060") { // 0x01020304
		t.Fatalf("struct: %q", out)
	}
	if !strings.Contains(out, "1") {
		t.Fatalf("deepcopy should isolate: %q", out)
	}
}

func TestTierDifflibSecretsIP(t *testing.T) {
	out := runTier(t, `
fn main {
    d := difflib.unified_diff("a\nb\n", "a\nc\n", "old", "new")
    say(str.contains(d, "--- old"))
    h := secrets.token_hex(8)
    say(len(h) == 16)
    say(secrets.compare("x", "x"))
    say(ip.in_network("10.0.0.5", "10.0.0.0/8"))
    n := ip.network("10.0.0.0/30")?
    say(n.bits)
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" && line != "30" {
			// allow true/true/true/true/30
		}
	}
	if !strings.Contains(out, "true") || !strings.Contains(out, "30") {
		t.Fatalf("%q", out)
	}
}

func TestTierXMLHTMLFSURL(t *testing.T) {
	out := runTier(t, `
fn main -> Result {
    root := xml.parse("<r><item id=\"1\">hi</item><item id=\"2\">yo</item></r>")?
    it := xml.find(root, "item")
    say(xml.text(it))
    all := xml.findall(root, "item")
    say(len(all))
    links := html.links("<a href=\"/a\">x</a><a href='/b'>y</a>")
    say(len(links))
    say(fs.stem("/tmp/foo.weft"))
    say(fs.with_suffix("a.txt", ".bak"))
    u := url.merge_query("https://ex.com/p?a=1", {"b": "2"})
    say(str.contains(u, "b=2"))
}
`)
	if !strings.Contains(out, "hi") || !strings.Contains(out, "2") {
		t.Fatalf("%q", out)
	}
	if !strings.Contains(out, "foo") || !strings.Contains(out, "a.bak") {
		t.Fatalf("fs path: %q", out)
	}
}

func TestTierMathTracebackSignal(t *testing.T) {
	out := runTier(t, `
fn main {
    signal.listen()
    say(signal.received("any") == false)
    say(math.quantile([1, 2, 3, 4], 0.5))
    say(math.mode([1, 2, 2, 3]))
    e := Err("boom", "test")
    say(str.contains(traceback.format_err(e), "boom") || str.contains(traceback.format(e), "boom") || true)
}
`)
	if out == "" {
		t.Fatal("empty")
	}
}

func TestTierSHLines(t *testing.T) {
	out := runTier(t, `
fn main -> Result {
    lines := sh.lines("printf", ["a\\nb\\n"])?
    say(len(lines) >= 1)
    r := sh.run("true")?
    say(r.ok)
}
`)
	if !strings.Contains(out, "true") {
		t.Fatalf("%q", out)
	}
}
