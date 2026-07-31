//go:build !js

package stdlib

import (
	"net"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageDNS — full DNS client: SRV, CNAME, NS, TXT, MX, PTR, A, AAAA.
func packageDNS() runtime.Value {
	p := pkg()

	set(p, "lookup", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dns.lookup(host)", "dns"), nil
		}
		ips, err := net.LookupIP(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dns"), nil
		}
		items := make([]runtime.Value, 0, len(ips))
		for _, ip := range ips {
			items = append(items, runtime.Str(ip.String()))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	set(p, "srv", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("dns.srv(service, proto, name)", "dns"), nil
		}
		_, addrs, err := net.LookupSRV(args[0].String(), args[1].String(), args[2].String())
		if err != nil {
			return errRes(err.Error(), "dns"), nil
		}
		items := make([]runtime.Value, 0, len(addrs))
		for _, a := range addrs {
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			mo.Keys = append(mo.Keys, "target", "port", "priority", "weight")
			mo.Vals["target"] = runtime.Str(strings.TrimSuffix(a.Target, "."))
			mo.Vals["port"] = runtime.Int(int64(a.Port))
			mo.Vals["priority"] = runtime.Int(int64(a.Priority))
			mo.Vals["weight"] = runtime.Int(int64(a.Weight))
			items = append(items, m)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 3)

	set(p, "cname", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dns.cname(host)", "dns"), nil
		}
		cname, err := net.LookupCNAME(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dns"), nil
		}
		return runtime.Ok(runtime.Str(strings.TrimSuffix(cname, "."))), nil
	}, 1)

	set(p, "ns", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dns.ns(domain)", "dns"), nil
		}
		nss, err := net.LookupNS(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dns"), nil
		}
		items := make([]runtime.Value, 0, len(nss))
		for _, ns := range nss {
			items = append(items, runtime.Str(strings.TrimSuffix(ns.Host, ".")))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	set(p, "mx", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dns.mx(domain)", "dns"), nil
		}
		mxs, err := net.LookupMX(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dns"), nil
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

	set(p, "txt", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dns.txt(domain)", "dns"), nil
		}
		txts, err := net.LookupTXT(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dns"), nil
		}
		items := make([]runtime.Value, 0, len(txts))
		for _, t := range txts {
			items = append(items, runtime.Str(t))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	set(p, "reverse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("dns.reverse(ip)", "dns"), nil
		}
		names, err := net.LookupAddr(args[0].String())
		if err != nil {
			return errRes(err.Error(), "dns"), nil
		}
		items := make([]runtime.Value, 0, len(names))
		for _, n := range names {
			items = append(items, runtime.Str(strings.TrimSuffix(n, ".")))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	return p
}
