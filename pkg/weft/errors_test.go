package weft_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestErrorManagementSuite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "err_test.weft")
	src := `
fn load(path) -> Result {
    ensure(path != "", "path required")?
    if path == "missing" {
        return bail("not found", "fs")
    }
    Ok("data:" + path)
}

fn test_ensure_bail {
    a := load("x")?
    test.eq(a, "data:x")

    e1 := load("")
    test.err(e1)
    test.contains(e1.err.message, "path required")

    e2 := load("missing")
    test.err(e2)
    test.eq(e2.err.kind, "fs")
}

fn test_context_and_or {
    e2 := Err("not found", "fs")
    e3 := e2.context("open config")
    test.err(e3)
    test.contains(e3.err.message, "open config")
    test.contains(e3.err.message, "not found")

    r := Err("a").or(Ok("b"))
    test.eq(r?, "b")
}

fn test_unwrap_or {
    n := int.parse("nope").unwrap_or(7)
    test.eq(n, 7)
    m := int.parse("42").unwrap_or(0)
    test.eq(m, 42)
}

fn test_is_ok_err {
    test.is_true(is_ok(Ok(1)))
    test.is_true(is_err(Err("x")))
    test.is_true(Ok(1).is_ok)
    test.is_true(Err("x").is_err)
}

fn test_error_constructors {
    e := Error.new("boom", "custom")
    test.eq(e.kind, "custom")
    test.eq(e.message, "boom")

    w := Error.wrap(e, "outer")
    test.contains(w.message, "outer")
    test.contains(w.message, "boom")

    rich := Error.with("pay", {"kind": "http", "code": 402})
    test.eq(rich.kind, "http")
    test.eq(rich.code, 402)
}

fn fail_read() -> Result {
    Err("nope", "fs")
}

fn call_fail() -> Result {
    fail_read()?
}

fn test_question_stamps_at {
    e := call_fail()
    test.err(e)
    // ? stamps Error.at with file:line
    test.is_true(e.err.at != null)
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := weft.RunTests(weft.TestOptions{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Fail > 0 {
		for _, c := range rep.Cases {
			if !c.OK && !c.Skipped {
				t.Errorf("%s: %s", c.Name, c.Err)
			}
		}
		t.Fatalf("fail=%d pass=%d", rep.Fail, rep.Pass)
	}
	if rep.Pass < 6 {
		t.Fatalf("pass=%d", rep.Pass)
	}
}
