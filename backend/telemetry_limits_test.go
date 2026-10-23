package main

import (
	"fmt"
	"testing"
)

func TestAgentFloodIsLimitedPerEnrolledNode(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	one, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.issueAgentToken("windows-02")
	if err != nil {
		t.Fatal(err)
	}
	limited := false
	for i := 1; i <= 40; i++ {
		body := fmt.Sprintf(`{"bootId":"limited","sequence":%d,"os":"Linux","sample":{"cpu":1}}`, i)
		w := reportRequest(a, "production-01", one, body)
		if w.Code == 429 {
			if w.Header().Get("Retry-After") == "" {
				t.Fatal("missing retry guidance")
			}
			limited = true
			break
		}
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if !limited {
		t.Fatal("authenticated report flood never throttled")
	}
	w := reportRequest(a, "windows-02", two, `{"bootId":"other","sequence":1,"os":"Windows","sample":{"cpu":1}}`)
	if w.Code != 200 {
		t.Fatal("one node exhausted another node budget", w.Code)
	}
}
