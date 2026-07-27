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

func runOps(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "ops.weft", src); err != nil {
		t.Fatalf("%v\nout=%s\nsrc=\n%s", err, out.String(), src)
	}
	return strings.TrimSpace(out.String())
}

func mustContain(t *testing.T, out string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in:\n%s", w, out)
		}
	}
}

func TestShlexPackage(t *testing.T) {
	out := runOps(t, `
fn main -> Result {
    p := shlex.split("cmd --flag 'a b'")?
    say(len(p))
    say(p[0])
    say(p[2])
    say(shlex.quote("x y"))
    j := shlex.join(["git", "commit", "-m", "hi there"])
    say(str.contains(j, "hi there") || str.contains(j, "'hi there'"))
    bad := shlex.split("'nope")
    say(bad.ok == false)
}
`)
	mustContain(t, out, "3", "cmd", "a b", "'x y'", "true")
}

func TestSignalPackage(t *testing.T) {
	out := runOps(t, `
fn main {
    signal.listen()
    say(signal.received() == false)
    say(signal.received("SIGINT") == false)
    signal.reset()
    say(signal.received("any") == false)
}
`)
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if line != "true" {
			t.Fatalf("unexpected line %q in %q", line, out)
		}
	}
	if len(lines) != 3 {
		t.Fatalf("want 3 trues, got %q", out)
	}
}

func TestSHLinesAndTimeoutOpts(t *testing.T) {
	out := runOps(t, `
fn main -> Result {
    lines := sh.lines("printf", ["one\\ntwo\\n"])?
    say(len(lines))
    say(lines[0])
    r := sh.run("true")?
    say(r.ok)
    r2 := sh.run("printf", ["x"], {"timeout": "5s"})?
    say(r2.ok)
    r3 := sh.run("printf", ["y"], {"merge": true})?
    say(r3.ok)
    which := sh.which("sh")
    say(which != null)
}
`)
	mustContain(t, out, "2", "one", "true")
}

func TestSecretsTokens(t *testing.T) {
	out := runOps(t, `
fn main {
    h := secrets.token_hex(4)
    say(len(h) == 8)
    u := secrets.token_urlsafe(16)
    say(len(u) > 10)
    say(secrets.compare("same", "same"))
    say(secrets.compare("a", "b") == false)
    s := secrets.from("secret-value")
    say(secrets.compare(s, secrets.from("secret-value")))
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" {
			t.Fatalf("line %q full=%q", line, out)
		}
	}
}

func TestLogSetJSON(t *testing.T) {
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	err := ctx.RunSource(context.Background(), "log.weft", `
fn main {
    log.set_level("info")
    log.set_json(true)
    log.info("hello", {"k": "v"})
}
`)
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, `"msg"`) && !strings.Contains(s, `"hello"`) {
		// json may order fields; require json-ish braces and hello
		if !strings.Contains(s, "{") || !strings.Contains(s, "hello") {
			t.Fatalf("expected json log line: %q", s)
		}
	}
	if !strings.Contains(s, "hello") {
		t.Fatalf("%q", s)
	}
	if !strings.Contains(s, "level") {
		t.Fatalf("missing level: %q", s)
	}
}

func TestFSStem(t *testing.T) {
	out := runOps(t, `
fn main {
    say(fs.stem("foo.weft"))
    say(fs.stem("/a/b/c.tar.gz"))
    say(fs.stem("noext"))
}
`)
	mustContain(t, out, "foo", "c.tar", "noext")
}

func TestXMLFindFindall(t *testing.T) {
	out := runOps(t, `
fn main -> Result {
    root := xml.parse("<r><a>1</a><b><a>2</a></b></r>")?
    first := xml.find(root, "a")
    say(first.text)
    all := xml.findall(root, "a")
    say(len(all))
    say(all[1].text)
    miss := xml.find(root, "nope")
    say(miss == null)
}
`)
	mustContain(t, out, "1", "2", "true")
	if !strings.Contains(out, "2\n") && !strings.Contains(out, "\n2\n") {
		// len is 2
		lines := strings.Split(out, "\n")
		foundLen := false
		for _, l := range lines {
			if l == "2" {
				foundLen = true
			}
		}
		if !foundLen {
			t.Fatalf("want len 2: %q", out)
		}
	}
}

func TestHTMLLinks(t *testing.T) {
	out := runOps(t, `
fn main {
    ls := html.links("<a href=\"/a\">x</a> <A HREF='/b'>y</A> <a href=\"/a\">dup</a>")
    say(len(ls))
    say(ls[0])
    say(ls[1])
}
`)
	mustContain(t, out, "2", "/a", "/b")
}

func TestURLMergeQuery(t *testing.T) {
	out := runOps(t, `
fn main {
    u := url.merge_query("https://ex.com/p?a=1&c=3", {"b": "2", "a": "9"})
    say(str.contains(u, "a=9"))
    say(str.contains(u, "b=2"))
    say(str.contains(u, "c=3"))
    say(str.starts_with(u, "https://ex.com/p"))
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" {
			t.Fatalf("%q", out)
		}
	}
}

func TestSHCaptureCheck(t *testing.T) {
	out := runOps(t, `
fn main -> Result {
    s := sh.capture("printf", ["hi"])?
    say(s)
    ok := sh.ok("true")?
    say(ok)
    bad := sh.ok("false")?
    say(bad == false)
}
`)
	mustContain(t, out, "hi", "true")
}

func TestSecretsRequireEnv(t *testing.T) {
	t.Setenv("WEFT_TEST_SECRET", "sekrit")
	var out bytes.Buffer
	ctx := weft.New(weft.Options{
		Stdout:  &out,
		Environ: map[string]string{"WEFT_TEST_SECRET": "sekrit"},
	})
	err := ctx.RunSource(context.Background(), "sec.weft", `
fn main -> Result {
    s := secrets.require("WEFT_TEST_SECRET")?
    say(secrets.unwrap(s))
    miss := secrets.require("WEFT_TEST_MISSING_ZZZ")
    say(miss.ok == false)
}
`)
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "sekrit") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "true") {
		t.Fatal(s)
	}
}

func TestFSWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	// embed path into source carefully
	src := `
fn main -> Result {
    fs.write("` + path + `", "hello")?
    t := fs.read("` + path + `")?
    say(t)
    say(fs.exists("` + path + `"))
    say(fs.stem("` + path + `"))
}
`
	out := runOps(t, src)
	mustContain(t, out, "hello", "true", "x")
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "hello" {
		t.Fatalf("%v %q", err, b)
	}
}
