package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanRegistryAndLastNodeDeletion(t *testing.T) {
	dir := t.TempDir()
	a, err := newApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.serverList()) != 0 {
		t.Fatal("clean installation must not seed servers")
	}
	if a.dataSource() != "waiting" {
		t.Fatal("empty registry must wait for agents")
	}
	body := `{"id":"node","name":"node","address":"https://node.example","os":"","group":"Test","online":false,"cpu":0,"memory":0,"disk":0,"interval":5,"threshold":85}`
	if w := request(t, a, "PUT", "/api/servers/node", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, a, "DELETE", "/api/servers/node", ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	a, err = newApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.serverList()) != 0 {
		t.Fatal("empty registry must survive restart")
	}
}

func TestMigrationPreservesEnrolledLegacyNodeAndOriginalLogs(t *testing.T) {
	dir := t.TempDir()
	nodes := []server{{ID: "real", Name: "Real", Source: "demo", Address: "https://real.example", Interval: 5, Threshold: 85}}
	if err := saveJSON(filepath.Join(dir, "servers.json"), nodes); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(filepath.Join(dir, "agents", "real.json"), agentNode{Hash: strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
	original := "{\"dataSource\":\"demo\",\"message\":\"synthetic\"}\n{\"dataSource\":\"agent\",\"message\":\"actual\"}\nmalformed retained"
	if err := os.WriteFile(filepath.Join(dir, "logs.jsonl"), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	a := &app{dir: dir, servers: nodes}
	if err := a.migrateDemo(); err != nil {
		t.Fatal(err)
	}
	if len(a.servers) != 1 || a.servers[0].Source != "waiting" {
		t.Fatal(a.servers)
	}
	backup, err := os.ReadFile(filepath.Join(dir, "logs.pre-real.jsonl"))
	if err != nil || string(backup) != original {
		t.Fatal("original backup changed", err)
	}
	kept, err := os.ReadFile(filepath.Join(dir, "logs.jsonl"))
	if err != nil || string(kept) != "{\"dataSource\":\"agent\",\"message\":\"actual\"}\nmalformed retained" {
		t.Fatal("real records changed", err, string(kept))
	}
	if err := a.migrateDemo(); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(filepath.Join(dir, "logs.jsonl"))
	if string(again) != string(kept) {
		t.Fatal("restart rewrote logs")
	}
}

func TestDemoMigrationPreservesRealRecords(t *testing.T) {
	dir := t.TempDir()
	nodes := []server{{ID: "demo-node", Name: "Demo", Source: "demo", Address: "https://demo.example", Interval: 5, Threshold: 85}, {ID: "real", Name: "Real", Source: "waiting", Address: "https://real.example", Interval: 5, Threshold: 85}}
	raw, _ := json.Marshal(nodes)
	if err := os.WriteFile(filepath.Join(dir, "servers.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := newApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.serverList(); len(got) != 1 || got[0].ID != "real" {
		t.Fatalf("migration retained wrong nodes: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "servers.pre-real.json")); err != nil {
		t.Fatal("migration must retain backup", err)
	}
}
