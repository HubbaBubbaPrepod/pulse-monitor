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
)

var moduleHostCapabilities = []string{"state.read", "state.write", "events.receive", "network.outbound", "actions.execute"}

type moduleConfiguration struct {
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
func (a *app) configurationPath(module, component, server string) string {
	scope := "monitor"
	if server != "" {
		scope = "server-" + server
	}
	return filepath.Join(a.dir, "extension-config", module, component, scope+".json")
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
	for key, value := range c.Config {
		config[key] = value
	}
	secrets := []string{}
	for _, f := range fields {
		if f.Type == "password" {
			if v, ok := config[f.Key].(string); ok && v != "" {
				secrets = append(secrets, f.Key)
			}
			delete(config, f.Key)
		}
	}
	return map[string]any{"enabled": c.Enabled, "version": c.Version, "config": config, "grants": c.Grants, "secrets": secrets}
}
func (a *app) installExtension(w http.ResponseWriter, r *http.Request) {
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
	if _, err := a.module(p.ID); !errors.Is(err, os.ErrNotExist) {
		apiError(w, 409, "Remove the legacy module before installing its v2 replacement; saved data remains")
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
			if os.WriteFile(filepath.Join(dir, target), b.Content, 0600) != nil {
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
				data, err := os.ReadFile(filepath.Join(version, c.ID, target))
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
	if err := a.revokeExtensionConfigurations(p); err != nil {
		apiError(w, 500, "Could not reset extension authorization")
		return
	}
	if err := saveJSON(filepath.Join(root, "manifest.json"), p); err != nil {
		apiError(w, 500, "Could not save extension manifest")
		return
	}
	a.cancelModuleOperations(p.ID, "", "", true)
	writeJSON(w, 201, p)
}

func (a *app) extensionRoutes(mux *http.ServeMux) {
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
				for _, required := range c.Requires {
					if !slices.Contains(next.Grants, required) || !slices.Contains(moduleHostCapabilities, required) {
						apiError(w, 403, "Required capability is not granted or supported")
						return
					}
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
