package stdlib_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func run(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "test.weft", src); err != nil {
		t.Fatalf("run: %v", err)
	}
	return strings.TrimSpace(out.String())
}

// --- math ---

func TestStdlibMath(t *testing.T) {
	cases := []struct{ src, want string }{
		{`use math
fn main { say(math.abs(-5)) }`, "5"},
		{`use math
fn main { say(math.max(1, 3)) }`, "3"},
		{`use math
fn main { say(math.min(1, 3)) }`, "1"},
		{`use math
fn main { say(math.floor(3.7)) }`, "3"},
		{`use math
fn main { say(math.ceil(3.2)) }`, "4"},
		{`use math
fn main { say(math.round(3.5)) }`, "4"},
		{`use math
fn main { say(math.sqrt(9)) }`, "3"},
		{`use math
fn main { say(math.pow(2, 3)) }`, "8"},
		{`use math
fn main { say(math.clamp(5, 1, 10)) }`, "5"},
	}
	for _, tc := range cases {
		if got := run(t, tc.src); got != tc.want {
			t.Errorf("got %q want %q for %q", got, tc.want, tc.src[:40])
		}
	}
}

// --- str ---

func TestStdlibStr(t *testing.T) {
	cases := []struct{ src, want string }{
		{`use str
fn main { say(str.upper("hello")) }`, "HELLO"},
		{`use str
fn main { say(str.lower("HELLO")) }`, "hello"},
		{`use str
fn main { say(str.trim("  hi  ")) }`, "hi"},
		{`use str
fn main { say(str.starts_with("hello", "he")) }`, "true"},
		{`use str
fn main { say(str.ends_with("hello", "lo")) }`, "true"},
		{`use str
fn main { say(str.contains("hello", "ell")) }`, "true"},
		{`use str
fn main { say(str.replace("hello", "l", "r")) }`, "herro"},
		{`use str
fn main { say(str.split("a,b,c", ",")) }`, "[a, b, c]"},
		{`use str
fn main { say(str.join(["a", "b"], "-")) }`, "a-b"},
		{`use str
fn main { say(str.repeat("ab", 3)) }`, "ababab"},
	}
	for _, tc := range cases {
		if got := run(t, tc.src); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
}

// --- json ---

func TestStdlibJSON(t *testing.T) {
	out := run(t, `
use json
fn main -> Result {
    s := json.stringify({"a": 1})
    say(s)
}`)
	if !strings.Contains(out, `"a"`) {
		t.Fatal(out)
	}
}

func TestStdlibJSONParse(t *testing.T) {
	out := run(t, `
use json
fn main -> Result {
    v := json.parse("{\"x\": 42}")?
    say(v["x"])
}`)
	if out != "42" {
		t.Fatal(out)
	}
}

// --- env ---

func TestStdlibEnv(t *testing.T) {
	out := run(t, `
use env
fn main {
    say(env.get("PATH", "none"))
}`)
	if out == "none" || out == "" {
		t.Fatal("PATH should be set")
	}
}

// --- cli ---

func TestStdlibCLI(t *testing.T) {
	out := run(t, `
use cli
fn main {
    say(cli.args())
}`)
	// args should return a list
	if !strings.Contains(out, "[") {
		t.Fatal(out)
	}
}

// --- listops ---

func TestStdlibListOps(t *testing.T) {
	cases := []struct{ src, want string }{
		{`fn main { say(map([1, 2, 3], fn(x) { x * 2 })) }`, "[2, 4, 6]"},
		{`fn main { say(filter([1, 2, 3, 4], fn(x) { x > 2 })) }`, "[3, 4]"},
		{`fn main { say(reduce([1, 2, 3], 0, fn(a, x) { a + x })) }`, "6"},
		{`fn main { say(sort([3, 1, 2])) }`, "[1, 2, 3]"},
		{`fn main { say(reverse([1, 2, 3])) }`, "[3, 2, 1]"},
		{`fn main { say(any([false, true, false], fn(x) { x })) }`, "true"},
		{`fn main { say(all([true, true, true], fn(x) { x })) }`, "true"},
		{`fn main { say(all([true, false], fn(x) { x })) }`, "false"},
		{`fn main { say(zip([1, 2], ["a", "b"])) }`, "[[1, a], [2, b]]"},
		{`fn main { say(flatten([[1, 2], [3, 4]])) }`, "[1, 2, 3, 4]"},
		{`fn main { say(unique([1, 2, 2, 3])) }`, "[1, 2, 3]"},
		{`fn main { say(enumerate(["a", "b"])) }`, "[[0, a], [1, b]]"},
		{`fn main { say(find([1, 2, 3], fn(x) { x > 1 })) }`, "2"},
		{`fn main { say(reduce([1, 2, 3], 0, fn(a, x) { a + x })) }`, "6"},
	}
	for _, tc := range cases {
		if got := run(t, tc.src); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
}

// --- base64 ---

func TestStdlibBase64(t *testing.T) {
	out := run(t, `
use base64
fn main {
    enc := base64.encode("hello")
    say(base64.decode(enc))
}`)
	if !strings.Contains(out, "hello") {
		t.Fatal(out)
	}
}

// --- uuid ---

func TestStdlibUUID(t *testing.T) {
	out := run(t, `
use uuid
fn main {
    id := uuid.v4()
    say(len(id))
}`)
	if out != "36" {
		t.Fatalf("uuid length: %s", out)
	}
}

// --- random ---

func TestStdlibRandom(t *testing.T) {
	out := run(t, `
use random
fn main {
    n := random.int(1, 100)
    say(n > 0)
}`)
	if out != "true" {
		t.Fatal(out)
	}
}

// --- yaml ---

func TestStdlibYAML(t *testing.T) {
	out := run(t, `
use yaml
fn main -> Result {
    v := yaml.parse("name: weft\nversion: 1")?
    say(v["name"])
}`)
	if out != "weft" {
		t.Fatal(out)
	}
}

// --- xml ---

func TestStdlibXML(t *testing.T) {
	out := run(t, `
use xml
fn main -> Result {
    v := xml.parse("<root><item>hello</item></root>")?
    say(v)
}`)
	if out == "" {
		t.Fatal("empty xml parse")
	}
}

// --- csv ---

func TestStdlibCSV(t *testing.T) {
	out := run(t, `
use csv
fn main -> Result {
    rows := csv.parse("a,b\n1,2")?
    say(rows)
}`)
	if !strings.Contains(out, "1") {
		t.Fatal(out)
	}
}

// --- url ---

func TestStdlibURL(t *testing.T) {
	out := run(t, `
use url
fn main -> Result {
    u := url.parse("https://example.com/path?q=1")?
    say(u["host"])
}`)
	if out != "example.com" {
		t.Fatal(out)
	}
}

// --- ip ---

func TestStdlibIP(t *testing.T) {
	out := run(t, `
use ip
fn main {
    say(ip.is_valid("192.168.1.1"))
}`)
	if out != "true" {
		t.Fatal(out)
	}
}

// --- platform ---

func TestStdlibPlatform(t *testing.T) {
	out := run(t, `
use platform
fn main {
    say(platform.os())
}`)
	if out == "" {
		t.Fatal("empty os")
	}
}

// --- decimal ---

func TestStdlibDecimal(t *testing.T) {
	out := run(t, `
use decimal
fn main -> Result {
    a := decimal.new("10.5")?
    b := decimal.new("3.2")?
    say(decimal.add(a, b))
}`)
	if !strings.Contains(out, "13.7") {
		t.Fatalf("decimal: %s", out)
	}
}

// --- log ---

func TestStdlibLog(t *testing.T) {
	var buf bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &buf, Stderr: &buf})
	err := ctx.RunSource(context.Background(), "test.weft", `
use log
fn main {
    log.info("hello")
}`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- ini ---

func TestStdlibINI(t *testing.T) {
	out := run(t, `
use ini
fn main -> Result {
    v := ini.parse("[section]\nkey=value")?
    say(v["section"]["key"])
}`)
	if out != "value" {
		t.Fatal(out)
	}
}

// --- archive ---

func TestStdlibArchiveZip(t *testing.T) {
	// Just test the package loads
	out := run(t, `
use archive
fn main {
    say("ok")
}`)
	if out != "ok" {
		t.Fatal(out)
	}
}

// --- table ---

func TestStdlibTable(t *testing.T) {
	out := run(t, `
use table
fn main {
    rows := [{"name": "a", "age": 1}, {"name": "b", "age": 2}]
    say(table.project(rows, ["name"]))
}`)
	if !strings.Contains(out, "a") {
		t.Fatal(out)
	}
}

// --- bisect ---

func TestStdlibBisect(t *testing.T) {
	out := run(t, `
use bisect
fn main {
    say(bisect.left([1, 3, 5, 7], 4))
}`)
	if out != "2" {
		t.Fatal(out)
	}
}

// --- heap ---

func TestStdlibHeap(t *testing.T) {
	out := run(t, `
use heap
fn main -> Result {
    mut h := heap.push([], 3)
    h = heap.push(h, 1)
    h = heap.push(h, 2)
    r := heap.pop(h)?
    say(r.value)
}`)
	if out != "1" {
		t.Fatal(out)
	}
}

// --- collections ---

func TestStdlibCollections(t *testing.T) {
	out := run(t, `
use collections
fn main {
    c := collections.counter(["a", "b", "a", "a"])
    say(c["a"])
}`)
	if out != "3" {
		t.Fatal(out)
	}
}

// --- sh ---

func TestStdlibSh(t *testing.T) {
	out := run(t, `
use sh
fn main -> Result {
    r := sh.run("echo", ["hello"])?
    say(r.stdout)
}`)
	if !strings.Contains(out, "hello") {
		t.Fatal(out)
	}
}

func TestStdlibShCapture(t *testing.T) {
	out := run(t, `
use sh
fn main -> Result {
    s := sh.capture("echo", ["hi"])?
    say(s)
}`)
	if !strings.Contains(out, "hi") {
		t.Fatal(out)
	}
}

func TestStdlibShOk(t *testing.T) {
	out := run(t, `
use sh
fn main -> Result {
    say(sh.ok("true")?)
}`)
	if out != "true" {
		t.Fatal(out)
	}
}
