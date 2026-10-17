package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type hardware struct {
	CPUCores int     `json:"cpuCores"`
	CPUModel string  `json:"cpuModel"`
	MemoryGB float64 `json:"memoryGB"`
	Uptime   uint64  `json:"uptime"`
	Hostname string  `json:"hostname"`
}
type agentReport struct {
	LogCursor    string            `json:"logCursor,omitempty"`
	BootID       string            `json:"bootId"`
	Sequence     uint64            `json:"sequence"`
	OS           string            `json:"os"`
	Hardware     hardware          `json:"hardware"`
	Sample       sample            `json:"sample"`
	Processes    []process         `json:"processes"`
	Disks        []disk            `json:"disks"`
	GPUs         []gpu             `json:"gpus"`
	Connections  []connection      `json:"connections"`
	Packets      []packet          `json:"packets"`
	Logs         []logEntry        `json:"logs"`
	Capabilities map[string]string `json:"capabilities"`
}
type agentNode struct {
	RetiredBoots []string    `json:"retiredBoots,omitempty"`
	Hash         string      `json:"hash"`
	Seen         time.Time   `json:"seen"`
	Report       agentReport `json:"report"`
	History      []sample    `json:"-"`
	Restored     bool        `json:"-"`
}

func saveJSON(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pulse-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	_, writeErr := f.Write(raw)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func (a *app) nodePath(id string) string    { return filepath.Join(a.dir, "agents", id+".json") }
func (a *app) historyPath(id string) string { return filepath.Join(a.dir, "agents", id+".jsonl") }
func (a *app) loadTelemetry() error {
	a.nodes = map[string]*agentNode{}
	if err := os.MkdirAll(filepath.Join(a.dir, "agents"), 0700); err != nil {
		return err
	}
	for _, s := range a.servers {
		raw, err := os.ReadFile(a.nodePath(s.ID))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		n := new(agentNode)
		if err := json.Unmarshal(raw, n); err != nil {
			return fmt.Errorf("agent %s: %w", s.ID, err)
		}
		n.Seen = time.Time{}
		n.Restored = true
		f, err := os.Open(a.historyPath(s.ID))
		if err == nil {
			scan := bufio.NewScanner(f)
			for scan.Scan() {
				var v sample
				if err := json.Unmarshal(scan.Bytes(), &v); err != nil {
					f.Close()
					return err
				}
				stamp, _ := time.Parse(time.RFC3339Nano, v.At)
				if time.Since(stamp) <= 24*time.Hour {
					n.History = append(n.History, v)
				}
			}
			scanErr := scan.Err()
			f.Close()
			if scanErr != nil {
				return scanErr
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		a.nodes[s.ID] = n
	}
	return nil
}
func (a *app) issueAgentToken(id string) (string, error) {
	a.ingestMu.Lock()
	defer a.ingestMu.Unlock()
	if _, ok := a.findServer(id); !ok {
		return "", errors.New("server not found")
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))
	a.telemetryMu.RLock()
	old := a.nodes[id]
	n := agentNode{}
	if old != nil {
		n = *old
	}
	a.telemetryMu.RUnlock()
	n.Hash = hex.EncodeToString(hash[:])
	n.Seen = time.Time{}
	n.Report.BootID = ""
	n.Report.Sequence = 0
	n.RetiredBoots = nil
	if err := saveJSON(a.nodePath(id), n); err != nil {
		return "", err
	}
	a.telemetryMu.Lock()
	a.nodes[id] = &n
	a.telemetryMu.Unlock()
	return token, nil
}
func (a *app) agentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/servers/{id}/token", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.findServer(r.PathValue("id")); !ok {
			apiError(w, 404, "Server not found")
			return
		}
		token, err := a.issueAgentToken(r.PathValue("id"))
		if err != nil {
			apiError(w, 500, "Could not issue token")
			return
		}
		writeJSON(w, 201, map[string]string{"token": token, "serverId": r.PathValue("id")})
	})
	mux.HandleFunc("DELETE /api/servers/{id}/token", func(w http.ResponseWriter, r *http.Request) {
		a.ingestMu.Lock()
		defer a.ingestMu.Unlock()
		id := r.PathValue("id")
		a.telemetryMu.RLock()
		old := a.nodes[id]
		a.telemetryMu.RUnlock()
		if old == nil {
			apiError(w, 404, "Agent not enrolled")
			return
		}
		n := *old
		n.Hash = ""
		n.Seen = time.Time{}
		if err := saveJSON(a.nodePath(id), n); err != nil {
			apiError(w, 500, "Could not revoke token")
			return
		}
		a.telemetryMu.Lock()
		a.nodes[id] = &n
		a.telemetryMu.Unlock()
		writeJSON(w, 200, map[string]bool{"revoked": true})
	})
	mux.HandleFunc("POST /api/agent/{id}/report", a.receiveReport)
}
func validateReport(v agentReport) error {
	if !validID.MatchString(v.BootID) || v.Sequence == 0 || len(v.OS) == 0 || len(v.OS) > 80 || len(v.Processes) > 2048 || len(v.Disks) > 128 || len(v.GPUs) > 32 || len(v.Connections) > 4096 || len(v.Packets) > 256 || len(v.Logs) > 100 || len(v.Capabilities) > 16 {
		return errors.New("invalid report identity or size")
	}
	for _, x := range []float64{v.Sample.CPU, v.Sample.Memory} {
		if x < 0 || x > 100 {
			return errors.New("invalid utilization")
		}
	}
	for _, x := range []float64{v.Sample.Incoming, v.Sample.Outgoing, v.Sample.Read, v.Sample.Write, v.Hardware.MemoryGB} {
		if x < 0 {
			return errors.New("negative metric")
		}
	}
	for _, p := range v.Processes {
		if p.PID < 0 || p.CPU < 0 || p.CPU > 100 || p.Memory < 0 || len(p.Name) > 256 || len(p.Command) > 4096 || len(p.User) > 256 || (p.GPU != nil && (*p.GPU < 0 || *p.GPU > 100)) || (p.GPUMemory != nil && *p.GPUMemory < 0) {
			return errors.New("invalid process")
		}
	}
	for _, g := range v.GPUs {
		if g.Utilization < 0 || g.Utilization > 100 || g.MemoryUsed < 0 || g.MemoryTotal < g.MemoryUsed || len(g.History) > 61 || len(g.Name) > 256 {
			return errors.New("invalid GPU")
		}
	}
	for _, d := range v.Disks {
		if d.TotalGB < 0 || d.UsedGB < 0 || d.UsedGB > d.TotalGB || d.Read < 0 || d.Write < 0 || len(d.Mount) > 1024 {
			return errors.New("invalid disk")
		}
	}
	for _, l := range v.Logs {
		if !slices.Contains([]string{"INFO", "WARN", "ERROR"}, l.Level) || len(l.Source) > 128 || len(l.Message) > 8192 {
			return errors.New("invalid log")
		}
		if l.Time != "" {
			if _, err := time.Parse(time.RFC3339Nano, l.Time); err != nil {
				return errors.New("invalid log time")
			}
		}
	}
	return nil
}
func (a *app) receiveReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := a.findServer(id); !ok {
		apiError(w, 401, "Invalid agent credentials")
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	hash := sha256.Sum256([]byte(token))
	encoded := hex.EncodeToString(hash[:])
	a.ingestMu.Lock()
	defer a.ingestMu.Unlock()
	a.telemetryMu.RLock()
	old := a.nodes[id]
	a.telemetryMu.RUnlock()
	if old == nil || old.Hash == "" || len(token) != 64 || subtle.ConstantTimeCompare([]byte(old.Hash), []byte(encoded)) != 1 {
		apiError(w, 401, "Invalid agent credentials")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var report agentReport
	if err := d.Decode(&report); err != nil {
		apiError(w, 400, "Invalid report JSON")
		return
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		apiError(w, 400, "Expected one report")
		return
	}
	if err := validateReport(report); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	if slices.Contains(old.RetiredBoots, report.BootID) {
		apiError(w, 409, "Superseded agent instance")
		return
	}
	if report.BootID == old.Report.BootID && report.Sequence <= old.Report.Sequence {
		n := *old
		n.Seen = time.Now().UTC()
		a.telemetryMu.Lock()
		a.nodes[id] = &n
		a.telemetryMu.Unlock()
		writeJSON(w, 200, map[string]bool{"accepted": true})
		return
	}
	now := time.Now().UTC()
	report.Sample.At = now.Format(time.RFC3339Nano)
	n := *old
	if old.Report.BootID != "" && old.Report.BootID != report.BootID {
		n.RetiredBoots = append(slices.Clone(old.RetiredBoots), old.Report.BootID)
		if len(n.RetiredBoots) > 128 {
			apiError(w, 409, "Rotate the agent token before further restarts")
			return
		}
	}
	n.Report = report
	n.Seen = now
	n.Restored = false
	n.History = slices.Clone(old.History)
	if len(n.History) == 0 || sampleAge(n.History[len(n.History)-1], now) >= 10*time.Second {
		f, err := os.OpenFile(a.historyPath(id), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			apiError(w, 500, "Could not save history")
			return
		}
		err = json.NewEncoder(f).Encode(report.Sample)
		syncErr := f.Sync()
		closeErr := f.Close()
		if errors.Join(err, syncErr, closeErr) != nil {
			apiError(w, 500, "Could not save history")
			return
		}
		n.History = append(n.History, report.Sample)
	}
	for len(n.History) > 0 && sampleAge(n.History[0], now) > 24*time.Hour {
		n.History = n.History[1:]
	}
	logs := slices.Clone(report.Logs)
	for i := range logs {
		logs[i].ServerID = id
		logs[i].DataSource = "agent"
		logs[i].ID = fmt.Sprintf("%s-%s-%d-%d", id, report.BootID, report.Sequence, i)
	}
	if err := a.appendAgentLogs(logs); err != nil {
		apiError(w, 500, "Could not save logs")
		return
	}
	if err := a.evaluateAgentEvents(id, old, &n); err != nil {
		apiError(w, 500, "Could not save events")
		return
	}
	n.Seen = time.Now().UTC()
	if err := saveJSON(a.nodePath(id), n); err != nil {
		apiError(w, 500, "Could not save report")
		return
	}
	a.telemetryMu.Lock()
	a.nodes[id] = &n
	a.telemetryMu.Unlock()
	writeJSON(w, 200, map[string]bool{"accepted": true})
}
func sampleAge(v sample, now time.Time) time.Duration {
	stamp, _ := time.Parse(time.RFC3339Nano, v.At)
	return now.Sub(stamp)
}
func (a *app) appendAgentLog(l logEntry) error {
	return a.appendAgentLogs([]logEntry{l})
}

// Retry deduplication scans the archive once per report, never once per line.
// A single synced append keeps acknowledgements durable, including partial retries.
func (a *app) appendAgentLogs(rows []logEntry) error {
	if len(rows) == 0 {
		return nil
	}
	a.logMu.Lock()
	defer a.logMu.Unlock()
	path := filepath.Join(a.dir, "logs.jsonl")
	wanted := make(map[string]bool, len(rows))
	for _, row := range rows {
		wanted[row.ID] = true
	}
	f, err := os.Open(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 4096), 64*1024)
		for scanner.Scan() {
			var entry struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				f.Close()
				return fmt.Errorf("decode log archive: %w", err)
			}
			delete(wanted, entry.ID)
		}
		scanErr := scanner.Err()
		closeErr := f.Close()
		if err := errors.Join(scanErr, closeErr); err != nil {
			return err
		}
	}
	var data strings.Builder
	for _, entry := range rows {
		if !wanted[entry.ID] {
			continue
		}
		if entry.Time == "" {
			entry.Time = time.Now().UTC().Format(time.RFC3339Nano)
		}
		raw, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		data.Write(raw)
		data.WriteByte('\n')
		delete(wanted, entry.ID)
	}
	if data.Len() == 0 {
		return nil
	}
	f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(f, data.String())
	syncErr := f.Sync()
	closeErr := f.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}
func (a *app) resetAgent(id string, clear bool) error {
	a.ingestMu.Lock()
	defer a.ingestMu.Unlock()
	a.telemetryMu.RLock()
	old := a.nodes[id]
	a.telemetryMu.RUnlock()
	if old == nil {
		return nil
	}
	n := *old
	n.Hash = ""
	n.Seen = time.Time{}
	if clear {
		n.Report = agentReport{}
	}
	if err := saveJSON(a.nodePath(id), n); err != nil {
		return err
	}
	a.telemetryMu.Lock()
	a.nodes[id] = &n
	a.telemetryMu.Unlock()
	return nil
}
func (a *app) agentSnapshot(s server, period int) (snapshot, bool) {
	a.telemetryMu.RLock()
	defer a.telemetryMu.RUnlock()
	n := a.nodes[s.ID]
	if n == nil && s.OS != "" && s.Source != "agent" && s.Source != "waiting" {
		return snapshot{}, false
	}
	out := snapshot{ServerID: s.ID, Source: "waiting", At: time.Now().UTC().Format(time.RFC3339Nano), Samples: []sample{}, Processes: []process{}, Disks: []disk{}, GPUs: []gpu{}, Connections: []connection{}, Packets: []packet{}}
	if n == nil || n.Report.OS == "" {
		return out, true
	}
	out.Source = "agent"
	out.At = n.Report.Sample.At
	out.Tick = int(n.Report.Sequence)
	out.Hardware = n.Report.Hardware
	out.Capabilities = n.Report.Capabilities
	values := []sample{}
	cut := time.Now().Add(-time.Duration(period) * time.Minute)
	for _, v := range n.History {
		stamp, _ := time.Parse(time.RFC3339Nano, v.At)
		if !stamp.Before(cut) {
			values = append(values, v)
		}
	}
	if len(values) > 0 && values[len(values)-1].At != n.Report.Sample.At {
		values = append(values, n.Report.Sample)
	}
	if len(values) > 61 {
		for i := range 61 {
			out.Samples = append(out.Samples, values[i*(len(values)-1)/60])
		}
	} else {
		out.Samples = values
	}
	if n.Report.Processes != nil {
		out.Processes = n.Report.Processes
	}
	if n.Report.Disks != nil {
		out.Disks = n.Report.Disks
	}
	if n.Report.GPUs != nil {
		out.GPUs = n.Report.GPUs
	}
	if n.Report.Connections != nil {
		out.Connections = n.Report.Connections
	}
	if n.Report.Packets != nil {
		out.Packets = n.Report.Packets
	}
	return out, true
}
