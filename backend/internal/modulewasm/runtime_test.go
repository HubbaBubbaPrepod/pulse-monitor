package modulewasm

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var fixtureOnce sync.Once
var fixtureBytes []byte
var fixtureErr error

func guest(t *testing.T) []byte {
	t.Helper()
	fixtureOnce.Do(func() {
		directory, err := os.MkdirTemp("", "pulse-wasi-guest-")
		if err != nil {
			fixtureErr = err
			return
		}
		defer os.RemoveAll(directory)
		output := filepath.Join(directory, "guest.wasm")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", output, "./testdata/guest")
		command.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
		if raw, err := command.CombinedOutput(); err != nil {
			fixtureErr = &fixtureBuildError{err: err, output: string(raw)}
			return
		}
		fixtureBytes, fixtureErr = os.ReadFile(output)
	})
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	return fixtureBytes
}

type fixtureBuildError struct {
	err    error
	output string
}

func (e *fixtureBuildError) Error() string { return e.err.Error() + ": " + e.output }

func probe(t *testing.T, mode, path string, handler Handler) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"mode": mode, "path": path})
	output, err := Run(context.Background(), guest(t), raw, handler)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(output, &result); err != nil {
		t.Fatalf("invalid output: %v", err)
	}
	return result
}
func TestGuestCannotReadHostFiles(t *testing.T) {
	file := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(file, []byte("host-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	result := probe(t, "read", file, nil)
	if result["allowed"] != false || result["data"] != "" {
		t.Fatal("guest read host file", result)
	}
}
func TestGuestCannotWriteHostFiles(t *testing.T) {
	file := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(file, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if probe(t, "write", file, nil)["allowed"] != false {
		t.Fatal("guest wrote host file")
	}
	raw, err := os.ReadFile(file)
	if err != nil || string(raw) != "untouched" {
		t.Fatal("host file changed")
	}
}
func TestGuestHasNoInheritedSecrets(t *testing.T) {
	t.Setenv("PULSE_TEST_SECRET", "do-not-leak")
	if probe(t, "env", "", nil)["secret"] != "" {
		t.Fatal("host environment leaked")
	}
}
func TestGuestCannotConnectToHostNetwork(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if probe(t, "network", listener.Addr().String(), nil)["allowed"] != false {
		t.Fatal("guest opened a network socket")
	}
}
func TestGuestCannotStartProcesses(t *testing.T) {
	command := "/bin/true"
	if runtime.GOOS == "windows" {
		command = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	if _, err := os.Stat(command); err != nil {
		t.Fatal(err)
	}
	if probe(t, "process", command, nil)["allowed"] != false {
		t.Fatal("guest started host process")
	}
}

func TestGuestDeadline(t *testing.T) {
	wasm := guest(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	_, err := Run(ctx, wasm, []byte(`{"mode":"loop"}`), nil)
	if err == nil || ctx.Err() == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("unbounded guest: %v / %v", err, time.Since(start))
	}
}
func TestGuestOutputLimit(t *testing.T) {
	_, err := Run(context.Background(), guest(t), []byte(`{"mode":"output"}`), nil)
	if err == nil {
		t.Fatal("oversized guest output accepted")
	}
}
func TestGuestMemoryLimit(t *testing.T) {
	// A valid module declaring a minimum of 2049 pages, above the 2048-page limit.
	wasm := []byte{0, 97, 115, 109, 1, 0, 0, 0, 5, 4, 1, 0, 129, 16}
	if _, err := Run(context.Background(), wasm, []byte(`{}`), nil); err == nil {
		t.Fatal("oversized linear memory accepted")
	}
}
func TestInvalidHostBuffersDoNotDispatch(t *testing.T) {
	calls := 0
	handler := func(context.Context, json.RawMessage) (json.RawMessage, error) {
		calls++
		return json.RawMessage(`{"ok":true}`), nil
	}
	if probe(t, "buffers", "", handler)["rejected"] != true || calls != 0 {
		t.Fatalf("invalid memory dispatched %d host calls", calls)
	}
}
func TestGuestHostCallLimit(t *testing.T) {
	calls := 0
	handler := func(context.Context, json.RawMessage) (json.RawMessage, error) {
		calls++
		return json.RawMessage(`{"ok":true}`), nil
	}
	if probe(t, "calls", "", handler)["limited"] != true || calls != 64 {
		t.Fatalf("host call budget failed: %d", calls)
	}
}
func TestGuestCanUseOnlyExplicitBroker(t *testing.T) {
	calls := 0
	handler := func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		calls++
		if !strings.Contains(string(raw), `"echo"`) {
			t.Fatal("wrong request")
		}
		return json.RawMessage(`{"ok":true,"data":{"value":"bounded"}}`), nil
	}
	result := probe(t, "broker", "", handler)
	if calls != 1 || result["ok"] != true {
		t.Fatal("explicit broker unavailable", result)
	}
}

func TestGuestCannotGrowBeyondMemoryLimit(t *testing.T) {
	if _, err := Run(context.Background(), guest(t), []byte(`{"mode":"memory"}`), nil); err == nil {
		t.Fatal("guest grew beyond memory quota")
	}
	if probe(t, "normal", "", nil)["ok"] != true {
		t.Fatal("host did not survive memory exhaustion")
	}
}
func TestGuestCannotImportNativeHostFunctions(t *testing.T) {
	wasm := []byte{0, 97, 115, 109, 1, 0, 0, 0, 1, 4, 1, 96, 0, 0, 2, 14, 1, 3, 'e', 'n', 'v', 6, 's', 'y', 's', 't', 'e', 'm', 0, 0}
	if _, err := Run(context.Background(), wasm, []byte(`{}`), nil); err == nil {
		t.Fatal("unknown native host import accepted")
	}
}
