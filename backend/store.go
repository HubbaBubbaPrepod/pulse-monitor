package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

type server struct {
	Source           string  `json:"source,omitempty"`
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Address          string  `json:"address"`
	OS               string  `json:"os"`
	Group            string  `json:"group"`
	Online           bool    `json:"online"`
	CPU              float64 `json:"cpu"`
	Memory           float64 `json:"memory"`
	Disk             float64 `json:"disk"`
	Interval         int     `json:"interval"`
	Threshold        int     `json:"threshold"`
	LogRetentionDays *int    `json:"logRetentionDays,omitempty"`
}
type logEntry struct {
	DataSource string `json:"dataSource,omitempty"`
	ID         string `json:"id"`
	ServerID   string `json:"serverId"`
	Time       string `json:"time"`
	Level      string `json:"level"`
	Source     string `json:"source"`
	Message    string `json:"message"`
}
type logPage struct {
	Items []logEntry `json:"items"`
	Total int        `json:"total"`
}
type app struct {
	mu            sync.RWMutex
	writeMu       sync.Mutex
	logMu         sync.RWMutex
	storage       *sqliteStore
	servers       []server
	retentions    map[string]int
	dir           string
	started       time.Time
	auth          *authState
	telemetryMu   sync.RWMutex
	ingestMu      sync.Mutex
	nodes         map[string]*agentNode
	publicHost    string
	eventMu       sync.Mutex
	events        []monitorEvent
	extensionMu   sync.Mutex
	moduleCancels map[string]context.CancelFunc
}

func newApp(dir string) (result *app, initErr error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	a := &app{dir: dir, started: time.Now().UTC(), servers: []server{}}
	raw, err := os.ReadFile(filepath.Join(dir, "servers.json"))
	if err == nil {
		if err := json.Unmarshal(raw, &a.servers); err != nil {
			return nil, fmt.Errorf("read server definitions: %w", err)
		}
		for _, s := range a.servers {
			if err := validateServer(s); err != nil {
				return nil, fmt.Errorf("invalid saved server: %w", err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read server definitions: %w", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pulse.db")); errors.Is(err, os.ErrNotExist) {
		if err := a.migrateDemo(); err != nil {
			return nil, err
		}
	}
	a.auth, err = loadAuth(dir)
	if err != nil {
		return nil, err
	}
	a.storage, err = openSQLite(dir)
	if err != nil {
		return nil, err
	}
	defer func() {
		if initErr != nil {
			a.storage.Close()
		}
	}()
	if err := a.storage.migrateLegacy(context.Background(), dir, a.servers); err != nil {
		return nil, err
	}
	a.retentions, err = a.storage.retentionPolicies(context.Background())
	if err != nil {
		return nil, err
	}
	if err := a.loadTelemetry(); err != nil {
		return nil, err
	}
	if err := a.loadEvents(); err != nil {
		return nil, err
	}
	a.moduleCancels = map[string]context.CancelFunc{}
	if err := a.recoverModuleOperations(); err != nil {
		return nil, err
	}
	return a, nil
}
func (a *app) serverList() []server {
	a.mu.RLock()
	defer a.mu.RUnlock()
	list := slices.Clone(a.servers)
	a.telemetryMu.RLock()
	defer a.telemetryMu.RUnlock()
	for i := range list {
		days := a.retentions[list[i].ID]
		list[i].LogRetentionDays = &days
		if n := a.nodes[list[i].ID]; n != nil {
			list[i].Source = "agent"
			list[i].Online = n.Hash != "" && !n.Seen.IsZero() && time.Since(n.Seen) < 15*time.Second
			list[i].OS = n.Report.OS
			list[i].CPU = n.Report.Sample.CPU
			list[i].Memory = n.Report.Sample.Memory
			if len(n.Report.Disks) > 0 && n.Report.Disks[0].TotalGB > 0 {
				list[i].Disk = 100 * n.Report.Disks[0].UsedGB / n.Report.Disks[0].TotalGB
			}
		} else if list[i].OS == "" || list[i].Source == "agent" || list[i].Source == "waiting" {
			list[i].Source = "waiting"
			list[i].Online = false
			list[i].CPU = 0
			list[i].Memory = 0
			list[i].Disk = 0
		} else {
			list[i].Source = "waiting"
			list[i].Online = false
			list[i].CPU = 0
			list[i].Memory = 0
			list[i].Disk = 0
		}
	}
	return list
}
func (a *app) findServer(id string) (server, bool) {
	list := a.serverList()
	i := slices.IndexFunc(list, func(s server) bool { return s.ID == id })
	if i < 0 {
		return server{}, false
	}
	return list[i], true
}

func (a *app) dataSource() string {
	rows := a.serverList()
	if len(rows) == 0 {
		return "waiting"
	}
	source := rows[0].Source
	for _, s := range rows {
		if s.Source != source {
			return "mixed"
		}
	}
	return source
}

var validID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func validateServer(s server) error {
	// Push agents connect to the Monitor origin, never to this optional label.
	address := strings.TrimSpace(s.Address)
	if address != "" {
		if !strings.Contains(address, "://") {
			address = "https://" + address
		}
		u, err := url.Parse(address)
		if err != nil || u.Hostname() == "" || len(address) > 512 || strings.ContainsAny(address, " \t\r\n") || !slices.Contains([]string{"http", "https"}, u.Scheme) || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("invalid optional server address")
		}
	}
	if !validID.MatchString(s.ID) || strings.TrimSpace(s.Name) == "" || len(s.Name) > 48 || len(s.OS) > 80 || len(s.Group) > 32 {
		return errors.New("invalid server identity")
	}
	for _, v := range []float64{s.CPU, s.Memory, s.Disk} {
		if v < 0 || v > 100 {
			return errors.New("metrics must be between 0 and 100")
		}
	}
	if !slices.Contains([]int{2, 5, 10, 30}, s.Interval) || s.Threshold < 50 || s.Threshold > 99 {
		return errors.New("invalid monitoring settings")
	}
	return nil
}
func (a *app) persistServers(list []server) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(a.dir, "servers-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(a.dir, "servers.json"))
}
func (a *app) appendLog(entry logEntry) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.storage.appendLogs(ctx, []logEntry{entry}, time.Now().UTC())
}
func (a *app) logHistory(id, level, source, query, before string, offset, limit int, category ...string) (logPage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cat := ""
	if len(category) > 0 {
		cat = category[0]
	}
	return a.storage.queryLogs(ctx, id, level, source, query, before, offset, limit, cat)
}
