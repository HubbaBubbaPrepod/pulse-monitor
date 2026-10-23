package main

import (
	"os"
	"os/exec"
	"syscall"
)

// Modules own their process group. Clean up descendants on cancellation and
// normal exit; this is lifetime management, not an OS security sandbox.
func runModuleCommand(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	kill := func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.Cancel = kill
	if err := cmd.Start(); err != nil {
		return err
	}
	defer kill()
	return cmd.Wait()
}
