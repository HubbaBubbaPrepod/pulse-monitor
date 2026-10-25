package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

type moduleOperation struct {
	Runtime             string          `json:"runtime,omitempty"`
	ID                  string          `json:"id"`
	ModuleID            string          `json:"moduleId"`
	ComponentID         string          `json:"componentId"`
	Version             string          `json:"version"`
	ServerID            string          `json:"serverId,omitempty"`
	Location            string          `json:"location"`
	Trigger             string          `json:"trigger"`
	Action              string          `json:"action,omitempty"`
	Mutating            bool            `json:"mutating"`
	Status              string          `json:"status"`
	Platform            string          `json:"platform,omitempty"`
	SHA256              string          `json:"sha256,omitempty"`
	StartedAt           time.Time       `json:"startedAt,omitempty"`
	Target              map[string]any  `json:"target,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	ExpiresAt           time.Time       `json:"expiresAt"`
	FinishedAt          time.Time       `json:"finishedAt,omitempty"`
	Input               map[string]any  `json:"input,omitempty"`
	Event               json.RawMessage `json:"event,omitempty"`
	Secrets             []string        `json:"secrets,omitempty"`
	Data                json.RawMessage `json:"data,omitempty"`
	Error               string          `json:"error,omitempty"`
	RequestedBy         string          `json:"requestedBy,omitempty"`
	ConfigurationDigest string          `json:"configurationDigest,omitempty"`
}

func publicModuleOperation(op moduleOperation) moduleOperation {
	op.ConfigurationDigest = ""
	op.Input = nil
	op.Secrets = nil
	op.Event = nil
	return op
}
func (a *app) operationPath(id string) string {
	return filepath.Join(a.dir, "module-operations", id+".json")
}
func (a *app) moduleOperation(id string) (moduleOperation, error) {
	var op moduleOperation
	if len(id) != 64 || !validID.MatchString(id) {
		return op, os.ErrNotExist
	}
	err := readModuleJSON(a.operationPath(id), &op)
	if err == nil && op.ID != id {
		err = errors.New("Operation identity mismatch")
	}
	return op, err
}
func (a *app) saveModuleOperation(op moduleOperation) error {
	path := a.operationPath(op.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return saveJSON(path, op)
}
func (a *app) moduleOperations() ([]moduleOperation, error) {
	out := []moduleOperation{}
	dirs, err := os.ReadDir(filepath.Join(a.dir, "module-operations"))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if dir.IsDir() || !strings.HasSuffix(dir.Name(), ".json") {
			continue
		}
		op, err := a.moduleOperation(strings.TrimSuffix(dir.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	slices.SortFunc(out, func(a, b moduleOperation) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}
func (a *app) recoverModuleOperations() error {
	ops, err := a.moduleOperations()
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.Status == "running" {
			op.Status = "unknown"
			op.Error = "Monitor restarted after dispatch; operation was not replayed"
			op.FinishedAt = time.Now().UTC()
			if err := a.saveModuleOperation(op); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *app) expireModuleOperations(ops []moduleOperation) error {
	for i := range ops {
		op := &ops[i]
		if time.Now().Before(op.ExpiresAt) {
			continue
		}
		if op.Status == "pending" {
			op.Status = "expired"
		} else if op.Status == "running" {
			op.Status = "unknown"
			op.Error = "Operation deadline exceeded; not replayed"
			if cancel := a.moduleCancels[op.ID]; cancel != nil {
				cancel()
			}
		} else {
			continue
		}
		op.FinishedAt = time.Now().UTC()
		if err := a.saveModuleOperation(*op); err != nil {
			return err
		}
	}
	return nil
}
func (a *app) cancelModuleOperations(module, component, server string, all bool) {
	ops, err := a.moduleOperations()
	if err != nil {
		return
	}
	for _, op := range ops {
		if op.ModuleID != module || (!all && (op.ComponentID != component || op.ServerID != server)) {
			continue
		}
		if op.Status != "pending" && op.Status != "running" {
			continue
		}
		if cancel := a.moduleCancels[op.ID]; cancel != nil {
			cancel()
		}
		if op.Status == "pending" {
			op.Status = "cancelled"
		} else {
			op.Status = "unknown"
		}
		op.Error = "Component disabled, updated or its grants changed"
		op.FinishedAt = time.Now().UTC()
		_ = a.saveModuleOperation(op)
	}
}
func (a *app) authorizedOperation(op moduleOperation) (modulePackageV2, moduleComponentV2, moduleConfiguration, error) {
	p, err := a.extension(op.ModuleID)
	if err != nil {
		return p, moduleComponentV2{}, moduleConfiguration{}, err
	}
	c, ok := extensionComponent(p, op.ComponentID)
	if !ok || p.Version != op.Version || c.Location != op.Location || !validModuleScope(a, c, op.ServerID) || componentRuntime(c) != componentRuntime(moduleComponentV2{Runtime: op.Runtime}) {
		return p, c, moduleConfiguration{}, errors.New("Component changed")
	}
	cfg, err := a.extensionConfiguration(p.ID, c.ID, op.ServerID)
	if err != nil || !cfg.Enabled || cfg.Version != p.Version {
		return p, c, cfg, errors.New("Component disabled or configuration version changed")
	}
	if op.ConfigurationDigest != "" && op.ConfigurationDigest != moduleConfigurationDigest(cfg) {
		return p, c, cfg, errors.New("Component configuration changed")
	}
	for _, capability := range c.Requires {
		if !slices.Contains(cfg.Grants, capability) || !slices.Contains(moduleHostCapabilities, capability) {
			return p, c, cfg, errors.New("Required capability not granted")
		}
	}
	if op.Mutating && !slices.Contains(cfg.Grants, "actions.execute") {
		return p, c, cfg, errors.New("Action execution is not granted")
	}
	if trigger, ok := manualModuleTrigger(c, op.Action); ok && op.Trigger == "manual" {
		for _, required := range trigger.Requires {
			if !slices.Contains(cfg.Grants, required) || !declaredModuleGrant(c, required) {
				return p, c, cfg, errors.New("Action capability not granted")
			}
		}
	}
	return p, c, cfg, nil
}
func (a *app) queueModuleOperation(p modulePackageV2, c moduleComponentV2, server, trigger, action string, input map[string]any, event json.RawMessage, scheduleKey ...string) (moduleOperation, int, error) {
	op := moduleOperation{ModuleID: p.ID, ComponentID: c.ID, Version: p.Version, ServerID: server, Location: c.Location, Trigger: trigger, Action: action, Input: input, Event: event, Status: "pending", CreatedAt: time.Now().UTC()}
	if c.Runtime == "wasip1" {
		op.Runtime = "wasip1"
	}
	op.ExpiresAt = op.CreatedAt.Add(time.Minute)
	if trigger == "manual" {
		a.auth.mu.Lock()
		if a.auth.account != nil {
			op.RequestedBy = a.auth.account.ID
		}
		a.auth.mu.Unlock()
		t, ok := manualModuleTrigger(c, action)
		if !ok {
			return op, 400, errors.New("Unknown component action")
		}
		op.Mutating = t.Mutating
		op.Target = map[string]any{}
		for _, field := range t.InputFields {
			if field.Type != "password" {
				if value, ok := input[field.Key]; ok {
					op.Target[field.Key] = value
				}
			}
		}
		if validateModuleConfig(moduleManifest{Fields: t.InputFields}, input, true) != nil {
			return op, 400, errors.New("Invalid action input")
		}
	}
	_, _, cfg, err := a.authorizedOperation(op)
	if err != nil {
		return op, 403, err
	}
	op.ConfigurationDigest = moduleConfigurationDigest(cfg)
	if len(scheduleKey) > 0 {
		digest := sha256.Sum256([]byte(scheduleKey[0]))
		op.ID = hex.EncodeToString(digest[:])
		if old, err := a.moduleOperation(op.ID); err == nil {
			return old, 202, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return op, 500, err
		}
	}
	if c.Location == "agent" {
		a.telemetryMu.RLock()
		node := a.nodes[server]
		ready := node != nil && !node.Seen.IsZero() && time.Since(node.Seen) < 15*time.Second && node.Report.Capabilities["moduleRuntime"] == "2"
		if ready && c.Runtime == "wasip1" {
			ready = node.Report.Capabilities["moduleWASI"] == "1"
		}
		a.telemetryMu.RUnlock()
		if !ready {
			return op, 409, errors.New("Agent offline or module runtime v2 update required")
		}
	}
	ops, err := a.moduleOperations()
	if err != nil {
		return op, 500, err
	}
	if err := a.expireModuleOperations(ops); err != nil {
		return op, 500, err
	}
	pending := 0
	for _, old := range ops {
		if old.Status == "pending" || old.Status == "running" {
			pending++
			if op.Mutating && old.Mutating && old.ModuleID == p.ID && old.ComponentID == c.ID && old.ServerID == server {
				return op, 409, errors.New("Another control operation is pending")
			}
		}
	}
	if pending >= 64 {
		return op, 429, errors.New("Module operation queue is full")
	}
	// Keep a bounded audit window; never remove active/uncertain recent operations.
	if len(ops) >= 1000 {
		removed := false
		for _, old := range ops {
			if old.Status != "pending" && old.Status != "running" && (old.Status != "unknown" || time.Now().After(old.ExpiresAt.Add(time.Minute))) {
				if os.Remove(a.operationPath(old.ID)) == nil {
					removed = true
					break
				}
			}
		}
		if !removed {
			return op, 429, errors.New("Module audit capacity reached")
		}
	}
	if op.ID == "" {
		token, e := randomToken()
		if e != nil {
			return op, 500, e
		}
		digest := sha256.Sum256([]byte(token))
		op.ID = hex.EncodeToString(digest[:])
	}
	err = a.saveModuleOperation(op)
	if err != nil {
		return op, 500, err
	}
	return op, 202, nil
}

type moduleLease struct {
	Runtime           string           `json:"runtime,omitempty"`
	ID                string           `json:"id"`
	ModuleID          string           `json:"moduleId"`
	ComponentID       string           `json:"componentId"`
	Version           string           `json:"version"`
	SHA256            string           `json:"sha256"`
	Mutating          bool             `json:"mutating"`
	Invocation        moduleInvocation `json:"invocation"`
	ExpiresAt         time.Time        `json:"expiresAt"`
	SecretFields      []string         `json:"secretFields"`
	SecretInputFields []string         `json:"secretInputFields"`
}

func (a *app) claimModuleOperation(op moduleOperation, target string) (moduleLease, error) {
	var lease moduleLease
	p, c, cfg, err := a.authorizedOperation(op)
	if err != nil {
		return lease, err
	}
	if c.Runtime == "wasip1" {
		if c.Location == "agent" {
			a.telemetryMu.RLock()
			node := a.nodes[op.ServerID]
			supported := node != nil && node.Report.Capabilities["moduleWASI"] == "1"
			a.telemetryMu.RUnlock()
			if !supported {
				return lease, errors.New("WASI Agent update required")
			}
		}
		target = "wasip1-wasm"
	}
	binary, ok := c.Binaries[target]
	if !ok {
		return lease, errors.New("Unsupported component platform")
	}
	op.Status = "running"
	op.StartedAt = time.Now().UTC()
	op.Platform = target
	op.SHA256 = binary.SHA256
	secrets := []string{}
	for _, f := range c.Fields {
		if f.Type == "password" {
			if s, ok := cfg.Config[f.Key].(string); ok && s != "" {
				secrets = append(secrets, s)
			}
		}
	}
	if t, ok := manualModuleTrigger(c, op.Action); ok {
		for _, f := range t.InputFields {
			if f.Type == "password" {
				if s, ok := op.Input[f.Key].(string); ok && s != "" {
					secrets = append(secrets, s)
				}
			}
		}
	}
	op.Secrets = secrets
	if err := a.saveModuleOperation(op); err != nil {
		return lease, err
	}
	state := json.RawMessage(`{}`)
	if slices.Contains(cfg.Grants, "state.read") {
		var stored json.RawMessage
		if readModuleJSON(a.componentStatePath(op), &stored) == nil {
			state = stored
		}
	}
	input := op.Input
	if input == nil {
		input = map[string]any{}
	}
	lease = moduleLease{ID: op.ID, ModuleID: p.ID, ComponentID: c.ID, Version: p.Version, SHA256: binary.SHA256, Mutating: op.Mutating, Invocation: moduleInvocation{Protocol: 2, ID: op.ID, ServerID: op.ServerID, Trigger: op.Trigger, Action: op.Action, Config: cfg.Config, Input: input, State: state, Grants: cfg.Grants, Event: op.Event}}
	if c.Runtime == "wasip1" {
		lease.Runtime = "wasip1"
	}
	lease.ExpiresAt = op.ExpiresAt
	lease.SecretFields = append([]string{}, cfg.SecretKeys...)
	lease.SecretInputFields = []string{}
	for _, field := range c.Fields {
		if field.Type == "password" {
			lease.SecretFields = append(lease.SecretFields, field.Key)
		}
	}
	if trigger, ok := manualModuleTrigger(c, op.Action); ok {
		for _, field := range trigger.InputFields {
			if field.Type == "password" {
				lease.SecretInputFields = append(lease.SecretInputFields, field.Key)
			}
		}
	}
	return lease, nil
}
func (a *app) componentStatePath(op moduleOperation) string {
	scope := moduleScopeKey(op.ServerID)
	if op.Runtime == "wasip1" {
		scope += "-wasip1"
	}
	return filepath.Join(a.dir, "extension-state", op.ModuleID, op.ComponentID, scope+".json")
}
func (a *app) finishModuleOperation(op moduleOperation, out moduleInvocationResult, runErr error) error {
	op.FinishedAt = time.Now().UTC()
	op.Status = "succeeded"
	op.Error = ""
	if runErr != nil {
		op.Status = "failed"
		if op.Mutating {
			op.Status = "unknown"
		}
		op.Error = "Component failed or timed out; check observed state before retrying"
	} else if out.Status != "ok" {
		op.Status = "failed"
		if out.Status == "unknown" {
			op.Status = "unknown"
		}
		op.Error = "Component reported an error"
		if out.Error != "" {
			op.Error = out.Error
			op.Error = moduleSecretReplacer(op.Secrets).Replace(op.Error)
		}
	}
	if len(out.Data) > 0 {
		op.Data = redactModuleData(out.Data, op.Secrets)
		if len(op.Data) == 0 {
			op.Status = "failed"
			if op.Mutating {
				op.Status = "unknown"
			}
			op.Error = "Sanitized component output exceeds limit"
			op.Data = json.RawMessage(`{}`)
		}
	}
	if runErr == nil {
		_, _, cfg, err := a.authorizedOperation(op)
		if err != nil {
			op.Status = "unknown"
			op.Error = "Component authorization changed during execution"
		} else if len(out.State) > 0 && !slices.Contains(cfg.Grants, "state.write") {
			op.Status = "failed"
			if op.Mutating {
				op.Status = "unknown"
			}
			op.Error = "Component attempted an ungranted state write"
		} else if len(out.State) > 0 && slices.Contains(cfg.Grants, "state.write") {
			encoded, _ := json.Marshal(out.State)
			if len(encoded) > 1024*1024 {
				op.Status = "failed"
				if op.Mutating {
					op.Status = "unknown"
				}
				op.Error = "Encoded component state exceeds limit"
			} else {
				path := a.componentStatePath(op)
				if os.MkdirAll(filepath.Dir(path), 0700) != nil || saveJSON(path, out.State) != nil {
					return errors.New("Could not persist component state")
				}
			}
		}
	}
	if op.Target != nil {
		raw, _ := json.Marshal(op.Target)
		var target map[string]any
		json.Unmarshal(redactModuleData(raw, op.Secrets), &target)
		op.Target = target
	}
	op.Input = nil
	op.Secrets = nil
	op.Event = nil
	if err := a.saveModuleOperation(op); err != nil {
		return err
	}
	if op.Trigger == "interval" {
		return a.saveComponentSnapshot(op)
	}
	return nil
}

func (a *app) moduleOperationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/extensions/{module}/components/{component}/operations", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Action string         `json:"action"`
			Input  map[string]any `json:"input"`
		}
		if decodeJSON(w, r, &input) != nil {
			apiError(w, 400, "Invalid action request")
			return
		}
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
		op, status, err := a.queueModuleOperation(p, c, server, "manual", input.Action, input.Input, nil)
		if err != nil {
			apiError(w, status, err.Error())
			return
		}
		writeJSON(w, 202, publicModuleOperation(op))
	})
	mux.HandleFunc("GET /api/module-operations/{operation}", func(w http.ResponseWriter, r *http.Request) {
		a.extensionMu.Lock()
		defer a.extensionMu.Unlock()
		op, err := a.moduleOperation(r.PathValue("operation"))
		if err != nil {
			apiError(w, 404, "Operation not found")
			return
		}
		writeJSON(w, 200, publicModuleOperation(op))
	})
	mux.HandleFunc("GET /api/agent/{id}/module-operations", func(w http.ResponseWriter, r *http.Request) {
		if !a.agentAuthorized(r) {
			apiError(w, 401, "Invalid agent credentials")
			return
		}
		a.extensionMu.Lock()
		defer a.extensionMu.Unlock()
		target := r.URL.Query().Get("os") + "-" + r.URL.Query().Get("arch")
		ops, err := a.moduleOperations()
		if err != nil {
			apiError(w, 500, "Could not read operation queue")
			return
		}
		for _, op := range ops {
			if op.Location == "agent" && op.ServerID == r.PathValue("id") && op.Status == "running" && time.Now().Before(op.ExpiresAt) {
				writeJSON(w, 200, []moduleLease{})
				return
			}
		}
		for _, op := range ops {
			if op.Status != "pending" || op.Location != "agent" || op.ServerID != r.PathValue("id") {
				continue
			}
			if time.Now().After(op.ExpiresAt) {
				op.Status = "expired"
				op.FinishedAt = time.Now().UTC()
				_ = a.saveModuleOperation(op)
				continue
			}
			lease, err := a.claimModuleOperation(op, target)
			if err != nil {
				op.Status = "failed"
				op.Error = "Component unavailable or permission revoked"
				_ = a.saveModuleOperation(op)
				continue
			}
			writeJSON(w, 200, []moduleLease{lease})
			return
		}
		writeJSON(w, 200, []moduleLease{})
	})
	mux.HandleFunc("GET /api/agent/{id}/module-operations/{operation}/status", func(w http.ResponseWriter, r *http.Request) {
		if !a.agentAuthorized(r) {
			apiError(w, 401, "Invalid agent credentials")
			return
		}
		a.extensionMu.Lock()
		defer a.extensionMu.Unlock()
		op, err := a.moduleOperation(r.PathValue("operation"))
		if err != nil || op.ServerID != r.PathValue("id") {
			apiError(w, 404, "Operation not found")
			return
		}
		_, _, _, err = a.authorizedOperation(op)
		writeJSON(w, 200, map[string]bool{"allowed": err == nil && op.Status == "running" && time.Now().Before(op.ExpiresAt)})
	})
	mux.HandleFunc("GET /api/agent/{id}/extensions/{module}/{component}/binary", func(w http.ResponseWriter, r *http.Request) {
		if !a.agentAuthorized(r) {
			apiError(w, 401, "Invalid agent credentials")
			return
		}
		a.extensionMu.Lock()
		defer a.extensionMu.Unlock()
		op, err := a.moduleOperation(r.URL.Query().Get("operation"))
		if err != nil || op.Status != "running" || op.ServerID != r.PathValue("id") || op.ModuleID != r.PathValue("module") || op.ComponentID != r.PathValue("component") || time.Now().After(op.ExpiresAt) {
			apiError(w, 403, "Invalid operation artifact request")
			return
		}
		if _, _, _, err = a.authorizedOperation(op); err != nil {
			apiError(w, 403, "Component permission revoked")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, filepath.Join(a.dir, "extensions", op.ModuleID, op.Version, op.ComponentID, moduleArtifactFilename(op.Platform)))
	})
	mux.HandleFunc("POST /api/agent/{id}/module-operations/{operation}/result", func(w http.ResponseWriter, r *http.Request) {
		if !a.agentAuthorized(r) {
			apiError(w, 401, "Invalid agent credentials")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 768*1024)
		var report struct {
			Result moduleInvocationResult `json:"result"`
			Failed bool                   `json:"failed"`
		}
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if d.Decode(&report) != nil || !errors.Is(d.Decode(new(any)), io.EOF) {
			apiError(w, 400, "Invalid component result")
			return
		}
		a.extensionMu.Lock()
		defer a.extensionMu.Unlock()
		op, err := a.moduleOperation(r.PathValue("operation"))
		if err != nil || op.ServerID != r.PathValue("id") || op.Location != "agent" {
			apiError(w, 404, "Operation not found")
			return
		}
		if op.Status != "running" && op.Status != "unknown" {
			writeJSON(w, 200, map[string]bool{"accepted": true})
			return
		}
		if time.Now().After(op.ExpiresAt.Add(time.Minute)) {
			apiError(w, 409, "Operation receipt expired")
			return
		}
		var runErr error
		if report.Failed {
			runErr = errors.New("Agent component failed")
		}
		if runErr == nil && validateModuleInvocationResult(&report.Result) != nil {
			apiError(w, 400, "Invalid component protocol")
			return
		}
		if err = a.finishModuleOperation(op, report.Result, runErr); err != nil {
			apiError(w, 500, "Could not persist operation result")
			return
		}
		writeJSON(w, 200, map[string]bool{"accepted": true})
	})
}

// Native Monitor components run independently of telemetry and HTTP handlers.
func (a *app) runModuleRuntime(ctx context.Context) error {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		_ = a.scheduleExtensionOperations(time.Now().UTC())
		a.extensionMu.Lock()
		ops, err := a.moduleOperations()
		var lease moduleLease
		var selected moduleOperation
		if err == nil {
			err = a.expireModuleOperations(ops)
		}
		if err == nil {
			for _, op := range ops {
				if op.Status == "running" && time.Now().After(op.ExpiresAt) {
					op.Status = "unknown"
					op.Error = "Operation deadline exceeded; not replayed"
					op.FinishedAt = time.Now().UTC()
					_ = a.saveModuleOperation(op)
				}
				if op.Status == "pending" && op.Location == "monitor" {
					if time.Now().After(op.ExpiresAt) {
						op.Status = "expired"
						_ = a.saveModuleOperation(op)
						continue
					}
					lease, err = a.claimModuleOperation(op, runtime.GOOS+"-"+runtime.GOARCH)
					if err == nil {
						selected, _ = a.moduleOperation(op.ID)
						break
					}
					op.Status = "failed"
					op.Error = "Component unavailable or permission revoked"
					_ = a.saveModuleOperation(op)
				}
			}
		}
		a.extensionMu.Unlock()
		// Queue errors prevent dispatch; authenticated API calls expose storage errors.
		if lease.ID != "" {
			path := filepath.Join(a.dir, "extensions", lease.ModuleID, lease.Version, lease.ComponentID, moduleArtifactFilename(selected.Platform))
			data, readErr := os.ReadFile(path)
			sum := sha256.Sum256(data)
			var result moduleInvocationResult
			runErr := readErr
			child, cancel := context.WithTimeout(ctx, 30*time.Second)
			a.extensionMu.Lock()
			current, e := a.moduleOperation(lease.ID)
			if e != nil || current.Status != "running" {
				runErr = errors.New("Operation was revoked before execution")
			} else if _, _, _, e = a.authorizedOperation(current); e != nil {
				runErr = e
			} else {
				a.moduleCancels[lease.ID] = cancel
			}
			a.extensionMu.Unlock()
			if runErr == nil && !strings.EqualFold(hex.EncodeToString(sum[:]), lease.SHA256) {
				runErr = errors.New("Artifact digest changed")
			}
			if runErr == nil {
				mode := os.FileMode(0700)
				if lease.Runtime == "wasip1" {
					mode = 0600
				}
				runErr = os.Chmod(path, mode)
			}
			if runErr == nil {
				result, runErr = a.runModuleLease(child, path, lease)
			}
			cancel()
			a.extensionMu.Lock()
			delete(a.moduleCancels, lease.ID)
			if current, e := a.moduleOperation(lease.ID); e == nil {
				selected = current
			}
			finishErr := a.finishModuleOperation(selected, result, runErr)
			a.extensionMu.Unlock()
			if finishErr != nil {
				slog.Error("Component result could not be persisted", "operation", selected.ID)
			}
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
	}
}
