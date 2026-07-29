//go:build !js

package stdlib

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/loreste/weft/internal/netsafe"
	"github.com/loreste/weft/internal/runtime"
)

// packageNetutil — network diagnostics for ops scripts (port check, DNS, TCP ping).
func packageNetutil(env *runtime.Env) runtime.Value {
	p := pkg()

	// netutil.port_open(host, port, timeout_sec?) -> Result[bool]
	set(p, "port_open", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("netutil.port_open(host, port, timeout?)", "netutil"), nil
		}
		host := args[0].String()
		port, err := runtime.AsInt(args[1])
		if err != nil {
			return errRes("port must be int", "netutil"), nil
		}
		timeout := 5 * time.Second
		if len(args) >= 3 {
			if n, e := runtime.AsInt(args[2]); e == nil && n > 0 {
				timeout = time.Duration(n) * time.Second
			}
		}
		addr := fmt.Sprintf("%s:%d", host, port)
		if err := netsafe.CheckHost(host); err != nil {
			return errRes(err.Error(), "netutil"), nil
		}
		conn, err := netsafe.DialContext(env.Context(), "tcp", addr, timeout)
		if err != nil {
			return runtime.Ok(runtime.Bool(false)), nil
		}
		conn.Close()
		return runtime.Ok(runtime.Bool(true)), nil
	}, 3)

	// netutil.tcp_ping(host, port, timeout_sec?) -> Result[map]  {open, latency_ms}
	set(p, "tcp_ping", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("netutil.tcp_ping(host, port, timeout?)", "netutil"), nil
		}
		host := args[0].String()
		port, err := runtime.AsInt(args[1])
		if err != nil {
			return errRes("port must be int", "netutil"), nil
		}
		timeout := 5 * time.Second
		if len(args) >= 3 {
			if n, e := runtime.AsInt(args[2]); e == nil && n > 0 {
				timeout = time.Duration(n) * time.Second
			}
		}
		addr := fmt.Sprintf("%s:%d", host, port)
		if err := netsafe.CheckHost(host); err != nil {
			return errRes(err.Error(), "netutil"), nil
		}
		start := time.Now()
		conn, err := netsafe.DialContext(env.Context(), "tcp", addr, timeout)
		elapsed := time.Since(start)

		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		if err != nil {
			put("open", runtime.Bool(false))
			put("latency_ms", runtime.Float(-1))
			put("error", runtime.Str(err.Error()))
		} else {
			conn.Close()
			put("open", runtime.Bool(true))
			put("latency_ms", runtime.Float(float64(elapsed.Microseconds())/1000.0))
			put("error", runtime.Null())
		}
		return runtime.Ok(m), nil
	}, 3)

	// netutil.resolve(host) -> Result[[str]]  DNS lookup → IPs
	set(p, "resolve", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("netutil.resolve(host)", "netutil"), nil
		}
		host := args[0].String()
		ips, err := net.LookupIP(host)
		if err != nil {
			return errRes(err.Error(), "netutil"), nil
		}
		items := make([]runtime.Value, 0, len(ips))
		for _, ip := range ips {
			items = append(items, runtime.Str(ip.String()))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// netutil.lookup_host(host) -> Result[[str]]  DNS name lookup
	set(p, "lookup_host", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("netutil.lookup_host(host)", "netutil"), nil
		}
		addrs, err := net.LookupHost(args[0].String())
		if err != nil {
			return errRes(err.Error(), "netutil"), nil
		}
		items := make([]runtime.Value, 0, len(addrs))
		for _, a := range addrs {
			items = append(items, runtime.Str(a))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// netutil.lookup_txt(host) -> Result[[str]]  DNS TXT records
	set(p, "lookup_txt", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("netutil.lookup_txt(host)", "netutil"), nil
		}
		txts, err := net.LookupTXT(args[0].String())
		if err != nil {
			return errRes(err.Error(), "netutil"), nil
		}
		items := make([]runtime.Value, 0, len(txts))
		for _, t := range txts {
			items = append(items, runtime.Str(t))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// netutil.lookup_mx(host) -> Result[[map]]  DNS MX records
	set(p, "lookup_mx", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("netutil.lookup_mx(host)", "netutil"), nil
		}
		mxs, err := net.LookupMX(args[0].String())
		if err != nil {
			return errRes(err.Error(), "netutil"), nil
		}
		items := make([]runtime.Value, 0, len(mxs))
		for _, mx := range mxs {
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			mo.Keys = append(mo.Keys, "host", "pref")
			mo.Vals["host"] = runtime.Str(strings.TrimSuffix(mx.Host, "."))
			mo.Vals["pref"] = runtime.Int(int64(mx.Pref))
			items = append(items, m)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// netutil.reverse_lookup(ip) -> Result[[str]]  reverse DNS
	set(p, "reverse_lookup", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("netutil.reverse_lookup(ip)", "netutil"), nil
		}
		names, err := net.LookupAddr(args[0].String())
		if err != nil {
			return errRes(err.Error(), "netutil"), nil
		}
		items := make([]runtime.Value, 0, len(names))
		for _, n := range names {
			items = append(items, runtime.Str(strings.TrimSuffix(n, ".")))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// netutil.scan_ports(host, ports) -> Result[[map]]  check multiple ports
	set(p, "scan_ports", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("netutil.scan_ports(host, [ports])", "netutil"), nil
		}
		host := args[0].String()
		if err := netsafe.CheckHost(host); err != nil {
			return errRes(err.Error(), "netutil"), nil
		}
		if args[1].Kind != runtime.KindList {
			return errRes("ports must be a list", "netutil"), nil
		}
		lo := args[1].Obj.(*runtime.ListObj)
		items := make([]runtime.Value, 0, len(lo.Items))
		for _, pv := range lo.Items {
			port, err := runtime.AsInt(pv)
			if err != nil {
				continue
			}
			addr := fmt.Sprintf("%s:%d", host, port)
			conn, err := netsafe.DialContext(env.Context(), "tcp", addr, 2*time.Second)
			open := err == nil
			if open {
				conn.Close()
			}
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			mo.Keys = append(mo.Keys, "port", "open")
			mo.Vals["port"] = runtime.Int(port)
			mo.Vals["open"] = runtime.Bool(open)
			items = append(items, m)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 2)

	return p
}
