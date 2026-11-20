package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// This helper deliberately survives until its host is killed by the test.
func TestModuleCrashHelper(t *testing.T) {
	mode := os.Getenv("PULSE_CRASH_TEST_MODE")
	if mode == "" {
		return
	}
	marker := os.Getenv("PULSE_CRASH_TEST_MARKER")
	switch mode {
	case "host":
		cmd := crashChild("root")
		cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
		cmd.WaitDelay = time.Second
		if err := runModuleCommand(cmd); err != nil {
			os.Exit(7)
		}
	case "root":
		if err := os.WriteFile(marker+".root", []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(8)
		}
		child := crashChild("leaf")
		if err := child.Start(); err != nil {
			os.Exit(9)
		}
		for {
			time.Sleep(time.Second)
		}
	case "leaf":
		if err := os.WriteFile(marker+".leaf", []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(10)
		}
		for {
			time.Sleep(time.Second)
		}
	case "exit":
		os.Exit(0)
	}
	os.Exit(0)
}

func crashChild(mode string) *exec.Cmd {
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestModuleCrashHelper$")
	cmd.Env = append(os.Environ(), "PULSE_CRASH_TEST_MODE="+mode)
	return cmd
}

func crashPID(t *testing.T, path string) int {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		raw, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(string(raw))
			if err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("child did not become ready: %s", path)
	return 0
}

func assertCrashProcessStopped(t *testing.T, pid int) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		if !crashProcessAlive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("module process %d survived abrupt host death", pid)
}

func TestModuleHostDeathTerminatesDescendants(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "crash")
	t.Setenv("PULSE_CRASH_TEST_MARKER", marker)
	host := crashChild("host")
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Process.Kill(); _ = host.Wait() })
	root := crashPID(t, marker+".root")
	t.Cleanup(func() {
		if p, e := os.FindProcess(root); e == nil {
			_ = p.Kill()
		}
	})
	leaf := crashPID(t, marker+".leaf")
	t.Cleanup(func() {
		if p, e := os.FindProcess(leaf); e == nil {
			_ = p.Kill()
		}
	})
	if err := host.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = host.Wait()
	assertCrashProcessStopped(t, root)
	assertCrashProcessStopped(t, leaf)
}
