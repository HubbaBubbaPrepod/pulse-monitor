package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testCredentials = `{"name":"Test owner","email":"owner@example.test","password":"test-only-strong-password-2026"}`

func rawAuthRequest(a *app, method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "127.0.0.1:8080"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	return w
}

func TestExpiredSessionCannotReadMetrics(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cookie, _ := testSession(t, a)
	a.auth.mu.Lock()
	key := sha256.Sum256([]byte(cookie.Value))
	session := a.auth.sessions[key]
	session.Expires = time.Now().Add(-time.Second)
	a.auth.sessions[key] = session
	a.auth.mu.Unlock()
	if w := rawAuthRequest(a, "GET", "/api/bootstrap", "", cookie, ""); w.Code != 401 {
		t.Fatalf("expired session: %d", w.Code)
	}
}

func TestLoginAttemptsAreLimited(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if w := rawAuthRequest(a, "POST", "/api/auth/login", `{"email":"owner@example.test","password":"wrong-password"}`, nil, ""); w.Code != 401 {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	if w := rawAuthRequest(a, "POST", "/api/auth/login", `{"email":"owner@example.test","password":"wrong-password"}`, nil, ""); w.Code != 429 {
		t.Fatalf("rate limit: %d", w.Code)
	}
}

func TestAuthRequiresSessionForEveryMonitoringEndpoint(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/bootstrap", "/api/servers/production-01/snapshot", "/api/servers/production-01/logs", "/api/servers/production-01/stream"} {
		if w := rawAuthRequest(a, "GET", path, "", nil, ""); w.Code != 401 {
			t.Errorf("%s: got %d, want 401", path, w.Code)
		}
	}
}

func TestRegistrationPersistsOwnerAndClosesSetup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	w := rawAuthRequest(a, "POST", "/api/auth/register", testCredentials, nil, "")
	if w.Code != 201 {
		t.Fatalf("registration: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unsafe cookie: %+v", cookies)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("password")) || bytes.Contains(w.Body.Bytes(), []byte("hash")) {
		t.Fatal("credentials leaked in response")
	}
	saved, err := os.ReadFile(filepath.Join(dir, "account.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(saved, []byte("test-only-strong-password")) || !bytes.Contains(saved, []byte("$2a$")) {
		t.Fatal("password not stored as bcrypt")
	}
	if w := rawAuthRequest(a, "POST", "/api/auth/register", testCredentials, nil, ""); w.Code != 409 {
		t.Fatalf("second owner allowed: %d", w.Code)
	}
	restarted, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	w = rawAuthRequest(restarted, "POST", "/api/auth/login", `{"email":"OWNER@example.test","password":"test-only-strong-password-2026"}`, nil, "")
	if w.Code != 200 {
		t.Fatalf("owner lost after restart: %d %s", w.Code, w.Body.String())
	}
}

func TestLoginLogoutAndCSRF(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := rawAuthRequest(a, "POST", "/api/auth/register", testCredentials, nil, "")
	if w.Code != 201 {
		t.Fatalf("register: %d", w.Code)
	}
	cookie := w.Result().Cookies()[0]
	var result struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.CSRFToken == "" {
		t.Fatal("missing csrf token")
	}
	if w := rawAuthRequest(a, "GET", "/api/bootstrap", "", cookie, ""); w.Code != 200 {
		t.Fatalf("valid session rejected: %d", w.Code)
	}
	for _, token := range []string{"", "wrong"} {
		if w := rawAuthRequest(a, "POST", "/api/auth/logout", "{}", cookie, token); w.Code != 403 {
			t.Fatalf("csrf bypass: %d", w.Code)
		}
	}
	w = rawAuthRequest(a, "POST", "/api/auth/logout", "{}", cookie, result.CSRFToken)
	if w.Code != 200 {
		t.Fatalf("logout: %d", w.Code)
	}
	if w := rawAuthRequest(a, "GET", "/api/bootstrap", "", cookie, ""); w.Code != 401 {
		t.Fatal("logout did not revoke session")
	}
	bad := rawAuthRequest(a, "POST", "/api/auth/login", `{"email":"owner@example.test","password":"wrong-password-2026"}`, nil, "")
	unknown := rawAuthRequest(a, "POST", "/api/auth/login", `{"email":"unknown@example.test","password":"wrong-password-2026"}`, nil, "")
	if bad.Code != 401 || unknown.Code != 401 || bad.Body.String() != unknown.Body.String() {
		t.Fatal("login error reveals identity or accepts wrong password")
	}
}

func TestRegistrationRejectsWeakAndOversizePasswords(t *testing.T) {
	t.Parallel()
	for _, password := range []string{"short", strings.Repeat("x", 73)} {
		a, err := newTestApp(t, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(map[string]string{"name": "Owner", "email": "owner@example.test", "password": password})
		if w := rawAuthRequest(a, "POST", "/api/auth/register", string(body), nil, ""); w.Code != 400 {
			t.Errorf("password length %d: got %d", len(password), w.Code)
		}
	}
}

func TestNewServerDoesNotTrustClientOperatingSystem(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := rawAuthRequest(a, "POST", "/api/auth/register", testCredentials, nil, "")
	if w.Code != 201 {
		t.Fatalf("register: %d", w.Code)
	}
	var auth struct {
		CSRFToken string `json:"csrfToken"`
	}
	json.Unmarshal(w.Body.Bytes(), &auth)
	w = rawAuthRequest(a, "PUT", "/api/servers/new-node", `{"id":"new-node","name":"new-node","address":"https://192.0.2.8:9443","os":"Windows 11","online":true,"cpu":99,"memory":99,"disk":99,"group":"Test","interval":5,"threshold":85}`, w.Result().Cookies()[0], auth.CSRFToken)
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	var s server
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if s.OS != "" || s.Online || s.CPU != 0 {
		t.Fatalf("client supplied fake system information: %+v", s)
	}
}

func TestGPUProcessMetrics(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range a.serverList() {
		value := a.snapshot(s, 60)
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var response struct {
			Processes []struct {
				Name      string
				GPU       *float64 `json:"gpu"`
				GPUMemory *float64 `json:"gpuMemory"`
			}
			GPUs []gpu `json:"gpus"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatal(err)
		}
		active := false
		total := 0.0
		for _, p := range response.Processes {
			if len(response.GPUs) == 0 {
				if p.GPU != nil || p.GPUMemory != nil {
					t.Fatal("GPU metrics advertised on node without GPU")
				}
				continue
			}
			if p.GPU == nil || p.GPUMemory == nil {
				t.Fatalf("missing GPU metric for %s", p.Name)
			}
			if *p.GPU > 0 {
				active = true
			}
			total += *p.GPUMemory
		}
		if len(response.GPUs) > 0 && (!active || total <= 0 || total > response.GPUs[0].MemoryTotal*1024) {
			t.Fatal("GPU processes missing or memory exceeds GPU capacity")
		}
	}
}
