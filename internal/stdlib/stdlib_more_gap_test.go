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

func TestMathStatsJSONPrettyEnvMimeHashFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "x.bin")
	_ = os.WriteFile(fp, []byte("hello"), 0o644)
	fpw := filepath.ToSlash(fp)

	code := `
fn main -> Result {
    say(math.sum([1, 2, 3]))
    say(math.mean([2, 4, 6]))
    say(math.median([1, 3, 2]))
    say(math.stdev([2, 4, 4, 4, 5, 5, 7, 9]) > 0)
    j := json.pretty({"a": 1, "b": [2]})
    say(str.contains(j, "\n"))
    j2 := json.stringify({"x": 1}, 2)
    say(str.contains(j2, "\n"))
    env.set("WEFT_TEST_X", "42")
    say(env.get("WEFT_TEST_X"))
    say(env.pid() > 0)
    say(len(env.hostname()) >= 0)
    say(mime.by_ext("a.json"))
    say(mime.by_ext(".weft"))
    h := crypto.file_hash("` + fpw + `", "sha256")?
    say(len(h) == 64)
    Ok(1)
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "more.weft", code); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	for _, need := range []string{"6", "4", "2", "true", "42", "application/json", "text/x-weft"} {
		if !strings.Contains(s, need) {
			t.Fatalf("missing %q in %q", need, s)
		}
	}
}
