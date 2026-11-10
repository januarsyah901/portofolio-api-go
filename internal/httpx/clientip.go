package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP resolves the address of the real client.
//
// Behind a reverse proxy every request arrives from the proxy's address, so per-client
// limits would collapse into one shared bucket. When the TCP peer is a trusted proxy,
// the client is the right-most X-Forwarded-For entry that is not itself a trusted
// proxy. That entry was appended by our own proxy, so a client cannot forge it.
// Requests from any other peer use the peer address and ignore the header.
type ClientIP struct {
	Trusted []netip.Prefix
}

// Resolve returns the client IP for r.
func (c ClientIP) Resolve(r *http.Request) string {
	peer := peerAddr(r)
	if !c.trusted(peer) {
		return addrString(peer, r.RemoteAddr)
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break // malformed chain: stop trusting it
		}
		ip = ip.Unmap()
		if !c.trusted(ip) {
			return ip.String()
		}
	}
	return addrString(peer, r.RemoteAddr)
}

func (c ClientIP) trusted(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	for _, p := range c.Trusted {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func peerAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}

func addrString(ip netip.Addr, fallback string) string {
	if ip.IsValid() {
		return ip.String()
	}
	return fallback
}
