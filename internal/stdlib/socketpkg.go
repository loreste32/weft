package stdlib

import (
	"io"
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
		if err := checkDialAddress(address); err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		timeout := 30 * time.Second
		if len(args) >= 3 {
			if n, e := runtime.AsInt(args[2]); e == nil && n > 0 {
				timeout = time.Duration(n) * time.Second
			} else if f, ok := asFloat64(args[2]); ok && f > 0 {
				timeout = time.Duration(f * float64(time.Second))
			}
		}
		d := net.Dialer{Timeout: timeout}
		c, err := d.DialContext(env.Context(), network, address)
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

func wrapConn(c net.Conn) runtime.Value {
	var mu sync.Mutex
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
		mu.Lock()
		_ = c.SetReadDeadline(time.Now().Add(60 * time.Second))
		nr, err := c.Read(buf)
		mu.Unlock()
		if err != nil && err != io.EOF {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Str(string(buf[:nr]))), nil
	})
	put("read_all", 0, func(args []runtime.Value) (runtime.Value, error) {
		mu.Lock()
		_ = c.SetReadDeadline(time.Now().Add(60 * time.Second))
		b, err := io.ReadAll(io.LimitReader(c, 32<<20))
		mu.Unlock()
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	})
	put("write", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("conn.write(data)", "socket"), nil
		}
		mu.Lock()
		_ = c.SetWriteDeadline(time.Now().Add(60 * time.Second))
		nw, err := c.Write([]byte(args[0].String()))
		mu.Unlock()
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Int(int64(nw))), nil
	})
	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		mu.Lock()
		err := c.Close()
		mu.Unlock()
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("set_deadline", 1, func(args []runtime.Value) (runtime.Value, error) {
		// seconds from now
		sec := int64(30)
		if len(args) >= 1 {
			if n, e := runtime.AsInt(args[0]); e == nil {
				sec = n
			}
		}
		mu.Lock()
		err := c.SetDeadline(time.Now().Add(time.Duration(sec) * time.Second))
		mu.Unlock()
		if err != nil {
			return errRes(err.Error(), "socket"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
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
