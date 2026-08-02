//go:build !js

package stdlib

import (
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func callWrappedConn(t *testing.T, conn runtime.Value, name string, args ...runtime.Value) runtime.Value {
	t.Helper()
	value, err := callWrappedConnValue(conn, name, args...)
	if err != nil {
		t.Fatalf("conn.%s returned host error: %v", name, err)
	}
	return value
}

func callWrappedConnValue(conn runtime.Value, name string, args ...runtime.Value) (runtime.Value, error) {
	obj, ok := conn.Obj.(*runtime.MapObj)
	if !ok {
		return runtime.Value{}, &unexpectedSocketData{got: "non-map value", want: "map"}
	}
	builtin, ok := obj.Vals[name].Obj.(*runtime.BuiltinObj)
	if !ok {
		return runtime.Value{}, &unexpectedSocketData{got: name, want: "conn builtin"}
	}
	return builtin.Fn(args)
}

func resultOf(t *testing.T, value runtime.Value) *runtime.ResultObj {
	t.Helper()
	if value.Kind != runtime.KindResult {
		t.Fatalf("got %v, want Result", value.Kind)
	}
	return value.Obj.(*runtime.ResultObj)
}

func closeWrappedConn(t *testing.T, conn runtime.Value) {
	t.Helper()
	result := resultOf(t, callWrappedConn(t, conn, "close"))
	if !result.Ok {
		t.Fatalf("close failed: %s", result.Err.String())
	}
}

func TestWrappedConnReadHonorsCallerDeadlineBounds(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	conn := wrapConn(client)
	defer closeWrappedConn(t, conn)

	deadline := resultOf(t, callWrappedConn(t, conn, "set_read_deadline", runtime.Int(1)))
	if !deadline.Ok {
		t.Fatalf("set_read_deadline failed: %s", deadline.Err.String())
	}

	started := time.Now()
	done := make(chan runtime.Value, 1)
	go func() {
		value, err := callWrappedConnValue(conn, "read", runtime.Int(1))
		if err != nil {
			done <- runtime.Err(runtime.NewError(err.Error(), "test"))
			return
		}
		done <- value
	}()

	select {
	case <-time.After(700 * time.Millisecond):
		// The lower bound is checked below when the result arrives.
	case value := <-done:
		t.Fatalf("read returned too early after %s: %v", time.Since(started), value)
	}

	select {
	case value := <-done:
		elapsed := time.Since(started)
		if elapsed > 1800*time.Millisecond {
			t.Fatalf("read exceeded upper timing bound: %s", elapsed)
		}
		result := resultOf(t, value)
		if result.Ok || !strings.Contains(strings.ToLower(result.Err.String()), "timeout") {
			t.Fatalf("read returned %v, want timeout error", value)
		}
	case <-time.After(1200 * time.Millisecond):
		t.Fatal("read did not honor the caller deadline")
	}
}

func TestWrappedConnCloseInterruptsBlockedRead(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	conn := wrapConn(client)

	done := make(chan runtime.Value, 1)
	go func() {
		value, err := callWrappedConnValue(conn, "read", runtime.Int(1))
		if err != nil {
			done <- runtime.Err(runtime.NewError(err.Error(), "test"))
			return
		}
		done <- value
	}()

	// Closing the connection is the synchronization event; no sleep is needed
	// and the read must be allowed to remain blocked until this point.
	closeWrappedConn(t, conn)
	select {
	case value := <-done:
		result := resultOf(t, value)
		if result.Ok || !strings.Contains(strings.ToLower(result.Err.String()), "closed") {
			t.Fatalf("read returned %v, want closed-connection error", value)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not interrupt the blocked read")
	}
}

func TestWrappedConnReadWriteAreIndependent(t *testing.T) {
	client, peer := net.Pipe()
	conn := wrapConn(client)
	defer closeWrappedConn(t, conn)

	peerDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 3)
		if _, err := peer.Read(buf); err != nil {
			peerDone <- err
			return
		}
		if string(buf) != "hey" {
			peerDone <- &unexpectedSocketData{got: string(buf), want: "hey"}
			return
		}
		_, err := peer.Write([]byte("ack"))
		peerDone <- err
	}()

	readDone := make(chan runtime.Value, 1)
	go func() {
		value, err := callWrappedConnValue(conn, "read", runtime.Int(3))
		if err != nil {
			readDone <- runtime.Err(runtime.NewError(err.Error(), "test"))
			return
		}
		readDone <- value
	}()

	written := resultOf(t, callWrappedConn(t, conn, "write", runtime.Str("hey")))
	if !written.Ok || written.Val.I != 3 {
		t.Fatalf("write returned %v, want three bytes", written)
	}

	select {
	case value := <-readDone:
		read := resultOf(t, value)
		if !read.Ok || read.Val.S != "ack" {
			t.Fatalf("read returned %v, want ack", value)
		}
	case <-time.After(time.Second):
		t.Fatal("concurrent read/write did not complete")
	}

	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatalf("peer failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("peer goroutine did not complete")
	}
	if err := peer.Close(); err != nil {
		t.Fatalf("peer close failed: %v", err)
	}
}

func TestWrappedConnDeadlineSettersRejectInvalidValues(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	conn := wrapConn(client)
	defer closeWrappedConn(t, conn)

	for _, name := range []string{"set_deadline", "set_read_deadline", "set_write_deadline"} {
		for _, arg := range []runtime.Value{
			runtime.Int(0),
			runtime.Int(-1),
			runtime.Float(math.NaN()),
			runtime.Float(math.Inf(1)),
			runtime.Float(math.Inf(-1)),
			runtime.Float(1e-12),
			runtime.Float(1e20),
			runtime.Str("not-a-duration"),
		} {
			result := resultOf(t, callWrappedConn(t, conn, name, arg))
			if result.Ok {
				t.Fatalf("conn.%s accepted invalid argument %v", name, arg)
			}
		}
	}
}

func TestWrappedConnTimeoutHelpersRejectInvalidFloatDurations(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	conn := wrapConn(client)
	defer closeWrappedConn(t, conn)

	invalid := []runtime.Value{
		runtime.Float(math.NaN()),
		runtime.Float(math.Inf(1)),
		runtime.Float(1e-12),
		runtime.Float(1e20),
	}
	for _, arg := range invalid {
		checks := []struct {
			name string
			args []runtime.Value
		}{
			{"read_timeout", []runtime.Value{runtime.Int(1), arg}},
			{"read_all_timeout", []runtime.Value{arg}},
			{"write_timeout", []runtime.Value{runtime.Str("x"), arg}},
		}
		for _, check := range checks {
			result := resultOf(t, callWrappedConn(t, conn, check.name, check.args...))
			if result.Ok {
				t.Fatalf("conn.%s accepted invalid duration %v", check.name, arg)
			}
		}
	}

	valid := resultOf(t, callWrappedConn(t, conn, "set_read_deadline", runtime.Float(0.001)))
	if !valid.Ok {
		t.Fatalf("valid sub-second deadline rejected: %s", valid.Err.String())
	}
}

type unexpectedSocketData struct {
	got  string
	want string
}

func (e *unexpectedSocketData) Error() string {
	return "unexpected socket data: got " + e.got + ", want " + e.want
}
