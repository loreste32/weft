package stdlib_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loreste/weft/pkg/weft"
)

func runSrc(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "t.weft", src); err != nil {
		t.Fatalf("%v\nout=%s", err, out.String())
	}
	return out.String()
}

func TestDecimalXMLEmailPickleTimezones(t *testing.T) {
	out := runSrc(t, `
fn main -> Result {
    a := decimal.new("10.50")?
    b := decimal.add(a, "0.25")?
    say(decimal.string(b, 2)?)
    say(decimal.eq(b, "10.75"))

    doc := xml.parse("<root id=\"1\"><item>hi</item></root>")?
    say(doc.name)
    say(doc.children[0].text)
    s := xml.stringify(doc)
    say(s)

    raw := email.build({
        "from": "a@x.com",
        "to": "b@y.com",
        "subject": "hello",
        "body": "world",
    })
    m := email.parse(raw)?
    say(m.subject)
    say(m.body)

    u := time.parse("2020-06-01T12:00:00Z")?
    loc := time.format_in(u, "America/New_York", "datetime")?
    say(loc)
    off := time.offset("UTC")?
    say(off == 0)
}
`)
	if !strings.Contains(out, "10.75") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "root") || !strings.Contains(out, "hi") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "2020-06-01") {
		t.Fatal(out)
	}
}

func TestPickleGuardAndRoundTrip(t *testing.T) {
	// loads blocked without flag
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	err := ctx.RunSource(context.Background(), "t.weft", `
fn main -> Result {
    s := pickle.dumps({"a": 1, "b": [2, 3]})?
    pickle.loads(s)?
}
`)
	if err == nil || !strings.Contains(err.Error(), "ALLOW_PICKLE") {
		t.Fatalf("want pickle guard, got %v", err)
	}

	t.Setenv("WEFT_ALLOW_PICKLE", "1")
	out.Reset()
	ctx2 := weft.New(weft.Options{Stdout: &out})
	if err := ctx2.RunSource(context.Background(), "t.weft", `
fn main -> Result {
    s := pickle.dumps({"a": 1, "b": [2, 3]})?
    v := pickle.loads(s)?
    say(v.a)
    say(v.b[1])
}
`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1") || !strings.Contains(out.String(), "3") {
		t.Fatal(out.String())
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "x.gob")
	pathW := filepath.ToSlash(path)
	out.Reset()
	ctx3 := weft.New(weft.Options{Stdout: &out})
	if err := ctx3.RunSource(context.Background(), "t.weft", `
fn main -> Result {
    pickle.dump("`+pathW+`", [1, 2, "z"])?
    v := pickle.load("`+pathW+`")?
    say(v[2])
}
`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "z") {
		t.Fatal(out.String())
	}
}

func TestSocketTCP(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "1") // not used for loopback; loopback always ok
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 64)
		n, _ := c.Read(buf)
		_, _ = c.Write([]byte("echo:" + string(buf[:n])))
	}()
	// give listener a moment
	time.Sleep(20 * time.Millisecond)

	out := runSrc(t, `
fn main -> Result {
    c := socket.dial("tcp", "`+addr+`")?
    c.write("ping")?
    r := c.read(64)?
    say(r)
    c.close()?
}
`)
	if !strings.Contains(out, "echo:ping") {
		t.Fatal(out)
	}
	_ = ln.Close()
	_ = os.Getenv
}
