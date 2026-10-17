package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMarketplaceFetchRejectsRedirects(t *testing.T) {
	var reached atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Store(true)
	}))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer redirect.Close()
	origin, err := url.Parse(redirect.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := marketplaceFetch(context.Background(), origin, "/api/v1/catalog", 1024); err == nil {
		t.Fatal("redirect was accepted")
	}
	if reached.Load() {
		t.Fatal("redirect destination was contacted")
	}
}

func TestMarketplaceRequiresOwnerSession(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, method, path string }{
		{"settings", "GET", "/api/marketplace"},
		{"catalog", "GET", "/api/marketplace/catalog"},
		{"versions", "GET", "/api/marketplace/versions"},
		{"configure", "PUT", "/api/marketplace"},
		{"install", "POST", "/api/marketplace/install"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Host = "127.0.0.1:8080"
			r.Header.Set("Content-Type", "application/json")
			a.handler().ServeHTTP(w, r)
			if w.Code != 401 {
				t.Fatalf("anonymous access: %d", w.Code)
			}
		})
	}
}

func TestMarketplaceOptInAndInstallation(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manifest := collectorPackage(t)
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])
	metadata := map[string]any{
		"owner": "Publisher", "id": manifest.ID, "version": manifest.Version,
		"name": manifest.Name, "sha256": digest, "size": len(data),
		"permissions": []string{}, "platforms": []string{"windows-amd64"},
	}
	var calls atomic.Int64
	var corrupt, withdrawn atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Cookie") != "" || r.Header.Get("X-CSRF-Token") != "" {
			t.Error("Monitor credentials leaked upstream")
		}
		switch strings.TrimRight(r.URL.Path, "/") {
		case "/api/v1/catalog":
			writeJSON(w, 200, map[string]any{"items": []any{metadata}, "nextCursor": nil})
		case "/api/v1/modules/Publisher/example/versions/1.0.0":
			if withdrawn.Load() {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, 200, metadata)
		case "/api/v1/packages/" + digest:
			if corrupt.Load() {
				w.Write([]byte(strings.Repeat("x", len(data))))
				return
			}
			w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	for _, path := range []string{"/api/marketplace", "/api/marketplace/catalog"} {
		w := request(t, a, "GET", path, "")
		if path == "/api/marketplace" && (w.Code != 200 || !strings.Contains(w.Body.String(), `"enabled":false`)) {
			t.Fatalf("default configuration: %d %s", w.Code, w.Body.String())
		}
		if path != "/api/marketplace" && w.Code != 409 {
			t.Fatalf("disabled: %d", w.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("disabled marketplace contacted upstream")
	}
	config, _ := json.Marshal(map[string]any{"enabled": true, "url": upstream.URL})
	if w := request(t, a, "PUT", "/api/marketplace", string(config)); w.Code != 200 {
		t.Fatalf("enable: %d %s", w.Code, w.Body.String())
	}
	if w := request(t, a, "GET", "/api/marketplace/catalog", ""); w.Code != 200 {
		t.Fatalf("catalog: %d %s", w.Code, w.Body.String())
	}
	input := `{"owner":"Publisher","id":"example","version":"1.0.0","sha256":"` + digest + `"}`
	wrong := strings.Replace(input, digest, strings.Repeat("0", 64), 1)
	if w := request(t, a, "POST", "/api/marketplace/install", wrong); w.Code != 409 {
		t.Fatalf("changed digest: %d %s", w.Code, w.Body.String())
	}
	corrupt.Store(true)
	if w := request(t, a, "POST", "/api/marketplace/install", input); w.Code != 409 {
		t.Fatalf("corrupt package: %d", w.Code)
	}
	if _, err := a.module("example"); err == nil {
		t.Fatal("corrupt package installed")
	}
	corrupt.Store(false)
	withdrawn.Store(true)
	if w := request(t, a, "POST", "/api/marketplace/install", input); w.Code != 502 {
		t.Fatalf("withdrawn package: %d", w.Code)
	}
	withdrawn.Store(false)
	if w := request(t, a, "POST", "/api/marketplace/install", input); w.Code != 201 {
		t.Fatalf("install: %d %s", w.Code, w.Body.String())
	}
	if _, err := a.module("example"); err != nil {
		t.Fatal(err)
	}
	if w := request(t, a, "POST", "/api/marketplace/install", input); w.Code != 409 {
		t.Fatalf("duplicate: %d", w.Code)
	}
	b, err := newApp(a.dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(t, b, "GET", "/api/marketplace", ""); !strings.Contains(w.Body.String(), upstream.URL) {
		t.Fatal("configuration did not survive restart")
	}
	if w := request(t, a, "PUT", "/api/marketplace", `{"enabled":false,"url":""}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	before := calls.Load()
	if w := request(t, a, "GET", "/api/marketplace/catalog", ""); w.Code != 409 {
		t.Fatal(w.Code)
	}
	if calls.Load() != before {
		t.Fatal("disabled connection still used")
	}
}

func TestMarketplaceRejectsUnsafeSources(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, url string }{
		{"credentials", "https://user:password@example.com"},
		{"plaintext", "http://example.com"},
		{"private", "https://192.168.1.1"},
		{"metadata", "https://169.254.169.254"},
		{"path", "https://example.com/other"},
		{"query", "https://example.com?token=secret"},
		{"shared network", "https://100.100.100.100"},
		{"translated metadata", "https://[64:ff9b::a9fe:a9fe]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"enabled": true, "url": tc.url})
			if w := request(t, a, "PUT", "/api/marketplace", string(body)); w.Code != 400 {
				t.Fatalf("unsafe source accepted: %d", w.Code)
			}
		})
	}
}

type extensionInstallBarrier struct {
	*httptest.ResponseRecorder
	entered chan struct{}
	release chan struct{}
}

func (w *extensionInstallBarrier) SetReadDeadline(time.Time) error {
	close(w.entered)
	<-w.release
	return nil
}
func TestMarketplaceV2RechecksPolicyAtInstallationBoundary(t *testing.T) {
	for _, mode := range []string{"disabled", "identity"} {
		t.Run(mode, func(t *testing.T) {
			a, err := newTestApp(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			raw := extensionFixturePackage(t, "monitor")
			sum := sha256.Sum256(raw)
			digest := hex.EncodeToString(sum[:])
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/v1/packages/") {
					w.Write(raw)
					return
				}
				writeJSON(w, 200, map[string]any{"owner": "Publisher", "id": "example", "version": "2.0.0", "sha256": digest, "size": len(raw)})
			}))
			defer upstream.Close()
			config, _ := json.Marshal(marketplaceConfig{Enabled: true, URL: upstream.URL})
			if w := request(t, a, "PUT", "/api/marketplace", string(config)); w.Code != 200 {
				t.Fatal(w.Code)
			}
			input, _ := json.Marshal(map[string]string{"owner": "Publisher", "id": "example", "version": "2.0.0", "sha256": digest})
			w := &extensionInstallBarrier{ResponseRecorder: httptest.NewRecorder(), entered: make(chan struct{}), release: make(chan struct{})}
			r := httptest.NewRequest("POST", "/api/marketplace/install", strings.NewReader(string(input)))
			r.Host = "127.0.0.1:8080"
			done := make(chan struct{})
			go func() { defer close(done); a.marketplaceInstall(w, r) }()
			select {
			case <-w.entered:
			case <-time.After(5 * time.Second):
				close(w.release)
				t.Fatal("installer boundary not reached")
			}
			if mode == "disabled" {
				if response := request(t, a, "PUT", "/api/marketplace", `{"enabled":false,"url":"`+upstream.URL+`"}`); response.Code != 200 {
					t.Error(response.Code)
				}
			} else {
				var p map[string]any
				json.Unmarshal(raw, &p)
				p["version"] = "2.0.1"
				newer, _ := json.Marshal(p)
				if response := request(t, a, "POST", "/api/extensions", string(newer)); response.Code != 201 {
					t.Error(response.Code, response.Body.String())
				}
			}
			close(w.release)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("installer blocked")
			}
			if w.Code != 409 {
				t.Fatal("catalog policy changed during transfer but install continued", mode, w.Code)
			}
		})
	}
}
