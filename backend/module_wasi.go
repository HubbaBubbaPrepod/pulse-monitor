package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/HubbaBubbaPrepod/pulse-monitor/backend/internal/modulewasm"
)

func componentRuntime(c moduleComponentV2) string {
	if c.Runtime == "" {
		return "native"
	}
	return c.Runtime
}

func validateWASIArtifacts(binaries map[string]moduleBinary, content bool) error {
	binary, ok := binaries["wasip1-wasm"]
	if !ok || len(binaries) != 1 || len(binary.SHA256) != 64 {
		return errors.New("Invalid WASI platform or digest")
	}
	if _, err := hex.DecodeString(binary.SHA256); err != nil {
		return err
	}
	if content {
		if len(binary.Content) < 8 || len(binary.Content) > modulewasm.MaxArtifactBytes || !bytes.Equal(binary.Content[:8], []byte{0, 97, 115, 109, 1, 0, 0, 0}) {
			return errors.New("Invalid WASM artifact header")
		}
		sum := sha256.Sum256(binary.Content)
		if !strings.EqualFold(binary.SHA256, hex.EncodeToString(sum[:])) {
			return errors.New("WASM artifact digest mismatch")
		}
	}
	return nil
}

func validWASICapabilities(c moduleComponentV2) bool {
	if c.Runtime != "wasip1" {
		return len(c.OptionalCapabilities) == 0
	}
	if !validModuleNames(c.OptionalCapabilities, 32) || len(c.Requires)+len(c.OptionalCapabilities) > 32 {
		return false
	}
	allowed := []string{"state.read", "state.write", "actions.execute"}
	if c.Location == "agent" {
		allowed = append(allowed, "docker.read", "docker.logs", "docker.control")
	} else {
		allowed = append(allowed, "telemetry.read", "notifications.telegram")
	}
	for _, capability := range c.Requires {
		if !slices.Contains(allowed, capability) {
			return false
		}
	}
	for _, capability := range c.OptionalCapabilities {
		if !slices.Contains(allowed, capability) || slices.Contains(c.Requires, capability) {
			return false
		}
	}
	return true
}

func validWASIHostField(location string, field moduleField) bool {
	switch field.Key {
	case "socket":
		return false
	case "serverId", "telegramChatId", "telegramToken":
		if location != "monitor" {
			return false
		}
		kind := "text"
		if field.Key == "telegramToken" {
			kind = "password"
		}
		return field.Type == kind && (field.Default == nil || field.Default == "")
	default:
		return true
	}
}

func declaredModuleGrant(c moduleComponentV2, grant string) bool {
	return c.Runtime != "wasip1" || slices.Contains(c.Requires, grant) || slices.Contains(c.OptionalCapabilities, grant)
}

func (a *app) runModuleLease(ctx context.Context, path string, lease moduleLease) (moduleInvocationResult, error) {
	if lease.Runtime == "" || lease.Runtime == "native" {
		return runModuleInvocation(ctx, path, lease.Invocation)
	}
	if lease.Runtime != "wasip1" {
		return moduleInvocationResult{}, errors.New("Unsupported component runtime")
	}
	child, cancel := context.WithDeadline(ctx, lease.ExpiresAt)
	defer cancel()
	file, err := os.Open(path)
	if err != nil {
		return moduleInvocationResult{}, err
	}
	defer file.Close()
	artifact, err := io.ReadAll(io.LimitReader(file, modulewasm.MaxArtifactBytes+1))
	if err != nil || len(artifact) > modulewasm.MaxArtifactBytes {
		return moduleInvocationResult{}, errors.New("WASI artifact exceeds limit")
	}
	digest := sha256.Sum256(artifact)
	if !strings.EqualFold(lease.SHA256, hex.EncodeToString(digest[:])) {
		return moduleInvocationResult{}, errors.New("WASI artifact digest changed")
	}
	private := lease.Invocation
	if private.ServerID == "" {
		private.ServerID, _ = private.Config["serverId"].(string)
	}
	broker := &monitorModuleBroker{invocation: private, snapshot: func(ctx context.Context, id string) (json.RawMessage, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if _, ok := a.findServer(id); !ok {
			return nil, errors.New("Unknown node")
		}
		a.telemetryMu.RLock()
		defer a.telemetryMu.RUnlock()
		node := a.nodes[id]
		if node == nil {
			return nil, errors.New("Node telemetry unavailable")
		}
		return json.Marshal(map[string]any{"serverId": id, "seen": node.Seen, "sample": node.Report.Sample, "hardware": node.Report.Hardware, "disks": node.Report.Disks, "gpus": node.Report.GPUs})
	}}
	public := private
	public.Config = modulewasm.PublicFields(private.Config, lease.SecretFields)
	public.Input = modulewasm.PublicFields(private.Input, lease.SecretInputFields)
	raw, err := json.Marshal(public)
	if err != nil {
		return moduleInvocationResult{}, err
	}
	output, runErr := modulewasm.Run(child, artifact, raw, broker.Call)
	var result moduleInvocationResult
	if runErr == nil {
		result, runErr = decodeModuleInvocationResult(output)
	}
	return broker.Finalize(result, runErr)
}
