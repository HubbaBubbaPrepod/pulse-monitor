package main

import (
	"encoding/json"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAgentVerificationSurvivesDeadlineWithoutReplayingCommand(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installExtensionFixture(t, a, "agent")
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	a.nodes["production-01"].Seen = time.Now()
	a.nodes["production-01"].Report.Capabilities = map[string]string{"moduleRuntime": "2"}
	configPath := "/api/extensions/example/components/a/configuration?server=production-01"
	if w := request(t, a, "PUT", configPath, `{"enabled":true,"grants":["state.read","actions.execute"],"config":{"token":"private-token"}}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	w := idempotentOperationRequest(t, a, "verification-request-123", `{"action":"stop","input":{"containerId":"target"}}`)
	var op moduleOperation
	json.Unmarshal(w.Body.Bytes(), &op)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	agentRequest := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8080"
		r.Header.Set("Authorization", "Bearer "+token)
 r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		return w
	}
	claim := agentRequest("GET", "/api/agent/production-01/module-operations?os="+runtime.GOOS+"&arch="+runtime.GOARCH, "")
	if claim.Code != 200 || !strings.Contains(claim.Body.String(), `"configurationDigest"`) {
		t.Fatal("lease has no durable config binding", claim.Code, claim.Body.String())
	}
	resultPath := "/api/agent/production-01/module-operations/" + op.ID + "/result"
	phase := agentRequest("POST", resultPath, `{"phase":"verifying"}`)
	if phase.Code != 200 {
		t.Fatal("verification phase rejected", phase.Code, phase.Body.String())
	}
	saved, err := a.moduleOperation(op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "verifying" {
		t.Fatal("phase not persisted", saved.Status)
	}
	saved.ExpiresAt = time.Now().Add(-time.Second)
	if err = a.saveModuleOperation(saved); err != nil {
		t.Fatal(err)
	}
	status := agentRequest("GET", "/api/agent/production-01/module-operations/"+op.ID+"/status", "")
	var proof struct {
		Allowed          bool
		ReconcileAllowed bool
		Binding          struct {
			ID                  string
			ModuleID            string
			ConfigurationDigest string
			Target              map[string]any
		}
	}
	if status.Code != 200 || json.Unmarshal(status.Body.Bytes(), &proof) != nil || proof.Allowed || !proof.ReconcileAllowed || proof.Binding.ID != op.ID || proof.Binding.ModuleID != "example" || proof.Binding.ConfigurationDigest == "" || proof.Binding.Target["containerId"] != "target" {
		t.Fatal("readonly recovery was not bound", status.Code, status.Body.String())
	}
	final := agentRequest("POST", resultPath, `{"result":{"protocol":2,"status":"ok","data":{"containerId":"target"}},"failed":false}`)
	if final.Code != 200 {
		t.Fatal("verified late receipt rejected", final.Code, final.Body.String())
	}
	saved, err = a.moduleOperation(op.ID)
	if err != nil || saved.Status != "succeeded" {
		t.Fatal("verified result not persisted", saved.Status, err)
	}
	// A revoked grant must prevent even an inspect-only recovery from using its broker.
	saved.Status = "unknown"
	if err = a.saveModuleOperation(saved); err != nil {
		t.Fatal(err)
	}
	if w := request(t, a, "PUT", configPath, `{"enabled":false,"grants":[],"config":{}}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	status = agentRequest("GET", "/api/agent/production-01/module-operations/"+op.ID+"/status", "")
	if json.Unmarshal(status.Body.Bytes(), &proof) != nil || proof.ReconcileAllowed {
		t.Fatal("revoked operation still allowed recovery", status.Body.String())
	}
}
