package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func collectorPackage(t *testing.T) moduleManifest {
	t.Helper()
	content, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	return moduleManifest{SchemaVersion: 1, Protocol: 1, ID: "example", Version: "1.0.0", Name: "Example", Fields: []moduleField{{Key: "endpoint", Label: "Endpoint", Type: "text", Required: true}}, Binaries: map[string]moduleBinary{runtime.GOOS + "-" + runtime.GOARCH: {SHA256: hex.EncodeToString(hash[:]), Content: content}}}
}

type delayedModuleBody struct {
	*strings.Reader
	delayed bool
}

func (b *delayedModuleBody) Read(p []byte) (int, error) {
	if !b.delayed {
		b.delayed = true
		time.Sleep(200 * time.Millisecond)
	}
	return b.Reader.Read(p)
}

func TestModuleUploadAllowsBoundedSlowTransfer(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cookie, csrf := testSession(t, a)
	srv := httptest.NewUnstartedServer(a.handler())
	srv.Config.ReadTimeout = 50 * time.Millisecond
	srv.Start()
	defer srv.Close()
	raw, err := json.Marshal(collectorPackage(t))
	if err != nil {
		t.Fatal(err)
	}
	r, err := http.NewRequest("POST", srv.URL+"/api/modules", &delayedModuleBody{Reader: strings.NewReader(string(raw))})
	if err != nil {
		t.Fatal(err)
	}
	r.ContentLength = int64(len(raw))
	r.AddCookie(cookie)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", csrf)
	client := srv.Client()
	client.Timeout = 5 * time.Second
	resp, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatal("slow valid module rejected", resp.StatusCode)
	}
}

func installTestModule(t *testing.T, a *app, m moduleManifest) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return request(t, a, "POST", "/api/modules", string(raw))
}

func TestDisabledModuleDoesNotRequireConfiguration(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if w := installTestModule(t, a, collectorPackage(t)); w.Code != http.StatusCreated {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "PUT", "/api/servers/production-01/modules/example", `{"enabled":false,"config":{}}`); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "PUT", "/api/servers/production-01/modules/example", `{"enabled":true,"config":{}}`); w.Code != http.StatusBadRequest {
		t.Fatal("enabled module accepted missing required field", w.Code)
	}
}

func TestDeletedModuleCanBeReinstalledWithoutLosingData(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := collectorPackage(t)
	if w := installTestModule(t, a, m); w.Code != http.StatusCreated {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "DELETE", "/api/modules/example", ""); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := installTestModule(t, a, m); w.Code != http.StatusCreated {
		t.Fatal("cannot reinstall removed version", w.Code, w.Body.String())
	}
}

func TestModuleRejectsBinaryForDifferentPlatform(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := collectorPackage(t)
	for target, b := range m.Binaries {
		delete(m.Binaries, target)
		wrong := "linux-arm64"
		if target == wrong {
			wrong = "windows-amd64"
		}
		m.Binaries[wrong] = b
		break
	}
	if w := installTestModule(t, a, m); w.Code != http.StatusBadRequest {
		t.Fatal("wrong platform binary accepted", w.Code)
	}
}

func TestDeletedModuleRejectsChangedVersionBytes(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := collectorPackage(t)
	if w := installTestModule(t, a, m); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "DELETE", "/api/modules/example", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for target, binary := range m.Binaries {
		binary.Content = append(binary.Content, 1)
		digest := sha256.Sum256(binary.Content)
		binary.SHA256 = hex.EncodeToString(digest[:])
		m.Binaries[target] = binary
	}
	if w := installTestModule(t, a, m); w.Code != 409 {
		t.Fatal("replaced immutable binary", w.Code, w.Body.String())
	}
}

func TestModulePackageRejectsUnsafePathsAndWrongHash(t *testing.T) {
	for _, raw := range []string{`{"schemaVersion":1,"id":"../escape","version":"1.0.0"}`, `{"schemaVersion":1,"id":"test","version":"1.0.0","protocol":1,"name":"Test","binaries":{"linux-amd64":{"sha256":"bad","content":"aGVsbG8="}}}`} {
		a, err := newApp(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		w := request(t, a, "POST", "/api/modules", raw)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
func TestModuleAgentAccessRequiresNodeToken(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/agent/production-01/modules?os=linux&arch=amd64", nil)
	r.Host = "127.0.0.1:8080"
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 64))
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}

func TestModuleLifecycleSurvivesRestartAndRevokesDisabledCollector(t *testing.T) {
	dir := t.TempDir()
	a, err := newTestApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := collectorPackage(t)
	if w := installTestModule(t, a, m); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	installed, err := a.module(m.ID)
	if err != nil || installed.SchemaVersion != 1 || installed.Protocol != 1 || installed.Version != m.Version || len(installed.Fields) != 1 {
		t.Fatal("v1 manifest changed", installed, err)
	}
	for target, binary := range m.Binaries {
		if installed.Binaries[target].SHA256 != binary.SHA256 || len(installed.Binaries[target].Content) != 0 {
			t.Fatal("stored v1 artifact metadata changed")
		}
	}
	if w := request(t, a, "PUT", "/api/servers/production-01/modules/example", `{"enabled":true,"config":{"endpoint":"local-socket"}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	agentRequest := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8080"
		r.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		return w
	}
	target := "?os=" + runtime.GOOS + "&arch=" + runtime.GOARCH
	if w := agentRequest("GET", "/api/agent/production-01/modules"+target, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "local-socket") {
		t.Fatal(w.Code, w.Body.String())
	} else {
		var assignments []struct {
			ID      string            `json:"id"`
			Version string            `json:"version"`
			SHA256  string            `json:"sha256"`
			Config  map[string]string `json:"config"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &assignments); err != nil || len(assignments) != 1 {
			t.Fatal("invalid v1 assignment", err)
		}
		v := assignments[0]
		if v.ID != m.ID || v.Version != m.Version || v.SHA256 != m.Binaries[runtime.GOOS+"-"+runtime.GOARCH].SHA256 || v.Config["endpoint"] != "local-socket" {
			t.Fatal("v1 assignment contract changed", v)
		}
	}
	if w := agentRequest("GET", "/api/agent/windows-02/modules"+target, ""); w.Code != 401 {
		t.Fatal("token accepted for another node", w.Code)
	}
	if w := agentRequest("POST", "/api/agent/production-01/modules/example/data", `{"status":"ok","data":{"containers":[{"name":"actual-container"}]}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	m.Version = "1.0.1"
	if w := installTestModule(t, a, m); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	a, err = newTestApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(t, a, "GET", "/api/servers/production-01/modules/example/data", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "actual-container") {
		t.Fatal("data lost after restart", w.Code, w.Body.String())
	}
	if w := request(t, a, "PUT", "/api/servers/production-01/modules/example", `{"enabled":false,"config":{}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := agentRequest("GET", "/api/agent/production-01/modules"+target, ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("disabled collector still desired", w.Code, w.Body.String())
	}
	if w := agentRequest("GET", "/api/agent/production-01/modules/example/binary"+target, ""); w.Code != 403 {
		t.Fatal("disabled collector can download", w.Code)
	}
	if w := agentRequest("POST", "/api/agent/production-01/modules/example/data", `{"status":"ok","data":{}}`); w.Code != 403 {
		t.Fatal("disabled collector can report", w.Code)
	}
	if w := request(t, a, "DELETE", "/api/modules/example", ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "GET", "/api/servers/production-01/modules/example/data", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "actual-container") {
		t.Fatal("data lost after removal", w.Code, w.Body.String())
	}
}

func TestModuleOptionalCollectionsAreArrays(t *testing.T) {
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := collectorPackage(t)
	m.Fields = nil
	m.Views = []moduleView{{Kind: "logs", Key: "logs", Title: "Logs"}}
	if w := installTestModule(t, a, m); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/modules"} {
		w := request(t, a, "GET", path, "")
		if w.Code != 200 || strings.Contains(w.Body.String(), `"fields":null`) || strings.Contains(w.Body.String(), `"columns":null`) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
