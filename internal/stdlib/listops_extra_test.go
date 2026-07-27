package stdlib_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestListOpsSortZipUnique(t *testing.T) {
	src := `
fn main {
    say(sort([3, 1, 2]))
    say(reverse([1, 2, 3]))
    say(unique([1, 2, 2, 3, 1]))
    say(zip([1, 2], ["a", "b", "c"]))
    say(flatten([[1, 2], [3], 4]))
    say(enumerate(["x", "y"]))
    say(count([1, 2, 3, 4], fn(x) { x > 2 }))
    say(count([1, 1, 2], 1))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "lo.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	// sort
	if !strings.Contains(s, "1, 2, 3") {
		t.Fatal("sort", s)
	}
	// reverse
	if !strings.Contains(s, "3, 2, 1") {
		t.Fatal("reverse", s)
	}
	// unique keeps first-seen
	if !strings.Contains(s, "1, 2, 3") {
		t.Fatal("unique", s)
	}
	// zip length 2
	if !strings.Contains(s, "a") || !strings.Contains(s, "b") {
		t.Fatal("zip", s)
	}
	// flatten
	if !strings.Contains(s, "1, 2, 3, 4") {
		t.Fatal("flatten", s)
	}
	// count pred → 2; count value → 2
	if !strings.Contains(s, "2") {
		t.Fatal("count", s)
	}
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) < 2 || lines[len(lines)-1] != "2" || lines[len(lines)-2] != "2" {
		t.Fatalf("count lines: %q", s)
	}
}
