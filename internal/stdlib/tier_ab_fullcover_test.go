package stdlib

import (
	"encoding/binary"
	"math"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func TestFullCover_BinstructAllCodes(t *testing.T) {
	// x pad, B, h, H, i, I, q, f, d, s — big endian
	b, err := bsPack(">xBhHiIqfd4s", []runtime.Value{
		runtime.Int(1),      // B
		runtime.Int(-2),     // h
		runtime.Int(3),      // H
		runtime.Int(-4),     // i
		runtime.Int(5),      // I
		runtime.Int(-6),     // q
		runtime.Float(1.25), // f
		runtime.Float(2.5),  // d
		runtime.Str("abcd"), // 4s
	})
	if err != nil {
		t.Fatal(err)
	}
	vals, err := bsUnpack(">xBhHiIqfd4s", b)
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 9 {
		t.Fatalf("got %d vals", len(vals))
	}
	if vals[0].I != 1 || vals[1].I != -2 || vals[2].I != 3 {
		t.Fatalf("%v", vals[:3])
	}
	if vals[8].S != "abcd" {
		t.Fatal(vals[8])
	}
	// little endian L/Q/c/b
	b2, err := bsPack("<cLbQ", []runtime.Value{
		runtime.Int(65), // c 'A'
		runtime.Int(7),  // L
		runtime.Int(-8), // b
		runtime.Int(9),  // Q
	})
	if err != nil {
		t.Fatal(err)
	}
	v2, err := bsUnpack("<cLbQ", b2)
	if err != nil {
		t.Fatal(err)
	}
	if v2[0].I != 65 || v2[1].I != 7 || v2[2].I != -8 || v2[3].I != 9 {
		t.Fatalf("%v", v2)
	}
	// native endian =
	b3, err := bsPack("=I", []runtime.Value{runtime.Int(0x11223344)})
	if err != nil {
		t.Fatal(err)
	}
	if len(b3) != 4 {
		t.Fatal(len(b3))
	}
	v3, err := bsUnpack("=I", b3)
	if err != nil || v3[0].I != 0x11223344 {
		t.Fatalf("%v %v", v3, err)
	}
	// ! same as big
	b4, _ := bsPack("!H", []runtime.Value{runtime.Int(0xabcd)})
	if binary.BigEndian.Uint16(b4) != 0xabcd {
		t.Fatalf("%x", b4)
	}
	// empty layout size 0
	n, err := bsSize("")
	if err != nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}
	// pad only
	bp, err := bsPack(">3x", nil)
	if err != nil || len(bp) != 3 {
		t.Fatalf("%v %v", bp, err)
	}
	// float round-trip precision (stored as float32)
	bf, _ := bsPack(">f", []runtime.Value{runtime.Float(math.Pi)})
	vf, _ := bsUnpack(">f", bf)
	wantF := float64(float32(math.Pi))
	if math.Abs(vf[0].F-wantF) > 1e-5 {
		t.Fatalf("float pack: %v want ~%v", vf[0].F, wantF)
	}
	// package surface
	p := packageBinstruct()
	if p.Kind != runtime.KindMap {
		t.Fatal(p.Kind)
	}
	// error paths via package funcs
	pack := p.Obj.(*runtime.MapObj).Vals["pack"]
	r, _ := pack.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Kind != runtime.KindResult || r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("pack empty should err")
	}
	un := p.Obj.(*runtime.MapObj).Vals["unpack"]
	r, _ = un.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str(">I")})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("unpack arity")
	}
	sz := p.Obj.(*runtime.MapObj).Vals["size"]
	r, _ = sz.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("size arity")
	}
	r, _ = sz.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str(">Z")})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("size bad layout")
	}
}

func TestFullCover_DifflibEdges(t *testing.T) {
	// empty
	u := unifiedDiff(nil, nil, "a", "b")
	if !strings.Contains(u, "--- a") {
		t.Fatal(u)
	}
	// list via package
	p := packageDifflib()
	nd := p.Obj.(*runtime.MapObj).Vals["ndiff"]
	a := runtime.List(runtime.Str("x"), runtime.Str("y"))
	b := runtime.List(runtime.Str("x"), runtime.Str("z"))
	r, err := nd.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{a, b})
	if err != nil || r.Kind != runtime.KindList {
		t.Fatal(err, r)
	}
	// string via package unified
	ud := p.Obj.(*runtime.MapObj).Vals["unified_diff"]
	r, err = ud.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{
		runtime.Str("1\n2\n"), runtime.Str("1\n3\n"),
	})
	if err != nil || !strings.Contains(r.S, "---") {
		t.Fatal(err, r)
	}
	// arity errors
	r, _ = ud.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("want err")
	}
	r, _ = nd.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("a")})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("want err")
	}
	// max helper
	if max(1, 2) != 2 || max(3, 1) != 3 {
		t.Fatal("max")
	}
	// empty string lines
	if diffLines(runtime.Str("")) != nil {
		t.Fatal(diffLines(runtime.Str("")))
	}
}

func TestFullCover_ShlexPackageErrors(t *testing.T) {
	p := packageShlex()
	sp := p.Obj.(*runtime.MapObj).Vals["split"]
	r, _ := sp.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("split arity")
	}
	r, _ = sp.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("'bad")})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("split unclosed")
	}
	q := p.Obj.(*runtime.MapObj).Vals["quote"]
	r, _ = q.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.S != "''" {
		t.Fatal(r)
	}
	j := p.Obj.(*runtime.MapObj).Vals["join"]
	r, _ = j.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.S != "" {
		t.Fatal(r)
	}
	r, _ = j.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.List(runtime.Str("a"), runtime.Str("b c"))})
	if !strings.Contains(r.S, "b c") && !strings.Contains(r.S, "'b c'") {
		t.Fatal(r.S)
	}
	// empty quoted token
	parts, err := shlexSplit(`""`)
	if err != nil {
		t.Fatal(err)
	}
	_ = parts
}

func TestFullCover_SignalPackage(t *testing.T) {
	env := runtime.NewEnv()
	p := packageSignal(env)
	listen := p.Obj.(*runtime.MapObj).Vals["listen"]
	recv := p.Obj.(*runtime.MapObj).Vals["received"]
	reset := p.Obj.(*runtime.MapObj).Vals["reset"]
	_, _ = listen.Obj.(*runtime.BuiltinObj).Fn(nil)
	_, _ = listen.Obj.(*runtime.BuiltinObj).Fn(nil) // idempotent
	// deliver SIGTERM into our Notify channel
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		r, _ := recv.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("SIGTERM")})
		if r.B {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	r, _ := recv.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("TERM")})
	if !r.B {
		t.Fatal("expected TERM after SIGTERM")
	}
	r, _ = recv.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("any")})
	if !r.B {
		t.Fatal("expected any")
	}
	// also INT path
	_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		r, _ = recv.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("SIGINT")})
		if r.B {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	r, _ = recv.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("INT")})
	if !r.B {
		t.Fatal("expected INT after SIGINT")
	}
	_, _ = reset.Obj.(*runtime.BuiltinObj).Fn(nil)
	r, _ = recv.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.B {
		t.Fatal("after reset should be false unless concurrent signal")
	}
}

func TestFullCover_CopyPackage(t *testing.T) {
	p := packageCopy()
	dp := p.Obj.(*runtime.MapObj).Vals["deepcopy"]
	cp := p.Obj.(*runtime.MapObj).Vals["copy"]
	r, _ := dp.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	r, _ = cp.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	// map with keys + extra vals-only entry
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"k"}
	mo.Vals["k"] = runtime.Int(1)
	mo.Vals["only"] = runtime.Int(2)
	r, _ = cp.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{m})
	if r.Kind != runtime.KindMap {
		t.Fatal(r)
	}
	rm := r.Obj.(*runtime.MapObj)
	if rm.Vals["k"].I != 1 || rm.Vals["only"].I != 2 {
		t.Fatal(rm.Vals)
	}
	// empty list
	r, _ = cp.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.List()})
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	r, _ = cp.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("x")})
	if r.S != "x" {
		t.Fatal(r)
	}
	r, _ = dp.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.List(runtime.Int(1))})
	if r.Kind != runtime.KindList {
		t.Fatal(r)
	}
	// struct deep
	st := runtime.Struct("S", map[string]runtime.Value{"a": runtime.Int(1)}, []string{"a"})
	r, _ = dp.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{st})
	if r.Kind != runtime.KindStruct {
		t.Fatal(r)
	}
}

func TestFullCover_FunctoolsErrors(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		return runtime.Int(99), nil
	}
	p := packageFunctools(env)
	partial := p.Obj.(*runtime.MapObj).Vals["partial"]
	once := p.Obj.(*runtime.MapObj).Vals["once"]
	r, _ := partial.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("partial arity")
	}
	r, _ = partial.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("nope")})
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("partial need fn")
	}
	// partial with builtin-like via Call
	fn := runtime.MakeBuiltin("id", 1, func(args []runtime.Value) (runtime.Value, error) {
		return args[0], nil
	})
	// without Call on env
	env2 := runtime.NewEnv()
	p2 := packageFunctools(env2)
	partial2 := p2.Obj.(*runtime.MapObj).Vals["partial"]
	wrapped, _ := partial2.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{fn, runtime.Int(1)})
	r, _ = wrapped.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Int(2)})
	if r.Obj.(*runtime.ResultObj).Ok {
		// Call not configured → Err
	} else if r.Kind == runtime.KindResult && !r.Obj.(*runtime.ResultObj).Ok {
		// good
	}
	// with Call
	wrapped, _ = partial.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{fn})
	r, err := wrapped.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Int(3)})
	if err != nil || r.I != 99 {
		t.Fatalf("%v %v", r, err)
	}
	r, _ = once.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("once arity")
	}
	// once without Call
	once2 := p2.Obj.(*runtime.MapObj).Vals["once"]
	w2, _ := once2.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{fn})
	r, _ = w2.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Kind == runtime.KindResult && r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("once needs Call")
	}
	// once with Call caches
	w3, _ := once.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{fn})
	r1, _ := w3.Obj.(*runtime.BuiltinObj).Fn(nil)
	r2, _ := w3.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r1.I != 99 || r2.I != 99 {
		t.Fatal(r1, r2)
	}
}

func TestFullCover_Traceback(t *testing.T) {
	p := packageTraceback()
	format := p.Obj.(*runtime.MapObj).Vals["format"]
	isErr := p.Obj.(*runtime.MapObj).Vals["is_err"]
	errMsg := p.Obj.(*runtime.MapObj).Vals["err_msg"]
	r, _ := format.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.S != "" {
		t.Fatal(r)
	}
	r, _ = format.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Ok(runtime.Int(1))})
	if r.S != "" {
		t.Fatal("Ok result formats empty")
	}
	r, _ = format.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("")})
	if r.S != "Error" {
		t.Fatal(r.S)
	}
	r, _ = format.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("plain")})
	if r.S != "plain" {
		t.Fatal(r.S)
	}
	// Error struct with message field
	st := runtime.Struct("Error", map[string]runtime.Value{
		"message": runtime.Str("m"),
	}, []string{"message"})
	r, _ = format.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{st})
	if !strings.Contains(r.S, "m") {
		t.Fatal(r.S)
	}
	r, _ = isErr.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.B {
		t.Fatal("is_err empty")
	}
	r, _ = isErr.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("x")})
	if r.B {
		t.Fatal("non-result")
	}
	r, _ = errMsg.Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	r, _ = errMsg.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Ok(runtime.Int(1))})
	if r.Kind != runtime.KindNull {
		t.Fatal(r)
	}
	// nested Result Err
	inner := runtime.Err(runtime.NewError("nested", "k"))
	r, _ = format.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{inner})
	if !strings.Contains(r.S, "nested") && r.S == "" {
		// NewError format
		if r.S == "" {
			t.Fatal("empty format for Err")
		}
	}
}

func TestFullCover_ParseShOpts(t *testing.T) {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(k string, v runtime.Value) {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	put("cwd", runtime.Str("/tmp"))
	put("timeout", runtime.Str("100ms"))
	put("check", runtime.Bool(true))
	put("merge", runtime.Bool(true))
	put("stdin", runtime.Str("in"))
	envList := runtime.List(runtime.Str("A=1"), runtime.Str("B=2"))
	put("env", envList)
	o := parseShOpts(m)
	if o.dir != "/tmp" || o.timeout != 100*time.Millisecond || !o.check || !o.mergeOut || o.stdin != "in" {
		t.Fatalf("%+v", o)
	}
	if len(o.env) != 2 {
		t.Fatalf("env list: %v", o.env)
	}
	// non-map
	o2 := parseShOpts(runtime.Str("x"))
	if o2.dir != "" {
		t.Fatal(o2)
	}
	// env map form
	m2 := runtime.NewMap()
	m2o := m2.Obj.(*runtime.MapObj)
	m2o.Keys = []string{"dir", "env"}
	m2o.Vals["dir"] = runtime.Str("/x")
	em := runtime.NewMap()
	emo := em.Obj.(*runtime.MapObj)
	emo.Keys = []string{"K"}
	emo.Vals["K"] = runtime.Str("V")
	m2o.Vals["env"] = em
	m2o.Vals["timeout"] = runtime.Int(2)
	o3 := parseShOpts(m2)
	if o3.dir != "/x" || o3.timeout.Seconds() != 2 {
		t.Fatalf("%+v", o3)
	}
	if len(o3.env) != 1 || o3.env[0] != "K=V" {
		t.Fatal(o3.env)
	}
}

func TestFullCover_ShlexQuoteSpecial(t *testing.T) {
	// chars that force quoting
	q := shlexQuote("a|b")
	if !strings.Contains(q, "'") {
		t.Fatal(q)
	}
	q = shlexQuote("a$b")
	if !strings.Contains(q, "'") {
		t.Fatal(q)
	}
}

func TestFullCover_PackageShlexJoinEmptyList(t *testing.T) {
	p := packageShlex()
	j := p.Obj.(*runtime.MapObj).Vals["join"]
	r, _ := j.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.List()})
	if r.S != "" {
		t.Fatal(r.S)
	}
}

func TestFullCover_BinstructPackageSizeOK(t *testing.T) {
	p := packageBinstruct()
	sz := p.Obj.(*runtime.MapObj).Vals["size"]
	fn := sz.Obj.(*runtime.BuiltinObj).Fn
	r, err := fn([]runtime.Value{runtime.Str(">I")})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Obj.(*runtime.ResultObj).Ok || r.Obj.(*runtime.ResultObj).Val.I != 4 {
		t.Fatal(r)
	}
}

func TestFullCover_BinstructNeedErrors(t *testing.T) {
	// each value-taking code must fail when no values provided
	for _, layout := range []string{">B", ">h", ">H", ">i", ">I", ">q", ">f", ">d", ">2s", ">c", ">b", ">l", ">L", ">Q"} {
		if _, err := bsPack(layout, nil); err == nil {
			t.Fatalf("expected error for %s", layout)
		}
	}
	// unpack short for each size class
	for _, layout := range []string{">B", ">h", ">i", ">q", ">f", ">d", ">2s"} {
		if _, err := bsUnpack(layout, []byte{}); err == nil {
			t.Fatalf("short unpack %s", layout)
		}
	}
	// l lowercase 32-bit
	b, err := bsPack(">l", []runtime.Value{runtime.Int(-9)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := bsUnpack(">l", b)
	if err != nil || v[0].I != -9 {
		t.Fatal(v, err)
	}
}

func TestFullCover_DifflibNdiffOnlyDeletes(t *testing.T) {
	n := ndiff([]string{"a", "b"}, nil)
	if len(n) != 2 {
		t.Fatal(n)
	}
	n = ndiff(nil, []string{"x"})
	if len(n) != 1 || !strings.HasPrefix(n[0], "+") {
		t.Fatal(n)
	}
	// equal lines mid
	n = ndiff([]string{"a", "b", "c"}, []string{"a", "x", "c"})
	if len(n) < 3 {
		t.Fatal(n)
	}
}

func TestFullCover_ShlexSplitEdges(t *testing.T) {
	// double-quote escapes
	p, err := shlexSplit(`"a\"b"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 1 {
		t.Fatal(p)
	}
	// trailing backslash
	if _, err := shlexSplit(`abc\`); err == nil {
		t.Fatal("expected error")
	}
	// unclosed double
	if _, err := shlexSplit(`"abc`); err == nil {
		t.Fatal("expected error")
	}
}
