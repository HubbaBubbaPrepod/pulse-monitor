package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestModuleProcessTreeHelper(t *testing.T) {
	mode := os.Getenv("PULSE_TREE_TEST_MODE")
	if mode == "" {
		return
	}
	marker := os.Getenv("PULSE_TREE_TEST_MARKER")
	if mode == "child" {
		_ = os.WriteFile(marker+".ready", []byte("ready"), 0600)
		until := time.Now().Add(8 * time.Second)
		for time.Now().Before(until) {
			if _, e := os.Stat(marker + ".go"); e == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		_ = os.WriteFile(marker, []byte("survived"), 0600)
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestModuleProcessTreeHelper$")
	cmd.Env = append(os.Environ(), "PULSE_TREE_TEST_MODE=child")
	if cmd.Start() != nil {
		os.Exit(7)
	}
	_ = os.WriteFile(marker+".pid", []byte(strconv.Itoa(cmd.Process.Pid)), 0600)
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		if _, e := os.Stat(marker + ".ready"); e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if mode == "exit" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol": 2, "status": "ok", "data": map[string]any{}})
		os.Exit(0)
	}
	time.Sleep(10 * time.Second)
	os.Exit(0)
}
func TestModuleRunTerminatesDescendants(t *testing.T) {
	for _, mode := range []string{"cancel", "exit"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "marker")
			t.Setenv("PULSE_TREE_TEST_MODE", mode)
			t.Setenv("PULSE_TREE_TEST_MARKER", marker)
			defer func() {
				raw, _ := os.ReadFile(marker + ".pid")
				pid, _ := strconv.Atoi(string(raw))
				if pid > 0 {
					p, e := os.FindProcess(pid)
					if e == nil {
						_ = p.Kill()
					}
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := runModuleInvocation(ctx, os.Args[0], moduleInvocation{Protocol: 2}, "-test.run=^TestModuleProcessTreeHelper$")
				done <- err
			}()
			until := time.Now().Add(5 * time.Second)
			for time.Now().Before(until) {
				if _, e := os.Stat(marker + ".ready"); e == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if _, e := os.Stat(marker + ".ready"); e != nil {
				cancel()
				<-done
				t.Fatal("child did not become ready")
			}
			if mode == "cancel" {
				cancel()
			}
			err := <-done
			if mode == "cancel" && err == nil {
				t.Fatal("cancelled invocation succeeded")
			}
			if mode == "exit" && err != nil {
				t.Fatal(err)
			}
			_ = os.WriteFile(marker+".go", []byte("go"), 0600)
			time.Sleep(350 * time.Millisecond)
			if _, e := os.Stat(marker); e == nil {
				t.Fatal("module child survived invocation completion")
			}
		})
	}
}
