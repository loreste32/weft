package weft_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/pkg/weft"
)

func runOut(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "test.weft", src); err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	return out.String()
}

func runErr(t *testing.T, src string) error {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	return ctx.RunSource(context.Background(), "test.weft", src)
}

func TestHello(t *testing.T) {
	out := runOut(t, `fn main() { println("hello, weft") }`)
	if !strings.Contains(out, "hello, weft") {
		t.Fatalf("got %q", out)
	}
}

func TestFib(t *testing.T) {
	src := `
fn fib(n: int) -> int {
    if n < 2 { return n }
    return fib(n - 1) + fib(n - 2)
}
fn main() { println(fib(10)) }
`
	if strings.TrimSpace(runOut(t, src)) != "55" {
		t.Fatal("fib")
	}
}

func TestForSum(t *testing.T) {
	src := `
fn main() {
    let mut total = 0
    for n in [1, 2, 3, 4, 5] {
        total = total + n
    }
    println(total)
}
`
	if strings.TrimSpace(runOut(t, src)) != "15" {
		t.Fatal("for sum")
	}
}

func TestIfElse(t *testing.T) {
	src := `
fn main() {
    if 1 < 2 {
        println("yes")
    } else {
        println("no")
    }
    if false {
        println("bad")
    }
    println("done")
}
`
	out := runOut(t, src)
	if !strings.Contains(out, "yes") || !strings.Contains(out, "done") {
		t.Fatalf("%q", out)
	}
	if strings.Contains(out, "bad") {
		t.Fatalf("else leaked: %q", out)
	}
}

func TestWhile(t *testing.T) {
	src := `
fn main() {
    let mut i = 0
    let mut s = 0
    while i < 5 {
        s = s + i
        i = i + 1
    }
    println(s)
}
`
	if strings.TrimSpace(runOut(t, src)) != "10" {
		t.Fatal("while")
	}
}

func TestResultWrap(t *testing.T) {
	src := `
fn f() -> Result[int] {
    return 42
}
fn main() {
    println(f())
}
`
	if strings.TrimSpace(runOut(t, src)) != "Ok(42)" {
		t.Fatalf("want Ok(42), got %q", runOut(t, src))
	}
}

func TestResultNoDoubleWrap(t *testing.T) {
	src := `
fn f() -> Result[int] {
    return Ok(7)
}
fn main() {
    println(f())
}
`
	if strings.TrimSpace(runOut(t, src)) != "Ok(7)" {
		t.Fatalf("got %q", runOut(t, src))
	}
}

func TestMutRequired(t *testing.T) {
	src := `
fn main() {
    let x = 1
    x = 2
    println(x)
}
`
	err := runErr(t, src)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("want immutable reassignment error, got %v", err)
	}
}

func TestImportUnknown(t *testing.T) {
	src := `
import nonexistent
fn main() {}
`
	err := runErr(t, src)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want package not found error, got %v", err)
	}
}

func TestDivZero(t *testing.T) {
	src := `
fn main() {
    println(1 / 0)
}
`
	err := runErr(t, src)
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("want div zero, got %v", err)
	}
}

func TestAndOr(t *testing.T) {
	src := `
fn main() {
    if true && false {
        println("no")
    }
    if false || true {
        println("yes")
    }
}
`
	out := runOut(t, src)
	if strings.Contains(out, "no") || !strings.Contains(out, "yes") {
		t.Fatalf("%q", out)
	}
}

func TestArgs(t *testing.T) {
	var out bytes.Buffer
	ctx := weft.New(weft.Options{
		Stdout: &out,
		Args:   []string{"script.weft", "a", "b"},
	})
	src := `fn main() { println(args[1]) }`
	if err := ctx.RunSource(context.Background(), "t.weft", src); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "a" {
		t.Fatalf("got %q", out.String())
	}
}

func TestJSON(t *testing.T) {
	src := `
fn main() -> Result {
    let data = json.parse("{\"x\":1,\"y\":\"hi\"}")?
    println(data.x, data.y)
}
`
	out := runOut(t, src)
	if !strings.Contains(out, "1") || !strings.Contains(out, "hi") {
		t.Fatalf("%q", out)
	}
}

func TestBareResult(t *testing.T) {
	src := `
fn f() -> Result { return 1 }
fn main() { println(f()) }
`
	if !strings.Contains(runOut(t, src), "Ok(1)") {
		t.Fatal(runOut(t, src))
	}
}

func TestAgentAskMock(t *testing.T) {
	var out bytes.Buffer
	step := 0
	ctx := weft.New(weft.Options{
		Stdout: &out,
		LLMDo: func(reqBody []byte) (string, []runtime.ToolCall, error) {
			step++
			body := string(reqBody)
			if step == 1 {
				// model requests tool
				if !strings.Contains(body, "weather") && !strings.Contains(body, "tools") {
					// still return tool call
				}
				return "", []runtime.ToolCall{{
					ID: "1", Name: "weather", ArgsJSON: `{"city":"Paris"}`,
				}}, nil
			}
			// final answer
			return "clear in Paris", nil, nil
		},
	})
	src := `
fn weather(city) { "sunny " + city }
fn main() -> Result {
    let r = llm.ask("weather?", [llm.tool("weather", weather)])?
    println(r)
}
`
	if err := ctx.RunSource(context.Background(), "a.weft", src); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "clear in Paris") {
		t.Fatalf("got %q step=%d", out.String(), step)
	}
	if step < 2 {
		t.Fatalf("expected 2 llm rounds, got %d", step)
	}
}

func TestImportLLM(t *testing.T) {
	src := `
import llm
fn main() -> Result {
    println("ok")
}
`
	if !strings.Contains(runOut(t, src), "ok") {
		t.Fatal("import llm")
	}
}

func TestEvalExpr(t *testing.T) {
	ctx := weft.New(weft.Options{})
	v, err := ctx.Eval("1 + 2 * 3")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "7" {
		t.Fatalf("got %s", v.String())
	}
}

func TestEvalLetPersist(t *testing.T) {
	ctx := weft.New(weft.Options{})
	if _, err := ctx.Eval("let mut n = 10"); err != nil {
		t.Fatal(err)
	}
	v, err := ctx.Eval("n + 1")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "11" {
		t.Fatalf("got %s", v.String())
	}
}

func TestHexAndScientificLits(t *testing.T) {
	src := `
fn main {
    say(0xff)
    say(0x10)
    say(1e2)
    say(0b1010)
    say(0o10)
    say(1_000)
    say(0xFF_00)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "num.weft", src); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	if got != "255\n16\n100\n10\n8\n1000\n65280" {
		t.Fatalf("%q", got)
	}
}

func TestPipelineSay(t *testing.T) {
	// say is a statement keyword but must work as a pipeline sink expression.
	src := `
fn double(x) { x * 2 }
fn main {
    21 |> double |> say
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "pipe.weft", src); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "42" {
		t.Fatalf("%q", out.String())
	}
}

func TestStreamMock(t *testing.T) {
	var out bytes.Buffer
	ctx := weft.New(weft.Options{
		Stdout: &out,
		LLMDo: func(reqBody []byte) (string, []runtime.ToolCall, error) {
			return "hello", nil, nil
		},
	})
	src := `
fn main() -> Result {
    let events = llm.stream("x")?
    let mut s = ""
    for e in events {
        if e.kind == "text" {
            s = s + e.text
        }
    }
    println(s)
}
`
	if err := ctx.RunSource(context.Background(), "s.weft", src); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "hello" {
		t.Fatalf("%q", out.String())
	}
}

func TestStreamTextMock(t *testing.T) {
	var out bytes.Buffer
	ctx := weft.New(weft.Options{
		Stdout: &out,
		LLMDo: func(reqBody []byte) (string, []runtime.ToolCall, error) {
			return "streamed ok", nil, nil
		},
	})
	src := `
fn main -> Result {
    say(llm.stream_text("x")?)
}
`
	if err := ctx.RunSource(context.Background(), "st.weft", src); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "streamed ok" {
		t.Fatalf("%q", out.String())
	}
}

func TestChatMessagesAndAskOpts(t *testing.T) {
	var out bytes.Buffer
	var sawSystem bool
	step := 0
	ctx := weft.New(weft.Options{
		Stdout: &out,
		LLMDo: func(reqBody []byte) (string, []runtime.ToolCall, error) {
			body := string(reqBody)
			if strings.Contains(body, "be terse") || strings.Contains(body, "system") {
				sawSystem = true
			}
			step++
			if strings.Contains(body, "add") || strings.Contains(body, "tools") {
				if step == 1 {
					return "", []runtime.ToolCall{{
						ID: "1", Name: "add", ArgsJSON: `{"a":2,"b":3}`,
					}}, nil
				}
			}
			return "reply-ok", nil, nil
		},
	})
	src := `
fn add(a, b) { a + b }

fn main -> Result {
    say(llm.chat([
        {"role": "system", "content": "be terse"},
        {"role": "user", "content": "hi"},
    ])?)
    r := llm.ask("2+3?", [
        llm.tool("add", add, "add two numbers"),
    ], {"system": "math helper", "max_steps": 5})?
    say(r)
}
`
	if err := ctx.RunSource(context.Background(), "opts.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "reply-ok") {
		t.Fatalf("%q", got)
	}
	if !sawSystem {
		t.Fatal("expected system message in request body")
	}
}

func TestExtractMock(t *testing.T) {
	var out bytes.Buffer
	ctx := weft.New(weft.Options{
		Stdout: &out,
		LLMDo: func(reqBody []byte) (string, []runtime.ToolCall, error) {
			return `{"city":"Paris","temp_c":21}`, nil, nil
		},
	})
	src := `
fn main() -> Result {
    let d = llm.extract("x")?
    println(d.city)
}
`
	if err := ctx.RunSource(context.Background(), "e.weft", src); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Paris") {
		t.Fatalf("%q", out.String())
	}
}

func TestAnonFn(t *testing.T) {
	src := `
fn main() {
    let f = fn(x) { x + 1 }
    println(f(41))
}
`
	if strings.TrimSpace(runOut(t, src)) != "42" {
		t.Fatal(runOut(t, src))
	}
}

func TestPathImport(t *testing.T) {
	// write temp module
	dir := t.TempDir()
	mod := dir + "/util.weft"
	main := dir + "/main.weft"
	os.WriteFile(mod, []byte("pub fn inc(x) { x + 1 }\n"), 0644)
	os.WriteFile(main, []byte("import \"./util.weft\" as u\nfn main() { println(u.inc(41)) }\n"), 0644)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunFile(context.Background(), main); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "42" {
		t.Fatalf("%q", out.String())
	}
}

func TestParallel(t *testing.T) {
	src := `
fn main() -> Result {
    let r = parallel([fn() { 1 }, fn() { 2 }, fn() { 3 }])?
    println(r)
}
`
	out := runOut(t, src)
	if !strings.Contains(out, "1") || !strings.Contains(out, "2") || !strings.Contains(out, "3") {
		t.Fatalf("%q", out)
	}
}

func TestCheckUndefined(t *testing.T) {
	err := weft.CheckSource("t.weft", "fn main() { println(missing) }")
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("want undefined error, got %v", err)
	}
}

func TestCheckOK(t *testing.T) {
	if err := weft.CheckSource("t.weft", "fn main() { println(1) }"); err != nil {
		t.Fatal(err)
	}
}

func TestSpawnJoin(t *testing.T) {
	src := `
fn main() {
    let h = spawn(fn() { 40 + 2 })
    println(h.join())
}
`
	if strings.TrimSpace(runOut(t, src)) != "42" {
		t.Fatal(runOut(t, src))
	}
}

func TestPkgInstallAndImport(t *testing.T) {
	dir := t.TempDir()
	// package
	pkgDir := filepath.Join(dir, "mypkg")
	os.MkdirAll(pkgDir, 0755)
	os.WriteFile(filepath.Join(pkgDir, "lib.weft"), []byte("pub fn id(x) { x }\n"), 0644)
	// project
	proj := filepath.Join(dir, "app")
	os.MkdirAll(proj, 0755)
	os.WriteFile(filepath.Join(proj, "weft.json"), []byte(`{"name":"app","deps":{"mypkg":{"path":"../mypkg"}}}`), 0644)
	os.WriteFile(filepath.Join(proj, "main.weft"), []byte("import mypkg\nfn main() { println(mypkg.id(7)) }\n"), 0644)
	if err := weft.PkgInstall(proj); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunFile(context.Background(), filepath.Join(proj, "main.weft")); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "7" {
		t.Fatalf("%q", out.String())
	}
}

func TestChannels(t *testing.T) {
	src := `
fn main() -> Result {
    let ch = channel(1)
    send(ch, 42)
    let v = recv(ch)?
    println(v)
}
`
	if strings.TrimSpace(runOut(t, src)) != "42" {
		t.Fatal(runOut(t, src))
	}
}

func TestGroupWait(t *testing.T) {
	src := `
fn main() -> Result {
    let g = group()
    g.go(fn() { 1 })
    g.go(fn() { 2 })
    let r = g.wait()?
    println(r)
}
`
	out := runOut(t, src)
	if !strings.Contains(out, "1") || !strings.Contains(out, "2") {
		t.Fatalf("%q", out)
	}
}

func TestModuleInternalCall(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "p")
	os.MkdirAll(pkgDir, 0755)
	os.WriteFile(filepath.Join(pkgDir, "lib.weft"), []byte(`
pub fn a(x) { x + 1 }
pub fn b(x) { a(x) + 1 }
`), 0644)
	proj := filepath.Join(dir, "app")
	os.MkdirAll(proj, 0755)
	os.WriteFile(filepath.Join(proj, "weft.json"), []byte(`{"name":"app","deps":{"p":{"path":"`+pkgDir+`"}}}`), 0644)
	os.WriteFile(filepath.Join(proj, "main.weft"), []byte("import p\nfn main() { println(p.b(1)) }\n"), 0644)
	if err := weft.PkgInstall(proj); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunFile(context.Background(), filepath.Join(proj, "main.weft")); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "3" {
		t.Fatalf("want 3 got %q", out.String())
	}
}

func TestWeftSyntax(t *testing.T) {
	src := `
fn double(x) { x * 2 }
fn main {
    mut n := 10
    n = n + 1
    name := "weft"
    say("hi $name n=$n d=${double(3)}")
}
`
	out := runOut(t, src)
	if !strings.Contains(out, "hi weft") || !strings.Contains(out, "n=11") || !strings.Contains(out, "d=6") {
		t.Fatalf("%q", out)
	}
}

func TestPipe(t *testing.T) {
	src := `
fn inc(x) { x + 1 }
fn main {
    41 |> inc |> println
}
`
	if strings.TrimSpace(runOut(t, src)) != "42" {
		t.Fatal(runOut(t, src))
	}
}

func TestUseKeyword(t *testing.T) {
	// use is alias for import; stdlib
	src := `
use json
fn main -> Result {
    d := json.parse("{\"a\":1}")?
    say(d.a)
}
`
	if !strings.Contains(runOut(t, src), "1") {
		t.Fatal(runOut(t, src))
	}
}

func TestConcurrentDefault(t *testing.T) {
	src := `
fn main -> Result {
    rs := gather([fn() { 1 }, fn() { 2 }])?
    say(rs)
    w := race([fn() { "a" }, fn() { "b" }])?
    say(w)
    t := timeout(2, fn() { 99 })?
    say(t)
    h := spawn(fn(x) { x + 1 }, 41)
    say(h.await()?)
    // map is concurrent by default but order-preserving
    xs := map([1, 2, 3], fn(n) { n * 10 })
    say(xs)
    ys := seq_map([1, 2, 3], fn(n) { n + 1 })
    say(ys)
}
`
	out := runOut(t, src)
	if !strings.Contains(out, "99") || !strings.Contains(out, "42") {
		t.Fatalf("%q", out)
	}
	if !strings.Contains(out, "10") || !strings.Contains(out, "2") {
		t.Fatalf("map/seq_map missing: %q", out)
	}
}

func TestIfExprAsLastReturn(t *testing.T) {
	src := `
fn pick(x) {
    if x > 0 {
        "pos"
    } else {
        "neg"
    }
}
fn main {
    say(pick(1))
    say(pick(-1))
}
`
	out := runOut(t, src)
	if !strings.Contains(out, "pos") || !strings.Contains(out, "neg") {
		t.Fatalf("if-expr return broken: %q", out)
	}
}

func TestPackagePathImportEscapeRejected(t *testing.T) {
	root := t.TempDir()
	secret := filepath.Join(root, "secret")
	os.MkdirAll(secret, 0755)
	os.WriteFile(filepath.Join(secret, "lib.weft"), []byte(`pub fn secret() { "PWNED" }
`), 0644)
	pkg := filepath.Join(root, "evil")
	os.MkdirAll(pkg, 0755)
	os.WriteFile(filepath.Join(pkg, "lib.weft"), []byte(`
use "../secret/lib.weft" as s
pub fn leak() { s.secret() }
`), 0644)
	os.WriteFile(filepath.Join(pkg, "weft.json"), []byte(`{"name":"evil","type":"module","entry":"lib.weft","exports":["leak"]}`), 0644)
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0755)
	os.WriteFile(filepath.Join(app, "weft.json"), []byte(`{"name":"app","deps":{"evil":{"path":"../evil"}}}`), 0644)
	os.WriteFile(filepath.Join(app, "main.weft"), []byte(`
use evil
fn main { say(evil.leak()) }
`), 0644)
	if err := weft.PkgInstall(app); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	err := ctx.RunFile(context.Background(), filepath.Join(app, "main.weft"))
	if err == nil || !strings.Contains(err.Error(), "escapes package root") {
		t.Fatalf("want path escape error, got %v out=%q", err, out.String())
	}
}

func TestVendorTamperFailsRun(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "p")
	os.MkdirAll(pkg, 0755)
	os.WriteFile(filepath.Join(pkg, "lib.weft"), []byte("pub fn id(x) { x }\n"), 0644)
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0755)
	os.WriteFile(filepath.Join(app, "weft.json"), []byte(`{"name":"app","deps":{"p":{"path":"../p"}}}`), 0644)
	os.WriteFile(filepath.Join(app, "main.weft"), []byte("use p\nfn main { say(p.id(1)) }\n"), 0644)
	if err := weft.PkgInstall(app); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(app, "vendor", "p", "lib.weft"), []byte("pub fn id(x) { 999 }\n"), 0644)
	ctx := weft.New(weft.Options{})
	err := ctx.RunFile(context.Background(), filepath.Join(app, "main.weft"))
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("want integrity error, got %v", err)
	}
}

func TestTransitiveModuleDeps(t *testing.T) {
	root := t.TempDir()
	mathx := filepath.Join(root, "packages", "mathx")
	os.MkdirAll(mathx, 0755)
	os.WriteFile(filepath.Join(mathx, "lib.weft"), []byte(`
use "./h.weft" as h
pub fn double(x) { h.times(x, 2) }
`), 0644)
	os.WriteFile(filepath.Join(mathx, "h.weft"), []byte(`pub fn times(a, b) { a * b }
`), 0644)
	os.WriteFile(filepath.Join(mathx, "weft.json"), []byte(`{"name":"mathx","type":"module","entry":"lib.weft","exports":["double"]}`), 0644)

	mid := filepath.Join(root, "packages", "mid")
	os.MkdirAll(mid, 0755)
	os.WriteFile(filepath.Join(mid, "lib.weft"), []byte(`
use mathx
pub fn quad(x) { mathx.double(mathx.double(x)) }
`), 0644)
	os.WriteFile(filepath.Join(mid, "weft.json"), []byte(`{"name":"mid","type":"module","entry":"lib.weft","exports":["quad"],"deps":{"mathx":{"path":"../mathx"}}}`), 0644)

	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0755)
	// only mid — mathx must install transitively
	os.WriteFile(filepath.Join(app, "weft.json"), []byte(`{"name":"app","deps":{"mid":{"path":"../packages/mid"}}}`), 0644)
	os.WriteFile(filepath.Join(app, "main.weft"), []byte(`
use mid
fn main { say(mid.quad(3)) }
`), 0644)
	if err := weft.PkgInstall(app); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunFile(context.Background(), filepath.Join(app, "main.weft")); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "12" {
		t.Fatalf("want 12 got %q", out.String())
	}
}

func TestFnNoParens(t *testing.T) {
	src := `fn main { say("ok") }`
	if !strings.Contains(runOut(t, src), "ok") {
		t.Fatal(runOut(t, src))
	}
}
