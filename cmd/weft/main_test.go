package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// weftBin builds and returns path to the weft binary for exec-based tests.
func weftBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "weft")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/weft")
	// Find project root by walking up from this test file
	wd, _ := os.Getwd()
	root := filepath.Dir(filepath.Dir(wd))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build weft: %v\n%s", err, out)
	}
	return bin
}

func TestVersion(t *testing.T) {
	bin := weftBin(t)
	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "weft") {
		t.Fatalf("version: %q", string(out))
	}
}

func TestHelp(t *testing.T) {
	bin := weftBin(t)
	out, _ := exec.Command(bin, "help").CombinedOutput()
	if !strings.Contains(string(out), "weft") {
		t.Fatal("help missing")
	}
	if !strings.Contains(string(out), "registry") {
		t.Fatal("help should mention registry")
	}
}

func TestRunHello(t *testing.T) {
	bin := weftBin(t)
	script := filepath.Join(t.TempDir(), "hello.weft")
	os.WriteFile(script, []byte("fn main { say(\"hello\") }\n"), 0644)

	out, err := exec.Command(bin, "run", script).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "hello") {
		t.Fatalf("output: %q", string(out))
	}
}

func TestRunError(t *testing.T) {
	bin := weftBin(t)
	script := filepath.Join(t.TempDir(), "bad.weft")
	os.WriteFile(script, []byte("fn main { 1 / 0 }\n"), 0644)

	_, err := exec.Command(bin, "run", script).CombinedOutput()
	if err == nil {
		t.Fatal("should exit non-zero")
	}
}

func TestCheck(t *testing.T) {
	bin := weftBin(t)
	script := filepath.Join(t.TempDir(), "ok.weft")
	os.WriteFile(script, []byte("fn main { say(1) }\n"), 0644)

	out, err := exec.Command(bin, "check", script).CombinedOutput()
	if err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
}

func TestCheckTypes(t *testing.T) {
	bin := weftBin(t)
	script := filepath.Join(t.TempDir(), "typed.weft")
	os.WriteFile(script, []byte("fn add(a: int, b: int) -> int { a + b }\nfn main { say(add(1, 2)) }\n"), 0644)

	out, err := exec.Command(bin, "check", script, "--types").CombinedOutput()
	if err != nil {
		t.Fatalf("check --types: %v\n%s", err, out)
	}
}

func TestFmt(t *testing.T) {
	bin := weftBin(t)
	script := filepath.Join(t.TempDir(), "ugly.weft")
	os.WriteFile(script, []byte("fn main{say(  1  )}\n"), 0644)

	out, err := exec.Command(bin, "fmt", script).CombinedOutput()
	if err != nil {
		t.Fatalf("fmt: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "formatted") {
		t.Fatalf("fmt output: %q", string(out))
	}
}

func TestFmtCheck(t *testing.T) {
	bin := weftBin(t)
	script := filepath.Join(t.TempDir(), "ugly.weft")
	os.WriteFile(script, []byte("fn main{say(  1  )}\n"), 0644)

	_, err := exec.Command(bin, "fmt", "--check", script).CombinedOutput()
	if err == nil {
		t.Fatal("fmt --check should exit 1 for unformatted")
	}
}

func TestFmtCheckClean(t *testing.T) {
	bin := weftBin(t)
	// Format a file first, then check it — should pass
	script := filepath.Join(t.TempDir(), "clean.weft")
	os.WriteFile(script, []byte("fn main { say(1) }\n"), 0644)
	exec.Command(bin, "fmt", script).Run() // normalize it first

	out, err := exec.Command(bin, "fmt", "--check", script).CombinedOutput()
	if err != nil {
		t.Fatalf("formatted file should pass --check: %v\n%s", err, out)
	}
}

func TestStdlib(t *testing.T) {
	bin := weftBin(t)
	out, err := exec.Command(bin, "stdlib").CombinedOutput()
	if err != nil {
		t.Fatalf("stdlib: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "json") {
		t.Fatal("stdlib should list json")
	}
}

func TestDoctor(t *testing.T) {
	bin := weftBin(t)
	out, err := exec.Command(bin, "doctor").CombinedOutput()
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "weft") {
		t.Fatal("doctor should show version")
	}
}

func TestUnknownCommand(t *testing.T) {
	bin := weftBin(t)
	_, err := exec.Command(bin, "nonexistent").CombinedOutput()
	if err == nil {
		t.Fatal("unknown command should exit non-zero")
	}
}

func TestInit(t *testing.T) {
	bin := weftBin(t)
	dir := t.TempDir()
	cmd := exec.Command(bin, "init", "testapp")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "weft.json")); err != nil {
		t.Fatal("missing weft.json")
	}
}

func TestNotebook(t *testing.T) {
	bin := weftBin(t)
	script := filepath.Join(t.TempDir(), "nb.weft")
	os.WriteFile(script, []byte("fn main { say(42) }\n"), 0644)
	outPath := filepath.Join(t.TempDir(), "nb.html")

	out, err := exec.Command(bin, "notebook", script, "-o", outPath).CombinedOutput()
	if err != nil {
		t.Fatalf("notebook: %v\n%s", err, out)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatal("missing html output")
	}
}

func TestTest(t *testing.T) {
	bin := weftBin(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "math_test.weft"), []byte(`
fn test_add {
    test.eq(1 + 1, 2)
}
`), 0644)

	out, err := exec.Command(bin, "test", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("test: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "passed") {
		t.Fatalf("test output: %q", string(out))
	}
}

func TestNewModule(t *testing.T) {
	bin := weftBin(t)
	dir := t.TempDir()
	out, err := exec.Command(bin, "new", "module", "mymod", "--force").CombinedOutput()
	_ = dir
	if err != nil {
		t.Fatalf("new module: %v\n%s", err, out)
	}
}
