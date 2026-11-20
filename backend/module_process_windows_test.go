//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestModuleCancellationKillsSuspendedRootBeforeJobAssignment(t *testing.T) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)
	cmd := exec.Command(os.Args[0], "-test.run=^TestModuleProcessTreeHelper$")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED, HideWindow: true}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	if err = terminateModuleJob(job, cmd.Process); err != nil {
		t.Fatal(err)
	}
	// Waiting must complete even though the job was empty and the suspended
	// root never ran. Terminating only the empty job would leave Wait blocked.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("suspended process ran successfully")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled suspended root remained alive")
	}
}

func crashProcessAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	status, err := windows.WaitForSingleObject(h, 0)
	return err == nil && status == uint32(windows.WAIT_TIMEOUT)
}

func TestModuleUnassignedCrashHelper(t *testing.T) {
	if os.Getenv("PULSE_CRASH_TEST_MODE") != "unassigned" {
		return
	}
	// A completed invocation must retain the host job's lifetime guard.
	if err := runModuleCommand(crashChild("exit")); err != nil {
		os.Exit(7)
	}
	cmd := crashChild("leaf")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	if cmd.Start() != nil {
		os.Exit(8)
	}
	if err := os.WriteFile(os.Getenv("PULSE_CRASH_TEST_MARKER")+".root", []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
		os.Exit(9)
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestModuleHostDeathKillsUnassignedSuspendedRoot(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "unassigned")
	t.Setenv("PULSE_CRASH_TEST_MARKER", marker)
	host := exec.Command(os.Args[0], "-test.run=^TestModuleUnassignedCrashHelper$")
	host.Env = append(os.Environ(), "PULSE_CRASH_TEST_MODE=unassigned")
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
	if err := host.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = host.Wait()
	assertCrashProcessStopped(t, root)
}
