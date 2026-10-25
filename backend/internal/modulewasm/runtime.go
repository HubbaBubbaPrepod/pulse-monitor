// Package modulewasm executes guests without inherited host I/O authority.
package modulewasm

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const (
	MaxArtifactBytes = 16 * 1024 * 1024
	MaxInputBytes    = 2 * 1024 * 1024
	MaxOutputBytes   = 512 * 1024
	MaxRequestBytes  = 64 * 1024
	MaxMemoryPages   = 2048
	MaxHostCalls     = 64
)

// Handler receives copied guest bytes, never authority supplied by the guest.
// The host must bind grants, private config and action targets in its closure.
type Handler func(context.Context, json.RawMessage) (json.RawMessage, error)

var slots = make(chan struct{}, 2)
var errOutput = errors.New("WASI output exceeds limit")

type boundedOutput struct {
	bytes.Buffer
	exceeded bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(p) > MaxOutputBytes-b.Len() {
		b.exceeded = true
		return 0, errOutput
	}
	return b.Buffer.Write(p)
}

// Run does not mount host directories, inherit environment variables, expose
// sockets, or start native commands. The memory limit covers guest linear
// memory, not total host RSS; the runtime itself remains trusted Go code.
func Run(ctx context.Context, artifact, input []byte, handler Handler) ([]byte, error) {
	if len(artifact) < 8 || len(artifact) > MaxArtifactBytes || !bytes.Equal(artifact[:8], []byte{0, 97, 115, 109, 1, 0, 0, 0}) {
		return nil, errors.New("Invalid WASI artifact")
	}
	if len(input) > MaxInputBytes {
		return nil, errors.New("WASI input exceeds limit")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	config := wazero.NewRuntimeConfigInterpreter().WithMemoryLimitPages(MaxMemoryPages).WithCloseOnContextDone(true).WithDebugInfoEnabled(false)
	runtime := wazero.NewRuntimeWithConfig(ctx, config)
	defer runtime.Close(context.Background())
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return nil, errors.New("WASI runtime unavailable")
	}
	calls := 0
	request := func(callCtx context.Context, module api.Module, ptr, length, out, capacity uint32) uint32 {
		calls++
		if calls > MaxHostCalls || callCtx.Err() != nil || length == 0 || length > MaxRequestBytes || capacity != MaxOutputBytes {
			return 0
		}
		memory := module.Memory()
		if memory == nil {
			return 0
		}
		raw, ok := memory.Read(ptr, length)
		if !ok {
			return 0
		}
		// Validate the entire output region BEFORE a potentially mutating call.
		if _, ok = memory.Read(out, capacity); !ok {
			return 0
		}
		copied := append(json.RawMessage(nil), raw...)
		if !json.Valid(copied) {
			return 0
		}
		reply := json.RawMessage(`{"ok":false,"status":"error","error":"Host API not granted"}`)
		if handler != nil {
			var err error
			reply, err = handler(callCtx, copied)
			if err != nil || len(reply) == 0 || !json.Valid(reply) || len(reply) > MaxOutputBytes {
				// The host may already have sent a request. Never turn an unavailable
				// response into success, retry the call, or reveal private host errors.
				reply = json.RawMessage(`{"ok":false,"status":"unknown","error":"Host result unavailable; verify its state"}`)
			}
		}
		if !memory.Write(out, reply) {
			return 0
		}
		return uint32(len(reply))
	}
	if _, err := runtime.NewHostModuleBuilder("pulse_host").NewFunctionBuilder().WithFunc(request).Export("request").Instantiate(ctx); err != nil {
		return nil, errors.New("Host API unavailable")
	}
	var output boundedOutput
	moduleConfig := wazero.NewModuleConfig().WithArgs("pulse-module").WithStdin(bytes.NewReader(input)).WithStdout(&output).WithStderr(io.Discard).WithRandSource(rand.Reader).WithSysWalltime().WithSysNanotime().WithSysNanosleep()
	_, err := runtime.InstantiateWithConfig(ctx, artifact, moduleConfig)
	if output.exceeded {
		return nil, errOutput
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, errors.New("WASI component failed")
	}
	return append([]byte(nil), output.Bytes()...), nil
}
