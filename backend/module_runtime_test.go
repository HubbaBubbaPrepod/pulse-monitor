package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestModuleRuntimeChild(t *testing.T) {
	if os.Getenv("PULSE_RUNTIME_TEST_HELPER") != "1" {
		return
	}
	var input struct {
		Protocol int
		Config   map[string]any
		Input    map[string]any
	}
	if json.NewDecoder(os.Stdin).Decode(&input) != nil {
		os.Exit(2)
	}
	if input.Config["hang"] == true {
		time.Sleep(10 * time.Second)
	}
	if input.Config["invalid"] == true {
		os.Stdout.WriteString("not-json")
		os.Exit(0)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol": 2, "status": "ok", "data": input.Input, "state": map[string]any{"count": 1}})
	os.Exit(0)
}

func TestModuleRuntimeHonorsProtocolAndCancellation(t *testing.T) {
	t.Setenv("PULSE_RUNTIME_TEST_HELPER", "1")
	invocation := moduleInvocation{Protocol: 2, Config: map[string]any{}, Input: map[string]any{"containerId": "actual-id"}}
	out, err := runModuleInvocation(context.Background(), os.Args[0], invocation, "-test.run=TestModuleRuntimeChild")
	if err != nil || out.Protocol != 2 || out.Status != "ok" || string(out.Data) != `{"containerId":"actual-id"}` {
		t.Fatal(out, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	invocation.Config["hang"] = true
	if _, err := runModuleInvocation(ctx, os.Args[0], invocation, "-test.run=TestModuleRuntimeChild"); err == nil {
		t.Fatal("hung component was not cancelled")
	}
	delete(invocation.Config, "hang")
	invocation.Config["invalid"] = true
	if _, err := runModuleInvocation(context.Background(), os.Args[0], invocation, "-test.run=TestModuleRuntimeChild"); err == nil {
		t.Fatal("invalid output was accepted")
	}
}

func TestModuleRuntimeRedactsSecretValuesWithoutCorruptingJSON(t *testing.T) {
	raw := json.RawMessage(`{"token":"prefix-private-token-suffix","nested":["private-token",false,12],"a":"a"}`)
	got := redactModuleData(raw, []string{"private-token", "a"})
	if !json.Valid(got) {
		t.Fatal("redaction corrupted JSON", string(got))
	}
	var value map[string]any
	json.Unmarshal(got, &value)
	if value["token"] != "prefix-[REDACTED]-suffix" {
		t.Fatal(string(got))
	}
}
func TestModuleRedactionIncludesKeysAndBoundsOutput(t *testing.T) {
	got := redactModuleData(json.RawMessage(`{"private-token":"private-token"}`), []string{"private-token", "a"})
	var data map[string]any
	json.Unmarshal(got, &data)
	if _, ok := data["private-token"]; ok {
		t.Fatal("secret key exposed")
	}
	if data["[REDACTED]"] != "[REDACTED]" {
		t.Fatal("replacement was reprocessed", string(got))
	}
	raw, _ := json.Marshal(map[string]string{"value": strings.Repeat("x", 100*1024)})
	if got := redactModuleData(raw, []string{"x"}); len(got) != 0 {
		t.Fatal("expanded output must fail closed", len(got))
	}
}
func TestModuleRuntimeExecutesStoredArtifactOnCurrentPlatform(t *testing.T) {
	t.Setenv("PULSE_RUNTIME_TEST_HELPER", "1")
	binary, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), moduleArtifactFilename(runtime.GOOS+"-"+runtime.GOARCH))
	if err = os.WriteFile(path, binary, 0700); err != nil {
		t.Fatal(err)
	}
	input := moduleInvocation{Protocol: 2, Config: map[string]any{}, Input: map[string]any{"fixture": true}}
	if _, err = runModuleInvocation(context.Background(), path, input, "-test.run=TestModuleRuntimeChild"); err != nil {
		t.Fatal("stored native artifact cannot run", err)
	}
}
