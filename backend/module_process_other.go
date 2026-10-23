//go:build !windows && !linux

package main

import (
	"errors"
	"os/exec"
)

func runModuleCommand(cmd *exec.Cmd) error {
	return errors.New("Module process containment is unavailable on this OS")
}
