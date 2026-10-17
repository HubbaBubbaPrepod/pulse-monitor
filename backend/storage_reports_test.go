package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSQLiteReportReceiptRollsBackAllEffects(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.storage.writer.ExecContext(context.Background(), "DROP TABLE monitor_events"); err != nil {
		t.Fatal(err)
	}
	body := `{"bootId":"atomic","sequence":1,"os":"Linux","sample":{"cpu":90},"logs":[{"level":"INFO","source":"systemd","message":"must roll back"}]}`
	w := reportRequest(a, "production-01", token, body)
	if w.Code != 500 {
		t.Fatalf("failed persistence accepted: %d", w.Code)
	}
	var count int
	if err := a.storage.reader.QueryRow("SELECT count(*) FROM collected_logs").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("partial logs committed")
	}
	var raw []byte
	if err := a.storage.reader.QueryRow("SELECT body FROM agent_nodes WHERE id='production-01'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var node agentNode
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	if node.Report.Sequence != 0 {
		t.Fatal("failed report checkpoint advanced")
	}
}

type gatedReportBody struct {
	reader           io.Reader
	started, release chan struct{}
	once             sync.Once
}

func (g *gatedReportBody) Read(p []byte) (int, error) {
	g.once.Do(func() { close(g.started); <-g.release })
	return g.reader.Read(p)
}
func TestSQLiteSlowUploadDoesNotBlockAnotherNode(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.issueAgentToken("windows-02")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"bootId":"slow","sequence":1,"os":"Linux","sample":{"cpu":1}}`
	gate := &gatedReportBody{reader: strings.NewReader(body), started: make(chan struct{}), release: make(chan struct{})}
	request := httptest.NewRequest("POST", "/api/agent/production-01/report", gate)
	request.Host = "127.0.0.1:8080"
	request.Header.Set("Authorization", "Bearer "+first)
	request.Header.Set("Content-Type", "application/json")
	done := make(chan struct{})
	go func() { defer close(done); a.handler().ServeHTTP(httptest.NewRecorder(), request) }()
	select {
	case <-gate.started:
	case <-done:
		t.Fatal("request ended before body read")
	case <-time.After(2 * time.Second):
		close(gate.release)
		<-done
		t.Fatal("body read did not start")
	}
	result := make(chan int, 1)
	go func() { result <- reportRequest(a, "windows-02", second, body).Code }()
	select {
	case code := <-result:
		if code != 200 {
			t.Errorf("other node: %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Error("slow upload held ingestion lock")
	}
	close(gate.release)
	<-done
}

func TestSQLiteDiskFullNeverAcknowledgesPartialReport(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	var pages int
	if err := a.storage.writer.QueryRow("PRAGMA page_count").Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if _, err := a.storage.writer.Exec(fmt.Sprintf("PRAGMA max_page_count=%d", pages)); err != nil {
		t.Fatal(err)
	}
	logs := make([]logEntry, 100)
	for i := range logs {
		logs[i] = logEntry{Level: "INFO", Source: "systemd", Message: strings.Repeat("x", 8000)}
	}
	body, _ := json.Marshal(agentReport{BootID: "full", Sequence: 1, OS: "Linux", Logs: logs})
	if w := reportRequest(a, "production-01", token, string(body)); w.Code != 500 {
		t.Fatalf("disk full accepted: %d %s", w.Code, w.Body.String())
	}
	var count int
	if err := a.storage.reader.QueryRow("SELECT count(*) FROM collected_logs").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial logs: %d %v", count, err)
	}
	if a.nodes["production-01"].Report.Sequence != 0 {
		t.Fatal("memory checkpoint advanced on disk full")
	}
}
