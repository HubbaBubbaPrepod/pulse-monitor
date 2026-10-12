package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
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
	Source    string  `json:"source,omitempty"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	OS        string  `json:"os"`
	Group     string  `json:"group"`
	Online    bool    `json:"online"`
	CPU       float64 `json:"cpu"`
	Memory    float64 `json:"memory"`
	Disk      float64 `json:"disk"`
	Interval  int     `json:"interval"`
	Threshold int     `json:"threshold"`
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
	mu          sync.RWMutex
	writeMu     sync.Mutex
	logMu       sync.RWMutex
	servers     []server
	dir         string
	started     time.Time
	auth        *authState
	telemetryMu sync.RWMutex
	ingestMu    sync.Mutex
	nodes       map[string]*agentNode
	publicHost  string
	eventMu     sync.Mutex
	events      []monitorEvent
}

func newApp(dir string) (*app, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	a := &app{dir: dir, started: time.Now().UTC(), servers: demoServers()}
	raw, err := os.ReadFile(filepath.Join(dir, "servers.json"))
	if err == nil {
		if err := json.Unmarshal(raw, &a.servers); err != nil {
			return nil, fmt.Errorf("read server definitions: %w", err)
		}
		if len(a.servers) == 0 {
			return nil, errors.New("server definitions cannot be empty")
		}
		for _, s := range a.servers {
			if err := validateServer(s); err != nil {
				return nil, fmt.Errorf("invalid saved server: %w", err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read server definitions: %w", err)
	}
	a.auth, err = loadAuth(dir)
	if err != nil {
		return nil, err
	}
	if err := a.loadTelemetry(); err != nil {
		return nil, err
	}
	if err := a.loadEvents(); err != nil {
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
			list[i].Source = "demo"
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
	u, err := url.Parse(s.Address)
	if err != nil || u.Hostname() == "" || !slices.Contains([]string{"http", "https"}, u.Scheme) || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid HTTP(S) agent address")
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
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	if entry.ID == "" {
		entry.ID = hex.EncodeToString(bytes)
	}
	if entry.Time == "" {
		entry.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	a.logMu.Lock()
	defer a.logMu.Unlock()
	f, err := os.OpenFile(filepath.Join(a.dir, "logs.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}
func (a *app) logHistory(id, level, source, query, before string, offset, limit int) (logPage, error) {
	result := logPage{Items: []logEntry{}}
	a.logMu.RLock()
	defer a.logMu.RUnlock()
	f, err := os.Open(filepath.Join(a.dir, "logs.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	matches := []logEntry{}
	query = strings.ToLower(query)
	var boundary time.Time
	if before != "" {
		boundary, err = time.Parse(time.RFC3339Nano, before)
		if err != nil {
			return result, err
		}
	}
	for scanner.Scan() {
		var entry logEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return result, fmt.Errorf("decode log archive: %w", err)
		}
		if entry.DataSource == "" {
			entry.DataSource = "demo"
			if strings.HasPrefix(entry.ID, entry.ServerID+"-") {
				entry.DataSource = "agent"
			}
		}
		if !boundary.IsZero() {
			stamp, err := time.Parse(time.RFC3339Nano, entry.Time)
			if err != nil {
				return result, fmt.Errorf("parse log timestamp: %w", err)
			}
			if stamp.After(boundary) {
				continue
			}
		}
		if entry.ServerID != id || (level != "" && entry.Level != level) || (source != "" && entry.Source != source) || !strings.Contains(strings.ToLower(entry.Source+" "+entry.Message+" "+entry.Time), query) {
			continue
		}
		result.Total++
		matches = append(matches, entry)
		if len(matches) > offset+limit {
			matches = matches[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	slices.Reverse(matches)
	if offset < len(matches) {
		result.Items = matches[offset:min(len(matches), offset+limit)]
	}
	return result, nil
}
