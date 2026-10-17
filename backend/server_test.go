package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAPIErrorsExposeStableCodeAndDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		name          string
		status        int
		message, code string
	}{
		{"credentials", 401, "Неверный email или пароль", "error.invalid_credentials"},
		{"registration", 409, "Владелец панели уже зарегистрирован", "error.registration_closed"},
		{"input", 400, "Invalid module package", "error.invalid_input"},
		{"internal", 500, "Could not save module data", "error.internal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			apiError(w, tc.status, tc.message)
			var v struct{ Error, Code string }
			if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
				t.Fatal(err)
			}
			if v.Error != tc.message || v.Code != tc.code {
				t.Fatalf("got %+v, want diagnostic and %s", v, tc.code)
			}
		})
	}
}

func TestRealtimeStreamDeliversUpdatesAndCancels(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.handler())
	defer srv.Close()
	cookie, _ := testSession(t, a)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/servers/production-01/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(cookie)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("unexpected stream type: %s", resp.Header.Get("Content-Type"))
	}
	scanner := bufio.NewScanner(resp.Body)
	var ticks []int64
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event struct {
			Tick     int64  `json:"tick"`
			ServerID string `json:"serverId"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		if event.ServerID != "production-01" {
			t.Fatalf("wrong server: %s", event.ServerID)
		}
		ticks = append(ticks, event.Tick)
		if len(ticks) == 2 {
			break
		}
	}
	if len(ticks) != 2 || ticks[1] != ticks[0] {
		t.Fatalf("expected two successive snapshots, got %v (error %v)", ticks, scanner.Err())
	}
	cancel()
}

func TestOriginGuard(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		origin string
		status int
	}{{"https://untrusted.example", 403}, {"http://127.0.0.1:3000", 200}} {
		r := httptest.NewRequest("GET", "/api/bootstrap", nil)
		r.Host = "127.0.0.1:8080"
		r.Header.Set("Origin", tc.origin)
		cookie, _ := testSession(t, a)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("origin %s: got %d, want %d", tc.origin, w.Code, tc.status)
		}
	}
}

func request(t *testing.T, a *app, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.Host = "127.0.0.1:8080"
	cookie, csrf := testSession(t, a)
	r.AddCookie(cookie)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", csrf)
	a.handler().ServeHTTP(w, r)
	return w
}

func TestBootstrap(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "GET", "/api/bootstrap", "")
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap status: got %d, want 200", w.Code)
	}
	var result struct {
		Servers []server `json:"servers"`
		Source  string   `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Servers) != 3 || result.Source != "waiting" {
		t.Fatalf("unexpected bootstrap: %+v", result)
	}
}

func TestArchiveBoundaryComparesInstants(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, stamp := range []string{"2026-10-12T10:00:10Z", "2026-10-12T10:00:10.700Z"} {
		if err := a.appendLog(logEntry{ServerID: "production-01", Time: stamp, Level: "INFO", Message: "boundary"}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := a.logHistory("production-01", "", "", "", "2026-10-12T10:00:10.500Z", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].Time != "2026-10-12T10:00:10Z" {
		t.Fatalf("wrong boundary page: %+v", page)
	}
}

func TestLogHistorySurvivesRestartAndFiltersOldRecords(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 125 {
		level := "INFO"
		if i == 0 {
			level = "ERROR"
		}
		if err := a.appendLog(logEntry{ServerID: "production-01", Level: level, Source: "test", Message: "retained record"}); err != nil {
			t.Fatal(err)
		}
	}
	restarted, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	w := request(t, restarted, "GET", "/api/servers/production-01/logs?level=ERROR&q=retained", "")
	if w.Code != 200 {
		t.Fatalf("logs status: %d", w.Code)
	}
	var result logPage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Level != "ERROR" {
		t.Fatalf("old filtered record lost: %+v", result)
	}
	w = request(t, restarted, "GET", "/api/servers/production-01/logs?limit=20&offset=100", "")
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 20 || result.Total < 125 {
		t.Fatalf("history pagination failed: %+v", result)
	}
}

func TestServerMutationsValidateAndPersist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"id":"test-node","name":"test-node","address":"https://192.0.2.1:9443","os":"Linux","group":"Test","online":true,"cpu":20,"memory":30,"disk":40,"interval":5,"threshold":85}`
	if w := request(t, a, "PUT", "/api/servers/test-node", body); w.Code != 200 {
		t.Fatalf("save status: %d: %s", w.Code, w.Body.String())
	}
	restarted, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(t, restarted, "GET", "/api/servers/test-node/snapshot", ""); w.Code != 200 {
		t.Fatalf("saved server lost: %d", w.Code)
	}
	for _, body := range []string{`{"name":"x","address":"javascript:alert(1)"}`, `{"name":"x","address":"https://u:p@example.com"}`, `{} {}`, `{"unknown":true}`} {
		if w := request(t, a, "PUT", "/api/servers/bad", body); w.Code != 400 {
			t.Fatalf("invalid payload accepted: %s (%d)", body, w.Code)
		}
	}
	for _, id := range []string{"test-node", "windows-02", "backup-03"} {
		if w := request(t, a, "DELETE", "/api/servers/"+id, ""); w.Code != 200 {
			t.Fatalf("delete status: %d", w.Code)
		}
	}
	if w := request(t, a, "DELETE", "/api/servers/production-01", ""); w.Code != 200 {
		t.Fatalf("last server deletion status: %d", w.Code)
	}
}

func TestWaitingSnapshotDoesNotFabricateTelemetry(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "GET", "/api/servers/production-01/snapshot", "")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var s snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if s.Source != "waiting" || len(s.Samples)+len(s.Processes)+len(s.Disks)+len(s.GPUs)+len(s.Packets) != 0 {
		t.Fatalf("fabricated telemetry: %+v", s)
	}
}

func TestHostGuardRejectsDNSRebinding(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://untrusted.example/api/bootstrap", nil)
	r.Header.Set("Origin", "http://untrusted.example")
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("unexpected rebinding status: %d", w.Code)
	}
}

var testSessions sync.Map

func testSession(t *testing.T, a *app) (*http.Cookie, string) {
	t.Helper()
	type entry struct {
		cookie *http.Cookie
		csrf   string
	}
	if v, ok := testSessions.Load(a); ok {
		x := v.(entry)
		return x.cookie, x.csrf
	}
	w := rawAuthRequest(a, "POST", "/api/auth/register", testCredentials, nil, "")
	if w.Code == 409 {
		w = rawAuthRequest(a, "POST", "/api/auth/login", `{"email":"owner@example.test","password":"test-only-strong-password-2026"}`, nil, "")
	}
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("test auth: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		CSRF string `json:"csrfToken"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	x := entry{w.Result().Cookies()[0], result.CSRF}
	testSessions.Store(a, x)
	return x.cookie, x.csrf
}
