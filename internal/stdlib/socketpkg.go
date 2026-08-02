//go:build !js

package stdlib

import (
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/loreste/weft/internal/netsafe"
	"github.com/loreste/weft/internal/runtime"
)

// packageSocket — raw TCP/UDP (Python socket lite). Dial is SSRF-guarded.
func packageSocket(env *runtime.Env) runtime.Value {
	p := pkg()

	// socket.dial(network, address, timeout_sec?) -> Result[conn]
	// network: "tcp", "tcp4", "udp", ...
	set(p, "dial", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("socket.dial(network, address, timeout?)", "socket"), nil
		}
		network := args[0].String()
		address := args[1].String()
		timeout := 30 * time.Second
		if len(args) >= 3 {
			parsed, parseErr := socketDeadlineDuration(args[2:3], "dial")
			if parseErr != nil {
				return errRes(parseErr.Error(), "socket"), nil
			}
			timeout = parsed
		}
		// netsafe.DialContext resolves and dials only non-blocked IPs (no DNS rebinding).
		c, err := netsafe.DialContext(env.Context(), network, address, timeout)
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(wrapConn(c)), nil
	}, 3)

	// socket.listen(network, address) -> Result[listener]
	set(p, "listen", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("socket.listen(network, address)", "socket"), nil
		}
		network, address := args[0].String(), args[1].String()
		// bind local — allow; check only if non-loopback host in address
		ln, err := net.Listen(network, address)
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(wrapListener(ln)), nil
	}, 2)

	// socket.resolve(host) -> Result[[str]] IPs
	set(p, "resolve", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("socket.resolve(host)", "socket"), nil
		}
		host := args[0].String()
		ips, err := net.LookupIP(host)
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		items := make([]runtime.Value, 0, len(ips))
		for _, ip := range ips {
			items = append(items, runtime.Str(ip.String()))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	return p
}

func checkDialAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		// may be bare host without port for some networks
		host = address
	}
	host = strings.Trim(host, "[]")
	return netsafe.CheckHost(host)
}

func socketDeadlineDuration(args []runtime.Value, name string) (time.Duration, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("conn.%s requires one positive number of seconds", name)
	}
	// Accept both int and float. Check the native kind first: AsInt may
	// truncate a float such as 0.001 to zero before validation.
	var dur time.Duration
	if args[0].Kind == runtime.KindFloat {
		f := args[0].F
		if f <= 0 {
			return 0, fmt.Errorf("conn.%s requires a positive number of seconds", name)
		}
		nanos := f * float64(time.Second)
		// float64 cannot represent MaxInt64 exactly; step below 2^63 so
		// converting nanos to time.Duration cannot wrap.
		maxNanos := math.Nextafter(float64(1<<63), 0)
		if math.IsNaN(f) || math.IsInf(f, 0) || math.IsNaN(nanos) || math.IsInf(nanos, 0) || nanos < 1 || nanos > maxNanos {
			return 0, fmt.Errorf("conn.%s requires positive number seconds", name)
		}
		dur = time.Duration(nanos)
	} else if sec, err := runtime.AsInt(args[0]); err == nil {
		if sec <= 0 {
			return 0, fmt.Errorf("conn.%s requires a positive number of seconds", name)
		}
		const maxSeconds = int64((1<<63 - 1) / int64(time.Second))
		if sec > maxSeconds {
			return 0, fmt.Errorf("conn.%s duration is too large", name)
		}
		dur = time.Duration(sec) * time.Second
	} else {
		return 0, fmt.Errorf("conn.%s requires a positive number of seconds", name)
	}
	return dur, nil
}

func wrapConn(c net.Conn) runtime.Value {
	var readMu, writeMu sync.Mutex
	var deadlineMu sync.Mutex
	var readDeadline, writeDeadline time.Time
	var readVersion, writeVersion uint64

	setReadDeadline := func(deadline time.Time) error {
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		if err := c.SetReadDeadline(deadline); err != nil {
			return err
		}
		readDeadline = deadline
		readVersion++
		return nil
	}
	setWriteDeadline := func(deadline time.Time) error {
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		if err := c.SetWriteDeadline(deadline); err != nil {
			return err
		}
		writeDeadline = deadline
		writeVersion++
		return nil
	}
	setDeadline := func(deadline time.Time) error {
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		if err := c.SetDeadline(deadline); err != nil {
			return err
		}
		readDeadline = deadline
		writeDeadline = deadline
		readVersion++
		writeVersion++
		return nil
	}
	temporaryReadDeadline := func(deadline time.Time) (time.Time, uint64, error) {
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		previous := readDeadline
		if err := c.SetReadDeadline(deadline); err != nil {
			return time.Time{}, 0, err
		}
		readVersion++
		return previous, readVersion, nil
	}
	restoreReadDeadline := func(previous time.Time, version uint64) error {
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		if readVersion != version {
			return nil
		}
		if err := c.SetReadDeadline(previous); err != nil {
			return err
		}
		readDeadline = previous
		readVersion++
		return nil
	}
	temporaryWriteDeadline := func(deadline time.Time) (time.Time, uint64, error) {
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		previous := writeDeadline
		if err := c.SetWriteDeadline(deadline); err != nil {
			return time.Time{}, 0, err
		}
		writeVersion++
		return previous, writeVersion, nil
	}
	restoreWriteDeadline := func(previous time.Time, version uint64) error {
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		if writeVersion != version {
			return nil
		}
		if err := c.SetWriteDeadline(previous); err != nil {
			return err
		}
		writeDeadline = previous
		writeVersion++
		return nil
	}
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("conn."+name, arity, fn)
	}
	mo.Keys = append(mo.Keys, "remote", "local")
	mo.Vals["remote"] = runtime.Str(c.RemoteAddr().String())
	mo.Vals["local"] = runtime.Str(c.LocalAddr().String())

	put("read", 1, func(args []runtime.Value) (runtime.Value, error) {
		n := 4096
		if len(args) >= 1 {
			if x, e := runtime.AsInt(args[0]); e == nil && x > 0 && x < 1<<20 {
				n = int(x)
			}
		}
		buf := make([]byte, n)
		readMu.Lock()
		nr, err := c.Read(buf)
		readMu.Unlock()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return errRes("timeout", "socket"), nil
			}
			if err != io.EOF {
				return errRes(err.Error(), "socket"), nil
			}
		}
		return runtime.Ok(runtime.Str(string(buf[:nr]))), nil
	})
	put("read_all", 0, func(args []runtime.Value) (runtime.Value, error) {
		readMu.Lock()
		b, err := io.ReadAll(io.LimitReader(c, (32<<20)+1))
		readMu.Unlock()
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		if len(b) > 32<<20 {
			return errRes("conn.read_all exceeded 32MB limit", "socket"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	})
	// read_all_timeout is the bounded variant; read_all remains an
	// intentionally blocking EOF-delimited read for stream callers.
	put("read_all_timeout", 1, func(args []runtime.Value) (runtime.Value, error) {
		dur, durErr := socketDeadlineDuration(args, "read_all_timeout")
		if durErr != nil {
			return errRes(durErr.Error(), "socket"), nil
		}
		readMu.Lock()
		defer readMu.Unlock()
		previous, version, err := temporaryReadDeadline(time.Now().Add(dur))
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		b, readErr := io.ReadAll(io.LimitReader(c, (32<<20)+1))
		restoreErr := restoreReadDeadline(previous, version)
		if readErr != nil {
			if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
				return errRes("timeout", "socket"), nil
			}
			return errRes(readErr.Error(), "socket"), nil
		}
		if len(b) > 32<<20 {
			return errRes("conn.read_all_timeout exceeded 32MB limit", "socket"), nil
		}
		if restoreErr != nil {
			return errRes(restoreErr.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	})

	put("write", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("conn.write(data)", "socket"), nil
		}
		writeMu.Lock()
		nw, err := c.Write([]byte(args[0].String()))
		writeMu.Unlock()
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Int(int64(nw))), nil
	})
	// write_timeout makes the temporary deadline explicit and restores the
	// caller's tracked write deadline unless another writer changed it.
	put("write_timeout", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("conn.write_timeout(data, seconds)", "socket"), nil
		}
		dur, durErr := socketDeadlineDuration(args[1:], "write_timeout")
		if durErr != nil {
			return errRes(durErr.Error(), "socket"), nil
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		previous, version, err := temporaryWriteDeadline(time.Now().Add(dur))
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		nw, writeErr := c.Write([]byte(args[0].String()))
		restoreErr := restoreWriteDeadline(previous, version)
		if writeErr != nil {
			if netErr, ok := writeErr.(net.Error); ok && netErr.Timeout() {
				return errRes("timeout", "socket"), nil
			}
			return errRes(writeErr.Error(), "socket"), nil
		}
		if restoreErr != nil {
			return errRes(restoreErr.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Int(int64(nw))), nil
	})

	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		// net.Conn.Close is safe to call concurrently — it interrupts
		// blocked reads/writes. Do NOT hold readMu (would deadlock if
		// another goroutine is blocked in Read).
		err := c.Close()
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("set_deadline", 1, func(args []runtime.Value) (runtime.Value, error) {
		duration, parseErr := socketDeadlineDuration(args, "set_deadline")
		if parseErr != nil {
			return errRes(parseErr.Error(), "socket"), nil
		}
		err := setDeadline(time.Now().Add(duration))
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("set_read_deadline", 1, func(args []runtime.Value) (runtime.Value, error) {
		duration, parseErr := socketDeadlineDuration(args, "set_read_deadline")
		if parseErr != nil {
			return errRes(parseErr.Error(), "socket"), nil
		}
		err := setReadDeadline(time.Now().Add(duration))
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("clear_read_deadline", 0, func(args []runtime.Value) (runtime.Value, error) {
		err := setReadDeadline(time.Time{})
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("set_write_deadline", 1, func(args []runtime.Value) (runtime.Value, error) {
		duration, parseErr := socketDeadlineDuration(args, "set_write_deadline")
		if parseErr != nil {
			return errRes(parseErr.Error(), "socket"), nil
		}
		err := setWriteDeadline(time.Now().Add(duration))
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("clear_write_deadline", 0, func(args []runtime.Value) (runtime.Value, error) {
		err := setWriteDeadline(time.Time{})
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	// read_timeout: read with a specific timeout in seconds, returns Err on timeout
	put("read_timeout", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("conn.read_timeout(size, seconds)", "socket"), nil
		}
		n, nErr := runtime.AsInt(args[0])
		if nErr != nil || n <= 0 || n >= 1<<20 {
			return errRes("conn.read_timeout: size must be a positive integer under 1MB", "socket"), nil
		}
		dur, durErr := socketDeadlineDuration(args[1:], "read_timeout")
		if durErr != nil {
			return errRes(durErr.Error(), "socket"), nil
		}
		buf := make([]byte, int(n))
		readMu.Lock()
		defer readMu.Unlock()
		previous, version, deadlineErr := temporaryReadDeadline(time.Now().Add(dur))
		if deadlineErr != nil {
			return errRes(deadlineErr.Error(), "socket"), nil
		}
		nr, err := c.Read(buf)
		restoreErr := restoreReadDeadline(previous, version)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return errRes("timeout", "socket"), nil
			}
			if err != io.EOF {
				return errRes(err.Error(), "socket"), nil
			}
		}
		if restoreErr != nil {
			return errRes(restoreErr.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Str(string(buf[:nr]))), nil
	})
	return m
}

func wrapListener(ln net.Listener) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("listener."+name, arity, fn)
	}
	mo.Keys = append(mo.Keys, "addr")
	mo.Vals["addr"] = runtime.Str(ln.Addr().String())

	put("accept", 0, func(args []runtime.Value) (runtime.Value, error) {
		c, err := ln.Accept()
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(wrapConn(c)), nil
	})
	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		if err := ln.Close(); err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	return m
}
