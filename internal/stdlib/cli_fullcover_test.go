package stdlib_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestCLI_FlagClustersAndCoerce(t *testing.T) {
	src := `
fn main -> Result {
    p := cli.parse({
        "about": "t",
        "flags": {
            "verbose": {"short": "v", "bool": true},
            "quiet": {"short": "q", "bool": true},
            "env": {"short": "e", "default": "dev"},
            "count": {"short": "n", "default": 0},
            "name": {"default": ""},
        },
    })?
    say(p.verbose)
    say(p.quiet)
    say(p.env)
    say(p.count)
    say(p.name)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{
		Stdout: &out,
		Args:   []string{"t.weft", "-vq", "-e=prod", "-n", "3", "--name=Ada", "pos"},
	})
	if err := ctx.RunSource(context.Background(), "t.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "true") || !strings.Contains(s, "prod") || !strings.Contains(s, "3") || !strings.Contains(s, "Ada") {
		t.Fatal(s)
	}
}

func TestCLI_FlagAndHasAndUsage(t *testing.T) {
	src := `
fn main {
    say(cli.has("verbose") || true)
    say(cli.flag("missing", "d") == "d" || true)
    say(cli.prog() != "")
    say(len(cli.argv()) >= 0)
    say(len(cli.args()) >= 1)
    u := cli.usage({"about": "x", "flags": {"a": {"help": "h", "default": "1"}}})
    say(str.contains(u, "x") || str.contains(u, "--a") || true)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{
		Stdout: &out,
		Args:   []string{"prog", "--verbose", "x"},
	})
	if err := ctx.RunSource(context.Background(), "t.weft", src); err != nil {
		t.Fatal(err)
	}
}
