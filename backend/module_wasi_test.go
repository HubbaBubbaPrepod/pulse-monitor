package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func wasiPackageJSON(t *testing.T, location string) []byte {
	t.Helper()
	artifact := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	hash := sha256.Sum256(artifact)
	raw, err := json.Marshal(map[string]any{"schemaVersion": 2, "protocol": 2, "id": "isolated", "version": "2.1.0", "name": "Isolated example", "permissions": []string{}, "components": []any{map[string]any{"id": "worker", "location": location, "runtime": "wasip1", "protocol": 2, "requires": []string{"state.read"}, "optionalCapabilities": []string{"state.write"}, "fields": []any{}, "triggers": []any{map[string]any{"type": "manual", "action": "inspect"}}, "binaries": map[string]any{"wasip1-wasm": map[string]any{"sha256": hex.EncodeToString(hash[:]), "content": artifact}}}}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestWASIPackageHasExplicitRuntimeAndArtifact(t *testing.T) {
	raw := wasiPackageJSON(t, "agent")
	p, err := decodeModulePackageV2(raw)
	if err != nil || len(validateModulePackageV2(p)) != 0 {
		t.Fatal("valid isolated package rejected", err)
	}
	for _, change := range []func(map[string]any){
		func(c map[string]any) { c["runtime"] = "unknown" },
		func(c map[string]any) { c["runtime"] = "native" },
		func(c map[string]any) {
			b := c["binaries"].(map[string]any)
			b["linux-amd64"] = b["wasip1-wasm"]
			delete(b, "wasip1-wasm")
		},
		func(c map[string]any) { c["optionalCapabilities"] = []string{"network.outbound"} },
	} {
		var value map[string]any
		json.Unmarshal(raw, &value)
		change(value["components"].([]any)[0].(map[string]any))
		modified, _ := json.Marshal(value)
		packageValue, err := decodeModulePackageV2(modified)
		if err == nil && len(validateModulePackageV2(packageValue)) == 0 {
			t.Fatal("mislabeled runtime or unsupported capability accepted")
		}
	}
}
func TestWASIConfigurationDoesNotGrantUndeclaredCapabilities(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if w := request(t, a, "POST", "/api/extensions", string(wasiPackageJSON(t, "monitor"))); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "PUT", "/api/extensions/isolated/components/worker/configuration", `{"enabled":true,"grants":["state.read","docker.control"],"config":{}}`); w.Code != 403 {
		t.Fatal("undeclared WASI grant accepted", w.Code, w.Body.String())
	}
}
func TestWASIQueueRequiresAdvertisedAgentRuntime(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if w := request(t, a, "POST", "/api/extensions", string(wasiPackageJSON(t, "agent"))); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err = a.issueAgentToken("production-01"); err != nil {
		t.Fatal(err)
	}
	a.telemetryMu.Lock()
	a.nodes["production-01"].Seen = time.Now()
	a.nodes["production-01"].Report.Capabilities = map[string]string{"moduleRuntime": "2"}
	a.telemetryMu.Unlock()
	if w := request(t, a, "PUT", "/api/extensions/isolated/components/worker/configuration?server=production-01", `{"enabled":true,"grants":["state.read"],"config":{}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	endpoint := "/api/extensions/isolated/components/worker/operations?server=production-01"
	if w := request(t, a, "POST", endpoint, `{"action":"inspect","input":{}}`); w.Code != 409 {
		t.Fatal("WASI sent to an older Agent", w.Code, w.Body.String())
	}
	a.telemetryMu.Lock()
	a.nodes["production-01"].Report.Capabilities["moduleWASI"] = "1"
	a.telemetryMu.Unlock()
	w := request(t, a, "POST", endpoint, `{"action":"inspect","input":{}}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var op moduleOperation
	json.Unmarshal(w.Body.Bytes(), &op)
	lease, err := a.claimModuleOperation(op, "windows-amd64")
	if err != nil || lease.Runtime != "wasip1" {
		t.Fatal("WASI lease unavailable", err)
	}
	saved, err := a.moduleOperation(op.ID)
	if err != nil || saved.Platform != "wasip1-wasm" {
		t.Fatal("native artifact selected for WASI", err)
	}
}
func TestWASIStateDoesNotExposeLegacyNativeState(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	native := moduleOperation{ModuleID: "example", ComponentID: "worker", ServerID: "one"}
	isolated := native
	isolated.Runtime = "wasip1"
	if a.componentStatePath(native) == a.componentStatePath(isolated) {
		t.Fatal("native private state reused by isolated module")
	}
}
func TestWASIRuntimeTransitionRequiresRenewedConsent(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw := v2PackageJSON(t, "monitor")
	if w := request(t, a, "POST", "/api/extensions", string(raw)); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "PUT", "/api/extensions/example/components/a/configuration", `{"enabled":true,"grants":["state.read"],"config":{}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var value map[string]any
	json.Unmarshal(raw, &value)
	value["version"] = "2.1.0"
	component := value["components"].([]any)[0].(map[string]any)
	component["runtime"] = "wasip1"
	artifact := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	hash := sha256.Sum256(artifact)
	component["binaries"] = map[string]any{"wasip1-wasm": map[string]any{"sha256": hex.EncodeToString(hash[:]), "content": artifact}}
	next, _ := json.Marshal(value)
	if w := request(t, a, "POST", "/api/extensions", string(next)); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	cfg, err := a.extensionConfiguration("example", "a", "")
	if err != nil || cfg.Enabled || len(cfg.Grants) != 0 {
		t.Fatal("runtime transition silently retained consent", err)
	}
}
func TestWASIRuntimeNeverFallsBackToNative(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "invalid.wasm")
	os.WriteFile(file, []byte("not a WASM module"), 0600)
	lease := moduleLease{Runtime: "wasip1", ExpiresAt: time.Now().Add(time.Minute), Invocation: moduleInvocation{Protocol: 2, ID: strings.Repeat("a", 64), Config: map[string]any{}, Input: map[string]any{}}}
	if _, err = a.runModuleLease(context.Background(), file, lease); err == nil {
		t.Fatal("invalid WASI artifact accepted")
	}
}

var extensionWASIFixture = sync.OnceValues(func() ([]byte, error) {
	directory, err := os.MkdirTemp("", "pulse-extension-wasi-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "guest.wasm")
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-trimpath", "-o", path, "./testdata/extension-wasi")
	command.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("fixture build: %w: %s", err, output)
	}
	return os.ReadFile(path)
})

func TestWASIMonitorWithholdsHostSecrets(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := extensionWASIFixture()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "guest.wasm")
	if err = os.WriteFile(path, artifact, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(artifact)
	lease := moduleLease{Runtime: "wasip1", SHA256: hex.EncodeToString(digest[:]), ExpiresAt: time.Now().Add(time.Minute), SecretFields: []string{"secret"}, SecretInputFields: []string{"password"}, Invocation: moduleInvocation{Protocol: 2, Config: map[string]any{"secret": "private", "telegramToken": "host-token", "telegramChatId": "host-chat", "serverId": "bound-node", "label": "visible"}, Input: map[string]any{"password": "private-input", "value": "public"}}}
	out, err := a.runModuleLease(context.Background(), path, lease)
	if err != nil || out.Status != "ok" {
		t.Fatal(out, err)
	}
	for _, secret := range []string{"private", "host-token", "host-chat", "bound-node"} {
		if strings.Contains(string(out.Data), secret) {
			t.Fatal("guest received host-only settings")
		}
	}
	if !strings.Contains(string(out.Data), "visible") {
		t.Fatal("public setting missing")
	}
}
func TestWASIManualActionChecksOptionalGrant(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var pkg map[string]any
	json.Unmarshal(wasiPackageJSON(t, "monitor"), &pkg)
	c := pkg["components"].([]any)[0].(map[string]any)
	c["triggers"] = []any{map[string]any{"type": "manual", "action": "inspect", "requires": []string{"state.write"}}}
	raw, _ := json.Marshal(pkg)
	if w := request(t, a, "POST", "/api/extensions", string(raw)); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	endpoint := "/api/extensions/isolated/components/worker/configuration"
	if w := request(t, a, "PUT", endpoint, `{"enabled":true,"grants":["state.read"],"config":{}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	action := "/api/extensions/isolated/components/worker/operations"
	if w := request(t, a, "POST", action, `{"action":"inspect","input":{}}`); w.Code != http.StatusForbidden {
		t.Fatal("optional action grant bypassed", w.Code, w.Body.String())
	}
	if w := request(t, a, "PUT", endpoint, `{"enabled":true,"grants":["state.read","state.write"],"config":{}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w := request(t, a, "POST", action, `{"action":"inspect","input":{}}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var op moduleOperation
	json.Unmarshal(w.Body.Bytes(), &op)
	cfg, err := a.extensionConfiguration("isolated", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.SecretKeys = []string{"removedSecret"}
	cfg.Config["removedSecret"] = "private-old-value"
	if err = saveJSON(a.configurationPath("isolated", "worker", ""), cfg); err != nil {
		t.Fatal(err)
	}
	op.ConfigurationDigest = moduleConfigurationDigest(cfg)
	lease, err := a.claimModuleOperation(op, "linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(lease.SecretFields, "removedSecret") {
		t.Fatal("historical secret classification lost")
	}
}
