package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type componentSchedule struct {
	ConfigurationDigest string    `json:"configurationDigest"`
	Next                time.Time `json:"next"`
}
type componentSnapshot struct {
	Version             string          `json:"version"`
	ConfigurationDigest string          `json:"configurationDigest,omitempty"`
	At                  time.Time       `json:"at"`
	Status              string          `json:"status"`
	Data                json.RawMessage `json:"data"`
	Error               string          `json:"error,omitempty"`
}

func (a *app) componentSnapshotPath(module, component, server string) string {
	return filepath.Join(a.dir, "extension-data", module, component, moduleScopeKey(server)+".json")
}
func (a *app) saveComponentSnapshot(op moduleOperation) error {
	snapshot := componentSnapshot{Version: op.Version, ConfigurationDigest: op.ConfigurationDigest, At: op.FinishedAt, Status: op.Status, Data: op.Data, Error: op.Error}
	if len(snapshot.Data) == 0 {
		snapshot.Data = json.RawMessage(`{}`)
	}
	path := a.componentSnapshotPath(op.ModuleID, op.ComponentID, op.ServerID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return saveJSON(path, snapshot)
}

// Scheduling is persistent and bounded: no catch-up burst and at most one
// outstanding interval invocation per component scope. Manual actions never run here.
func (a *app) scheduleExtensionOperations(now time.Time) error {
	a.extensionMu.Lock()
	defer a.extensionMu.Unlock()
	ops, err := a.moduleOperations()
	if err != nil {
		return err
	}
	if err = a.expireModuleOperations(ops); err != nil {
		return err
	}
	dirs, err := os.ReadDir(filepath.Join(a.dir, "extensions"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		p, err := a.extension(dir.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, c := range p.Components {
			scopes := []string{""}
			if c.Location == "agent" {
				scopes = nil
				for _, server := range a.serverList() {
					scopes = append(scopes, server.ID)
				}
			}
			for _, server := range scopes {
				cfg, err := a.extensionConfiguration(p.ID, c.ID, server)
				if err != nil {
					return err
				}
				if !cfg.Enabled || cfg.Version != p.Version {
					continue
				}
				busy := false
				for _, op := range ops {
					if op.ModuleID == p.ID && op.ComponentID == c.ID && op.ServerID == server && op.Trigger == "interval" && (op.Status == "pending" || op.Status == "running") && now.Before(op.ExpiresAt) {
						busy = true
						break
					}
				}
				if busy {
					continue
				}
				digest := moduleConfigurationDigest(cfg)
				for i, trigger := range c.Triggers {
					if trigger.Type != "interval" || trigger.Seconds == nil {
						continue
					}
					key := fmt.Sprintf("%s/%s/%s/%d/%s", p.ID, c.ID, server, i, digest)
					hash := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%d", p.ID, c.ID, server, i)))
					path := filepath.Join(a.dir, "extension-schedules", hex.EncodeToString(hash[:])+".json")
					var cursor componentSchedule
					err = readModuleJSON(path, &cursor)
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						return err
					}
					if cursor.ConfigurationDigest != digest {
						cursor = componentSchedule{}
					}
					if now.Before(cursor.Next) {
						continue
					}
					operationKey := key + "/" + cursor.Next.UTC().Format(time.RFC3339Nano)
					op, status, err := a.queueModuleOperation(p, c, server, "interval", "", map[string]any{}, nil, operationKey)
					if err != nil {
						if status == 403 || status == 409 || status == 429 {
							continue
						}
						return err
					}
					cursor = componentSchedule{ConfigurationDigest: digest, Next: now.Add(time.Duration(*trigger.Seconds) * time.Second)}
					if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						return err
					}
					if err = saveJSON(path, cursor); err != nil {
						return err
					}
					ops = append(ops, op)
					break
				}
			}
		}
	}
	return nil
}
func (a *app) componentSnapshotRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/extensions/{module}/components/{component}/data", func(w http.ResponseWriter, r *http.Request) {
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
		cfg, err := a.extensionConfiguration(p.ID, c.ID, server)
		if err != nil {
			apiError(w, 500, "Could not read component settings")
			return
		}
		empty := componentSnapshot{Version: p.Version, Status: "waiting", Data: json.RawMessage(`{}`)}
		var snapshot componentSnapshot
		err = readModuleJSON(a.componentSnapshotPath(p.ID, c.ID, server), &snapshot)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			apiError(w, 500, "Could not read component data")
			return
		}
		if err != nil || !cfg.Enabled || snapshot.Version != p.Version || snapshot.ConfigurationDigest != moduleConfigurationDigest(cfg) {
			snapshot = empty
		}
		snapshot.ConfigurationDigest = ""
		writeJSON(w, 200, snapshot)
	})
}
