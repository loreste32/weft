package stdlib

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func getConnFn(mo *runtime.MapObj, name string) runtime.Builtin {
	return mo.Vals[name].Obj.(*runtime.BuiltinObj).Fn
}

// TestReadHonorsCallerDeadline verifies that after set_read_deadline(1),
// a plain read() returns a timeout in ~1 second — NOT 60 seconds.
// This is the critical ESL timeout test.
func TestReadHonorsCallerDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			time.Sleep(10 * time.Second) // never send
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

	// Set a 1-second read deadline
	setFn := getConnFn(mo, "set_read_deadline")
	setFn([]runtime.Value{runtime.Int(1)})

	// read() should timeout in ~1 second, not 60
	start := time.Now()
	readFn := getConnFn(mo, "read")
	r, _ := readFn([]runtime.Value{runtime.Int(1024)})
	elapsed := time.Since(start)

	// Must complete in under 3 seconds (1s deadline + margin)
	if elapsed > 3*time.Second {
		t.Fatalf("read took %v — deadline was not honored (would be 60s without fix)", elapsed)
	}
	// Result should be an error containing "timeout"
	if r.Kind != runtime.KindResult {
		t.Fatalf("expected Result, got %v", r.Kind)
	}
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected timeout error, got Ok")
	}
}

// TestReadTimeoutReturnsInTime verifies read_timeout with a specific timeout.
func TestReadTimeoutReturnsInTime(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			time.Sleep(10 * time.Second)
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

	start := time.Now()
	rtFn := getConnFn(mo, "read_timeout")
	r, _ := rtFn([]runtime.Value{runtime.Int(1024), runtime.Int(1)})
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Fatalf("read_timeout took %v, expected ~1s", elapsed)
	}
	result := r.Obj.(*runtime.ResultObj)
	if result.Ok {
		t.Fatal("expected timeout error")
	}
}

// TestClearDeadlineRestoresBlocking verifies clear_read_deadline works.
func TestClearDeadlineRestoresBlocking(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			// Send data after 500ms — if deadline is cleared, read should succeed
			time.Sleep(500 * time.Millisecond)
			c.Write([]byte("hello"))
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

	// Set a very short deadline, then clear it
	setFn := getConnFn(mo, "set_read_deadline")
	setFn([]runtime.Value{runtime.Int(1)})
	clearFn := getConnFn(mo, "clear_read_deadline")
	clearFn(nil)

	// Read should now block until data arrives (~500ms), not timeout
	readFn := getConnFn(mo, "read")
	r, _ := readFn([]runtime.Value{runtime.Int(1024)})
	result := r.Obj.(*runtime.ResultObj)
	if !result.Ok {
		t.Fatal("expected successful read after clearing deadline")
	}
}

// TestConcurrentReadWrite verifies write succeeds while read is blocked.
func TestConcurrentReadWrite(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		if c != nil {
			// Read the write from client, then send reply
			buf := make([]byte, 1024)
			n, _ := c.Read(buf)
			if n > 0 {
				c.Write([]byte("reply"))
			}
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	wrapped := wrapConn(conn)
	mo := wrapped.Obj.(*runtime.MapObj)
	readFn := getConnFn(mo, "read")
	writeFn := getConnFn(mo, "write")

	// Start read in background — it will block until server sends "reply"
	var readResult runtime.Value
	var readDone sync.WaitGroup
	readDone.Add(1)
	go func() {
		defer readDone.Done()
		readResult, _ = readFn([]runtime.Value{runtime.Int(1024)})
	}()

	// Give the read goroutine time to block
	time.Sleep(100 * time.Millisecond)

	// Write should succeed even though read is blocked (separate mutexes)
	writeStart := time.Now()
	r, _ := writeFn([]runtime.Value{runtime.Str("ping")})
	writeElapsed := time.Since(writeStart)

	if writeElapsed > 2*time.Second {
		t.Fatalf("write blocked for %v — read/write mutexes not separate", writeElapsed)
	}
	if r.Kind != runtime.KindResult {
		t.Fatalf("write: expected Result, got %v", r.Kind)
	}
	result := r.Obj.(*runtime.ResultObj)
	if !result.Ok {
		t.Fatal("write failed")
	}

	// Wait for read to complete (server sends reply after receiving write)
	readDone.Wait()
	if readResult.Kind == runtime.KindResult {
		rr := readResult.Obj.(*runtime.ResultObj)
		if !rr.Ok {
			t.Fatal("read failed after write")
		}
	}
}

// TestCloseInterruptsBlockedRead verifies close() unblocks a reader.
func TestCloseInterruptsBlockedRead(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			time.Sleep(30 * time.Second) // never send
			c.Close()
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	wrapped := wrapConn(conn)
	mo := wrapped.Obj.(*runtime.MapObj)
	readFn := getConnFn(mo, "read")
	closeFn := getConnFn(mo, "close")

	var wg sync.WaitGroup
	wg.Add(1)
	start := time.Now()
	go func() {
		defer wg.Done()
		readFn([]runtime.Value{runtime.Int(1024)})
	}()

	// Close after 200ms — should unblock the read
	time.Sleep(200 * time.Millisecond)
	closeFn(nil)

	wg.Wait()
	elapsed := time.Since(start)
	if elapsed > 3*time.Second {
		t.Fatalf("close did not interrupt blocked read (took %v)", elapsed)
	}
}
