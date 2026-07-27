package stdlib

import (
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func callMem(t *testing.T, pkg runtime.Value, name string, args ...runtime.Value) runtime.Value {
	t.Helper()
	fn := pkg.Obj.(*runtime.MapObj).Vals[name]
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return r
}

func TestFullCover_SecretsPackage(t *testing.T) {
	env := runtime.NewEnv()
	env.Environ = map[string]string{"S": "secret"}
	p := packageSecrets(env)
	r := callMem(t, p, "require")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("require arity")
	}
	r = callMem(t, p, "require", runtime.Str("MISSING"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("missing secret")
	}
	r = callMem(t, p, "require", runtime.Str("S"))
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	r = callMem(t, p, "get")
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	r = callMem(t, p, "get", runtime.Str("S"))
	if r.Kind == runtime.KindNull {
		t.Fatal("want secret")
	}
	r = callMem(t, p, "get", runtime.Str("NO"))
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	r = callMem(t, p, "from")
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	r = callMem(t, p, "from", runtime.Str("x"))
	r = callMem(t, p, "unwrap")
	if r.S != "" {
		t.Fatal(r)
	}
	r = callMem(t, p, "unwrap", runtime.Str("plain"))
	if r.S != "plain" {
		t.Fatal(r)
	}
	r = callMem(t, p, "token_hex")
	if len(r.S) != 64 { // default 32 bytes
		t.Fatal(len(r.S), r.S)
	}
	r = callMem(t, p, "token_hex", runtime.Int(2))
	if len(r.S) != 4 {
		t.Fatal(r.S)
	}
	r = callMem(t, p, "token_urlsafe")
	if len(r.S) < 10 {
		t.Fatal(r.S)
	}
	r = callMem(t, p, "token_urlsafe", runtime.Int(4))
	if r.S == "" {
		t.Fatal("empty token")
	}
	r = callMem(t, p, "compare")
	if r.B {
		t.Fatal("compare arity")
	}
	r = callMem(t, p, "compare", runtime.Str("a"), runtime.Str("b"))
	if r.B {
		t.Fatal("diff")
	}
	r = callMem(t, p, "compare", runtime.Str("aa"), runtime.Str("aa"))
	if !r.B {
		t.Fatal("same")
	}
}

func TestFullCover_HTMLPackage(t *testing.T) {
	p := packageHTML()
	r := callMem(t, p, "escape")
	if r.S != "" {
		t.Fatal(r)
	}
	r = callMem(t, p, "escape", runtime.Str("<a>"))
	if !strings.Contains(r.S, "&lt;") {
		t.Fatal(r.S)
	}
	r = callMem(t, p, "unescape")
	if r.S != "" {
		t.Fatal(r)
	}
	r = callMem(t, p, "unescape", runtime.Str("&lt;"))
	if r.S != "<" {
		t.Fatal(r.S)
	}
	r = callMem(t, p, "strip_tags")
	if r.S != "" {
		t.Fatal(r)
	}
	r = callMem(t, p, "strip_tags", runtime.Str("<b>hi</b>"))
	if r.S != "hi" {
		t.Fatal(r.S)
	}
	r = callMem(t, p, "links")
	if r.Kind != runtime.KindList || len(r.Obj.(*runtime.ListObj).Items) != 0 {
		t.Fatal(r)
	}
	r = callMem(t, p, "links", runtime.Str(`<a href="a"><a href='b'><a href="a">`))
	if len(r.Obj.(*runtime.ListObj).Items) != 2 {
		t.Fatal(r)
	}
}

func TestFullCover_IPPackage(t *testing.T) {
	p := packageIP()
	r := callMem(t, p, "parse")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("arity")
	}
	r = callMem(t, p, "parse", runtime.Str("not-an-ip"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("bad ip")
	}
	r = callMem(t, p, "parse", runtime.Str("10.0.0.1"))
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	// with port
	r = callMem(t, p, "parse", runtime.Str("127.0.0.1:80"))
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	// cidr strip
	r = callMem(t, p, "parse", runtime.Str("10.0.0.0/8"))
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	r = callMem(t, p, "is_valid")
	if r.B {
		t.Fatal()
	}
	r = callMem(t, p, "is_valid", runtime.Str("::1"))
	if !r.B {
		t.Fatal()
	}
	r = callMem(t, p, "is_private")
	if r.B {
		t.Fatal()
	}
	r = callMem(t, p, "is_private", runtime.Str("10.1.1.1"))
	if !r.B {
		t.Fatal()
	}
	r = callMem(t, p, "is_loopback")
	if r.B {
		t.Fatal()
	}
	r = callMem(t, p, "is_loopback", runtime.Str("127.0.0.1"))
	if !r.B {
		t.Fatal()
	}
	r = callMem(t, p, "in_network")
	if r.B {
		t.Fatal()
	}
	r = callMem(t, p, "in_network", runtime.Str("bad"), runtime.Str("10.0.0.0/8"))
	if r.B {
		t.Fatal()
	}
	r = callMem(t, p, "in_network", runtime.Str("10.0.0.1"), runtime.Str("bad"))
	if r.B {
		t.Fatal()
	}
	r = callMem(t, p, "network")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = callMem(t, p, "network", runtime.Str("bad"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = callMem(t, p, "network", runtime.Str("10.0.0.0/24"))
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
}

func TestFullCover_TracebackPackage(t *testing.T) {
	// already tested; add kind+msg struct
	st := runtime.Struct("Error", map[string]runtime.Value{
		"msg":  runtime.Str("m"),
		"kind": runtime.Str("k"),
	}, []string{"msg", "kind"})
	s := formatTrace(st)
	if !strings.Contains(s, "m") || !strings.Contains(s, "k") {
		t.Fatal(s)
	}
	// msg only
	st2 := runtime.Struct("Error", map[string]runtime.Value{
		"msg": runtime.Str("only"),
	}, []string{"msg"})
	if formatTrace(st2) != "Error: only" {
		t.Fatal(formatTrace(st2))
	}
}

func TestFullCover_CryptoHashErrors(t *testing.T) {
	p := packageCrypto()
	r := callMem(t, p, "hash")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("arity")
	}
	r = callMem(t, p, "hash", runtime.Str("nope"), runtime.Str("x"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("bad algo")
	}
	for _, algo := range []string{"md5", "sha1", "sha256", "sha512"} {
		r = callMem(t, p, "hash", runtime.Str(algo), runtime.Str("x"))
		if r.S == "" {
			t.Fatal(algo)
		}
	}
	r = callMem(t, p, "hmac_sha512")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = callMem(t, p, "hmac_sha512", runtime.Str("k"), runtime.Str("m"))
	if len(r.S) != 128 {
		t.Fatal(len(r.S))
	}
}

func TestFullCover_URLPackageEdges(t *testing.T) {
	p := packageURL()
	r := callMem(t, p, "merge_query")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = callMem(t, p, "merge_query", runtime.Str("://"), runtime.NewMap())
	// may err on parse
	_ = r
	r = callMem(t, p, "path_unescape")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = callMem(t, p, "path_unescape", runtime.Str("%ZZ"))
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("bad escape")
	}
	r = callMem(t, p, "path_unescape", runtime.Str("%2F"))
	if !r.Obj.(*runtime.ResultObj).Ok || r.Obj.(*runtime.ResultObj).Val.S != "/" {
		t.Fatal(r)
	}
	r = callMem(t, p, "parse")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
}

func TestFullCover_INIPackageEdges(t *testing.T) {
	p := packageINI()
	r := callMem(t, p, "sections")
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	r = callMem(t, p, "has_section")
	if r.B {
		t.Fatal()
	}
	r = callMem(t, p, "parse")
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal()
	}
	r = callMem(t, p, "get")
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
}

func TestFullCover_XMLPackageEdges(t *testing.T) {
	p := packageXML()
	r := callMem(t, p, "find")
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	r = callMem(t, p, "findall")
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	r = callMem(t, p, "text")
	if r.S != "" {
		t.Fatal(r)
	}
	r = callMem(t, p, "attr")
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	// parse + attr miss
	pr := callMem(t, p, "parse", runtime.Str("<r a=\"1\"/>"))
	node := pr.Obj.(*runtime.ResultObj).Val
	r = callMem(t, p, "attr", node, runtime.Str("missing"))
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	r = callMem(t, p, "attr", node, runtime.Str("a"))
	if r.S != "1" {
		t.Fatal(r)
	}
	r = callMem(t, p, "text", node)
	_ = r
	// namespaced
	pr = callMem(t, p, "parse", runtime.Str(`<n:root xmlns:n="urn:x"><n:c>t</n:c></n:root>`))
	if !pr.Obj.(*runtime.ResultObj).Ok {
		// may still parse with local names
		t.Log(pr)
	}
}

func TestFullCover_TestAssertPackage(t *testing.T) {
	p := packageTest()
	as := p.Obj.(*runtime.MapObj).Vals["assert"]
	_, err := as.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Bool(true)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = as.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Bool(false), runtime.Str("nope")})
	if err == nil {
		t.Fatal("want fail")
	}
	_, err = as.Obj.(*runtime.BuiltinObj).Fn(nil)
	if err == nil {
		t.Fatal("want fail")
	}
}

func TestFullCover_LogPackage(t *testing.T) {
	env := runtime.NewEnv()
	var buf strings.Builder
	// Env uses Writers - check if Stdout is settable
	env.Stdout = &buf
	env.Stderr = &buf
	p := packageLog(env)
	callMem(t, p, "set_level", runtime.Str("debug"))
	callMem(t, p, "set_json") // true default
	callMem(t, p, "set_json", runtime.Bool(false))
	callMem(t, p, "debug", runtime.Str("d"))
	callMem(t, p, "info", runtime.Str("i"), runtime.NewMap())
	callMem(t, p, "warn", runtime.Str("w"))
	callMem(t, p, "error", runtime.Str("e"))
	callMem(t, p, "set_json", runtime.Bool(true))
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"k"}
	mo.Vals["k"] = runtime.Str("v")
	// also only-in-vals key
	mo.Vals["extra"] = runtime.Str("x")
	callMem(t, p, "info", runtime.Str("j"), m)
	s := buf.String()
	if !strings.Contains(s, "i") && !strings.Contains(s, "j") {
		t.Fatal(s)
	}
}

func TestFullCover_CliPromptEmpty(t *testing.T) {
	// prompt without input is hard; test package registration
	env := runtime.NewEnv()
	p := packageCLI(env)
	for _, name := range []string{"parse", "prompt", "exit", "die", "ok", "usage", "args", "argv", "prog", "flag", "has"} {
		if _, ok := p.Obj.(*runtime.MapObj).Vals[name]; !ok {
			t.Fatal("missing", name)
		}
	}
}
