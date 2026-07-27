package netsafe

import (
	"net"
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
}
