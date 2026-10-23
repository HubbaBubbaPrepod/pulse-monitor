//go:build linux

package main

import (
	"os"
	"testing"
)

func TestLowDiskReportDoesNotExhaustTheMonitorVolume(t *testing.T) {
	if os.Getenv("PULSE_LOW_DISK_TEST") != "1" {
		t.Skip("requires a bounded tmpfs fixture")
	}
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	w := reportRequest(a, "production-01", token, `{"bootId":"lowdisk","sequence":1,"os":"Linux","sample":{"cpu":1}}`)
	if w.Code != 503 {
		t.Fatalf("low disk report still accepted: %d %s", w.Code, w.Body.String())
	}
	if a.nodes["production-01"].Report.Sequence != 0 {
		t.Fatal("failed report advanced receipt")
	}
}
