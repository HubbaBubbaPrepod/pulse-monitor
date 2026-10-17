package main

import (
	"encoding/json"
	"testing"
)

func TestLogCategoriesFilterBeforePagination(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []logEntry{
		{ID: "category-system", ServerID: "production-01", Time: "2026-10-12T10:00:00Z", Level: "INFO", Source: "nginx.service", Message: "Started web server"},
		{ID: "category-security", ServerID: "production-01", Time: "2026-10-12T10:00:01Z", Level: "INFO", Source: "ssh.service", Message: "Failed password for root from 192.0.2.1 port 1234 ssh2"},
		{ID: "category-noise", ServerID: "production-01", Time: "2026-10-12T10:00:02Z", Level: "INFO", Source: "Netwtw08", Message: "7021 - Connection telemetry fields and analysis usage"},
		{ID: "category-wifi-error", ServerID: "production-01", Time: "2026-10-12T10:00:03Z", Level: "ERROR", Source: "Netwtw08", Message: "Adapter failed"},
	} {
		if err := a.appendLog(row); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, category, want string
		total                int
	}{
		{"system", "system", "category-wifi-error", 2},
		{"security", "security", "category-security", 1},
		{"telemetry", "telemetry", "category-noise", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := request(t, a, "GET", "/api/servers/production-01/logs?category="+tc.category+"&limit=1", "")
			if response.Code != 200 {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			var page logPage
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			if page.Total != tc.total || len(page.Items) != 1 || page.Items[0].ID != tc.want {
				t.Fatalf("filtered page: %+v", page)
			}
		})
	}
	if response := request(t, a, "GET", "/api/servers/production-01/logs?category=unknown", ""); response.Code != 400 {
		t.Fatalf("invalid category status: %d", response.Code)
	}
}
