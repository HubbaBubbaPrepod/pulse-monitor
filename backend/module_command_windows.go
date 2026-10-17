package main

import (
	"os/exec"
	"syscall"
)

func configureModuleCommand(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
