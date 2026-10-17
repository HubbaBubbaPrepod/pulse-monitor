package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExtensionUploadAllowsBoundedSlowTransfer(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cookie, csrf := testSession(t, a)
	server := httptest.NewUnstartedServer(a.handler())
	server.Config.ReadTimeout = 50 * time.Millisecond
	server.Start()
	defer server.Close()
	raw := extensionFixturePackage(t, "monitor")
	request, err := http.NewRequest("POST", server.URL+"/api/extensions", &delayedModuleBody{Reader: strings.NewReader(string(raw))})
	if err != nil {
		t.Fatal(err)
	}
	request.ContentLength = int64(len(raw))
	request.AddCookie(cookie)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", csrf)
	client := server.Client()
	client.Timeout = 10 * time.Second
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 201 {
		t.Fatal("v2 upload uses ordinary short request timeout", response.StatusCode)
	}
}

func TestModuleSchedulerQueuesOnlyEnabledIntervalWork(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	json.Unmarshal(extensionFixturePackage(t, "agent"), &value)
	c := value["components"].([]any)[0].(map[string]any)
	c["triggers"] = append(c["triggers"].([]any), map[string]any{"type": "interval", "seconds": 5})
	raw, _ := json.Marshal(value)
	if w := request(t, a, "POST", "/api/extensions", string(raw)); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	a.issueAgentToken("production-01")
	a.telemetryMu.Lock()
	a.nodes["production-01"].Seen = time.Now()
	a.nodes["production-01"].Report.Capabilities = map[string]string{"moduleRuntime": "2"}
	a.telemetryMu.Unlock()
	now := time.Now()
	if err := a.scheduleExtensionOperations(now); err != nil {
		t.Fatal(err)
	}
	ops, _ := a.moduleOperations()
	if len(ops) != 0 {
		t.Fatal("disabled component scheduled")
	}
	if w := request(t, a, "PUT", "/api/extensions/example/components/a/configuration?server=production-01", `{"enabled":true,"grants":["state.read"],"config":{"token":"private-token"}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := a.scheduleExtensionOperations(now); err != nil {
		t.Fatal(err)
	}
	if err := a.scheduleExtensionOperations(now); err != nil {
		t.Fatal(err)
	}
	ops, err = a.moduleOperations()
	if err != nil || len(ops) != 1 || ops[0].Trigger != "interval" || ops[0].Mutating {
		t.Fatal("interval schedule duplicated or invoked control action", ops, err)
	}
	a.extensionMu.Lock()
	lease, err := a.claimModuleOperation(ops[0], runtime.GOOS+"-"+runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	op, _ := a.moduleOperation(lease.ID)
	err = a.finishModuleOperation(op, moduleInvocationResult{Protocol: 2, Status: "ok", Data: json.RawMessage(`{"containers":[{"id":"actual-container"}]}`)}, nil)
	a.extensionMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "GET", "/api/extensions/example/components/a/data?server=production-01", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "actual-container") {
		t.Fatal("scheduled snapshot missing", w.Code, w.Body.String())
	}
	w = request(t, a, "GET", "/api/extensions/example/components/a/data?server=windows-02", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "actual-container") {
		t.Fatal("snapshot leaked to another node", w.Code, w.Body.String())
	}
}
