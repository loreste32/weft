package stdlib_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestJSONGetDefaultAndEnvDefault(t *testing.T) {
	src := `
fn main -> Result {
    doc := {"user": {"name": "ada"}, "n": 1}
    say(json.get(doc, "user.name")?)
    say(json.get(doc, "missing", "fallback")?)
    say(json.get(doc, "missing", 0)?)
    say(env.get("NO_SUCH_WEFT_ENV_XYZ_999", "def"))
    say(env.get("NO_SUCH_WEFT_ENV_XYZ_999") == null)
    say(str.starts_with("hello", "he"))
    say(str.ends_with("hello", "lo"))
    say(str.has_prefix("hello", "he"))
}
`
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "pain.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	for _, need := range []string{"ada", "fallback", "0", "def", "true"} {
		if !strings.Contains(s, need) {
			t.Fatalf("missing %q in %q", need, s)
		}
	}
}

func TestHTTPGetJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"city":"Paris","temp":21}`))
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main -> Result {
    d := http.get_json(%q)?
    say(d.city)
    say(d.temp)
}
`, srv.URL)
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "gj.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "Paris") || !strings.Contains(s, "21") {
		t.Fatal(s)
	}
}
