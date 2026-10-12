package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type monitorEvent struct {
	ID           string `json:"id"`
	ServerID     string `json:"serverId"`
	Title        string `json:"title"`
	Text         string `json:"text"`
	Time         string `json:"time"`
	Level        string `json:"level"`
	Acknowledged bool   `json:"acknowledged"`
}

func (a *app) loadEvents() error {
	a.events = []monitorEvent{}
	raw, err := os.ReadFile(filepath.Join(a.dir, "events.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, &a.events)
}
func (a *app) addEvent(id, title, text, level string, keys ...string) error {
	a.eventMu.Lock()
	defer a.eventMu.Unlock()
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return err
	}
	eventID := hex.EncodeToString(raw[:])
	if len(keys) > 0 {
		digest := sha256.Sum256([]byte(keys[0]))
		eventID = hex.EncodeToString(digest[:])
		if slices.ContainsFunc(a.events, func(row monitorEvent) bool { return row.ID == eventID }) {
			return nil
		}
	}
	next := append(slices.Clone(a.events), monitorEvent{ID: eventID, ServerID: id, Title: title, Text: text, Time: time.Now().UTC().Format(time.RFC3339Nano), Level: level})
	if err := saveJSON(filepath.Join(a.dir, "events.json"), next); err != nil {
		return err
	}
	a.events = next
	return nil
}
func (a *app) evaluateAgentEvents(id string, old, n *agentNode) error {
	key := fmt.Sprintf("%s-%s-%d", id, n.Report.BootID, n.Report.Sequence)
	if old.Seen.IsZero() || time.Since(old.Seen) >= 15*time.Second {
		if err := a.addEvent(id, "Агент подключён", "Начат приём данных от узла.", "INFO", key+"-online"); err != nil {
			return err
		}
	}
	s, _ := a.findServer(id)
	for _, metric := range []struct {
		name          string
		before, after float64
	}{{"CPU", old.Report.Sample.CPU, n.Report.Sample.CPU}, {"Память", old.Report.Sample.Memory, n.Report.Sample.Memory}, {"Диск", diskLoad(old.Report.Disks), diskLoad(n.Report.Disks)}, {"GPU", gpuLoad(old.Report.GPUs), gpuLoad(n.Report.GPUs)}} {
		before := metric.before >= float64(s.Threshold)
		after := metric.after >= float64(s.Threshold)
		if before != after {
			title, level := "Нагрузка вернулась к норме", "INFO"
			if after {
				title, level = "Превышен порог нагрузки", "WARN"
			}
			if err := a.addEvent(id, title, fmt.Sprintf("%s: %.1f%%, порог %d%%.", metric.name, metric.after, s.Threshold), level, key+metric.name); err != nil {
				return err
			}
		}
	}
	return nil
}
func diskLoad(rows []disk) float64 {
	value := 0.0
	for _, d := range rows {
		if d.TotalGB > 0 {
			value = max(value, d.UsedGB/d.TotalGB*100)
		}
	}
	return value
}
func gpuLoad(rows []gpu) float64 {
	value := 0.0
	for _, g := range rows {
		if supported, ok := g.Supported["utilization"]; !ok || supported {
			value = max(value, g.Utilization)
		}
	}
	return value
}
func (a *app) checkAgentOffline(s server) error {
	a.ingestMu.Lock()
	defer a.ingestMu.Unlock()
	a.telemetryMu.RLock()
	old := a.nodes[s.ID]
	a.telemetryMu.RUnlock()
	if old == nil || old.Seen.IsZero() || time.Since(old.Seen) < 15*time.Second {
		return nil
	}
	if err := a.addEvent(s.ID, "Связь с агентом потеряна", "Нет отчётов более 15 секунд. Последние данные сохранены.", "ERROR"); err != nil {
		return err
	}
	n := *old
	n.Seen = time.Time{}
	a.telemetryMu.Lock()
	a.nodes[s.ID] = &n
	a.telemetryMu.Unlock()
	return nil
}
func (a *app) eventRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		a.eventMu.Lock()
		rows := slices.Clone(a.events)
		a.eventMu.Unlock()
		slices.Reverse(rows)
		if len(rows) > 500 {
			rows = rows[:500]
		}
		writeJSON(w, 200, rows)
	})
	mux.HandleFunc("POST /api/events/{id}/ack", func(w http.ResponseWriter, r *http.Request) {
		a.eventMu.Lock()
		defer a.eventMu.Unlock()
		next := slices.Clone(a.events)
		found := false
		for i := range next {
			if r.PathValue("id") == "all" || next[i].ID == r.PathValue("id") {
				next[i].Acknowledged = true
				found = true
			}
		}
		if !found && r.PathValue("id") != "all" {
			apiError(w, 404, "Event not found")
			return
		}
		if err := saveJSON(filepath.Join(a.dir, "events.json"), next); err != nil {
			apiError(w, 500, "Could not save acknowledgment")
			return
		}
		a.events = next
		writeJSON(w, 200, map[string]bool{"acknowledged": true})
	})
}
