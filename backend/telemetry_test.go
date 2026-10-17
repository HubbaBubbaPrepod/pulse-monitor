package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func reportRequest(a *app, id, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/agent/"+id+"/report", strings.NewReader(body))
	r.Host = "127.0.0.1:8080"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	return w
}

func TestAgentLifecycleAndRestart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	cookie, csrf := testSession(t, a)
	w := rawAuthRequest(a, "POST", "/api/servers/production-01/token", "{}", cookie, csrf)
	if w.Code != 201 {
		t.Fatalf("issue: %d %s", w.Code, w.Body.String())
	}
	var issued struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	body := `{"bootId":"test-boot","sequence":1,"os":"Windows 11","hardware":{"cpuCores":8,"memoryGB":32},"sample":{"cpu":91,"memory":40},"logs":[{"level":"INFO","source":"agent","message":"real log"}]}`
	if w := reportRequest(a, "windows-02", issued.Token, body); w.Code != 401 {
		t.Fatalf("cross node token: %d", w.Code)
	}
	for range 2 {
		if w := reportRequest(a, "production-01", issued.Token, body); w.Code != 200 {
			t.Fatalf("report: %d %s", w.Code, w.Body.String())
		}
	}
	page, err := a.logHistory("production-01", "", "", "real log", "", 0, 100)
	if err != nil || page.Total != 1 {
		t.Fatalf("dedup: %+v %v", page, err)
	}
	s, _ := a.findServer("production-01")
	if !s.Online || s.OS != "Windows 11" || s.CPU != 91 {
		t.Fatalf("auto metadata: %+v", s)
	}
	restarted, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	s, _ = restarted.findServer("production-01")
	snap := restarted.snapshot(s, 60)
	if snap.Source != "agent" || len(snap.Samples) != 1 || snap.Samples[0].CPU != 91 {
		t.Fatalf("history not restored: %+v", snap)
	}
	if s.Online {
		t.Fatal("restart must wait for fresh heartbeat")
	}
	if w := reportRequest(restarted, "production-01", issued.Token, strings.Replace(body, `"sequence":1`, `"sequence":2`, 1)); w.Code != 200 {
		t.Fatalf("persist token: %d", w.Code)
	}
	cookie, csrf = testSession(t, restarted)
	if w := rawAuthRequest(restarted, "DELETE", "/api/servers/production-01/token", "", cookie, csrf); w.Code != 200 {
		t.Fatalf("revoke: %d", w.Code)
	}
	if w := reportRequest(restarted, "production-01", issued.Token, body); w.Code != 401 {
		t.Fatalf("revoked: %d", w.Code)
	}
}

func TestAgentInvalidReportDoesNotUpdateState(t *testing.T) {
	t.Parallel()
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"bootId":"x","sequence":1,"os":"Linux","sample":{"cpu":101}}`, `{"bootId":"x","sequence":1,"os":"Linux","sample":{"cpu":20},"gpus":[{"utilization":-1}]}`} {
		if w := reportRequest(a, "production-01", token, body); w.Code != 400 {
			t.Fatalf("invalid report: %d %s", w.Code, w.Body.String())
		}
	}
	a.telemetryMu.Lock()
	node := a.nodes["production-01"]
	node.Seen = time.Now().Add(-time.Minute)
	a.telemetryMu.Unlock()
	s, _ := a.findServer("production-01")
	if s.Online {
		t.Fatal("expired heartbeat")
	}
}

func TestAgentLogDedupBeyondHistoryPage(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 205; i++ {
		if err := a.appendLog(logEntry{ID: fmt.Sprintf("node-boot-1-%d", i), ServerID: "production-01", Message: "same message"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.appendAgentLog(logEntry{ID: "node-boot-1-0", ServerID: "production-01", Message: "same message"}); err != nil {
		t.Fatal(err)
	}
	page, err := a.logHistory("production-01", "", "", "", "", 0, 300)
	if err != nil || page.Total != 205 {
		t.Fatalf("retry duplicated archived log: total=%d err=%v", page.Total, err)
	}
}

func BenchmarkReceiveReportWithLogArchive(b *testing.B) {
	dir := b.TempDir()
	f, err := os.Create(filepath.Join(dir, "logs.jsonl"))
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 10000; i++ {
		line, _ := json.Marshal(logEntry{ID: fmt.Sprintf("historic-%d", i), ServerID: "production-01", DataSource: "agent", Time: "2026-10-01T00:00:00Z", Message: strings.Repeat("x", 3000)})
		if _, err = f.Write(append(line, '\n')); err != nil {
			b.Fatal(err)
		}
	}
	if err = f.Close(); err != nil {
		b.Fatal(err)
	}
	a, err := newTestApp(b, dir)
	if err != nil {
		b.Fatal(err)
	}
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		b.Fatal(err)
	}
	logs := make([]logEntry, 20)
	for i := range logs {
		logs[i] = logEntry{Level: "INFO", Source: "system", Message: fmt.Sprintf("message %d", i)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body, _ := json.Marshal(agentReport{BootID: "benchmark", Sequence: uint64(i + 1), OS: "Linux", Logs: logs})
		if w := reportRequest(a, "production-01", token, string(body)); w.Code != 200 {
			b.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestAgentLogBatchRecoversPartialRetry(t *testing.T) {
	dir := t.TempDir()
	a, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	first := logEntry{ID: "node-boot-1-0", ServerID: "production-01", Message: "first"}
	second := logEntry{ID: "node-boot-1-1", ServerID: "production-01", Message: "second"}
	if err := a.appendLog(first); err != nil {
		t.Fatal(err)
	}
	restarted, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := restarted.appendAgentLogs([]logEntry{first, second, second}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := restarted.logHistory("production-01", "", "", "", "", 0, 100)
	if err != nil || page.Total != 2 {
		t.Fatalf("partial retry: total=%d err=%v", page.Total, err)
	}
}
