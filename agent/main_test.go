package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDeliverDoesNotForwardTokenOnRedirect(t *testing.T) {
	t.Parallel()
	var forwarded atomic.Bool
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Store(true) }))
	defer sink.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, sink.URL, 307) }))
	defer server.Close()
	client := &http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	if err := deliver(context.Background(), client, server.URL, "test-secret", []byte(`{}`)); err == nil {
		t.Fatal("redirect was accepted")
	}
	if forwarded.Load() {
		t.Fatal("agent token was forwarded")
	}
}

func TestBackendURLValidation(t *testing.T) {
	for _, tc := range []struct {
		name, url string
		ok        bool
	}{{"localhost", "http://127.0.0.1:8080", true}, {"remote tls", "https://monitor.example.test", true}, {"remote plaintext", "http://example.test", false}, {"embedded password", "https://user:pass@example.test", false}, {"path", "https://example.test/path", false}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := backendURL(tc.url)
			if (err == nil) != tc.ok {
				t.Fatalf("url: %v", err)
			}
		})
	}
}
