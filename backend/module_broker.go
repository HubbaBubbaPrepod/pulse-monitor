package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"regexp"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/HubbaBubbaPrepod/pulse-monitor/backend/internal/modulewasm"
)

type monitorModuleBroker struct {
	invocation         moduleInvocation
	snapshot           func(context.Context, string) (json.RawMessage, error)
	client             *http.Client
	mu                 sync.Mutex
	sent, snapshotRead bool
	deliveryStatus     string
	deliveryProof      map[string]any
}

var telegramToken = regexp.MustCompile(`^[0-9]{5,16}:[a-zA-Z0-9_-]{20,128}$`)
var telegramChat = regexp.MustCompile(`^(?:-?[0-9]{1,20}|@[a-zA-Z0-9_]{5,32})$`)

func (b *monitorModuleBroker) Call(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	request, err := modulewasm.DecodeRequest(raw)
	if err != nil || ctx.Err() != nil {
		return modulewasm.Reply("error", nil, "Invalid or cancelled host request"), nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if ctx.Err() != nil {
		return modulewasm.Reply("error", nil, "Host request cancelled"), nil
	}
	denied := func() (json.RawMessage, error) {
		return modulewasm.Reply("error", nil, "Host operation is not granted or its scope is invalid"), nil
	}
	switch request.Method {
	case "telemetry.snapshot":
		if !slices.Contains(b.invocation.Grants, "telemetry.read") || len(request.Params) != 0 || b.invocation.ServerID == "" || b.snapshot == nil || b.snapshotRead {
			return denied()
		}
		b.snapshotRead = true
		data, err := b.snapshot(ctx, b.invocation.ServerID)
		if err != nil || !json.Valid(data) {
			return modulewasm.Reply("error", nil, "Selected node telemetry unavailable"), nil
		}
		return modulewasm.Reply("ok", data, ""), nil
	case "notification.telegram":
		if !slices.Contains(b.invocation.Grants, "notifications.telegram") || len(request.Params) != 1 || b.sent {
			return denied()
		}
		var message string
		if json.Unmarshal(request.Params["message"], &message) != nil || message == "" || len(message) > 16*1024 || utf8.RuneCountInString(message) > 4096 {
			return denied()
		}
		token, _ := b.invocation.Config["telegramToken"].(string)
		chat, _ := b.invocation.Config["telegramChatId"].(string)
		if !telegramToken.MatchString(token) || !telegramChat.MatchString(chat) {
			return denied()
		}
		if b.client == nil {
			transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, ResponseHeaderTimeout: 5 * time.Second, TLSHandshakeTimeout: 5 * time.Second}
			defer transport.CloseIdleConnections()
			b.client = &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		}
		b.sent = true
		b.deliveryStatus = "unknown"
		raw, _ := json.Marshal(map[string]any{"chat_id": chat, "text": message})
		child, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(child, http.MethodPost, "https://api.telegram.org/bot"+token+"/sendMessage", bytes.NewReader(raw))
		if err != nil {
			return denied()
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := b.client.Do(request)
		if err != nil {
			return modulewasm.Reply("unknown", nil, "Delivery outcome unavailable; message was not replayed"), nil
		}
		defer response.Body.Close()
		raw, err = io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
		if err != nil || len(raw) > 64*1024 || response.StatusCode >= 500 {
			return modulewasm.Reply("unknown", nil, "Delivery outcome unavailable; message was not replayed"), nil
		}
		var result struct {
			OK     bool `json:"ok"`
			Result struct {
				MessageID int64 `json:"message_id"`
			} `json:"result"`
		}
		if response.StatusCode != 200 {
			b.deliveryStatus = "error"
			return modulewasm.Reply("error", nil, "Telegram rejected delivery"), nil
		}
		if json.Unmarshal(raw, &result) != nil {
			return modulewasm.Reply("unknown", nil, "Delivery confirmation unavailable; message was not replayed"), nil
		}
		if !result.OK {
			b.deliveryStatus = "error"
			return modulewasm.Reply("error", nil, "Telegram rejected delivery"), nil
		}
		if result.Result.MessageID <= 0 {
			return modulewasm.Reply("unknown", nil, "Delivery confirmation unavailable; message was not replayed"), nil
		}
		b.deliveryStatus = "ok"
		b.deliveryProof = map[string]any{"messageId": result.Result.MessageID, "deliveryVerified": true}
		return modulewasm.Reply("ok", b.deliveryProof, ""), nil
	default:
		return denied()
	}
}

func (b *monitorModuleBroker) Finalize(result moduleInvocationResult, runErr error) (moduleInvocationResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.sent {
		return result, runErr
	}
	if b.deliveryStatus != "ok" || runErr != nil || result.Status != "ok" {
		status := b.deliveryStatus
		if status != "error" {
			status = "unknown"
		}
		raw, _ := json.Marshal(b.deliveryProof)
		if b.deliveryProof == nil {
			raw = []byte(`{}`)
		}
		return moduleInvocationResult{Protocol: 2, Status: status, Data: raw, Error: "Host delivery result requires verification; it was not replayed"}, nil
	}
	var data map[string]any
	if json.Unmarshal(result.Data, &data) != nil || data == nil {
		data = map[string]any{}
	}
	for key, value := range b.deliveryProof {
		data[key] = value
	}
	result.Data, runErr = json.Marshal(data)
	return result, runErr
}
