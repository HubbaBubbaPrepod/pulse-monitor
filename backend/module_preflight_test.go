package main

import (
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func snapshotModuleFiles(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		out[relative] = sha256.Sum256(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestModuleCapabilitiesRequiresOwnerSession(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testSession(t, a)
	if w := rawAuthRequest(a, "GET", "/api/module-capabilities", "", nil, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := request(t, a, "GET", "/api/module-capabilities", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var capabilities struct {
		SchemaVersions []int    `json:"schemaVersions"`
		Installable    []int    `json:"installableSchemaVersions"`
		Runtimes       []string `json:"runtimes"`
	}
	if json.Unmarshal(w.Body.Bytes(), &capabilities) != nil || !reflect.DeepEqual(capabilities.SchemaVersions, []int{1, 2}) || !reflect.DeepEqual(capabilities.Installable, []int{1, 2}) || capabilities.Runtimes == nil || len(capabilities.Runtimes) != 2 {
		t.Fatal(w.Body.String())
	}
}
func TestModulePreflightIsReadOnlyAndRedacted(t *testing.T) {
	dir := t.TempDir()
	a, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	testSession(t, a)
	before := snapshotModuleFiles(t, dir)
	w := request(t, a, "POST", "/api/module-packages/validate", string(v2PackageJSON(t, "monitor")))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		Valid, Compatible   bool
		Issues              []moduleValidationIssue
		CompatibilityIssues []moduleValidationIssue
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Valid || !result.Compatible || len(result.Issues) != 0 || len(result.CompatibilityIssues) != 0 {
		t.Fatal(w.Body.String())
	}
	for _, sensitive := range []string{"content", "sha256", "Example extension", "state.read"} {
		if strings.Contains(w.Body.String(), sensitive) {
			t.Fatal("preflight echoed payload", sensitive)
		}
	}
	if !reflect.DeepEqual(before, snapshotModuleFiles(t, dir)) {
		t.Fatal("preflight changed data files")
	}
}
func TestModulePreflightRejectsInvalidOrUnauthorizedRequests(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cookie, _ := testSession(t, a)
	for _, tc := range []struct {
		name, body string
		want       int
	}{{"malformed", `{"secret":"must-not-leak"}`, 400}, {"ambiguous", `{"id":"a","id":"b"}`, 400}, {"unsupported", `{"schemaVersion":3}`, 400}} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(t, a, "POST", "/api/module-packages/validate", tc.body)
			if w.Code != tc.want || strings.Contains(w.Body.String(), "must-not-leak") {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	if w := rawAuthRequest(a, "POST", "/api/module-packages/validate", "{}", nil, ""); w.Code != 401 {
		t.Fatal("missing session", w.Code)
	}
	if w := rawAuthRequest(a, "POST", "/api/module-packages/validate", "{}", cookie, "wrong"); w.Code != 403 {
		t.Fatal("invalid CSRF", w.Code)
	}
	r := httptest.NewRequest("POST", "/api/module-packages/validate", strings.NewReader("{}"))
	r.Host = "127.0.0.1:8080"
	r.ContentLength = 68*1024*1024 + 1
	_, csrf := testSession(t, a)
	r.AddCookie(cookie)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", csrf)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatal("oversize body", w.Code, w.Body.String())
	}
}
func TestModulePreflightDoesNotEnableV2Installation(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testSession(t, a)
	before := snapshotModuleFiles(t, a.dir)
	w := request(t, a, "POST", "/api/modules", string(v2PackageJSON(t, "agent")))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "module_runtime_unavailable") {
		t.Fatal(w.Code, w.Body.String())
	}
	if !reflect.DeepEqual(before, snapshotModuleFiles(t, a.dir)) {
		t.Fatal("unsupported install changed data")
	}
}
