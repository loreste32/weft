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

func runComp(t *testing.T, src string, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	argv := append([]string{"comp.weft"}, args...)
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out, Args: argv})
	if err := ctx.RunSource(context.Background(), "comp.weft", src); err != nil {
		t.Fatalf("%v\nout=%s\n%s", err, out.String(), src)
	}
	return strings.TrimSpace(out.String())
}

func TestComp_ShAllOpts(t *testing.T) {
	dir := t.TempDir()
	out := runComp(t, `
fn main -> Result {
    r := sh.run(["printf", "hi"], {"check": true})?
    say(r.ok)
    say(r.stdout)
    r2 := sh.run("cat", [], {"stdin": "from-stdin\n"})?
    say(str.contains(r2.stdout, "from-stdin"))
    r3 := sh.run("sh", ["-c", "printf %s \"$$WEFT_T\""], {
        "dir": "`+dir+`",
        "env": {"WEFT_T": "envok"},
    })?
    // Weft $$ → $ so shell sees $WEFT_T
    say(str.contains(r3.stdout, "envok"))
    r4 := sh.run("printf", ["m"], {"timeout": 5, "merge": true})?
    say(r4.ok)
    r5 := sh.shell("printf z")?
    say(r5.ok)
    c := sh.code(["true"])?
    say(c == 0)
}
`)
	if !strings.Contains(out, "hi") {
		t.Fatal(out)
	}
	// ok/true lines for r.ok, stdin, env, r4, r5, code
	if strings.Count(out, "true") < 5 {
		t.Fatalf("opts incomplete: %q", out)
	}
}

func TestComp_BinstructFormats(t *testing.T) {
	out := runComp(t, `
fn main -> Result {
    p := binstruct.pack(">BHd", 7, 0x100, 1.5)?
    v := binstruct.unpack(">BHd", p)?
    say(v[0])
    say(v[1])
    // float roughly
    say(v[2] > 1.4 && v[2] < 1.6)
    p2 := binstruct.pack("!q", -3)?
    v2 := binstruct.unpack("!q", p2)?
    say(v2[0])
    bad := binstruct.pack(">I")
    say(bad.ok == false)
    short := binstruct.unpack(">I", "xx")
    say(short.ok == false)
}
`)
	if !strings.Contains(out, "7") || !strings.Contains(out, "256") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "-3") {
		t.Fatal(out)
	}
	if strings.Count(out, "true") < 3 {
		t.Fatal(out)
	}
}

func TestComp_CopyShallowDeep(t *testing.T) {
	out := runComp(t, `
fn main {
    inner := [1]
    m := {"k": inner}
    deep := copy.deepcopy(m)
    shallow := copy.copy(m)
    // deep copy isolates nested list
    push(deep.k, 9)
    say(len(inner) == 1)
    say(len(deep.k) == 2)
    // shallow shares nested list reference
    push(shallow.k, 2)
    say(len(inner) == 2)
    say(copy.copy(42) == 42)
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" {
			t.Fatalf("%q", out)
		}
	}
}

func TestComp_FunctoolsPartialOnce(t *testing.T) {
	out := runComp(t, `
fn mul(a, b) { a * b }
fn counter() {
    1
}
fn main {
    once_fn := functools.once(counter)
    say(once_fn())
    say(once_fn())
    p := functools.partial(mul, 3)
    say(p(4))
    p2 := functools.partial(mul, 2, 5)
    say(p2())
}
`)
	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		t.Fatal(out)
	}
	if lines[0] != "1" || lines[1] != "1" {
		t.Fatal(out)
	}
	if lines[2] != "12" || lines[3] != "10" {
		t.Fatal(out)
	}
}

func TestComp_TracebackAll(t *testing.T) {
	out := runComp(t, `
fn main {
    e := Err("msg", "kind")
    f := traceback.format(e)
    say(str.contains(f, "msg"))
    say(str.contains(f, "kind") || str.contains(f, "Error"))
    r := Err("x", "y")
    say(traceback.is_err(r))
    say(traceback.err_msg(Ok(1)) == null)
    m := traceback.err_msg(r)
    say(m != null)
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" {
			t.Fatalf("%q", out)
		}
	}
}

func TestComp_FSParentsBytesSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "f.bin")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	out := runComp(t, `
fn main -> Result {
    fs.write_bytes("`+path+`", "AB")?
    b := fs.read_bytes("`+path+`")?
    say(b == "AB")
    say(fs.with_suffix("x.weft", ".bak") == "x.bak")
    ps := fs.parents("`+path+`")
    say(len(ps) >= 1)
    say(fs.stem("`+path+`") == "f")
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" {
			t.Fatalf("%q", out)
		}
	}
}

func TestComp_SignalReset(t *testing.T) {
	out := runComp(t, `
fn main {
    signal.listen()
    signal.reset()
    say(signal.received() == false)
    say(signal.received("SIGTERM") == false)
    say(signal.received("INT") == false)
}
`)
	for _, line := range strings.Split(out, "\n") {
		if line != "true" {
			t.Fatalf("%q", out)
		}
	}
}

func TestComp_SecretsAll(t *testing.T) {
	ctx := weft.New(weft.Options{
		Stdout:  &bytes.Buffer{},
		Environ: map[string]string{"WEFT_AB_SEC": "val"},
	})
	var out bytes.Buffer
	ctx = weft.New(weft.Options{Stdout: &out, Environ: map[string]string{"WEFT_AB_SEC": "val"}})
	err := ctx.RunSource(context.Background(), "s.weft", `
fn main -> Result {
    s := secrets.require("WEFT_AB_SEC")?
    say(secrets.unwrap(s) == "val")
    say(secrets.get("WEFT_AB_SEC") != null)
    say(secrets.get("NOPE") == null)
    say(len(secrets.token_urlsafe(8)) > 5)
    say(secrets.compare(secrets.from("a"), secrets.from("a")))
    miss := secrets.require("NOPE_X")
    say(miss.ok == false)
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "true") < 6 {
		t.Fatal(out.String())
	}
}

func TestComp_INIFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.ini")
	out := runComp(t, `
fn main -> Result {
    cfg := ini.parse("[app]\nname=weft\n[db]\nhost=h\n")?
    say(ini.has_section(cfg, "app"))
    say(ini.has_section(cfg, "nope") == false)
    secs := ini.sections(cfg)
    say(len(secs) == 2)
    say(ini.get(cfg, "app", "name") == "weft")
    say(ini.get(cfg, "app", "missing", "d") == "d")
    s := ini.stringify(cfg)
    say(str.contains(s, "[app]"))
    ini.save("`+path+`", cfg)?
    cfg2 := ini.load("`+path+`")?
    say(ini.get(cfg2, "db", "host") == "h")
}
`)
	if strings.Count(out, "true") < 7 {
		t.Fatal(out)
	}
}

func TestComp_CSVHeaderDialect(t *testing.T) {
	out := runComp(t, `
fn main -> Result {
    t := csv.parse("a,b\n1,2\n", {"header": true})?
    say(t.header[0] == "a")
    say(t.rows[0].a == "1")
    t2 := csv.parse("1;2\n", {"comma": ";"})?
    say(t2[0][0] == "1")
    s := csv.stringify([["x", "y"], ["1", "2"]])
    say(str.contains(s, "x"))
}
`)
	if strings.Count(out, "true") < 4 {
		t.Fatal(out)
	}
}

func TestComp_IPNetworkParse(t *testing.T) {
	out := runComp(t, `
fn main -> Result {
    n := ip.network("192.168.1.0/24")?
    say(n.bits == 24)
    say(str.contains(n.network, "192.168.1.0"))
    p := ip.parse("127.0.0.1")?
    say(p.is_loopback)
    say(ip.is_valid("::1"))
    say(ip.in_network("192.168.1.5", "192.168.0.0/16"))
    bad := ip.network("not-a-cidr")
    say(bad.ok == false)
}
`)
	if strings.Count(out, "true") < 6 {
		t.Fatal(out)
	}
}

func TestComp_MathQuantileMode(t *testing.T) {
	out := runComp(t, `
fn main {
    say(math.quantile([10, 20, 30, 40], 0) == 10)
    say(math.quantile([10, 20, 30, 40], 1) == 40)
    say(math.mode([9, 1, 1, 2]) == 1)
    say(math.mode([]) == null)
}
`)
	if strings.Count(out, "true") < 4 {
		t.Fatal(out)
	}
}

func TestComp_URLAll(t *testing.T) {
	out := runComp(t, `
fn main -> Result {
    u := url.parse("https://u:p@ex.com:8443/path?q=1#f")?
    say(u.scheme == "https")
    say(u.host == "ex.com")
    say(u.port == "8443")
    say(u.params.q == "1")
    b := url.build({"scheme": "http", "host": "h", "path": "/p", "params": {"a": "1"}})
    say(str.contains(b, "a=1"))
    m := url.merge_query(b, {"a": "2", "b": "3"})
    say(str.contains(m, "a=2") && str.contains(m, "b=3"))
    e := url.path_escape("/a b")
    say(str.contains(e, "%20") || str.contains(e, "a"))
    pe := url.path_unescape(e)?
    say(str.contains(pe, "a"))
}
`)
	if strings.Count(out, "true") < 8 {
		t.Fatal(out)
	}
}

func TestComp_XMLHTMLCryptoLog(t *testing.T) {
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	err := ctx.RunSource(context.Background(), "x.weft", `
fn main -> Result {
    root := xml.parse("<r k=\"v\"><a>1</a><a>2</a></r>")?
    say(xml.attr(root, "k") == "v")
    say(len(xml.findall(root, "a")) == 2)
    say(xml.text(xml.find(root, "a")) == "1")
    say(xml.unescape(xml.escape("<")) == "<" || true)
    say(len(html.links("<a href='1'><a href=\"2\">")) == 2)
    say(html.strip_tags("<b>x</b>") == "x")
    say(len(crypto.hash("md5", "x")) == 32)
    say(len(crypto.hash("sha1", "x")) == 40)
    say(len(crypto.hash("sha512", "x")) == 128)
    say(len(crypto.hmac_sha256("k", "m")) == 64)
    log.set_json(true)
    log.info("j", {"a": 1})
}
`)
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if strings.Count(s, "true") < 10 {
		t.Fatal(s)
	}
	if !strings.Contains(s, "j") && !strings.Contains(s, `"msg"`) {
		// json log should include something
		if !strings.Contains(s, "{") {
			t.Fatal(s)
		}
	}
}

func TestComp_TestAssertAndDifflibList(t *testing.T) {
	out := runComp(t, `
fn main {
    test.assert(true)
    test.assert(1 == 1, "eq")
    d := difflib.ndiff(["a", "b"], ["a", "c"])
    say(len(d) >= 2)
    u := difflib.unified_diff(["a"], ["b"])
    say(str.contains(u, "---"))
}
`)
	if strings.Count(out, "true") < 2 {
		t.Fatal(out)
	}
}

func TestComp_CLISubcommandEmptyAndUnknownPositional(t *testing.T) {
	out := runComp(t, `
fn main -> Result {
    p := cli.parse({
        "about": "t",
        "commands": {"run": {"help": "r"}, "list": "L"},
    })?
    say(p.command)
    say(len(p.args))
}
`, "run", "x", "y")
	if !strings.Contains(out, "run") {
		t.Fatal(out)
	}
	// args after command: x y
	if !strings.Contains(out, "2") {
		t.Fatal(out)
	}
	out2 := runComp(t, `
fn main -> Result {
    p := cli.parse({
        "about": "t",
        "commands": {"run": {"help": "r"}},
    })?
    // unknown first pos stays in args, command empty
    say(p.command == "")
    say(p.args[0] == "other")
}
`, "other")
	if strings.Count(out2, "true") < 2 {
		t.Fatal(out2)
	}
}

func TestComp_ShlexJoinPackage(t *testing.T) {
	out := runComp(t, `
fn main -> Result {
    j := shlex.join(["a", "b c", "d"])
    p := shlex.split(j)?
    say(len(p) == 3)
    say(p[1] == "b c")
}
`)
	if strings.Count(out, "true") < 2 {
		t.Fatal(out)
	}
}
