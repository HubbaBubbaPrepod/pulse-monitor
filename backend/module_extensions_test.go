package main

import (
	"encoding/json"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func installExtensionFixture(t *testing.T, a *app, location string) {
	t.Helper()
	var value map[string]any
	json.Unmarshal(v2PackageJSON(t, location), &value)
	c := value["components"].([]any)[0].(map[string]any)
	c["fields"] = []any{map[string]any{"key": "token", "label": "Token", "type": "password", "required": true}}
	c["triggers"] = []any{map[string]any{"type": "manual", "action": "inspect", "label": "Inspect", "mutating": false, "inputFields": []any{map[string]any{"key": "containerId", "label": "ID", "type": "text", "required": true}}}, map[string]any{"type": "manual", "action": "stop", "label": "Stop", "mutating": true, "inputFields": []any{map[string]any{"key": "containerId", "label": "ID", "type": "text", "required": true}}}}
	c["views"] = []any{map[string]any{"kind": "table", "key": "containers", "title": "Containers", "columns": []any{}, "rowKey": "id", "actions": []string{"inspect", "stop"}}}
	raw, _ := json.Marshal(value)
	w := request(t, a, "POST", "/api/extensions", string(raw))
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestModuleExtensionConfigurationProtectsSecretsAndGrants(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installExtensionFixture(t, a, "monitor")
	path := "/api/extensions/example/components/a/configuration"
	if w := request(t, a, "PUT", path, `{"enabled":true,"grants":[],"config":{"token":"private-token"}}`); w.Code != 403 {
		t.Fatal("required grant not enforced", w.Code, w.Body.String())
	}
	if w := request(t, a, "PUT", path, `{"enabled":true,"grants":["state.read"],"config":{"token":"private-token"}}`); w.Code != 200 || strings.Contains(w.Body.String(), "private-token") {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "GET", path, ""); w.Code != 200 || strings.Contains(w.Body.String(), "private-token") || !strings.Contains(w.Body.String(), "token") {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "PUT", path, `{"enabled":true,"grants":["state.read"],"config":{}}`); w.Code != 200 {
		t.Fatal("omitted secret not preserved", w.Code, w.Body.String())
	}
	opPath := "/api/extensions/example/components/a/operations"
	if w := request(t, a, "POST", opPath, `{"action":"stop","input":{"containerId":"target"}}`); w.Code != 403 {
		t.Fatal("mutation without action grant", w.Code, w.Body.String())
	}
	if w := request(t, a, "POST", opPath, `{"action":"inspect","input":{"containerId":"target"}}`); w.Code != 202 || strings.Contains(w.Body.String(), "private-token") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestModuleExtensionAgentClaimIsScopedAndNeverReplayed(t *testing.T) {
	dir := t.TempDir()
	a, err := newTestApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	installExtensionFixture(t, a, "agent")
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	a.telemetryMu.Lock()
	a.nodes["production-01"].Seen = time.Now()
	a.nodes["production-01"].Report.Capabilities = map[string]string{"moduleRuntime": "2"}
	a.telemetryMu.Unlock()
	configPath := "/api/extensions/example/components/a/configuration?server=production-01"
	if w := request(t, a, "PUT", configPath, `{"enabled":true,"grants":["state.read","actions.execute"],"config":{"token":"private-token"}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w := request(t, a, "POST", "/api/extensions/example/components/a/operations?server=production-01", `{"action":"stop","input":{"containerId":"target"}}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var operation moduleOperation
	json.Unmarshal(w.Body.Bytes(), &operation)
	agentRequest := func(id string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/agent/"+id+"/module-operations?os="+runtime.GOOS+"&arch="+runtime.GOARCH, nil)
		r.Host = "127.0.0.1:8080"
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		return w
	}
	if w := agentRequest("windows-02"); w.Code != 401 {
		t.Fatal("another node accepted", w.Code)
	}
	if w := agentRequest("production-01"); w.Code != 200 || !strings.Contains(w.Body.String(), operation.ID) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := agentRequest("production-01"); w.Code != 200 || strings.Contains(w.Body.String(), operation.ID) {
		t.Fatal("operation replayed", w.Code, w.Body.String())
	}
	if w := request(t, a, "POST", "/api/extensions/example/components/a/operations?server=production-01", `{"action":"stop","input":{"containerId":"target"}}`); w.Code != 409 {
		t.Fatal("conflicting action accepted", w.Code, w.Body.String())
	}
	a, err = newTestApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(t, a, "GET", "/api/module-operations/"+operation.ID, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"unknown"`) {
		t.Fatal("restart did not preserve uncertain outcome", w.Code, w.Body.String())
	}
}

func TestModuleExtensionReinstallNeverRestoresOldGrants(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installExtensionFixture(t, a, "monitor")
	path := "/api/extensions/example/components/a/configuration"
	if w := request(t, a, "PUT", path, `{"enabled":true,"grants":["state.read"],"config":{"token":"private-token"}}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "DELETE", "/api/extensions/example", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	installExtensionFixture(t, a, "monitor")
	w := request(t, a, "GET", path, "")
	var value struct {
		Enabled bool
		Grants  []string
	}
	json.Unmarshal(w.Body.Bytes(), &value)
	if w.Code != 200 || value.Enabled || len(value.Grants) > 0 {
		t.Fatal("reinstall revived native execution without new consent", w.Code, w.Body.String())
	}
}

func TestModuleExtensionConfigurationChangeRevokesQueuedOperations(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installExtensionFixture(t, a, "monitor")
	path := "/api/extensions/example/components/a/configuration"
	if w := request(t, a, "PUT", path, `{"enabled":true,"grants":["state.read"],"config":{"token":"first-secret"}}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	w := request(t, a, "POST", "/api/extensions/example/components/a/operations", `{"action":"inspect","input":{"containerId":"target"}}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var op moduleOperation
	json.Unmarshal(w.Body.Bytes(), &op)
	if w := request(t, a, "PUT", path, `{"enabled":true,"grants":["state.read"],"config":{"token":"new-secret"}}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	w = request(t, a, "GET", "/api/module-operations/"+op.ID, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"cancelled"`) {
		t.Fatal("old confirmation was rebound to new configuration", w.Code, w.Body.String())
	}
}
