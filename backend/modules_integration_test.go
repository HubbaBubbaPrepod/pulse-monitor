//go:build integration

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

var releasedModulePath = flag.String("module-package", "", "Released Docker module JSON path")
var releasedModuleSHA = flag.String("module-sha256", "", "Expected complete package SHA-256")
var releasedModuleVersion = flag.String("module-version", "", "Expected released Docker module version")
var releasedMarketplace = flag.String("module-marketplace", "", "Optional real marketplace origin for catalog installation")

func TestInstallReleasedDockerModule(t *testing.T) {
	if *releasedModulePath == "" {
		t.Skip("Pass -module-package, -module-sha256 and -module-version to verify a released package")
	}
	if len(*releasedModuleSHA) != 64 || !moduleVersion.MatchString(*releasedModuleVersion) {
		t.Fatal("Expected SHA-256 and release version are required")
	}
	raw, err := os.ReadFile(*releasedModulePath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != *releasedModuleSHA {
		t.Fatal("Released package digest differs from the expected snapshot")
	}
	a, err := newTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path, body := "/api/modules", string(raw)
	if *releasedMarketplace != "" {
		config, _ := json.Marshal(marketplaceConfig{Enabled: true, URL: *releasedMarketplace})
		response := request(
			t,
			a,
			http.MethodPut,
			"/api/marketplace",
			string(config),
		)
		if response.Code != 200 {
			t.Fatalf("Configure real catalog: %s", response.Body.String())
		}
		response = request(
			t,
			a,
			http.MethodGet,
			"/api/marketplace/catalog?query=Docker",
			"",
		)
		if response.Code != 200 {
			t.Fatalf("Fetch real catalog: %s", response.Body.String())
		}
		var page struct {
			Items []marketplaceModule `json:"items"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		owner := ""
		for _, item := range page.Items {
			if item.ID == "docker" && item.Version == *releasedModuleVersion && item.SHA256 == *releasedModuleSHA {
				owner = item.Owner
			}
		}
		if owner == "" {
			t.Fatal("Expected release missing from real approved catalog")
		}
		input, _ := json.Marshal(map[string]string{
			"owner": owner, "id": "docker", "version": *releasedModuleVersion, "sha256": *releasedModuleSHA,
		})
		path, body = "/api/marketplace/install", string(input)
	}
	response := request(
		t,
		a,
		http.MethodPost,
		path,
		body,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("Install released package: status %d: %s", response.Code, response.Body.String())
	}
	var installed moduleManifest
	if err := json.Unmarshal(response.Body.Bytes(), &installed); err != nil {
		t.Fatal(err)
	}
	validIdentity := installed.ID == "docker" && installed.Version == *releasedModuleVersion
	if !validIdentity || len(installed.Binaries) != 3 {
		t.Fatal("Installed module identity or platform set differs from the release")
	}
	for _, target := range []string{"linux-amd64", "linux-arm64", "windows-amd64"} {
		t.Run(target, func(t *testing.T) {
			binary, exists := installed.Binaries[target]
			if !exists || len(binary.Content) != 0 {
				t.Fatal("Platform missing or embedded content leaked in the manifest response")
			}
			content, err := os.ReadFile(filepath.Join(a.moduleRoot(), "docker", *releasedModuleVersion, target))
			if err != nil {
				t.Fatal(err)
			}
			actual := sha256.Sum256(content)
			if hex.EncodeToString(actual[:]) != binary.SHA256 {
				t.Fatal("Installed collector digest differs from the manifest")
			}
		})
	}
	response = request(
		t,
		a,
		http.MethodPost,
		"/api/modules",
		string(raw),
	)
	if response.Code != http.StatusConflict {
		t.Fatalf("Reinstall should preserve immutable versions: status %d", response.Code)
	}
}
