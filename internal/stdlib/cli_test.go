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

func runCLI(t *testing.T, src string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	argv := append([]string{"test.weft"}, args...)
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out, Args: argv})
	err := ctx.RunSource(context.Background(), "test.weft", src)
	return out.String(), err
}

func TestCLIParseFlags(t *testing.T) {
	src := `
fn main -> Result {
    p := cli.parse({
        "about": "t",
        "flags": {
            "env": {"short": "e", "default": "dev"},
            "verbose": {"short": "v", "bool": true},
        },
    })?
    println(p.env)
    println(p.verbose)
    println(p.args[0])
}
`
	out, err := runCLI(t, src, "--env", "prod", "-v", "deploy")
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "prod") || !strings.Contains(out, "true") || !strings.Contains(out, "deploy") {
		t.Fatal(out)
	}
}

func TestCLIHelp(t *testing.T) {
	src := `
fn main -> Result {
    p := cli.parse({
        "about": "my tool",
        "flags": {"x": {"help": "the x"}},
    })?
    if p.help {
        println(p.usage)
        cli.exit(0)
    }
}
`
	out, err := runCLI(t, src, "--help")
	if err != nil {
		if ee, ok := err.(weft.ExitError); ok && ee.Code == 0 {
			// ok
		} else {
			t.Fatal(err, out)
		}
	}
	if !strings.Contains(out, "my tool") || !strings.Contains(out, "--x") {
		t.Fatal(out)
	}
}

func TestLenPushRange(t *testing.T) {
	src := `
fn main {
    mut xs := []
    push(xs, 1)
    push(xs, 2)
    push(xs, 3)
    println(len(xs))
    println(slice(xs, 1, 3))
    println(concat([0], xs))
    println(range(2, 5))
}
`
	out, err := runCLI(t, src)
	if err != nil {
		t.Fatal(err, out)
	}
	// len=3, slice(1,3)=[2,3], concat, range(2,5)=[2,3,4]
	if !strings.Contains(out, "3\n") || !strings.Contains(out, "[2, 3]") || !strings.Contains(out, "[2, 3, 4]") {
		t.Fatal(out)
	}
}

func TestCSV(t *testing.T) {
	src := `
fn main -> Result {
    t := "name,n\nAda,1\nBob,2\n"
    d := csv.parse(t, {"header": true})?
    println(len(d.rows))
    println(d.rows[0].name)
    s := csv.stringify(d.rows, {"header": d.header})
    println(s)
}
`
	out, err := runCLI(t, src)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "2") || !strings.Contains(out, "Ada") || !strings.Contains(out, "name,n") {
		t.Fatal(out)
	}
}

func TestTimeISO(t *testing.T) {
	src := `
fn main {
    println(time.iso())
    println(time.format("date"))
}
`
	out, err := runCLI(t, src)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "T") && !strings.Contains(out, "-") {
		t.Fatal(out)
	}
}

func TestSHCapture(t *testing.T) {
	src := `
fn main -> Result {
    out := sh.capture("echo", ["hi-weft"])?
    println(str.trim(out))
    ok := sh.ok("true")?
    println(ok)
}
`
	out, err := runCLI(t, src)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "hi-weft") || !strings.Contains(out, "true") {
		t.Fatal(out)
	}
}

func TestFSLinesGlob(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	_ = os.WriteFile(p, []byte("one\ntwo\n"), 0o644)
	src := `
fn main -> Result {
    ls := fs.lines("` + p + `")?
    println(len(ls))
    g := fs.glob("` + filepath.Join(dir, "*.txt") + `")?
    println(len(g))
}
`
	out, err := runCLI(t, src)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "2") {
		t.Fatal(out)
	}
}

func TestCLIExitCode(t *testing.T) {
	src := `fn main { cli.exit(7) }`
	_, err := runCLI(t, src)
	ee, ok := err.(weft.ExitError)
	if !ok || ee.Code != 7 {
		t.Fatalf("got %v", err)
	}
}
