package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type brokerRoundTrip func(*http.Request) (*http.Response, error)

func (f brokerRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func monitorBrokerCall(t *testing.T, b *monitorModuleBroker, request string) map[string]any {
	t.Helper()
	raw, err := b.Call(context.Background(), json.RawMessage(request))
	if err != nil {
		t.Fatal(err)
	}
	var reply map[string]any
	if json.Unmarshal(raw, &reply) != nil {
		t.Fatal("invalid broker reply")
	}
	return reply
}
func TestBrokerTelemetryIsScopedAndRequiresGrant(t *testing.T) {
	calls := 0
	b := &monitorModuleBroker{invocation: moduleInvocation{ServerID: "one", Grants: []string{"telemetry.read"}}, snapshot: func(_ context.Context, id string) (json.RawMessage, error) {
		calls++
		if id != "one" {
			t.Fatal("scope escaped")
		}
		return json.RawMessage(`{"cpu":12}`), nil
	}}
	if monitorBrokerCall(t, b, `{"method":"telemetry.snapshot","params":{"serverId":"other"}}`)["ok"] != false || calls != 0 {
		t.Fatal("guest selected another server")
	}
	if monitorBrokerCall(t, b, `{"method":"telemetry.snapshot","params":{}}`)["ok"] != true || calls != 1 {
		t.Fatal("scoped snapshot unavailable")
	}
	b.invocation.Grants = nil
	if monitorBrokerCall(t, b, `{"method":"telemetry.snapshot","params":{}}`)["ok"] != false {
		t.Fatal("missing telemetry grant accepted")
	}
}
func TestBrokerTelegramKeepsCredentialsAndTargetOnHost(t *testing.T) {
	const token = "123456:abcdefghijklmnopqrstuvwxyz123456"
	calls := 0
	client := &http.Client{Transport: brokerRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Scheme != "https" || r.URL.Host != "api.telegram.org" || r.URL.Path != "/bot"+token+"/sendMessage" {
			t.Fatal("Telegram endpoint escaped")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["chat_id"] != "-12345" || body["text"] != "Alert" {
			t.Fatal("owner target was overridden")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":7}}`)), Header: make(http.Header)}, nil
	})}
	b := &monitorModuleBroker{invocation: moduleInvocation{Config: map[string]any{"telegramToken": token, "telegramChatId": "-12345"}, Grants: []string{"notifications.telegram"}}, client: client}
	for _, request := range []string{
		`{"method":"notification.telegram","params":{"message":"Alert","url":"https://outside.invalid"}}`,
		`{"method":"notification.telegram","params":{"message":"Alert","token":"forged"}}`,
		`{"method":"notification.telegram","params":{"message":"Alert","chatId":"other"}}`,
		`{"method":"notification.telegram","params":{"message":"` + strings.Repeat("a", 4097) + `"}}`,
	} {
		if monitorBrokerCall(t, b, request)["ok"] != false {
			t.Fatal("untrusted Telegram override accepted")
		}
	}
	if calls != 0 {
		t.Fatal("denied request was sent")
	}
	b.invocation.Grants = nil
	if monitorBrokerCall(t, b, `{"method":"notification.telegram","params":{"message":"Alert"}}`)["ok"] != false || calls != 0 {
		t.Fatal("missing Telegram grant accepted")
	}
	b.invocation.Grants = []string{"notifications.telegram"}
	reply := monitorBrokerCall(t, b, `{"method":"notification.telegram","params":{"message":"Alert"}}`)
	raw, _ := json.Marshal(reply)
	if reply["ok"] != true || calls != 1 || strings.Contains(string(raw), token) {
		t.Fatal("Telegram result leaked authority or failed")
	}
	if monitorBrokerCall(t, b, `{"method":"notification.telegram","params":{"message":"Alert"}}`)["ok"] != false || calls != 1 {
		t.Fatal("message was repeated in one invocation")
	}
}

func TestBrokerGuestCannotHideAmbiguousNotification(t *testing.T) {
	b := &monitorModuleBroker{sent: true, deliveryStatus: "unknown"}
	result, err := b.Finalize(moduleInvocationResult{Protocol: 2, Status: "ok", Data: json.RawMessage(`{}`)}, nil)
	if err != nil || result.Status != "unknown" {
		t.Fatal("guest hid an uncertain host effect")
	}
}

func TestBrokerTelegramAmbiguousResponseIsUnknownAndNotReplayed(t *testing.T) {
	calls := 0
	b := &monitorModuleBroker{invocation: moduleInvocation{Config: map[string]any{"telegramToken": "123456:abcdefghijklmnopqrstuvwxyz123456", "telegramChatId": "-12345"}, Grants: []string{"notifications.telegram"}}, client: &http.Client{Transport: brokerRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{`)), Header: make(http.Header)}, nil
	})}}
	if monitorBrokerCall(t, b, `{"method":"notification.telegram","params":{"message":"Alert"}}`)["status"] != "unknown" {
		t.Fatal("ambiguous delivery was reported as definite")
	}
	if monitorBrokerCall(t, b, `{"method":"notification.telegram","params":{"message":"Alert"}}`)["ok"] != false || calls != 1 {
		t.Fatal("ambiguous delivery was repeated")
	}
}
