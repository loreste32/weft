package stdlib_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestHTTP_AllMethodsAndHelpers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/json" {
			_, _ = w.Write([]byte(`{"ok":true,"n":1}`))
			return
		}
		_, _ = fmt.Fprintf(w, `{"m":%q,"b":%q}`, r.Method, string(body))
	}))
	defer srv.Close()

	src := fmt.Sprintf(`
fn main -> Result {
    u := %q
    g := http.get(u + "/", {"timeout": "2s"})?
    say(g.status == 200)
    pj := http.get_json(u + "/json")?
    say(pj.ok == true)
    p := http.post(u + "/", "{\"a\":1}")?
    say(p.status == 200)
    pu := http.put(u + "/", "x")?
    say(pu.ok)
    pa := http.patch(u + "/", "y")?
    say(pa.ok)
    d := http.delete(u + "/")?
    say(d.ok)
    f := http.fetch({"url": u + "/", "method": "GET", "timeout": "2s"})?
    say(f.ok)
    pf := http.post_form(u + "/", {"k": "v"})?
    say(pf.status == 200)
    t := http.text(200, "hi")
    say(t.status == 200)
    j := http.json({"a": 1})
    say(j.status == 200)
    j2 := http.json(201, {"b": 2})
    say(j2.status == 201)
}
`, srv.URL)

	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, HTTPClient: srv.Client()})
	if err := ctx.RunSource(context.Background(), "h.weft", src); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if strings.Count(out.String(), "true") < 10 {
		t.Fatalf("%q", out.String())
	}
}
