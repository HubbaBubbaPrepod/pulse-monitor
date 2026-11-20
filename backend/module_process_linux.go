package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const moduleSupervisorArgument = "--pulse-native-supervisor-v1"

// The supervisor runs before normal application or test flag processing.
// Its only lifetime authority is an anonymous pipe supplied by this host.
func init() {
	if len(os.Args) >= 4 && os.Args[1] == moduleSupervisorArgument {
		os.Exit(superviseModule(os.Args[2:]))
	}
}

// A supervisor survives host SIGKILL long enough to kill the module's process
// group. Parent-death signals alone do not protect the module's descendants.
// This manages trusted native code's lifetime; it is not an OS sandbox.
func runModuleCommand(cmd *exec.Cmd) error {
	if len(cmd.ExtraFiles) != 0 {
		return errors.New("Native module descriptors are not supported")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	lifetime, owner, err := os.Pipe()
	if err != nil {
		return err
	}
	defer lifetime.Close()
	defer owner.Close()
	args := cmd.Args
	if len(args) == 0 {
		args = []string{cmd.Path}
	}
	cmd.Args = append([]string{executable, moduleSupervisorArgument, cmd.Path}, args...)
	cmd.Path = executable
	cmd.ExtraFiles = []*os.File{lifetime}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := owner.Close()
		if errors.Is(err, os.ErrClosed) {
			return nil
		}
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	// Only the supervisor owns this end. Modules do not inherit fd 3.
	lifetime.Close()
	return cmd.Wait()
}

func superviseModule(args []string) int {
	lifetime := os.NewFile(3, "pulse-native-owner")
	if lifetime == nil {
		return 127
	}
	defer lifetime.Close()
	info, err := lifetime.Stat()
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		return 127
	}
	// Adopt grandchildren when their module parent exits, so cleanup reaps them
	// instead of leaving that duty to the host's init implementation.
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		return 127
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _, _ = io.Copy(io.Discard, lifetime); cancel() }()
	// Pdeathsig follows the creating OS thread: keep it alive until Wait.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cmd := exec.CommandContext(ctx, args[0], args[2:]...)
	cmd.Args = args[1:]
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
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
		return 127
	}
	defer func() {
		_ = kill()
		reapModuleDescendants()
	}()
	if err := cmd.Wait(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() > 0 {
			return exit.ExitCode()
		}
		return 1
	}
	return 0
}

func reapModuleDescendants() {
	// A trusted native process can deliberately leave its process group. Bound
	// the wait rather than letting such a descendant hang invocation cleanup.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		pid, err := syscall.Wait4(-1, nil, syscall.WNOHANG, nil)
		if err == syscall.ECHILD {
			return
		}
		if err != nil && err != syscall.EINTR {
			return
		}
		if pid == 0 {
			time.Sleep(time.Millisecond)
		}
	}
}
