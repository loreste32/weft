package netsafe

import (
	"net"
	"net/http"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", false},
		{"::1", false},
		{"8.8.8.8", false},
		{"169.254.169.254", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if got := IsBlockedIP(ip); got != tc.blocked {
			t.Errorf("%s: blocked=%v want %v", tc.ip, got, tc.blocked)
		}
	}
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "1")
	if IsBlockedIP(net.ParseIP("10.0.0.1")) {
		t.Fatal("private allowed when flag set")
	}
	// metadata still blocked
	if !IsBlockedIP(net.ParseIP("169.254.169.254")) {
		t.Fatal("metadata must stay blocked")
	}
}

func TestIsBlockedIPNil(t *testing.T) {
	if !IsBlockedIP(nil) {
		t.Fatal("nil IP should be blocked")
	}
}

func TestIsBlockedIPLinkLocal(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	if !IsBlockedIP(net.ParseIP("fe80::1")) {
		t.Fatal("link-local IPv6 should be blocked")
	}
}

func TestIsBlockedIPv6UniqueLocal(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	if !IsBlockedIP(net.ParseIP("fc00::1")) {
		t.Fatal("IPv6 unique local should be blocked")
	}
}

func TestAllowPrivate(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	if AllowPrivate() {
		t.Fatal("should be false by default")
	}
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "1")
	if !AllowPrivate() {
		t.Fatal("should be true with 1")
	}
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "true")
	if !AllowPrivate() {
		t.Fatal("should accept true")
	}
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "yes")
	if !AllowPrivate() {
		t.Fatal("should accept yes")
	}
}

func TestCheckURL(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	if err := CheckURL("https://example.com/pkg.zip"); err != nil {
		t.Fatal(err)
	}
	if err := CheckURL("ftp://x/y"); err == nil {
		t.Fatal("want scheme reject")
	}
	if err := CheckURL("http://169.254.169.254/latest"); err == nil {
		t.Fatal("want metadata reject")
	}
	if err := CheckURL("http://"); err == nil {
		t.Fatal("missing host")
	}
	if err := CheckURL("://bad"); err == nil {
		t.Fatal("bad url")
	}
}

func TestCheckHost(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	if err := CheckHost(""); err == nil {
		t.Fatal("empty host")
	}
	// loopback IP
	if err := CheckHost("127.0.0.1"); err != nil {
		t.Fatal("loopback should be ok")
	}
	// blocked IP
	if err := CheckHost("169.254.169.254"); err == nil {
		t.Fatal("metadata should be blocked")
	}
	// with port
	if err := CheckHost("127.0.0.1:8080"); err != nil {
		t.Fatal("loopback with port")
	}
	// IPv6 bracket
	if err := CheckHost("[::1]"); err != nil {
		t.Fatal("IPv6 loopback")
	}
}

func TestCheckHostAllowPrivate(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "1")
	if err := CheckHost("10.0.0.1"); err != nil {
		t.Fatal("private should be allowed")
	}
}

func TestSafeTransport(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	tr := SafeTransport(nil)
	if tr == nil {
		t.Fatal("nil transport")
	}
	// with base
	base := http.DefaultTransport.(*http.Transport).Clone()
	tr2 := SafeTransport(base)
	if tr2 == nil {
		t.Fatal("nil transport with base")
	}
}

func TestSafeHTTPClient(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	c := SafeHTTPClient(0) // defaults to 30s
	if c == nil {
		t.Fatal("nil client")
	}
	if c.Timeout == 0 {
		t.Fatal("should have timeout")
	}
}

func TestSafeHTTPClientCustomTimeout(t *testing.T) {
	c := SafeHTTPClient(5_000_000_000) // 5s
	if c.Timeout != 5_000_000_000 {
		t.Fatal("custom timeout")
	}
}

func TestCheckingTransportBlocksPrivate(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "")
	c := SafeHTTPClient(0)
	req, _ := http.NewRequest("GET", "http://10.0.0.1/", nil)
	_, err := c.Transport.RoundTrip(req)
	if err == nil {
		t.Fatal("should block private address")
	}
}

func TestSafeDialContextAllowPrivate(t *testing.T) {
	t.Setenv("WEFT_HTTP_ALLOW_PRIVATE", "1")
	// just verify it doesn't error on construction
	tr := SafeTransport(nil)
	if tr == nil {
		t.Fatal("nil")
	}
}
