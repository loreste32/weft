package format_test

import (
	"strings"
	"testing"

	"github.com/loreste/weft/internal/format"
)

func TestFormatRoundTripBasics(t *testing.T) {
	src := `fn main{mut  x:=1+2
say(  x  )
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fn main {") {
		t.Fatalf("fn main brace: %q", out)
	}
	if !strings.Contains(out, "mut x := 1 + 2") {
		t.Fatalf("bind: %q", out)
	}
	if !strings.Contains(out, "say(x)") {
		t.Fatalf("say: %q", out)
	}
	// second format stable
	out2, err := format.Source("t.weft", out)
	if err != nil {
		t.Fatal(err)
	}
	if out != out2 {
		t.Fatalf("not stable:\n%s\nvs\n%s", out, out2)
	}
}

func TestFormatUseAndMatch(t *testing.T) {
	src := `use json
fn main{
x:=match k{"text"{1}_ {0}}
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "use json") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "match k") {
		t.Fatal(out)
	}
}

func TestFormatEnum(t *testing.T) {
	src := `enum Status{Ok,Err,Pending}
fn main{say(Status.Ok)}
`
	out, err := format.Source("e.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	// short enum on one line
	if !strings.Contains(out, "enum Status { Ok, Err, Pending }") {
		t.Fatal(out)
	}
	out2, err := format.Source("e.weft", out)
	if err != nil {
		t.Fatal(err)
	}
	if out != out2 {
		t.Fatalf("enum format not stable:\n%s\nvs\n%s", out, out2)
	}
}

func TestFormatMapStringKeysStayQuoted(t *testing.T) {
	src := `fn main {
p := {"about": "demo", "flags": {"verbose": true}}
say(p.about)
}
`
	out, err := format.Source("m.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"about"`) {
		t.Fatalf("string key must stay quoted: %q", out)
	}
	if strings.Contains(out, "{about:") {
		t.Fatalf("bare ident key would change semantics: %q", out)
	}
	out2, _ := format.Source("m.weft", out)
	if out != out2 {
		t.Fatalf("not stable:\n%s\nvs\n%s", out, out2)
	}
}

func TestFormatMapMultilineNested(t *testing.T) {
	src := `fn main {
p := {"about": "demo", "flags": {"verbose": {"short": "v", "bool": true}, "env": {"short": "e", "default": "dev"}}}
say(p)
}
`
	out, err := format.Source("m.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	// nested maps should break across lines
	if !strings.Contains(out, "\"about\":") || !strings.Contains(out, "\n") {
		t.Fatalf("expected multi-line map:\n%s", out)
	}
	if !strings.Contains(out, "\"flags\":") {
		t.Fatal(out)
	}
	// stable
	out2, err := format.Source("m.weft", out)
	if err != nil {
		t.Fatal(err)
	}
	if out != out2 {
		t.Fatalf("not stable:\n%s\nvs\n%s", out, out2)
	}
	// small map stays one line
	small, err := format.Source("s.weft", "fn main {\nm := {\"a\": 1, \"b\": 2}\nsay(m)\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(small, `{"a": 1, "b": 2}`) {
		t.Fatalf("small map should stay one line:\n%s", small)
	}
}

func TestFormatMatchCompactAndClosure(t *testing.T) {
	src := `enum Status{Ok,Err}
fn main{
s:=Status.Ok
msg:=match s{Status.Ok{"good"}Status.Err{"bad"}_ {"?"}}
add:=fn(x){x+1}
say(add(1))
}
`
	out, err := format.Source("m.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `Status.Ok { "good" }`) {
		t.Fatalf("compact arm: %q", out)
	}
	if !strings.Contains(out, "fn(x) {") {
		t.Fatalf("closure: %q", out)
	}
	out2, _ := format.Source("m.weft", out)
	if out != out2 {
		t.Fatalf("not stable:\n%s\nvs\n%s", out, out2)
	}
}

func TestFormatType(t *testing.T) {
	src := `type Point {
    x: int
    y: float
}
fn main { say(1) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "type Point") {
		t.Fatal(out)
	}
}

func TestFormatTypeAlias(t *testing.T) {
	src := `type ID = str
fn main { say(1) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "type ID = str") {
		t.Fatal(out)
	}
}

func TestFormatIf(t *testing.T) {
	src := `fn main {
if true{say(1)}else{say(2)}
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "if true {") {
		t.Fatal(out)
	}
}

func TestFormatIfElseIf(t *testing.T) {
	src := `fn main {
if true{say(1)}else if false{say(2)}else{say(3)}
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "} else if") {
		t.Fatal(out)
	}
}

func TestFormatWhile(t *testing.T) {
	src := `fn main {
while true{break}
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "while true {") {
		t.Fatal(out)
	}
}

func TestFormatFor(t *testing.T) {
	src := `fn main {
for x in [1,2]{say(x)}
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "for x in") {
		t.Fatal(out)
	}
}

func TestFormatReturn(t *testing.T) {
	src := `fn f() { return 42 }
fn main { say(f()) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "return 42") {
		t.Fatal(out)
	}
}

func TestFormatDefer(t *testing.T) {
	src := `fn main{defer say("bye")
say("hi")}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "defer say") {
		t.Fatal(out)
	}
}

func TestFormatConst(t *testing.T) {
	src := `const N = 42
fn main { say(N) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "const N = 42") {
		t.Fatal(out)
	}
}

func TestFormatPub(t *testing.T) {
	src := `pub fn helper{1}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "pub fn helper") {
		t.Fatal(out)
	}
}

func TestFormatFuncLit(t *testing.T) {
	src := `fn main {
f := fn(x, y) { x + y }
say(f(1, 2))
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fn(x, y)") {
		t.Fatal(out)
	}
}

func TestFormatListLiteral(t *testing.T) {
	src := `fn main {
x := [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]
say(x)
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[") {
		t.Fatal(out)
	}
}

func TestFormatResultFn(t *testing.T) {
	src := `fn f() -> Result { 42 }
fn main { say(f()) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "-> Result") {
		t.Fatal(out)
	}
}

func TestFormatFString(t *testing.T) {
	src := "fn main {\n    say(f\"hello {name}\")\n}\n"
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestFormatDollarInterp(t *testing.T) {
	src := "fn main {\n    name := \"world\"\n    say(\"hello $name\")\n}\n"
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestFormatImport(t *testing.T) {
	src := `use http
use json
fn main { say(1) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "use http") {
		t.Fatal(out)
	}
}

func TestFormatImportPath(t *testing.T) {
	src := `import "./util.weft" as util
fn main { say(1) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"./util.weft"`) {
		t.Fatal(out)
	}
}

func TestFormatStructLit(t *testing.T) {
	src := `type Pt { x: int, y: int }
fn main {
    p := Pt{x: 1, y: 2}
    say(p)
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Pt{") {
		t.Fatal(out)
	}
}

func TestFormatPipeline(t *testing.T) {
	src := `fn double(x) { x * 2 }
fn main { 5 |> double |> say }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	// formatter may desugar pipelines to calls
	_ = out
}

func TestFormatNullCoalesce(t *testing.T) {
	src := `fn main {
    x := null
    say(x ?? 42)
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "??") {
		t.Fatal(out)
	}
}

func TestFormatQuestion(t *testing.T) {
	src := `fn f() -> Result {
    Ok(1)?
}
fn main { say(f()) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "?") {
		t.Fatal(out)
	}
}

func TestFormatAssignField(t *testing.T) {
	src := `fn main {
    mut m := {"a": 1}
    m.a = 2
    m["b"] = 3
    say(m)
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "m.a = 2") {
		t.Fatal(out)
	}
}

func TestFormatParseError(t *testing.T) {
	_, err := format.Source("bad.weft", "@@@")
	if err == nil {
		t.Fatal("parse error should propagate")
	}
}

func TestFormatPreservesNumberLits(t *testing.T) {
	// fmt must keep source forms (hex/bin/oct/sci/underscores), not rewrite to decimal.
	src := `fn main {
    say(0xff)
    say(0b1010)
    say(0o755)
    say(1_000)
    say(1e-6)
    say(2.5E+3)
    say(0xFF_00)
}
`
	out, err := format.Source("n.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"0xff", "0b1010", "0o755", "1_000", "1e-6", "2.5E+3", "0xFF_00"} {
		if !strings.Contains(out, want) {
			t.Fatalf("lost lit %q in:\n%s", want, out)
		}
	}
	out2, err := format.Source("n.weft", out)
	if err != nil {
		t.Fatal(err)
	}
	if out != out2 {
		t.Fatalf("not stable:\n%s\nvs\n%s", out, out2)
	}
}

func TestFormatTypeAnnotations(t *testing.T) {
	src := `fn add(a: int, b: int) -> int {
    a + b
}
fn main { say(add(1, 2)) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, ": int") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "-> int") {
		t.Fatal(out)
	}
}

func TestFormatListType(t *testing.T) {
	src := `type Bag {
    items: [str]
}
fn main { say(1) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[str]") {
		t.Fatal(out)
	}
}

func TestFormatMapType(t *testing.T) {
	src := `type Config {
    data: Map[str, int]
}
fn main { say(1) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestFormatResultType(t *testing.T) {
	src := `fn fetch() -> Result[str] { Ok("hi") }
fn main { say(fetch()) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Result") {
		t.Fatal(out)
	}
}

func TestFormatOptionalType(t *testing.T) {
	src := `type User {
    name: str
    email: str?
}
fn main { say(1) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "str?") {
		t.Fatal(out)
	}
}

func TestFormatSumTypeEnum(t *testing.T) {
	src := `enum Shape {
    Circle(r)
    Rect(w, h)
    Point
}
fn main { say(Shape.Circle(5)) }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Circle") {
		t.Fatal(out)
	}
}

func TestFormatMatchDestructure(t *testing.T) {
	src := `enum Shape {
    Circle(r)
    Point
}
fn main {
    s := Shape.Circle(5)
    r := match s {
        Shape.Circle(r) { r * r }
        Shape.Point { 0 }
        _ { -1 }
    }
    say(r)
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestFormatBinaryPrecedence(t *testing.T) {
	src := `fn main {
    x := 1 + 2 * 3
    y := (1 + 2) * 3
    z := true && false || true
    w := 1 == 2 || 3 != 4
    v := null ?? 42
    say(x, y, z, w, v)
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "??") {
		t.Fatal(out)
	}
}

func TestFormatNestedCalls(t *testing.T) {
	src := `fn main {
    say(len(filter([1, 2, 3, 4, 5], fn(x) { x > 2 })))
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestFormatLongList(t *testing.T) {
	src := `fn main {
    x := [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]
    say(x)
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestFormatStructLitMultiField(t *testing.T) {
	src := `type Config {
    name: str
    port: int
    debug: bool
    host: str
    workers: int
}
fn main {
    c := Config{name: "app", port: 8080, debug: true, host: "localhost", workers: 4}
    say(c)
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestFormatWhileBreakContinue(t *testing.T) {
	src := `fn main {
    mut i := 0
    while i < 10 {
        if i == 5 { break }
        if i % 2 == 0 {
            i = i + 1
            continue
        }
        say(i)
        i = i + 1
    }
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "break") && !strings.Contains(out, "continue") {
		t.Fatal(out)
	}
}

func TestFormatEmptyFn(t *testing.T) {
	src := `fn noop {}
fn main { noop() }
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fn noop") {
		t.Fatal(out)
	}
}

func TestFormatMultilineIfElse(t *testing.T) {
	src := `fn main {
    x := 5
    if x > 10 {
        say("big")
    } else if x > 5 {
        say("medium")
    } else {
        say("small")
    }
}
`
	out, err := format.Source("t.weft", src)
	if err != nil {
		t.Fatal(err)
	}
	out2, _ := format.Source("t.weft", out)
	if out != out2 {
		t.Fatalf("not stable:\n%s\nvs\n%s", out, out2)
	}
}
