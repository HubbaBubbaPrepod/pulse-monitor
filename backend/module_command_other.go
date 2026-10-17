//go:build !windows

package main

import "os/exec"

func configureModuleCommand(cmd *exec.Cmd) {}
