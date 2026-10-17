package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"
)

type moduleInvocation struct {
	Protocol int             `json:"protocol"`
	ID       string          `json:"id"`
	ServerID string          `json:"serverId,omitempty"`
	Trigger  string          `json:"trigger"`
	Action   string          `json:"action,omitempty"`
	Config   map[string]any  `json:"config"`
	Input    map[string]any  `json:"input"`
	State    json.RawMessage `json:"state"`
	Grants   []string        `json:"grants"`
	Event    json.RawMessage `json:"event,omitempty"`
}
type moduleInvocationResult struct {
	Protocol int             `json:"protocol"`
	Status   string          `json:"status"`
	Data     json.RawMessage `json:"data"`
	State    json.RawMessage `json:"state,omitempty"`
	Error    string          `json:"error,omitempty"`
}
type moduleOutputBuffer struct{ bytes.Buffer }

func (b *moduleOutputBuffer) Write(raw []byte) (int, error) {
	if b.Len()+len(raw) > 512*1024 {
		return 0, errors.New("Module output exceeds limit")
	}
	return b.Buffer.Write(raw)
}
func runModuleInvocation(ctx context.Context, path string, input moduleInvocation, args ...string) (moduleInvocationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result moduleInvocationResult
	raw, err := json.Marshal(input)
	if err != nil {
		return result, errors.New("Invalid invocation")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	configureModuleCommand(cmd)
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Stderr = io.Discard
	var output moduleOutputBuffer
	cmd.Stdout = &output
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return result, errors.New("Component failed or timed out; outcome may be unknown")
	}
	d := json.NewDecoder(bytes.NewReader(output.Bytes()))
	d.DisallowUnknownFields()
	if err := d.Decode(&result); err != nil {
		return result, errors.New("Invalid component response")
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return result, errors.New("Expected one component response")
	}
	if err := validateModuleInvocationResult(&result); err != nil {
		return result, err
	}
	return result, nil
}

func validateModuleInvocationResult(result *moduleInvocationResult) error {
	if result.Protocol != 2 || (result.Status != "ok" && result.Status != "error" && result.Status != "unknown") || len(result.Error) > 256 || len(result.Data)+len(result.State) > 512*1024 {
		return errors.New("Invalid component protocol or status")
	}
	if len(result.Data) == 0 {
		result.Data = json.RawMessage(`{}`)
	}
	for _, data := range []json.RawMessage{result.Data, result.State} {
		if len(data) == 0 {
			continue
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(data, &object) != nil || object == nil {
			return errors.New("Component data and state must be JSON objects")
		}
	}
	return nil
}

// Redact string values recursively, without replacing JSON syntax or numbers.
func redactModuleData(raw json.RawMessage, secrets []string) json.RawMessage {
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&value) != nil {
		return json.RawMessage(`{}`)
	}
	var walk func(any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case string:
			for _, secret := range secrets {
				if secret != "" {
					x = strings.ReplaceAll(x, secret, "[REDACTED]")
				}
			}
			return x
		case []any:
			for i, item := range x {
				x[i] = walk(item)
			}
		case map[string]any:
			for key, item := range x {
				x[key] = walk(item)
			}
		}
		return v
	}
	result, err := json.Marshal(walk(value))
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return result
}
