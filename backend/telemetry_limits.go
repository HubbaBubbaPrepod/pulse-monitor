package main

import "time"

type reportBudget struct {
	Tokens float64
	At     time.Time
}

func (a *app) allowReport(id string, now time.Time) bool {
	a.telemetryMu.Lock()
	defer a.telemetryMu.Unlock()
	if a.reportBudgets == nil {
		a.reportBudgets = make(map[string]reportBudget)
	}
	b, exists := a.reportBudgets[id]
	if !exists {
		b = reportBudget{Tokens: 16, At: now}
	}
	b.Tokens = min(16, b.Tokens+max(0, now.Sub(b.At).Seconds())*2)
	b.At = now
	allowed := b.Tokens >= 1
	if allowed {
		b.Tokens--
	}
	a.reportBudgets[id] = b
	return allowed
}
