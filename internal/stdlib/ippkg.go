package stdlib

import (
	"net"
	"net/netip"

	"github.com/loreste/weft/internal/runtime"
)

// packageIP — IPv4/IPv6 helpers.
func packageIP() runtime.Value {
	p := pkg()

	// ip.parse(s) -> Result[{str, version, is_private, is_loopback, is_global, is_link_local, is_multicast, is_unspecified}]
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("ip.parse(addr)", "ip"), nil
		}
		s := args[0].String()
		// strip optional CIDR for host checks
		host := s
		if a, _, err := net.ParseCIDR(s); err == nil {
			host = a.String()
		}
		addr, err := netip.ParseAddr(host)
		if err != nil {
			// try host:port
			if h, _, e := net.SplitHostPort(s); e == nil {
				addr, err = netip.ParseAddr(h)
			}
		}
		if err != nil {
			return errRes(err.Error(), "ip"), nil
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("str", runtime.Str(addr.String()))
		ver := int64(4)
		if addr.Is6() {
			ver = 6
		}
		put("version", runtime.Int(ver))
		put("is_private", runtime.Bool(addr.IsPrivate()))
		put("is_loopback", runtime.Bool(addr.IsLoopback()))
		put("is_global", runtime.Bool(addr.IsGlobalUnicast()))
		put("is_link_local", runtime.Bool(addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast()))
		put("is_multicast", runtime.Bool(addr.IsMulticast()))
		put("is_unspecified", runtime.Bool(addr.IsUnspecified()))
		put("is_v4", runtime.Bool(addr.Is4()))
		put("is_v6", runtime.Bool(addr.Is6()))
		return runtime.Ok(m), nil
	}, 1)

	// ip.is_private(s) / is_loopback / is_valid
	set(p, "is_valid", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		_, err := netip.ParseAddr(args[0].String())
		return runtime.Bool(err == nil), nil
	}, 1)

	set(p, "is_private", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		a, err := netip.ParseAddr(args[0].String())
		return runtime.Bool(err == nil && a.IsPrivate()), nil
	}, 1)

	set(p, "is_loopback", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		a, err := netip.ParseAddr(args[0].String())
		return runtime.Bool(err == nil && a.IsLoopback()), nil
	}, 1)

	// ip.in_network(addr, cidr) -> bool
	set(p, "in_network", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		addr, err := netip.ParseAddr(args[0].String())
		if err != nil {
			return runtime.Bool(false), nil
		}
		prefix, err := netip.ParsePrefix(args[1].String())
		if err != nil {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(prefix.Contains(addr)), nil
	}, 2)

	// ip.network(cidr) -> Result[{network, bits, addr}]
	set(p, "network", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("ip.network(cidr)", "ip"), nil
		}
		prefix, err := netip.ParsePrefix(args[0].String())
		if err != nil {
			return errRes(err.Error(), "ip"), nil
		}
		prefix = prefix.Masked()
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("network", runtime.Str(prefix.String()))
		put("bits", runtime.Int(int64(prefix.Bits())))
		put("addr", runtime.Str(prefix.Addr().String()))
		return runtime.Ok(m), nil
	}, 1)

	return p
}
