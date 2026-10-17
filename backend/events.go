package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
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
	var err error
	a.events, err = a.storage.loadEvents(context.Background())
	return err
}
func (a *app) cacheEvents(events []monitorEvent) {
	for _, event := range events {
		if !slices.ContainsFunc(a.events, func(v monitorEvent) bool { return v.ID == event.ID }) {
			a.events = append(a.events, event)
		}
	}
	if len(a.events) > 500 {
		a.events = slices.Clone(a.events[len(a.events)-500:])
	}
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
	event := monitorEvent{ID: eventID, ServerID: id, Title: title, Text: text, Time: time.Now().UTC().Format(time.RFC3339Nano), Level: level}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.storage.saveEvents(ctx, []monitorEvent{event}); err != nil {
		return err
	}
	a.cacheEvents([]monitorEvent{event})
	return nil
}
func (a *app) buildAgentEvents(id string, old, n *agentNode) []monitorEvent {
	rows := []monitorEvent{}
	key := fmt.Sprintf("%s-%s-%d", id, n.Report.BootID, n.Report.Sequence)
	add := func(title, text, level, key string) {
		digest := sha256.Sum256([]byte(key))
		rows = append(rows, monitorEvent{ID: hex.EncodeToString(digest[:]), ServerID: id, Title: title, Text: text, Level: level, Time: n.Seen.UTC().Format(time.RFC3339Nano)})
	}
	if old.Seen.IsZero() || time.Since(old.Seen) >= 15*time.Second {
		add("Агент подключён", "Начат приём данных от узла.", "INFO", key+"-online")
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
			add(title, fmt.Sprintf("%s: %.1f%%, порог %d%%.", metric.name, metric.after, s.Threshold), level, key+metric.name)
		}
	}
	return rows
}
func (a *app) evaluateAgentEvents(id string, old, n *agentNode) error {
	events := a.buildAgentEvents(id, old, n)
	a.eventMu.Lock()
	defer a.eventMu.Unlock()
	if err := a.storage.saveEvents(context.Background(), events); err != nil {
		return err
	}
	a.cacheEvents(events)
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
		found, err := a.storage.ackEvents(r.Context(), r.PathValue("id"))
		if err != nil {
			apiError(w, 500, "Could not save acknowledgment")
			return
		}
		if !found && r.PathValue("id") != "all" {
			apiError(w, 404, "Event not found")
			return
		}
		for i := range a.events {
			if r.PathValue("id") == "all" || a.events[i].ID == r.PathValue("id") {
				a.events[i].Acknowledged = true
			}
		}
		writeJSON(w, 200, map[string]bool{"acknowledged": true})
	})
}
