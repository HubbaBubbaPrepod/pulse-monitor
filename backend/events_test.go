package main

import (
	"encoding/json"
	"testing"
)

func TestEventsAndAcknowledgmentsSurviveRestart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	token, err := a.issueAgentToken("production-01")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"bootId":"event-boot","sequence":1,"os":"Linux","sample":{"cpu":95,"memory":90}}`
	if w := reportRequest(a, "production-01", token, body); w.Code != 200 {
		t.Fatalf("report: %s", w.Body.String())
	}
	cookie, csrf := testSession(t, a)
	w := rawAuthRequest(a, "GET", "/api/events", "", cookie, "")
	var events []monitorEvent
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected connection and two threshold events, got %d", len(events))
	}
	if w := rawAuthRequest(a, "POST", "/api/events/all/ack", "{}", cookie, csrf); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	restarted, err := newTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	cookie, _ = testSession(t, restarted)
	w = rawAuthRequest(restarted, "GET", "/api/events", "", cookie, "")
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if !e.Acknowledged {
			t.Fatal("acknowledgment lost")
		}
	}
}
