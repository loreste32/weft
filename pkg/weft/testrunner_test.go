package weft_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestRunTestsPassAndFail(t *testing.T) {
	dir := t.TempDir()
	pass := filepath.Join(dir, "ok_test.weft")
	if err := os.WriteFile(pass, []byte(`
fn test_one {
    test.eq(1 + 1, 2)
    test.is_true(true)
}
fn test_two {
    test.contains("abc", "b")
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := weft.RunTests(weft.TestOptions{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pass != 2 || rep.Fail != 0 {
		t.Fatalf("pass=%d fail=%d %+v", rep.Pass, rep.Fail, rep.Cases)
	}

	fail := filepath.Join(dir, "bad_test.weft")
	if err := os.WriteFile(fail, []byte(`
fn test_bad {
    test.eq(1, 2)
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep2, err := weft.RunTests(weft.TestOptions{Paths: []string{fail}})
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Fail != 1 {
		t.Fatalf("want 1 fail, got %+v", rep2)
	}
	if !strings.Contains(rep2.Cases[0].Err, "test.eq") {
		t.Fatalf("err %q", rep2.Cases[0].Err)
	}
}

func TestRunTestsSkip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skip_test.weft")
	if err := os.WriteFile(path, []byte(`
fn test_later {
    test.skip("not ready")
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := weft.RunTests(weft.TestOptions{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skip != 1 || rep.Fail != 0 {
		t.Fatalf("%+v", rep)
	}
}

func TestRunTestsFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f_test.weft")
	if err := os.WriteFile(path, []byte(`
fn test_alpha { test.eq(1, 1) }
fn test_beta { test.eq(1, 1) }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := weft.RunTests(weft.TestOptions{Paths: []string{path}, Filter: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 1 || rep.Pass != 1 {
		t.Fatalf("%+v", rep)
	}
}
