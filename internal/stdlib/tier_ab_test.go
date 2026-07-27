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

func runAB(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "ab.weft", src); err != nil {
		t.Fatalf("%v\nout=%s\n%s", err, out.String(), src)
	}
	return strings.TrimSpace(out.String())
}

func has(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Fatalf("missing %q in:\n%s", want, out)
	}
}

func TestAB_Binstruct(t *testing.T) {
	out := runAB(t, `
fn main -> Result {
    p := binstruct.pack(">I", 0x01020304)?
    v := binstruct.unpack(">I", p)?
    say(v[0])
    s := binstruct.size(">IH")?
    say(s)
    p2 := binstruct.pack("<H", 0x3412)?
    v2 := binstruct.unpack("<H", p2)?
    say(v2[0])
    p3 := binstruct.pack(">4s", "abcd")?
    v3 := binstruct.unpack(">4s", p3)?
    say(v3[0])
}
`)
	has(t, out, "16909060")
	has(t, out, "6")
	has(t, out, "13330") // 0x3412
	has(t, out, "abcd")
}

func TestAB_Difflib(t *testing.T) {
	out := runAB(t, `
fn main {
    d := difflib.unified_diff("a\nb\n", "a\nc\n", "old", "new")
    say(str.contains(d, "--- old"))
    say(str.contains(d, "+++ new"))
    n := difflib.ndiff(["x"], ["y"])
    say(len(n) >= 2)
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" {
			t.Fatalf("%q", out)
		}
	}
}

func TestAB_CopyFunctools(t *testing.T) {
	out := runAB(t, `
fn add(a, b) { a + b }
fn main {
    m := {"a": [1]}
    d := copy.deepcopy(m)
    s := copy.copy(m)
    say(d.a[0])
    say(len(s.a) == 1)
    p := functools.partial(add, 10)
    say(p(5))
    o := functools.once(fn() { 7 })
    say(o())
    say(o())
}
`)
	has(t, out, "1")
	has(t, out, "true")
	has(t, out, "15")
	has(t, out, "7")
}

func TestAB_Traceback(t *testing.T) {
	out := runAB(t, `
fn main {
    e := Err("boom", "test")
    say(str.contains(traceback.format(e), "boom"))
    r := Err("nope", "x")
    say(traceback.is_err(r))
    say(traceback.err_msg(r) != null)
    ok := Ok(1)
    say(traceback.is_err(ok) == false)
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" {
			t.Fatalf("%q", out)
		}
	}
}

func TestAB_FSPathBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin.dat")
	out := runAB(t, `
fn main -> Result {
    fs.write_bytes("`+path+`", "\x00\x01\x02")?
    b := fs.read_bytes("`+path+`")?
    say(len(b) == 3)
    say(fs.stem("`+path+`"))
    say(fs.with_suffix("a.txt", ".bak"))
    ps := fs.parents("/a/b/c")
    say(len(ps) >= 1)
}
`)
	has(t, out, "true")
	has(t, out, "bin")
	has(t, out, "a.bak")
}

func TestAB_SHCodeLines(t *testing.T) {
	out := runAB(t, `
fn main -> Result {
    lines := sh.lines("printf", ["a\\nb\\n"])?
    say(len(lines))
    c := sh.code("true")?
    say(c == 0)
    c2 := sh.code("false")?
    say(c2 != 0)
    r := sh.run("printf", ["z"], {"timeout": "2s", "merge": true})?
    say(r.ok)
}
`)
	has(t, out, "2")
	has(t, out, "true")
}

func TestAB_CryptoHash(t *testing.T) {
	out := runAB(t, `
fn main {
    h := crypto.hash("sha256", "hi")
    say(len(h) == 64)
    m := crypto.hmac_sha512("k", "m")
    say(len(m) == 128)
    say(crypto.equal(h, h))
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" {
			t.Fatalf("%q", out)
		}
	}
}

func TestAB_IPNetworkMath(t *testing.T) {
	out := runAB(t, `
fn main -> Result {
    n := ip.network("10.0.0.0/24")?
    say(n.bits)
    say(math.quantile([1, 2, 3, 4], 0.5))
    say(math.mode([1, 2, 2, 3]))
}
`)
	has(t, out, "24")
	has(t, out, "2.5")
	has(t, out, "2")
}

func TestAB_XMLHTMLINIURL(t *testing.T) {
	out := runAB(t, `
fn main -> Result {
    root := xml.parse("<r id=\"9\"><a>hi</a></r>")?
    a := xml.find(root, "a")
    say(xml.text(a))
    say(xml.attr(root, "id"))
    say(len(html.links("<a href=\"/x\">")))
    cfg := ini.parse("[db]\nhost=localhost\n")?
    say(ini.has_section(cfg, "db"))
    secs := ini.sections(cfg)
    say(len(secs) == 1)
    u := url.path_unescape("%2Ftmp%2Fa")?
    say(u)
    m := url.merge_query("http://x/?a=1", {"b": "2"})
    say(str.contains(m, "b=2"))
}
`)
	has(t, out, "hi")
	has(t, out, "9")
	has(t, out, "1")
	has(t, out, "true")
	has(t, out, "/tmp/a")
}

func TestAB_TestAssert(t *testing.T) {
	out := runAB(t, `
fn test_ok {
    test.assert(true, "ok")
    test.eq(1, 1)
}
fn main {
    say("ready")
}
`)
	has(t, out, "ready")
	// run via weft test style through RunSource on test file
	var buf bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &buf})
	// package test assert inside a normal main
	err := ctx.RunSource(context.Background(), "t.weft", `
fn main {
    test.assert(1 == 1, "yes")
    say("pass")
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "pass") {
		t.Fatal(buf.String())
	}
}

func TestAB_CLIPrompt(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	go func() {
		_, _ = w.WriteString("answer\n")
		_ = w.Close()
	}()
	out := runAB(t, `
fn main -> Result {
    s := cli.prompt("Q: ")?
    say(s)
}
`)
	has(t, out, "answer")
}

func TestAB_HTTPTimeoutDurationString(t *testing.T) {
	// local server would be heavy; just ensure opts parse doesn't break GET shape via check compile/run mock
	// use invalid host with short timeout — should err quickly
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	err := ctx.RunSource(context.Background(), "h.weft", `
fn main {
    r := http.get("http://127.0.0.1:1/", {"timeout": "50ms", "retries": 0})
    say(r.ok == false)
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "true") {
		t.Fatalf("%q", out.String())
	}
}
