package modulewasm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBrokerDecoderRejectsAmbiguityAndUnknownMembers(t *testing.T) {
	for _, raw := range []string{
		`null`, `[]`, `{"Method":"docker.snapshot"}`, `{"method":"one","method":"two"}`,
		`{"method":"one","params":{"id":"one","id":"two"}}`,
		`{"method":"one","params":null}`, `{"method":"one","params":[]}`,
		`{"method":"one","token":"private"}`, `{"method":"one"}{}`,
		`{"method":"one","params":{"nested":` + strings.Repeat(`[`, 9) + `0` + strings.Repeat(`]`, 9) + `}}`,
	} {
		if _, err := DecodeRequest([]byte(raw)); err == nil {
			t.Fatal("ambiguous broker request accepted")
		}
	}
	if _, err := DecodeRequest([]byte(`{"method":"telemetry.snapshot","params":{}}`)); err != nil {
		t.Fatal(err)
	}
}
func TestBrokerReplyBoundsErrorAndData(t *testing.T) {
	raw := Reply("ok", strings.Repeat("x", MaxOutputBytes+1), "")
	var result struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Status != "unknown" {
		t.Fatal("oversized broker response was not bounded")
	}
	raw = Reply("error", nil, strings.Repeat("s", 257))
	if strings.Contains(string(raw), strings.Repeat("s", 257)) {
		t.Fatal("unbounded error leaked")
	}
}
