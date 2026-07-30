package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunVersion(t *testing.T) {
	if code := run([]string{"version"}); code != 0 {
		t.Fatalf("version exit %d", code)
	}
}

func TestRunHelp(t *testing.T) {
	if code := run([]string{"help"}); code != 0 {
		t.Fatalf("help exit %d", code)
	}
}

func TestRunHelpShort(t *testing.T) {
	if code := run([]string{"-h"}); code != 0 {
		t.Fatal("help -h")
	}
}

func TestRunDoctor(t *testing.T) {
	if code := run([]string{"doctor"}); code != 0 {
		t.Fatalf("doctor exit %d", code)
	}
}

func TestRunStdlib(t *testing.T) {
	if code := run([]string{"stdlib"}); code != 0 {
		t.Fatal("stdlib")
	}
}

func TestRunStdlibPkg(t *testing.T) {
	if code := run([]string{"stdlib", "json"}); code != 0 {
		t.Fatal("stdlib json")
	}
}

func TestRunStdlibUnknownMember(t *testing.T) {
	if code := run([]string{"stdlib", "sysinfo.uptim"}); code == 0 {
		t.Fatal("misspelled stdlib member should exit non-zero")
	}
}

func TestStdlibMemberRequiresArguments(t *testing.T) {
	if code := stdlibMemberInfo("proc", "find"); code != 0 {
		t.Fatal("member help should not probe required arguments")
	}
}

func TestRunUnknown(t *testing.T) {
	if code := run([]string{"nonexistent_command"}); code == 0 {
		t.Fatal("unknown should exit non-zero")
	}
}

func TestRunScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.weft")
	os.WriteFile(path, []byte("fn main { say(1) }\n"), 0644)
	if code := run([]string{"run", path}); code != 0 {
		t.Fatalf("run exit %d", code)
	}
}

func TestRunScriptDirect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.weft")
	os.WriteFile(path, []byte("fn main { say(1) }\n"), 0644)
	// Direct file (no "run" command)
	if code := run([]string{path}); code != 0 {
		t.Fatalf("direct run exit %d", code)
	}
}

func TestRunScriptError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.weft")
	os.WriteFile(path, []byte("fn main { 1 / 0 }\n"), 0644)
	if code := run([]string{"run", path}); code == 0 {
		t.Fatal("error should exit non-zero")
	}
}

func TestRunCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.weft")
	os.WriteFile(path, []byte("fn main { say(1) }\n"), 0644)
	if code := run([]string{"check", path}); code != 0 {
		t.Fatal("check")
	}
}

func TestRunCheckTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "typed.weft")
	os.WriteFile(path, []byte("fn add(a: int, b: int) -> int { a + b }\nfn main { say(add(1, 2)) }\n"), 0644)
	if code := run([]string{"check", path, "--types"}); code != 0 {
		t.Fatal("check --types")
	}
}

func TestRunFmt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ugly.weft")
	os.WriteFile(path, []byte("fn main{say(  1  )}\n"), 0644)
	if code := run([]string{"fmt", path}); code != 0 {
		t.Fatal("fmt")
	}
}

func TestRunFmtCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ugly.weft")
	os.WriteFile(path, []byte("fn main{say(  1  )}\n"), 0644)
	if code := run([]string{"fmt", "--check", path}); code == 0 {
		t.Fatal("fmt --check should exit 1 for unformatted")
	}
}

func TestRunTest(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "math_test.weft"), []byte("fn test_add { test.eq(1+1, 2) }\n"), 0644)
	if code := run([]string{"test", dir}); code != 0 {
		t.Fatal("test")
	}
}

func TestRunTestCoverage(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "math_test.weft"), []byte("fn test_add { test.eq(1+1, 2) }\n"), 0644)
	if code := run([]string{"test", "--coverage", dir}); code != 0 {
		t.Fatal("test --coverage")
	}
}

func TestRunInit(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	if code := run([]string{"init", "testapp"}); code != 0 {
		t.Fatal("init")
	}
}

func TestRunNewModule(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	if code := run([]string{"new", "module", "mymod"}); code != 0 {
		t.Fatal("new module")
	}
}

func TestRunNewApp(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	if code := run([]string{"new", "app", "myapp"}); code != 0 {
		t.Fatal("new app")
	}
}

func TestRunNewCLI(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	if code := run([]string{"new", "cli", "mytool"}); code != 0 {
		t.Fatal("new cli")
	}
}

func TestRunNewNoArgs(t *testing.T) {
	if code := run([]string{"new"}); code == 0 {
		t.Fatal("new with no args should fail")
	}
}

func TestRunNewUnknownKind(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	if code := run([]string{"new", "unknown", "x"}); code == 0 {
		t.Fatal("new unknown kind")
	}
}

func TestRunNotebook(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "nb.weft")
	os.WriteFile(script, []byte("fn main { say(42) }\n"), 0644)
	out := filepath.Join(dir, "nb.html")
	if code := run([]string{"notebook", script, "-o", out}); code != 0 {
		t.Fatal("notebook")
	}
}

func TestRunREPL(t *testing.T) {
	// Feed stdin to avoid blocking
	old := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString(":quit\n")
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = old }()

	if code := run(nil); code != 0 {
		t.Fatalf("repl exit %d", code)
	}
}

func TestRunRunNoFile(t *testing.T) {
	if code := run([]string{"run"}); code == 0 {
		t.Fatal("run with no file")
	}
}

func TestRunCheckNoFile(t *testing.T) {
	if code := run([]string{"check"}); code == 0 {
		t.Fatal("check with no args")
	}
}

func TestRunPrompt(t *testing.T) {
	if code := run([]string{"prompt"}); code != 0 {
		t.Fatal("prompt")
	}
}

func TestRunList(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	run([]string{"init", "testapp"})
	if code := run([]string{"list"}); code != 0 {
		t.Fatal("list")
	}
}

func TestRunRegistryKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if code := run([]string{"registry", "keys"}); code != 0 {
		t.Fatal("registry keys")
	}
}

func TestRunRegistryKeygen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if code := run([]string{"registry", "keygen", "test"}); code != 0 {
		t.Fatal("registry keygen")
	}
}

func TestRunRegistryUnknown(t *testing.T) {
	if code := run([]string{"registry", "badcmd"}); code == 0 {
		t.Fatal("registry unknown")
	}
}

func TestRunPublishNoManifest(t *testing.T) {
	dir := t.TempDir()
	if code := run([]string{"publish", dir}); code == 0 {
		t.Fatal("publish without manifest")
	}
}

func TestRunModNoArgs(t *testing.T) {
	if code := run([]string{"mod"}); code == 0 {
		t.Fatal("mod no args")
	}
}

func TestRunGetNoArgs(t *testing.T) {
	if code := run([]string{"get"}); code == 0 {
		t.Fatal("get no args")
	}
}
