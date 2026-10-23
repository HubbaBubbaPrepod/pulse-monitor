//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
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
