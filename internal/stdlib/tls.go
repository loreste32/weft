//go:build !js

package stdlib

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageTLS — TLS certificate inspection and connection testing.
func packageTLS() runtime.Value {
	p := pkg()

	dial := func(addr string, cfg *tls.Config) (*tls.Conn, error) {
		return tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp", addr, cfg,
		)
	}

	appendPort := func(addr string) string {
		_, _, err := net.SplitHostPort(addr)
		if err != nil {
			return addr + ":443"
		}
		return addr
	}

	set(p, "cert_info", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("tls.cert_info(host_port)", "tls"), nil
		}
		addr := appendPort(args[0].String())
		conn, err := dial(addr, &tls.Config{})
		if err != nil {
			return errRes(err.Error(), "tls"), nil
		}
		defer conn.Close()

		state := conn.ConnectionState()
		if len(state.PeerCertificates) == 0 {
			return errRes("no peer certificates", "tls"), nil
		}
		cert := state.PeerCertificates[0]

		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = append(mo.Keys, "subject", "issuer", "not_before", "not_after",
			"dns_names", "serial", "version", "days_remaining")
		mo.Vals["subject"] = runtime.Str(cert.Subject.CommonName)
		mo.Vals["issuer"] = runtime.Str(cert.Issuer.CommonName)
		mo.Vals["not_before"] = runtime.Str(cert.NotBefore.Format(time.RFC3339))
		mo.Vals["not_after"] = runtime.Str(cert.NotAfter.Format(time.RFC3339))
		days := int64(time.Until(cert.NotAfter).Hours() / 24)
		mo.Vals["days_remaining"] = runtime.Int(days)
		mo.Vals["serial"] = runtime.Str(cert.SerialNumber.String())
		mo.Vals["version"] = runtime.Int(int64(cert.Version))
		names := make([]runtime.Value, 0, len(cert.DNSNames))
		for _, n := range cert.DNSNames {
			names = append(names, runtime.Str(n))
		}
		mo.Vals["dns_names"] = runtime.List(names...)
		return runtime.Ok(m), nil
	}, 1)

	set(p, "verify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("tls.verify(host_port)", "tls"), nil
		}
		addr := appendPort(args[0].String())
		conn, err := dial(addr, &tls.Config{})
		if err != nil {
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			mo.Keys = append(mo.Keys, "valid", "error")
			mo.Vals["valid"] = runtime.Bool(false)
			mo.Vals["error"] = runtime.Str(err.Error())
			return runtime.Ok(m), nil
		}
		defer conn.Close()

		state := conn.ConnectionState()
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = append(mo.Keys, "valid", "version", "cipher")
		mo.Vals["valid"] = runtime.Bool(true)
		mo.Vals["version"] = runtime.Str(tlsVersionName(state.Version))
		mo.Vals["cipher"] = runtime.Str(tls.CipherSuiteName(state.CipherSuite))
		return runtime.Ok(m), nil
	}, 1)

	set(p, "chain", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("tls.chain(host_port)", "tls"), nil
		}
		addr := appendPort(args[0].String())
		conn, err := dial(addr, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return errRes(err.Error(), "tls"), nil
		}
		defer conn.Close()

		certs := conn.ConnectionState().PeerCertificates
		items := make([]runtime.Value, 0, len(certs))
		for _, c := range certs {
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			mo.Keys = append(mo.Keys, "subject", "issuer", "is_ca")
			mo.Vals["subject"] = runtime.Str(c.Subject.CommonName)
			mo.Vals["issuer"] = runtime.Str(c.Issuer.CommonName)
			mo.Vals["is_ca"] = runtime.Bool(c.IsCA)
			items = append(items, m)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	set(p, "expiry_check", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("tls.expiry_check(host_port, warn_days?)", "tls"), nil
		}
		addr := appendPort(args[0].String())
		warnDays := int64(30)
		if len(args) > 1 {
			if d, err := runtime.AsInt(args[1]); err == nil {
				warnDays = d
			}
		}

		conn, err := dial(addr, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return errRes(err.Error(), "tls"), nil
		}
		defer conn.Close()

		if len(conn.ConnectionState().PeerCertificates) == 0 {
			return errRes("no peer certificates", "tls"), nil
		}
		cert := conn.ConnectionState().PeerCertificates[0]
		days := int64(time.Until(cert.NotAfter).Hours() / 24)

		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		status := "ok"
		if days < 0 {
			status = "expired"
		} else if days < warnDays {
			status = "warning"
		}
		mo.Keys = append(mo.Keys, "status", "days_remaining", "expires", "subject")
		mo.Vals["status"] = runtime.Str(status)
		mo.Vals["days_remaining"] = runtime.Int(days)
		mo.Vals["expires"] = runtime.Str(cert.NotAfter.Format(time.RFC3339))
		mo.Vals["subject"] = runtime.Str(cert.Subject.CommonName)
		return runtime.Ok(m), nil
	}, 1)

	set(p, "supported_versions", func(args []runtime.Value) (runtime.Value, error) {
		versions := []runtime.Value{
			runtime.Str("TLS 1.0"),
			runtime.Str("TLS 1.1"),
			runtime.Str("TLS 1.2"),
			runtime.Str("TLS 1.3"),
		}
		return runtime.List(versions...), nil
	}, 0)

	set(p, "system_roots", func(args []runtime.Value) (runtime.Value, error) {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return errRes(err.Error(), "tls"), nil
		}
		// Subjects() is deprecated but still the only way to count roots
		return runtime.Ok(runtime.Int(int64(len(pool.Subjects())))), nil //nolint:staticcheck
	}, 0)

	return p
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}
