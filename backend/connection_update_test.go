package main

import (
	"encoding/json"
	"testing"
)

func TestPushServerAddressIsOptionalMetadata(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.serverList()[0]
	for _, address := range []string{"", "195.2.75.235", "example.org", "192.168.1.2:9443", "https://example.org"} {
		s.Address = address
		if err := validateServer(s); err != nil {
			t.Errorf("metadata address %q: %v", address, err)
		}
	}
}

func TestChangingAddressMetadataKeepsAgentIdentity(t *testing.T) {
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.serverList()[0]
	if _, err := a.issueAgentToken(s.ID); err != nil {
		t.Fatal(err)
	}
	before := a.nodes[s.ID].Hash
	s.Address = ""
	raw, _ := json.Marshal(s)
	if w := request(t, a, "PUT", "/api/servers/"+s.ID, string(raw)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if a.nodes[s.ID].Hash != before {
		t.Fatal("editing metadata revoked the agent")
	}
}
