package main

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestManualLegacyModuleCanUpgradeToExtension(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(collectorPackage(t))
	if w := request(t, a, "POST", "/api/modules", string(raw)); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "POST", "/api/extensions", string(extensionFixturePackage(t, "monitor"))); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := a.module("example"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("legacy module remains active", err)
	}
	if _, err := a.extension("example"); err != nil {
		t.Fatal(err)
	}
}

func TestManualExtensionUpdatePreservesCompatibleConfiguration(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installExtensionFixture(t, a, "monitor")
	path := "/api/extensions/example/components/a/configuration"
	if w := request(t, a, "PUT", path, `{"enabled":true,"grants":["state.read"],"config":{"token":"private-token"}}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var p map[string]any
	json.Unmarshal(extensionFixturePackage(t, "monitor"), &p)
	p["version"] = "2.0.1"
	raw, _ := json.Marshal(p)
	if w := request(t, a, "POST", "/api/extensions", string(raw)); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	cfg, err := a.extensionConfiguration("example", "a", "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.Version != "2.0.1" || len(cfg.Grants) != 1 || cfg.Config["token"] != "private-token" {
		t.Fatalf("configuration lost: enabled=%v version=%s grants=%v", cfg.Enabled, cfg.Version, cfg.Grants)
	}
}

func TestManualExtensionUpdateDoesNotGrantNewCapabilities(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installExtensionFixture(t, a, "monitor")
	path := "/api/extensions/example/components/a/configuration"
	if w := request(t, a, "PUT", path, `{"enabled":true,"grants":["state.read"],"config":{"token":"private-token"}}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	var p map[string]any
	json.Unmarshal(extensionFixturePackage(t, "monitor"), &p)
	p["version"] = "2.0.1"
	p["components"].([]any)[0].(map[string]any)["requires"] = []string{"state.read", "network.outbound"}
	raw, _ := json.Marshal(p)
	if w := request(t, a, "POST", "/api/extensions", string(raw)); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	cfg, err := a.extensionConfiguration("example", "a", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled || len(cfg.Grants) != 0 || cfg.Config["token"] != "private-token" {
		t.Fatal("changed capabilities must require consent without deleting settings")
	}
}
