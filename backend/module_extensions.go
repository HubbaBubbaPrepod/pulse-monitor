package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var moduleHostCapabilities = []string{"state.read", "state.write", "network.outbound", "actions.execute"}

type moduleConfiguration struct {
	SecretKeys   []string       `json:"secretKeys,omitempty"`
	Enabled      bool           `json:"enabled"`
	Version      string         `json:"version"`
	Config       map[string]any `json:"config"`
	Grants       []string       `json:"grants"`
	ClearSecrets []string       `json:"clearSecrets,omitempty"`
}

func moduleConfigurationDigest(cfg moduleConfiguration) string {
	raw, _ := json.Marshal(cfg)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func readModuleJSON(path string, into any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 2*1024*1024+1))
	if err != nil {
		return err
	}
	if len(raw) > 2*1024*1024 {
		return errors.New("Module metadata exceeds limit")
	}
	return json.Unmarshal(raw, into)
}
func moduleArtifactFilename(platform string) string {
	if strings.HasPrefix(platform, "windows-") {
		return platform + ".exe"
	}
	return platform
}
func (a *app) extension(id string) (modulePackageV2, error) {
	var p modulePackageV2
	if !validID.MatchString(id) {
		return p, os.ErrNotExist
	}
	err := readModuleJSON(filepath.Join(a.dir, "extensions", id, "manifest.json"), &p)
	if err == nil && len(validateModulePackageV2Content(p, false)) > 0 {
		err = errors.New("Invalid extension metadata")
	}
	return p, err
}
func extensionComponent(p modulePackageV2, id string) (moduleComponentV2, bool) {
	for _, c := range p.Components {
		if c.ID == id {
			return c, true
		}
	}
	return moduleComponentV2{}, false
}
func manualModuleTrigger(c moduleComponentV2, action string) (moduleTriggerV2, bool) {
	for _, t := range c.Triggers {
		if t.Type == "manual" && t.Action != nil && *t.Action == action {
			return t, true
		}
	}
	return moduleTriggerV2{}, false
}
func moduleScopeKey(server string) string {
	if server == "" {
		return "monitor"
	}
	digest := sha256.Sum256([]byte(server))
	return "server-" + hex.EncodeToString(digest[:])
}
func (a *app) configurationPath(module, component, server string) string {
	return filepath.Join(a.dir, "extension-config", module, component, moduleScopeKey(server)+".json")
}

func (a *app) revokeExtensionConfigurations(p modulePackageV2) error {
	for _, component := range p.Components {
		root := filepath.Join(a.dir, "extension-config", p.ID, component.ID)
		files, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			path := filepath.Join(root, file.Name())
			var cfg moduleConfiguration
			if err := readModuleJSON(path, &cfg); err != nil {
				return err
			}
			cfg.Enabled = false
			cfg.Grants = []string{}
			if err := saveJSON(path, cfg); err != nil {
				return err
			}
		}
	}
	return nil
}
func (a *app) extensionConfiguration(module, component, server string) (moduleConfiguration, error) {
	c := moduleConfiguration{Config: map[string]any{}, Grants: []string{}}
	err := readModuleJSON(a.configurationPath(module, component, server), &c)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if c.Config == nil {
		c.Config = map[string]any{}
	}
	if c.Grants == nil {
		c.Grants = []string{}
	}
	// Recover secret classification from immutable version metadata for older records.
	if err == nil && c.SecretKeys == nil && moduleVersion.MatchString(c.Version) {
		var previous modulePackageV2
		if e := readModuleJSON(filepath.Join(a.dir, "extensions", module, c.Version, "manifest.json"), &previous); e != nil {
			return c, e
		}
		if previous.ID != module {
			return c, errors.New("Configuration identity mismatch")
		}
		for _, oldComponent := range previous.Components {
			if strings.EqualFold(oldComponent.ID, component) {
				for _, field := range oldComponent.Fields {
					if field.Type == "password" {
						c.SecretKeys = append(c.SecretKeys, field.Key)
					}
				}
				break
			}
		}
	}
	return c, err
}
func validModuleScope(a *app, c moduleComponentV2, server string) bool {
	if c.Location == "monitor" {
		return server == ""
	}
	if !validID.MatchString(server) {
		return false
	}
	_, ok := a.findServer(server)
	return ok
}
func publicModuleConfiguration(c moduleConfiguration, fields []moduleField) map[string]any {
	config := map[string]any{}
	secrets := []string{}
	for _, field := range fields {
		value, exists := c.Config[field.Key]
		if !exists {
			continue
		}
		if field.Type == "password" || slices.Contains(c.SecretKeys, field.Key) {
			if text, ok := value.(string); ok && text != "" {
				secrets = append(secrets, field.Key)
			}
			continue
		}
		config[field.Key] = value
	}
	return map[string]any{"enabled": c.Enabled, "version": c.Version, "config": config, "grants": c.Grants, "secrets": secrets}
}

// Canonical identities must be portable across case-insensitive filesystems.
func (a *app) checkModuleIdentityCase(id string) error {
	for _, root := range []string{a.moduleRoot(), filepath.Join(a.dir, "extensions")} {
		dirs, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, dir := range dirs {
			if dir.IsDir() && strings.EqualFold(dir.Name(), id) && dir.Name() != id {
				return errors.New("Module ID differs only by case from an existing identity")
			}
		}
	}
	return nil
}
func (a *app) installExtension(w http.ResponseWriter, r *http.Request) {
	a.installExtensionPackage(w, r, "")
}
func (a *app) installExtensionPackage(w http.ResponseWriter, r *http.Request, marketplace string) {
	controller := http.NewResponseController(w)
	deadline := time.Now().Add(2 * time.Minute)
	for _, err := range []error{controller.SetReadDeadline(deadline), controller.SetWriteDeadline(deadline)} {
		if err != nil && !errors.Is(err, http.ErrNotSupported) {
			apiError(w, 500, "Upload deadline unavailable")
			return
		}
	}
	if r.ContentLength > modulePackageBodyLimit {
		apiError(w, 413, "Extension upload exceeds limit")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, modulePackageBodyLimit))
	if err != nil {
		apiError(w, 400, "Invalid extension upload")
		return
	}
	p, err := decodeModulePackageV2(raw)
	if err != nil || len(validateModulePackageV2(p)) > 0 {
		apiError(w, 400, "Invalid extension package")
		return
	}
	a.extensionMu.Lock()
	defer a.extensionMu.Unlock()
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if err := a.checkModuleIdentityCase(p.ID); err != nil {
		apiError(w, 409, "Module identity conflicts with an existing directory")
		return
	}
	if marketplace != "" {
		config, err := a.marketplaceConfiguration()
		if err != nil || !config.Enabled || config.URL != marketplace {
			apiError(w, 409, "Marketplace settings changed")
			return
		}
		if err := a.checkMarketplaceUpdate(r, filepath.Join(a.dir, "extensions", p.ID), p.ID, p.Version, marketplace); err != nil {
			apiError(w, 409, err.Error())
			return
		}
	}
	legacy, legacyErr := a.module(p.ID)
	if legacyErr != nil && !errors.Is(legacyErr, os.ErrNotExist) {
		apiError(w, 409, "Could not read legacy module")
		return
	}
	root := filepath.Join(a.dir, "extensions", p.ID)
	version := filepath.Join(root, p.Version)
	if err := os.MkdirAll(root, 0700); err != nil {
		apiError(w, 500, "Could not create extension directory")
		return
	}
	stage, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		apiError(w, 500, "Could not stage extension")
		return
	}
	defer os.RemoveAll(stage)
	for i, c := range p.Components {
		dir := filepath.Join(stage, c.ID)
		if os.MkdirAll(dir, 0700) != nil {
			apiError(w, 500, "Could not stage component")
			return
		}
		for target, b := range c.Binaries {
			if os.WriteFile(filepath.Join(dir, moduleArtifactFilename(target)), b.Content, 0600) != nil {
				apiError(w, 500, "Could not stage artifact")
				return
			}
			b.Content = nil
			p.Components[i].Binaries[target] = b
		}
	}
	// Persist immutable metadata along with artifacts, not only binary hashes.
	if err := saveJSON(filepath.Join(stage, "manifest.json"), p); err != nil {
		apiError(w, 500, "Could not stage metadata")
		return
	}
	if _, err := os.Stat(version); err == nil {
		var existing modulePackageV2
		if readModuleJSON(filepath.Join(version, "manifest.json"), &existing) != nil {
			apiError(w, 409, "Existing version metadata unavailable")
			return
		}
		before, _ := json.Marshal(existing)
		after, _ := json.Marshal(p)
		if !bytes.Equal(before, after) {
			apiError(w, 409, "Immutable extension version differs")
			return
		}
		for _, c := range p.Components {
			for target, b := range c.Binaries {
				data, err := os.ReadFile(filepath.Join(version, c.ID, moduleArtifactFilename(target)))
				sum := sha256.Sum256(data)
				if err != nil || !strings.EqualFold(hex.EncodeToString(sum[:]), b.SHA256) {
					apiError(w, 409, "Existing extension artifact differs")
					return
				}
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) || os.Rename(stage, version) != nil {
		apiError(w, 500, "Could not finalize extension")
		return
	}
	if err := saveModuleSource(r, version); err != nil {
		apiError(w, 500, "Could not save module publisher")
		return
	}
	previous, previousErr := a.extension(p.ID)
	if previousErr != nil && !errors.Is(previousErr, os.ErrNotExist) {
		apiError(w, 500, "Could not read previous extension")
		return
	}
	rollback, err := a.migrateExtensionConfigurations(previous, p)
	if err != nil {
		apiError(w, 500, "Could not migrate extension configuration")
		return
	}
	legacyPath := filepath.Join(a.moduleRoot(), p.ID, "manifest.json")
	legacyBackup := filepath.Join(a.moduleRoot(), p.ID, "manifest-retired-"+legacy.Version+".json")
	if legacyErr == nil {
		if err := os.Rename(legacyPath, legacyBackup); err != nil {
			rollback()
			apiError(w, 500, "Could not retire legacy module")
			return
		}
	}
	if err := saveJSON(filepath.Join(root, "manifest.json"), p); err != nil {
		rollback()
		if legacyErr == nil {
			_ = os.Rename(legacyBackup, legacyPath)
		}
		apiError(w, 500, "Could not save extension manifest")
		return
	}
	a.cancelModuleOperations(p.ID, "", "", true)
	writeJSON(w, 201, p)
}

func (a *app) extensionRoutes(mux *http.ServeMux) {
	a.componentSnapshotRoutes(mux)
	mux.HandleFunc("POST /api/extensions", a.installExtension)
	mux.HandleFunc("GET /api/extensions", func(w http.ResponseWriter, r *http.Request) {
		a.extensionMu.Lock()
		defer a.extensionMu.Unlock()
		out := []modulePackageV2{}
		dirs, err := os.ReadDir(filepath.Join(a.dir, "extensions"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			apiError(w, 500, "Could not read extensions")
			return
		}
		for _, dir := range dirs {
			if dir.IsDir() {
				if p, err := a.extension(dir.Name()); err == nil {
					out = append(out, p)
				}
			}
		}
		writeJSON(w, 200, out)
	})
	mux.HandleFunc("DELETE /api/extensions/{module}", func(w http.ResponseWriter, r *http.Request) {
		a.extensionMu.Lock()
		defer a.extensionMu.Unlock()
		id := r.PathValue("module")
		p, err := a.extension(id)
		if err != nil {
			apiError(w, 404, "Extension not found")
			return
		}
		if err := a.revokeExtensionConfigurations(p); err != nil {
			apiError(w, 500, "Could not revoke extension authorization")
			return
		}
		if os.Remove(filepath.Join(a.dir, "extensions", id, "manifest.json")) != nil {
			apiError(w, 500, "Could not remove extension")
			return
		}
		a.cancelModuleOperations(id, "", "", true)
		writeJSON(w, 200, map[string]bool{"deleted": true})
	})
	for _, method := range []string{"GET", "PUT"} {
		mux.HandleFunc(method+" /api/extensions/{module}/components/{component}/configuration", func(w http.ResponseWriter, r *http.Request) {
			a.extensionMu.Lock()
			defer a.extensionMu.Unlock()
			p, err := a.extension(r.PathValue("module"))
			if err != nil {
				apiError(w, 404, "Extension not found")
				return
			}
			c, ok := extensionComponent(p, r.PathValue("component"))
			server := r.URL.Query().Get("server")
			if !ok || !validModuleScope(a, c, server) {
				apiError(w, 400, "Invalid component scope")
				return
			}
			current, err := a.extensionConfiguration(p.ID, c.ID, server)
			if err != nil {
				apiError(w, 500, "Could not read component settings")
				return
			}
			if r.Method == "GET" {
				writeJSON(w, 200, publicModuleConfiguration(current, c.Fields))
				return
			}
			var next moduleConfiguration
			if decodeJSON(w, r, &next) != nil {
				apiError(w, 400, "Invalid component settings")
				return
			}
			if next.Config == nil {
				next.Config = map[string]any{}
			}
			if !validModuleNames(next.Grants, 32) {
				apiError(w, 400, "Invalid capability grants")
				return
			}
			for _, grant := range next.Grants {
				if !slices.Contains(moduleHostCapabilities, grant) {
					apiError(w, 403, "Unsupported capability grant")
					return
				}
			}
			for _, f := range c.Fields {
				if f.Type == "password" {
					if _, exists := next.Config[f.Key]; !exists && !slices.Contains(next.ClearSecrets, f.Key) {
						if secret, ok := current.Config[f.Key]; ok {
							next.Config[f.Key] = secret
						}
					}
				}
			}
			for _, key := range next.ClearSecrets {
				f := slices.IndexFunc(c.Fields, func(f moduleField) bool { return f.Key == key && f.Type == "password" })
				if f < 0 {
					apiError(w, 400, "Invalid secret field")
					return
				}
				delete(next.Config, key)
			}
			if err := validateModuleConfig(moduleManifest{Fields: c.Fields}, next.Config, next.Enabled); err != nil {
				apiError(w, 400, "Invalid component configuration")
				return
			}
			if next.Enabled {
				for _, t := range c.Triggers {
					if t.Type == "event" {
						apiError(w, 403, "Event dispatch is not supported by this runtime yet")
						return
					}
				}
				for _, required := range c.Requires {
					if !slices.Contains(next.Grants, required) || !slices.Contains(moduleHostCapabilities, required) {
						apiError(w, 403, "Required capability is not granted or supported")
						return
					}
				}
			}
			next.SecretKeys = nil
			for _, field := range c.Fields {
				if field.Type == "password" {
					next.SecretKeys = append(next.SecretKeys, field.Key)
				}
			}
			next.Version = p.Version
			next.ClearSecrets = nil
			if next.Grants == nil {
				next.Grants = []string{}
			}
			path := a.configurationPath(p.ID, c.ID, server)
			if os.MkdirAll(filepath.Dir(path), 0700) != nil || saveJSON(path, next) != nil {
				apiError(w, 500, "Could not save component settings")
				return
			}
			if !next.Enabled || moduleConfigurationDigest(current) != moduleConfigurationDigest(next) {
				a.cancelModuleOperations(p.ID, c.ID, server, false)
			}
			writeJSON(w, 200, publicModuleConfiguration(next, c.Fields))
		})
	}
}
