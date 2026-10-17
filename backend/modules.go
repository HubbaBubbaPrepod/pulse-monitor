package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"debug/elf"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// validateCollectorPlatform checks the executable header against the declared target.
// The digest verifies integrity; it does not establish trust in the module author.
func validateCollectorPlatform(target string, content []byte) error {
	if target == "windows-amd64" {
		file, err := pe.NewFile(bytes.NewReader(content))
		if err != nil {
			return errors.New("Invalid Windows collector executable")
		}
		defer file.Close()
		if file.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return errors.New("Collector architecture mismatch")
		}
		return nil
	}
	file, err := elf.NewFile(bytes.NewReader(content))
	if err != nil {
		return errors.New("Invalid Linux collector executable")
	}
	defer file.Close()
	if file.Class != elf.ELFCLASS64 || file.Data != elf.ELFDATA2LSB || (target == "linux-amd64" && file.Machine != elf.EM_X86_64) || (target == "linux-arm64" && file.Machine != elf.EM_AARCH64) {
		return errors.New("Collector architecture mismatch")
	}
	return nil
}

type moduleBinary struct {
	SHA256  string `json:"sha256"`
	Content []byte `json:"content,omitempty"`
}
type moduleField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Default  any    `json:"default,omitempty"`
}
type moduleColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Unit  string `json:"unit,omitempty"`
}
type moduleView struct {
	Kind    string         `json:"kind"`
	Title   string         `json:"title"`
	Key     string         `json:"key"`
	Columns []moduleColumn `json:"columns"`
}
type moduleManifest struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Protocol      int                     `json:"protocol"`
	ID            string                  `json:"id"`
	Version       string                  `json:"version"`
	Name          string                  `json:"name"`
	Author        string                  `json:"author"`
	Description   string                  `json:"description"`
	Permissions   []string                `json:"permissions"`
	Fields        []moduleField           `json:"fields"`
	Views         []moduleView            `json:"views"`
	Binaries      map[string]moduleBinary `json:"binaries"`
}
type moduleAssignment struct {
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config"`
}
type moduleResult struct {
	At     string          `json:"at"`
	Status string          `json:"status"`
	Error  string          `json:"error,omitempty"`
	Data   json.RawMessage `json:"data"`
}

var moduleVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9-]+)?$`)

func normalizeModule(m *moduleManifest) {
	if m.Fields == nil {
		m.Fields = []moduleField{}
	}
	if m.Views == nil {
		m.Views = []moduleView{}
	}
	if m.Permissions == nil {
		m.Permissions = []string{}
	}
	for i := range m.Views {
		if m.Views[i].Columns == nil {
			m.Views[i].Columns = []moduleColumn{}
		}
	}
}

func validateModule(m moduleManifest, content bool) error {
	if m.SchemaVersion != 1 || m.Protocol != 1 || !validID.MatchString(m.ID) || !moduleVersion.MatchString(m.Version) || len(m.Version) > 48 || len(m.Name) == 0 || len(m.Name) > 64 || len(m.Author) > 128 || len(m.Description) > 2048 || len(m.Fields) > 32 || len(m.Views) > 16 || len(m.Binaries) == 0 || len(m.Binaries) > 3 || len(m.Permissions) > 16 {
		return errors.New("Invalid module manifest or protocol")
	}
	for _, permission := range m.Permissions {
		if len(permission) > 512 {
			return errors.New("Invalid permission")
		}
	}
	keys := map[string]bool{}
	for _, f := range m.Fields {
		if !validID.MatchString(f.Key) || keys[f.Key] || len(f.Label) > 128 || !slices.Contains([]string{"text", "password", "number", "boolean"}, f.Type) {
			return errors.New("Invalid module field")
		}
		keys[f.Key] = true
	}
	for _, v := range m.Views {
		if !slices.Contains([]string{"table", "cards", "chart", "logs"}, v.Kind) || !validID.MatchString(v.Key) || len(v.Title) > 128 || len(v.Columns) > 32 {
			return errors.New("Invalid module view")
		}
		for _, c := range v.Columns {
			if !validID.MatchString(c.Key) || len(c.Label) > 128 || len(c.Unit) > 32 {
				return errors.New("Invalid column")
			}
		}
	}
	total := 0
	for target, b := range m.Binaries {
		if !slices.Contains([]string{"linux-amd64", "linux-arm64", "windows-amd64"}, target) || len(b.SHA256) != 64 {
			return errors.New("Invalid module platform or hash")
		}
		if _, err := hex.DecodeString(b.SHA256); err != nil {
			return errors.New("Invalid binary digest")
		}
		if content {
			total += len(b.Content)
			if len(b.Content) == 0 || len(b.Content) > 16*1024*1024 {
				return errors.New("Invalid binary size")
			}
			hash := sha256.Sum256(b.Content)
			if !strings.EqualFold(hex.EncodeToString(hash[:]), b.SHA256) {
				return errors.New("Module binary digest mismatch")
			}
			if err := validateCollectorPlatform(target, b.Content); err != nil {
				return err
			}
		}
	}
	if total > 48*1024*1024 {
		return errors.New("Module package too large")
	}
	return nil
}
func (a *app) moduleRoot() string { return filepath.Join(a.dir, "modules") }
func (a *app) module(id string) (moduleManifest, error) {
	var m moduleManifest
	if !validID.MatchString(id) {
		return m, os.ErrNotExist
	}
	raw, err := os.ReadFile(filepath.Join(a.moduleRoot(), id, "manifest.json"))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(raw, &m)
	if err == nil {
		normalizeModule(&m)
		err = validateModule(m, false)
	}
	return m, err
}
func (a *app) moduleList() ([]moduleManifest, error) {
	items := []moduleManifest{}
	dirs, err := os.ReadDir(a.moduleRoot())
	if errors.Is(err, os.ErrNotExist) {
		return items, nil
	}
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if !dir.IsDir() || !validID.MatchString(dir.Name()) {
			continue
		}
		m, err := a.module(dir.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, nil
}
func (a *app) assignments(id string) (map[string]moduleAssignment, error) {
	out := map[string]moduleAssignment{}
	raw, err := os.ReadFile(filepath.Join(a.dir, "module-config", id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &out)
	return out, err
}
func validateModuleConfig(m moduleManifest, c map[string]any, enabled bool) error {
	known := map[string]moduleField{}
	for _, f := range m.Fields {
		known[f.Key] = f
		if enabled && f.Required && (c[f.Key] == nil || c[f.Key] == "") {
			return fmt.Errorf("Required field: %s", f.Key)
		}
	}
	for key, value := range c {
		field, ok := known[key]
		if !ok {
			return errors.New("Unknown module configuration field")
		}
		switch field.Type {
		case "text", "password":
			v, ok := value.(string)
			if !ok || len(v) > 4096 {
				return errors.New("Invalid module text field")
			}
		case "number":
			if _, ok := value.(float64); !ok {
				return errors.New("Invalid module number field")
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return errors.New("Invalid module boolean field")
			}
		}
	}
	return nil
}
func (a *app) agentAuthorized(r *http.Request) bool {
	id := r.PathValue("id")
	if _, ok := a.findServer(id); !ok {
		return false
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if len(token) != 64 {
		return false
	}
	h := sha256.Sum256([]byte(token))
	encoded := hex.EncodeToString(h[:])
	a.telemetryMu.RLock()
	defer a.telemetryMu.RUnlock()
	n := a.nodes[id]
	return n != nil && n.Hash != "" && subtle.ConstantTimeCompare([]byte(n.Hash), []byte(encoded)) == 1
}
func (a *app) moduleRoutes(mux *http.ServeMux) {
	a.modulePreflightRoutes(mux)
	a.extensionRoutes(mux)
	a.moduleOperationRoutes(mux)
	mux.HandleFunc("GET /api/modules", func(w http.ResponseWriter, r *http.Request) {
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		items, err := a.moduleList()
		if err != nil {
			apiError(w, 500, "Could not read modules")
			return
		}
		writeJSON(w, 200, items)
	})
	mux.HandleFunc("POST /api/modules", a.installModule)
	mux.HandleFunc("DELETE /api/modules/{module}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("module")
		if !validID.MatchString(id) {
			apiError(w, 400, "Invalid module ID")
			return
		}
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		if _, err := a.module(id); err != nil {
			apiError(w, 404, "Module not found")
			return
		}
		for _, s := range a.serverList() {
			items, err := a.assignments(s.ID)
			if err != nil {
				apiError(w, 500, "Could not read assignments")
				return
			}
			delete(items, id)
			if err := os.MkdirAll(filepath.Join(a.dir, "module-config"), 0700); err != nil {
				apiError(w, 500, "Could not update assignments")
				return
			}
			if err := saveJSON(filepath.Join(a.dir, "module-config", s.ID+".json"), items); err != nil {
				apiError(w, 500, "Could not update assignments")
				return
			}
		}
		if err := os.Remove(filepath.Join(a.moduleRoot(), id, "manifest.json")); err != nil {
			apiError(w, 500, "Could not remove module")
			return
		}
		writeJSON(w, 200, map[string]bool{"deleted": true})
	})
	mux.HandleFunc("GET /api/servers/{id}/modules", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.findServer(r.PathValue("id")); !ok {
			apiError(w, 404, "Server not found")
			return
		}
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		items, err := a.assignments(r.PathValue("id"))
		if err != nil {
			apiError(w, 500, "Could not read configuration")
			return
		}
		writeJSON(w, 200, items)
	})
	mux.HandleFunc("PUT /api/servers/{id}/modules/{module}", func(w http.ResponseWriter, r *http.Request) {
		id, key := r.PathValue("id"), r.PathValue("module")
		if _, ok := a.findServer(id); !ok {
			apiError(w, 404, "Server not found")
			return
		}
		var value moduleAssignment
		if err := decodeJSON(w, r, &value); err != nil {
			apiError(w, 400, "Invalid module configuration")
			return
		}
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		m, err := a.module(key)
		if err != nil {
			apiError(w, 404, "Module not found")
			return
		}
		if err := validateModuleConfig(m, value.Config, value.Enabled); err != nil {
			apiError(w, 400, err.Error())
			return
		}
		items, err := a.assignments(id)
		if err != nil {
			apiError(w, 500, "Could not read configuration")
			return
		}
		items[key] = value
		if err := os.MkdirAll(filepath.Join(a.dir, "module-config"), 0700); err != nil {
			apiError(w, 500, "Could not save configuration")
			return
		}
		if err := saveJSON(filepath.Join(a.dir, "module-config", id+".json"), items); err != nil {
			apiError(w, 500, "Could not save configuration")
			return
		}
		writeJSON(w, 200, value)
	})
	mux.HandleFunc("GET /api/servers/{id}/modules/{module}/data", func(w http.ResponseWriter, r *http.Request) {
		id, key := r.PathValue("id"), r.PathValue("module")
		if _, ok := a.findServer(id); !ok || !validID.MatchString(key) {
			apiError(w, 404, "Module data not found")
			return
		}
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		raw, err := os.ReadFile(filepath.Join(a.dir, "module-data", id, key+".json"))
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, 200, moduleResult{Status: "waiting", Data: json.RawMessage(`{}`)})
			return
		}
		var v moduleResult
		if err != nil || json.Unmarshal(raw, &v) != nil {
			apiError(w, 500, "Could not read module data")
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("GET /api/agent/{id}/modules", func(w http.ResponseWriter, r *http.Request) {
		if !a.agentAuthorized(r) {
			apiError(w, 401, "Invalid agent credentials")
			return
		}
		target := r.URL.Query().Get("os") + "-" + r.URL.Query().Get("arch")
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		assignments, err := a.assignments(r.PathValue("id"))
		if err != nil {
			apiError(w, 500, "Could not read modules")
			return
		}
		items := []map[string]any{}
		for id, value := range assignments {
			if !value.Enabled {
				continue
			}
			m, err := a.module(id)
			if err != nil {
				continue
			}
			binary, ok := m.Binaries[target]
			if !ok {
				items = append(items, map[string]any{"id": id, "version": m.Version, "unsupported": true})
				continue
			}
			items = append(items, map[string]any{"id": id, "version": m.Version, "sha256": binary.SHA256, "config": value.Config})
		}
		writeJSON(w, 200, items)
	})
	mux.HandleFunc("GET /api/agent/{id}/modules/{module}/binary", func(w http.ResponseWriter, r *http.Request) {
		if !a.agentAuthorized(r) {
			apiError(w, 401, "Invalid agent credentials")
			return
		}
		target := r.URL.Query().Get("os") + "-" + r.URL.Query().Get("arch")
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		m, err := a.module(r.PathValue("module"))
		if err != nil {
			apiError(w, 404, "Module not found")
			return
		}
		items, err := a.assignments(r.PathValue("id"))
		if err != nil || !items[m.ID].Enabled {
			apiError(w, 403, "Module is disabled")
			return
		}
		if _, ok := m.Binaries[target]; !ok {
			apiError(w, 404, "Unsupported platform")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, filepath.Join(a.moduleRoot(), m.ID, m.Version, target))
	})
	mux.HandleFunc("POST /api/agent/{id}/modules/{module}/data", func(w http.ResponseWriter, r *http.Request) {
		if !a.agentAuthorized(r) {
			apiError(w, 401, "Invalid agent credentials")
			return
		}
		id, key := r.PathValue("id"), r.PathValue("module")
		if !validID.MatchString(key) {
			apiError(w, 400, "Invalid module ID")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
		var v moduleResult
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if d.Decode(&v) != nil || !slices.Contains([]string{"ok", "error", "unsupported"}, v.Status) || len(v.Error) > 256 || !json.Valid(v.Data) {
			apiError(w, 400, "Invalid module result")
			return
		}
		if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
			apiError(w, 400, "Expected one result")
			return
		}
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		items, err := a.assignments(id)
		if err != nil || !items[key].Enabled {
			apiError(w, 403, "Module disabled")
			return
		}
		v.At = time.Now().UTC().Format(time.RFC3339Nano)
		root := filepath.Join(a.dir, "module-data", id)
		if err := os.MkdirAll(root, 0700); err != nil {
			apiError(w, 500, "Could not save module data")
			return
		}
		if err := saveJSON(filepath.Join(root, key+".json"), v); err != nil {
			apiError(w, 500, "Could not save module data")
			return
		}
		writeJSON(w, 200, map[string]bool{"accepted": true})
	})
}

func (a *app) installModule(w http.ResponseWriter, r *http.Request) {
	a.installModulePackage(w, r, "")
}

func (a *app) installModulePackage(w http.ResponseWriter, r *http.Request, marketplace string) {
	// Native multi-platform packages need longer than ordinary small API calls.
	// Keep both transfer time and body size bounded.
	controller := http.NewResponseController(w)
	deadline := time.Now().Add(2 * time.Minute)
	if err := controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		apiError(w, 500, "Could not set module upload deadline")
		return
	}
	if err := controller.SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		apiError(w, 500, "Could not set module upload deadline")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 68*1024*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		apiError(w, 400, "Invalid module package")
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		apiError(w, 400, "Expected one package")
		return
	}
	var envelope struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		apiError(w, 400, "Invalid module package")
		return
	}
	if envelope.SchemaVersion == 2 {
		writeJSON(w, 409, map[string]string{"error": "Module runtime is not available", "code": "module_runtime_unavailable"})
		return
	}
	var m moduleManifest
	typed := json.NewDecoder(bytes.NewReader(raw))
	typed.DisallowUnknownFields()
	if err := typed.Decode(&m); err != nil {
		apiError(w, 400, "Invalid module package")
		return
	}
	if err := validateModule(m, true); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	normalizeModule(&m)
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if marketplace != "" {
		config, err := a.marketplaceConfiguration()
		if err != nil || !config.Enabled || config.URL != marketplace {
			apiError(w, 409, "Marketplace settings changed")
			return
		}
		if _, err := a.module(m.ID); !errors.Is(err, os.ErrNotExist) {
			apiError(w, 409, "Module ID already installed")
			return
		}
	}
	root := filepath.Join(a.moduleRoot(), m.ID)
	if _, err := os.Stat(filepath.Join(a.dir, "extensions", m.ID, "manifest.json")); !errors.Is(err, os.ErrNotExist) {
		apiError(w, 409, "An extension with this ID is already installed")
		return
	}
	version := filepath.Join(root, m.Version)
	_, versionErr := os.Stat(version)
	if versionErr == nil {
		// Removed packages keep their binaries and data. Reactivate only the
		// identical package; a version must never silently change its bytes.
		if _, err := os.Stat(filepath.Join(root, "manifest.json")); !errors.Is(err, os.ErrNotExist) {
			apiError(w, 409, "Module version already installed")
			return
		}
		for target, b := range m.Binaries {
			raw, err := os.ReadFile(filepath.Join(version, target))
			if err != nil || !bytes.Equal(raw, b.Content) {
				apiError(w, 409, "Module version content differs")
				return
			}
			b.Content = nil
			m.Binaries[target] = b
		}
		if err := saveJSON(filepath.Join(root, "manifest.json"), m); err != nil {
			apiError(w, 500, "Could not save manifest")
			return
		}
		writeJSON(w, 201, m)
		return
	}
	if !errors.Is(versionErr, os.ErrNotExist) {
		apiError(w, 500, "Could not read module version")
		return
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		apiError(w, 500, "Could not install module")
		return
	}
	stage, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		apiError(w, 500, "Could not stage module")
		return
	}
	defer os.RemoveAll(stage)
	for target, b := range m.Binaries {
		if err := os.WriteFile(filepath.Join(stage, target), b.Content, 0600); err != nil {
			apiError(w, 500, "Could not write module binary")
			return
		}
		b.Content = nil
		m.Binaries[target] = b
	}
	if err := os.Rename(stage, version); err != nil {
		apiError(w, 500, "Could not finalize module")
		return
	}
	if err := saveJSON(filepath.Join(root, "manifest.json"), m); err != nil {
		apiError(w, 500, "Could not save manifest")
		return
	}
	writeJSON(w, 201, m)
}
