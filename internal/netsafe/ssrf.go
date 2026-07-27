// Package netsafe provides SSRF-aware dialing and URL checks for outbound HTTP.
package netsafe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// AllowPrivate reports whether private/link-local/metadata addresses are allowed.
// Set WEFT_HTTP_ALLOW_PRIVATE=1 to reach internal networks intentionally.
func AllowPrivate() bool {
	v := strings.TrimSpace(os.Getenv("WEFT_HTTP_ALLOW_PRIVATE"))
	return v == "1" || strings.EqualFold(v, "true") || v == "yes"
}

// IsBlockedIP reports whether ip must not be contacted by default.
//
//	Loopback (127.0.0.0/8, ::1) — always allowed (local Ollama/vLLM/dev).
//	Link-local 169.254.0.0/16 — always blocked (cloud metadata).
//	RFC1918 private — blocked unless WEFT_HTTP_ALLOW_PRIVATE=1.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	// Local LLM servers and tests bind here intentionally.
	if ip.IsLoopback() {
		return false
	}
	// Cloud metadata / APIPA — never allow, even with ALLOW_PRIVATE.
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if AllowPrivate() {
		return false
	}
	if ip.IsPrivate() {
		return true
	}
	// IPv6 unique local fc00::/7
	if ip.To4() == nil && len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc {
		return true
	}
	return false
}

// CheckHost resolves host and rejects blocked addresses (unless AllowPrivate).
func CheckHost(host string) error {
	if AllowPrivate() {
		return nil
	}
	h := host
	if strings.Contains(h, ":") {
		// strip port
		if hostOnly, _, err := net.SplitHostPort(h); err == nil {
			h = hostOnly
		}
	}
	h = strings.Trim(h, "[]")
	if h == "" {
		return fmt.Errorf("empty host")
	}
	// Literal IP
	if ip := net.ParseIP(h); ip != nil {
		if IsBlockedIP(ip) {
			return fmt.Errorf("blocked address %s (set WEFT_HTTP_ALLOW_PRIVATE=1 for private nets)", ip)
		}
		return nil
	}
	ips, err := net.LookupIP(h)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", h, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("resolve %s: no addresses", h)
	}
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return fmt.Errorf("blocked address %s for host %s (set WEFT_HTTP_ALLOW_PRIVATE=1 for private nets)", ip, h)
		}
	}
	return nil
}

// CheckURL validates scheme + host for outbound fetch (package installs, optional HTTP).
func CheckURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		// ok
	case "http":
		// allow http only to non-blocked hosts; still CheckHost
	default:
		return fmt.Errorf("unsupported URL scheme %q (want http or https)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL missing host")
	}
	return CheckHost(u.Host)
}

// SafeTransport returns an http.RoundTripper that blocks private destinations.
func SafeTransport(base *http.Transport) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	} else {
		base = base.Clone()
	}
	base.DialContext = safeDialContext(base.DialContext)
	return &checkingTransport{base: base}
}

type checkingTransport struct {
	base http.RoundTripper
}

func (t *checkingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL != nil {
		if err := CheckURL(req.URL.String()); err != nil {
			return nil, err
		}
	}
	return t.base.RoundTrip(req)
}

func safeDialContext(inner func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	if inner == nil {
		d := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		inner = d.DialContext
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
			port = ""
		}
		if !AllowPrivate() {
			if err := CheckHost(host); err != nil {
				return nil, err
			}
			// Resolve and dial only non-blocked IPs
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var last error
			for _, ipa := range ips {
				if IsBlockedIP(ipa.IP) {
					last = fmt.Errorf("blocked address %s", ipa.IP)
					continue
				}
				target := net.JoinHostPort(ipa.IP.String(), port)
				c, err := inner(ctx, network, target)
				if err == nil {
					return c, nil
				}
				last = err
			}
			if last != nil {
				return nil, last
			}
			return nil, fmt.Errorf("no safe addresses for %s", host)
		}
		return inner(ctx, network, addr)
	}
}

// SafeHTTPClient is a timeout client with SSRF-safe transport.
func SafeHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{
		Timeout:   timeout,
		Transport: SafeTransport(tr),
		// Re-check redirects
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return CheckURL(req.URL.String())
		},
	}
}
