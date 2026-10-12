package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPublicHTTPSOriginAndSecureSession(t *testing.T) {
	t.Parallel()
	a, err := newApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(a.handler())
	defer srv.Close()
	a.publicHost = srv.Listener.Addr().String()
	client := srv.Client()
	client.Timeout = 5 * time.Second
	req, err := http.NewRequest("POST", srv.URL+"/api/auth/register", strings.NewReader(testCredentials))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", srv.URL)
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 201 {
		t.Fatalf("HTTPS registration: %d", response.StatusCode)
	}
	if len(response.Cookies()) != 1 || !response.Cookies()[0].Secure {
		t.Fatal("HTTPS cookie is not Secure")
	}
	req, err = http.NewRequest("GET", srv.URL+"/api/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://"+a.publicHost)
	response, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatalf("plaintext origin accepted: %d", response.StatusCode)
	}
}
