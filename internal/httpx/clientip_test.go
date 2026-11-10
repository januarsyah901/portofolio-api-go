package httpx

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	c := ClientIP{Trusted: []netip.Prefix{netip.MustParsePrefix("172.18.0.0/16"), netip.MustParsePrefix("10.0.0.1/32")}}

	cases := []struct {
		name, remote, xff, want string
	}{
		{"direct client, header ignored", "203.0.113.9:5000", "1.2.3.4", "203.0.113.9"},
		{"via trusted proxy", "172.18.0.4:4000", "198.51.100.7", "198.51.100.7"},
		{"spoofed left entries ignored", "172.18.0.4:4000", "6.6.6.6, 198.51.100.7", "198.51.100.7"},
		{"skips chained trusted proxies", "172.18.0.4:4000", "198.51.100.7, 10.0.0.1", "198.51.100.7"},
		{"trusted proxy without header", "172.18.0.4:4000", "", "172.18.0.4"},
		{"malformed header falls back to peer", "172.18.0.4:4000", "not-an-ip", "172.18.0.4"},
		{"ipv4-mapped peer", "[::ffff:172.18.0.4]:4000", "198.51.100.7", "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", nil)
			r.RemoteAddr = tc.remote
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := c.Resolve(r); got != tc.want {
				t.Errorf("Resolve = %q, want %q", got, tc.want)
			}
		})
	}

	if got := (ClientIP{}).Resolve(httptest.NewRequest("GET", "/", nil)); got != "192.0.2.1" {
		t.Errorf("no trusted proxies: got %q", got)
	}
}
