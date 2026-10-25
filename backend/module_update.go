package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func compareModuleVersions(a, b string) int {
	left, lp, _ := strings.Cut(a, "-")
	right, rp, _ := strings.Cut(b, "-")
	ls, rs := strings.Split(left, "."), strings.Split(right, ".")
	if len(ls) != 3 || len(rs) != 3 {
		return 0
	}
	for i := range ls {
		l, r := strings.TrimLeft(ls[i], "0"), strings.TrimLeft(rs[i], "0")
		if len(l) < len(r) {
			return -1
		}
		if len(l) > len(r) {
			return 1
		}
		if n := strings.Compare(l, r); n != 0 {
			return n
		}
	}
	if lp == rp {
		return 0
	}
	if lp == "" {
		return 1
	}
	if rp == "" {
		return -1
	}
	numeric := func(s string) bool { return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0 }
	ln, rn := numeric(lp), numeric(rp)
	if ln && rn {
		l, r := strings.TrimLeft(lp, "0"), strings.TrimLeft(rp, "0")
		if len(l) < len(r) {
			return -1
		}
		if len(l) > len(r) {
			return 1
		}
		return strings.Compare(l, r)
	}
	if ln {
		return -1
	}
	if rn {
		return 1
	}
	return strings.Compare(lp, rp)
}

func (a *app) marketplaceUpdates(w http.ResponseWriter, r *http.Request) {
	config, err := a.marketplaceConfiguration()
	if err != nil {
		apiError(w, 500, "Could not read marketplace settings")
		return
	}
	items := []map[string]any{}
	if !config.Enabled {
		writeJSON(w, 200, map[string]any{"enabled": false, "items": items})
		return
	}
	origin, err := marketplaceOrigin(config.URL)
	if err != nil {
		apiError(w, 400, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	failures := []string{}
	for _, root := range []string{a.moduleRoot(), filepath.Join(a.dir, "extensions")} {
		dirs, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			apiError(w, 500, "Could not read installed modules")
			return
		}
		for _, dir := range dirs {
			if !dir.IsDir() || !validID.MatchString(dir.Name()) {
				continue
			}
			var active struct{ ID, Version string }
			if readModuleJSON(filepath.Join(root, dir.Name(), "manifest.json"), &active) != nil || active.ID != dir.Name() || !moduleVersion.MatchString(active.Version) {
				continue
			}
			var source moduleInstallSource
			if readModuleJSON(filepath.Join(root, dir.Name(), active.Version, "marketplace-source.json"), &source) != nil || source.Origin != origin.String() || !marketplaceOwner.MatchString(source.Owner) {
				continue
			}
			current, err := a.marketplaceConfiguration()
			if err != nil || !current.Enabled || current.URL != config.URL {
				writeJSON(w, 200, map[string]any{"enabled": false, "items": []any{}})
				return
			}
			raw, err := marketplaceFetch(ctx, origin, "/api/v1/modules/"+source.Owner+"/"+active.ID+"?runtime=all", 1024*1024)
			var result struct {
				Versions []marketplaceModule `json:"versions"`
			}
			if err != nil || json.Unmarshal(raw, &result) != nil {
				failures = append(failures, active.ID)
				continue
			}
			var latest *marketplaceModule
			for i := range result.Versions {
				m := &result.Versions[i]
				if m.ID != active.ID || !strings.EqualFold(m.Owner, source.Owner) || !moduleVersion.MatchString(m.Version) || !marketplaceDigest.MatchString(m.SHA256) || m.Size <= 0 || m.Size > modulePackageBodyLimit {
					continue
				}
				if strings.Contains(m.Version, "-") && r.URL.Query().Get("preview") != "1" {
					continue
				}
				if compareModuleVersions(m.Version, active.Version) > 0 && (latest == nil || compareModuleVersions(m.Version, latest.Version) > 0) {
					latest = m
				}
			}
			if latest != nil {
				items = append(items, map[string]any{"installed": active.Version, "module": latest})
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"enabled": true, "items": items, "unavailable": failures})
}

type moduleSourceKey struct{}
type moduleInstallSource struct {
	Origin string `json:"origin"`
	Owner  string `json:"owner"`
	Adopt  bool   `json:"-"`
}

// Source records are created by the installer, never decoded from module JSON.
func (a *app) checkMarketplaceUpdate(r *http.Request, root, id, version, origin string) error {
	source, ok := r.Context().Value(moduleSourceKey{}).(moduleInstallSource)
	if !ok || source.Origin != origin {
		return errors.New("Missing verified publisher")
	}
	previous := ""
	if m, err := a.module(id); err == nil {
		previous = m.Version
		root = filepath.Join(a.moduleRoot(), id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if m, err := a.extension(id); err == nil {
		previous = m.Version
		root = filepath.Join(a.dir, "extensions", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if previous == "" {
		return nil
	}
	if previous == version {
		return errors.New("Module version already installed")
	}
	var existing moduleInstallSource
	err := readModuleJSON(filepath.Join(root, previous, "marketplace-source.json"), &existing)
	if errors.Is(err, os.ErrNotExist) && source.Adopt {
		return nil
	}
	if err != nil {
		return errors.New("Confirm publisher for a manually installed module")
	}
	if existing.Origin != source.Origin || !strings.EqualFold(existing.Owner, source.Owner) {
		return errors.New("Installed module belongs to a different marketplace publisher")
	}
	return nil
}

func saveModuleSource(r *http.Request, version string) error {
	path := filepath.Join(version, "marketplace-source.json")
	if source, ok := r.Context().Value(moduleSourceKey{}).(moduleInstallSource); ok {
		return saveJSON(path, source)
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Update only configurations with an unchanged execution contract. A different
// capability, trigger, location or field schema requires renewed consent.
func (a *app) migrateExtensionConfigurations(previous, next modulePackageV2) (func(), error) {
	backups := map[string][]byte{}
	rollback := func() {
		for path, raw := range backups {
			_ = saveRawModuleConfiguration(path, raw)
		}
	}
	for _, component := range next.Components {
		old, exists := extensionComponent(previous, component.ID)
		compatible := exists && old.Location == component.Location && old.Protocol == component.Protocol && componentRuntime(old) == componentRuntime(component)
		before, _ := json.Marshal([]any{old.Requires, old.OptionalCapabilities, old.Fields, old.Triggers, previous.Permissions})
		after, _ := json.Marshal([]any{component.Requires, component.OptionalCapabilities, component.Fields, component.Triggers, next.Permissions})
		compatible = compatible && bytes.Equal(before, after)
		root := filepath.Join(a.dir, "extension-config", next.ID, component.ID)
		files, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			rollback()
			return nil, err
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			path := filepath.Join(root, file.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				rollback()
				return nil, err
			}
			var cfg moduleConfiguration
			if err = json.Unmarshal(raw, &cfg); err != nil {
				rollback()
				return nil, err
			}
			backups[path] = raw
			// Do not revive configurations from removed components or a different version.
			if !compatible || cfg.Version != previous.Version {
				cfg.Enabled = false
				cfg.Grants = []string{}
			}
			for _, field := range old.Fields {
				if field.Type == "password" {
					cfg.SecretKeys = append(cfg.SecretKeys, field.Key)
				}
			}
			cfg.Version = next.Version
			if err = saveJSON(path, cfg); err != nil {
				rollback()
				return nil, err
			}
		}
	}
	return rollback, nil
}

func saveRawModuleConfiguration(path string, raw []byte) error {
	var cfg moduleConfiguration
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	return saveJSON(path, cfg)
}
