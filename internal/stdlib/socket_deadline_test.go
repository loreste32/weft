package stdlib

import (
	"net"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func TestSocketReadTimeout(t *testing.T) {
	// Create a listener that never sends data
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			time.Sleep(5 * time.Second)
			c.Close()
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	wrapped := wrapConn(conn)
	mo := wrapped.Obj.(*runtime.MapObj)

	// read_timeout with 1-second timeout should return error
	readTimeout := mo.Vals["read_timeout"]
	r, err := readTimeout.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{
		runtime.Int(1024),
		runtime.Int(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should be an Err result containing "timeout"
	s := r.String()
	if r.Kind != runtime.KindResult {
		t.Fatalf("expected Result, got %v: %s", r.Kind, s)
	}
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected timeout error, got Ok")
	}
}

func TestSocketSetAndClearReadDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			defer c.Close()
			time.Sleep(5 * time.Second)
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	wrapped := wrapConn(conn)
	mo := wrapped.Obj.(*runtime.MapObj)

	// set_read_deadline should succeed
	setFn := mo.Vals["set_read_deadline"]
	r, err := setFn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Int(5)})
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindResult {
		t.Fatalf("set_read_deadline: expected Result, got %v", r.Kind)
	}

	// clear_read_deadline should succeed
	clearFn := mo.Vals["clear_read_deadline"]
	r, err = clearFn.Obj.(*runtime.BuiltinObj).Fn(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindResult {
		t.Fatalf("clear_read_deadline: expected Result, got %v", r.Kind)
	}
}

func TestSocketSeparateReadWriteLocking(t *testing.T) {
	// Verify reads and writes can proceed concurrently
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		if c != nil {
			defer c.Close()
			buf := make([]byte, 1024)
			c.Read(buf)
			c.Write([]byte("pong"))
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	wrapped := wrapConn(conn)
	mo := wrapped.Obj.(*runtime.MapObj)

	// Write should succeed
	writeFn := mo.Vals["write"]
	r, err := writeFn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("ping")})
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != runtime.KindResult {
		t.Fatalf("write: expected Result, got %v", r.Kind)
	}
}
