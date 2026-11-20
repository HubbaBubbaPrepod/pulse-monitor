package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func idempotentOperationRequest(t *testing.T, a *app, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/api/extensions/example/components/a/operations?server=production-01", strings.NewReader(body))
	r.Host = "127.0.0.1:8080"
	cookie, csrf := testSession(t, a)
	r.AddCookie(cookie)
	r.Header.Set("X-CSRF-Token", csrf)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	return w
}

func TestManualOperationRetryReturnsSameIDAfterCompletionAndRestart(t *testing.T) {
	dir := t.TempDir()
	a, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	installExtensionFixture(t, a, "agent")
	if _, err = a.issueAgentToken("production-01"); err != nil {
		t.Fatal(err)
	}
	a.nodes["production-01"].Seen = time.Now()
	a.nodes["production-01"].Report.Capabilities = map[string]string{"moduleRuntime": "2"}
	if w := request(t, a, "PUT", "/api/extensions/example/components/a/configuration?server=production-01", `{"enabled":true,"grants":["state.read","actions.execute"],"config":{"token":"private-token"}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	key := "retry-request-123456"
	body := `{"action":"stop","input":{"containerId":"target"}}`
	first := idempotentOperationRequest(t, a, key, body)
	if first.Code != 202 {
		t.Fatal(first.Code, first.Body.String())
	}
	var op moduleOperation
	if err = json.Unmarshal(first.Body.Bytes(), &op); err != nil {
		t.Fatal(err)
	}
	op, err = a.moduleOperation(op.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"pending", "succeeded", "unknown"} {
		op.Status = phase
		if err = a.saveModuleOperation(op); err != nil {
			t.Fatal(err)
		}
		again := idempotentOperationRequest(t, a, key, body)
		var got moduleOperation
		json.Unmarshal(again.Body.Bytes(), &got)
		if again.Code != 202 || got.ID != op.ID {
			t.Fatalf("%s retry created/conflicted instead of returning original: %d %s", phase, again.Code, again.Body.String())
		}
	}
	a, err = newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	again := idempotentOperationRequest(t, a, key, body)
	var got moduleOperation
	json.Unmarshal(again.Body.Bytes(), &got)
	if again.Code != 202 || got.ID != op.ID {
		t.Fatalf("restart/offline retry: %d %s", again.Code, again.Body.String())
	}
	if strings.Contains(again.Body.String(), "requestDigest") || strings.Contains(again.Body.String(), "private-token") {
		t.Fatal("private request binding leaked")
	}
	ops, err := a.moduleOperations()
	if err != nil || len(ops) != 1 {
		t.Fatal("duplicate persisted operations", len(ops), err)
	}
}

func TestManualOperationKeyRejectsChangedPayloadAndMalformedKey(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installExtensionFixture(t, a, "agent")
	if _, err = a.issueAgentToken("production-01"); err != nil {
		t.Fatal(err)
	}
	a.nodes["production-01"].Seen = time.Now()
	a.nodes["production-01"].Report.Capabilities = map[string]string{"moduleRuntime": "2"}
	if w := request(t, a, "PUT", "/api/extensions/example/components/a/configuration?server=production-01", `{"enabled":true,"grants":["state.read","actions.execute"],"config":{"token":"private-token"}}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	body := `{"action":"stop","input":{"containerId":"target"}}`
	first := idempotentOperationRequest(t, a, "retry-request-123456", body)
	if first.Code != 202 {
		t.Fatal(first.Code, first.Body.String())
	}
	changed := idempotentOperationRequest(t, a, "retry-request-123456", `{"action":"stop","input":{"containerId":"other"}}`)
	if changed.Code != 409 {
		t.Fatal("changed payload accepted", changed.Code, changed.Body.String())
	}
	malformed := idempotentOperationRequest(t, a, "bad key", body)
	if malformed.Code != 400 {
		t.Fatal("malformed key accepted", malformed.Code, malformed.Body.String())
	}
}
