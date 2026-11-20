package main

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func crashProcessAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	// A zombie has stopped executing; its new parent owns reaping it.
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	parts := strings.Fields(string(raw)[strings.LastIndex(string(raw), ")")+1:])
	return len(parts) > 0 && parts[0] != "Z"
}

func TestModuleSupervisorReapsOrphanedDescendants(t *testing.T) {
	// Adopt un-reaped orphans here so an init daemon cannot hide a runner leak.
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 0, 0, 0, 0) })
	marker := filepath.Join(t.TempDir(), "reap")
	t.Setenv("PULSE_CRASH_TEST_MARKER", marker)
	host := crashChild("host")
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Process.Kill(); _ = host.Wait() })
	root := crashPID(t, marker+".root")
	leaf := crashPID(t, marker+".leaf")
	t.Cleanup(func() {
		_ = syscall.Kill(root, syscall.SIGKILL)
		_ = syscall.Kill(leaf, syscall.SIGKILL)
		_, _ = syscall.Wait4(leaf, nil, 0, nil)
	})
	if err := host.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = host.Wait()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		_, err := os.Stat("/proc/" + strconv.Itoa(leaf))
		if os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant %d remained allocated after module cleanup (including zombies)", leaf)
}
