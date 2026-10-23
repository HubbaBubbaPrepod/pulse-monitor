package main

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// Suspend before assigning the job: a module must not spawn an untracked
// descendant between process creation and assignment. Closing the job also
// removes children left behind by an otherwise successful invocation.
func runModuleCommand(cmd *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_SUSPENDED}
	cmd.Cancel = func() error { return terminateModuleJob(job, cmd.Process) }
	if err = cmd.Start(); err != nil {
		return err
	}
	abort := func(e error) error { _ = cmd.Process.Kill(); _ = cmd.Wait(); return e }
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return abort(err)
	}
	err = windows.AssignProcessToJobObject(job, process)
	windows.CloseHandle(process)
	if err != nil {
		return abort(err)
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return abort(err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != uint32(cmd.Process.Pid) {
			continue
		}
		thread, e := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if e != nil {
			return abort(e)
		}
		_, e = windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		if e != nil {
			return abort(e)
		}
		return cmd.Wait()
	}
	return abort(errors.New("Suspended module thread unavailable"))
}

// Cancellation may arrive while the suspended root has not yet joined the
// job. Kill that root as well as the job so an empty job cannot acknowledge
// cancellation while leaving a resumable process behind.
func terminateModuleJob(job windows.Handle, process *os.Process) error {
	err := process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		err = nil
	}
	return errors.Join(err, windows.TerminateJobObject(job, 1))
}
