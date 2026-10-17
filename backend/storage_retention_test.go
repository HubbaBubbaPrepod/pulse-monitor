package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestSQLiteRetentionIsScopedAndUsesReceiptTime(t *testing.T) {
	db, err := openSQLite(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	for _, id := range []string{"one", "two"} {
		if err := db.setRetention(ctx, id, 7); err != nil {
			t.Fatal(err)
		}
		for _, age := range []int{1, 8} {
			entry := logEntry{ID: id + time.Duration(age).String(), ServerID: id, Time: now.Add(-365 * 24 * time.Hour).Format(time.RFC3339Nano), Level: "INFO", Message: "source clock irrelevant"}
			if err := db.appendLogs(ctx, []logEntry{entry}, now.Add(-time.Duration(age)*24*time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
	}
	count, err := db.cleanupLogs(ctx, "one", now, 1000)
	if err != nil || count != 1 {
		t.Fatalf("cleanup: %d %v", count, err)
	}
	for id, total := range map[string]int{"one": 1, "two": 2} {
		page, err := db.queryLogs(ctx, id, "", "", "", "", 0, 100, "")
		if err != nil || page.Total != total {
			t.Fatalf("scope %s: %+v %v", id, page, err)
		}
	}
	if err := db.setRetention(ctx, "two", 0); err != nil {
		t.Fatal(err)
	}
	count, err = db.cleanupLogs(ctx, "two", now, 1000)
	if err != nil || count != 0 {
		t.Fatal("disabled retention removed rows", count, err)
	}
	for _, days := range []int{-1, 3651} {
		if err := db.setRetention(ctx, "one", days); err == nil {
			t.Fatal("invalid days accepted", days)
		}
	}
}

func TestSQLiteRetentionAPIKeepsExistingNodesAndDefaultsNewNodes(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	old, _ := a.findServer("production-01")
	if old.LogRetentionDays == nil || *old.LogRetentionDays != 0 {
		t.Fatal("migration enabled deletion")
	}
	body := `{"id":"new","name":"new","address":"https://new.example","group":"Test","interval":5,"threshold":85}`
	if w := request(t, a, "PUT", "/api/servers/new", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	added, _ := a.findServer("new")
	if added.LogRetentionDays == nil || *added.LogRetentionDays != 30 {
		t.Fatal("new node missing default")
	}
	if w := request(t, a, "PUT", "/api/servers/new/log-retention", `{"days":7}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, invalid := range []string{`{}`, `{"days":null}`, `{"days":-1}`, `{"days":3651}`, `{"days":1.5}`} {
		if w := request(t, a, "PUT", "/api/servers/new/log-retention", invalid); w.Code != 400 {
			t.Fatal("invalid policy accepted", invalid, w.Code)
		}
	}
	restarted, err := openTestApp(t, a.dir)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := restarted.findServer("new")
	if saved.LogRetentionDays == nil || *saved.LogRetentionDays != 7 {
		raw, _ := json.Marshal(saved)
		t.Fatal("policy lost", string(raw))
	}
}
