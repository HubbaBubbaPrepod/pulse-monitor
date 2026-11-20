package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestReportBucketPreservesEveryObservation(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	for i, body := range []string{
		`{"cpu":10,"memory":80,"incoming":1,"outgoing":2,"read":3,"write":4}`,
		`{"cpu":90,"memory":20,"incoming":9,"outgoing":8,"read":7,"write":6}`,
	} {
		report := fmt.Sprintf(`{"bootId":"aggregate","sequence":%d,"os":"Linux","sample":%s}`, i+1, body)
		if w := reportRequest(a, "production-01", token, report); w.Code != 200 {
			t.Fatalf("report: %d %s", w.Code, w.Body.String())
		}
	}
	var raw []byte
	if err := a.storage.reader.QueryRow("SELECT body FROM metric_samples WHERE server_id='production-01'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var stored struct {
		CPU     float64 `json:"cpu"`
		Summary struct {
			Count int                `json:"count"`
			Min   map[string]float64 `json:"min"`
			Max   map[string]float64 `json:"max"`
			Avg   map[string]float64 `json:"avg"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.CPU != 90 || stored.Summary.Count != 2 {
		t.Fatalf("bucket lost reports: %s", raw)
	}
	for _, tc := range []struct {
		name          string
		min, max, avg float64
	}{
		{"cpu", 10, 90, 50}, {"memory", 20, 80, 50}, {"incoming", 1, 9, 5}, {"outgoing", 2, 8, 5}, {"read", 3, 7, 5}, {"write", 4, 6, 5},
	} {
		if stored.Summary.Min[tc.name] != tc.min || stored.Summary.Max[tc.name] != tc.max || stored.Summary.Avg[tc.name] != tc.avg {
			t.Fatalf("%s summary: %s", tc.name, raw)
		}
	}
	restarted, err := openTestApp(t, a.dir)
	if err != nil {
		t.Fatal(err)
	}
	server, _ := restarted.findServer("production-01")
	encoded, err := json.Marshal(restarted.snapshot(server, 60))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Samples []struct {
			Summary struct {
				Count int `json:"count"`
			} `json:"summary"`
		} `json:"samples"`
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, v := range result.Samples {
		count += v.Summary.Count
	}
	if count != 2 {
		t.Fatalf("restart lost or double-counted observations: %s", encoded)
	}
	third := `{"bootId":"aggregate","sequence":3,"os":"Linux","sample":{"cpu":50,"memory":60,"incoming":3,"outgoing":4,"read":5,"write":6}}`
	for range 2 {
		if w := reportRequest(restarted, "production-01", token, third); w.Code != 200 {
			t.Fatalf("report retry after restart: %d %s", w.Code, w.Body.String())
		}
	}
	encoded, err = json.Marshal(restarted.snapshot(server, 60))
	if err != nil || json.Unmarshal(encoded, &result) != nil {
		t.Fatalf("snapshot after retry: %v", err)
	}
	count = 0
	for _, v := range result.Samples {
		count += v.Summary.Count
	}
	if count != 3 {
		t.Fatalf("retry after restart lost or duplicated a report: %s", encoded)
	}
}

func TestReducedHistoryWeightsUnevenReportCounts(t *testing.T) {
	values := []sample{
		{At: "2026-11-20T10:00:00Z", CPU: 30, Summary: &sampleSummary{Count: 3, Start: "2026-11-20T10:00:00Z", End: "2026-11-20T10:00:09Z", Min: sampleValues{CPU: 10}, Max: sampleValues{CPU: 30}, Avg: sampleValues{CPU: 20}}},
		{At: "2026-11-20T10:00:10Z", CPU: 100},
	}
	points := reduceSamples(values, 1)
	if len(points) != 1 {
		t.Fatal("graph points were not bounded", len(points))
	}
	s := points[0]
	if s.CPU != 100 || s.Summary.Count != 4 || s.Summary.Avg.CPU != 40 || s.Summary.Min.CPU != 10 || s.Summary.Max.CPU != 100 {
		t.Fatalf("uneven observation counts were averaged as equal buckets: %+v %+v", s, s.Summary)
	}
	if s.Summary.Start != "2026-11-20T10:00:00Z" || s.Summary.End != "2026-11-20T10:00:10Z" {
		t.Fatalf("aggregate lost its covered interval: %+v", s.Summary)
	}
}

func TestAgentCannotSupplyHistoricalSummary(t *testing.T) {
	report := agentReport{BootID: "node", Sequence: 1, OS: "Linux", Sample: sample{CPU: 1, Summary: &sampleSummary{Count: 10, Max: sampleValues{CPU: 99}}}}
	if err := validateReport(report); err == nil {
		t.Fatal("agent-controlled aggregate accepted instead of computing it from observations")
	}
}

func TestSnapshotIncludesSummaryOverlappingPeriodBoundary(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	start, end := time.Now().Add(-65*time.Second).UTC().Format(time.RFC3339Nano), time.Now().Add(-55*time.Second).UTC().Format(time.RFC3339Nano)
	point := sample{At: start, CPU: 5, Summary: &sampleSummary{Count: 2, Start: start, End: end, Min: sampleValues{CPU: 5}, Max: sampleValues{CPU: 100}, Avg: sampleValues{CPU: 52.5}}}
	a.nodes["production-01"] = &agentNode{Report: agentReport{OS: "Linux", Sample: sample{At: end, CPU: 5}}, History: []sample{point}}
	server, _ := a.findServer("production-01")
	result := a.snapshot(server, 1)
	if len(result.Samples) == 0 || result.Samples[0].Summary == nil || result.Samples[0].Summary.Max.CPU != 100 {
		t.Fatalf("period boundary discarded an overlapping history bucket: %+v", result.Samples)
	}
}

func TestSnapshotAggregationPreservesPeaks(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	history := make([]sample, 121)
	start := time.Now().Add(-21 * time.Minute)
	for i := range history {
		history[i] = sample{At: start.Add(time.Duration(i) * 10 * time.Second).UTC().Format(time.RFC3339Nano), CPU: 1, Memory: 2}
	}
	history[1].CPU = 99
	a.nodes["production-01"] = &agentNode{Report: agentReport{OS: "Linux", Sample: history[len(history)-1]}, History: history}
	server, _ := a.findServer("production-01")
	snapshot := a.snapshot(server, 60)
	if len(snapshot.Samples) > 61 {
		t.Fatalf("unbounded chart: %d", len(snapshot.Samples))
	}
	encoded, err := json.Marshal(snapshot.Samples)
	if err != nil {
		t.Fatal(err)
	}
	var result []struct {
		Summary struct {
			Count int `json:"count"`
			Max   struct {
				CPU float64 `json:"cpu"`
			} `json:"max"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	peak, count := 0.0, 0
	for _, v := range result {
		count += v.Summary.Count
		if v.Summary.Max.CPU > peak {
			peak = v.Summary.Max.CPU
		}
	}
	if peak != 99 || count != 121 {
		t.Fatalf("chart lost peak or observations: peak=%v count=%v", peak, count)
	}
}
