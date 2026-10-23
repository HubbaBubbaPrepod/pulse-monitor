package main

import (
	"net/http/httptest"
	"testing"
)

func TestClientAddressTrustsOnlyCloudflarePeers(t *testing.T) {
	for _, tc := range []struct{ peer, header, want string }{
		{"192.0.2.1:123", "192.0.2.9", "192.0.2.1"},
		{"104.16.1.1:123", "192.0.2.9", "192.0.2.9"},
		{"104.16.1.1:123", "192.0.2.9, 192.0.2.8", "104.16.1.1"},
		{"[2606:4700::1]:123", "2001:db8::1", "2001:db8::1"},
	} {
		r := httptest.NewRequest("POST", "https://localhost/api/auth/login", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("CF-Connecting-IP", tc.header)
		r.Header.Set("X-Forwarded-For", "192.0.2.99")
		if got := clientAddress(r); got != tc.want {
			t.Fatalf("peer %s: got %s want %s", tc.peer, got, tc.want)
		}
	}
}
