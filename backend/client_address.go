package main

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Published Cloudflare proxy ranges, verified 2026-10-23:
// https://www.cloudflare.com/ips-v4 and https://www.cloudflare.com/ips-v6.
var cloudflarePrefixes = func() []netip.Prefix {
	var out []netip.Prefix
	for _, raw := range strings.Fields(`173.245.48.0/20 103.21.244.0/22 103.22.200.0/22 103.31.4.0/22 141.101.64.0/18 108.162.192.0/18 190.93.240.0/20 188.114.96.0/20 197.234.240.0/22 198.41.128.0/17 162.158.0.0/15 104.16.0.0/13 104.24.0.0/14 172.64.0.0/13 131.0.72.0/22 2400:cb00::/32 2606:4700::/32 2803:f800::/32 2405:b500::/32 2405:8100::/32 2a06:98c0::/29 2c0f:f248::/32`) {
		out = append(out, netip.MustParsePrefix(raw))
	}
	return out
}()

func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	peer = peer.Unmap()
	for _, prefix := range cloudflarePrefixes {
		if prefix.Contains(peer) {
			client, err := netip.ParseAddr(r.Header.Get("CF-Connecting-IP"))
			if err == nil {
				return client.Unmap().String()
			}
			break
		}
	}
	return peer.String()
}
