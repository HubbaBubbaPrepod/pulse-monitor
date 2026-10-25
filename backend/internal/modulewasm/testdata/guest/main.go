package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

//go:wasmimport pulse_host request
func hostRequest(request, length, response, capacity uint32) uint32

func call(raw []byte, invalid string) []byte {
	output := make([]byte, 512*1024)
	ptr := uint32(uintptr(unsafe.Pointer(&raw[0])))
	dst := uint32(uintptr(unsafe.Pointer(&output[0])))
	if invalid == "request" {
		ptr = 0xfffffff0
	}
	if invalid == "response" {
		dst = 0xfffffff0
	}
	capacity := uint32(len(output))
	if invalid == "capacity" {
		capacity = 64
	}
	size := hostRequest(ptr, uint32(len(raw)), dst, capacity)
	runtime.KeepAlive(raw)
	runtime.KeepAlive(output)
	if size > uint32(len(output)) {
		panic("invalid host length")
	}
	return output[:size]
}

func main() {
	var input struct {
		Mode string `json:"mode"`
		Path string `json:"path"`
	}
	if json.NewDecoder(os.Stdin).Decode(&input) != nil {
		os.Exit(1)
	}
	var result any
	switch input.Mode {
	case "read":
		data, err := os.ReadFile(input.Path)
		result = map[string]any{"allowed": err == nil, "data": string(data)}
	case "write":
		err := os.WriteFile(input.Path, []byte("guest-write"), 0600)
		result = map[string]any{"allowed": err == nil}
	case "env":
		result = map[string]any{"secret": os.Getenv("PULSE_TEST_SECRET")}
	case "network":
		connection, err := net.DialTimeout("tcp", input.Path, 100*time.Millisecond)
		if connection != nil {
			connection.Close()
		}
		result = map[string]any{"allowed": err == nil}
	case "process":
		err := exec.Command(input.Path, "/c", "exit", "0").Run()
		result = map[string]any{"allowed": err == nil}
	case "loop":
		for {
		}
	case "memory":
		data := make([]byte, 160*1024*1024)
		for i := range data {
			data[i] = 1
		}
		result = map[string]any{"allocated": len(data)}
	case "output":
		fmt.Print(strings.Repeat("x", 512*1024+1))
		return
	case "broker":
		result = json.RawMessage(call([]byte(`{"method":"echo","params":{}}`), ""))
	case "buffers":
		first := call([]byte(`{"method":"echo","params":{}}`), "request")
		second := call([]byte(`{"method":"echo","params":{}}`), "response")
		third := call([]byte(`{"method":"echo","params":{}}`), "capacity")
		result = map[string]any{"rejected": len(first) == 0 && len(second) == 0 && len(third) == 0}
	case "calls":
		var last []byte
		for i := 0; i < 65; i++ {
			last = call([]byte(`{"method":"echo","params":{}}`), "")
		}
		result = map[string]any{"limited": len(last) == 0}
	default:
		result = map[string]any{"ok": true}
	}
	if json.NewEncoder(os.Stdout).Encode(result) != nil {
		os.Exit(1)
	}
}
