package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersProtectHTMLAPIAndErrors(t *testing.T) {
	for _, path := range []string{"/", "/api/health", "/missing"} {
		for _, secure := range []bool{false, true} {
			r := httptest.NewRequest("GET", "http://localhost"+path, nil)
			if secure {
				r.TLS = &tls.ConnectionState{}
			}
			w := httptest.NewRecorder()
			securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404); w.Write([]byte("page")) })).ServeHTTP(w, r)
			if w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("unprotected response", w.Header())
			}
			if (w.Header().Get("Strict-Transport-Security") != "") != secure {
				t.Fatal("HSTS must follow TLS, not spoofable proxy headers")
			}
		}
	}
}
