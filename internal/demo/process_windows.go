package demo

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Start suspended so the executable cannot create children before joining its job.
func startProcess(cmd *exec.Cmd) (func(bool), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = windows.CloseHandle(job) }
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		cleanup()
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	if err = cmd.Start(); err != nil {
		cleanup()
		return nil, err
	}
	if err = joinAndResume(job, uint32(cmd.Process.Pid)); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		cleanup()
		return nil, err
	}
	return func(bool) {
		defer cleanup()
		_ = windows.TerminateJobObject(job, 1)
		var account struct {
			TotalUserTime, TotalKernelTime, PeriodUserTime, PeriodKernelTime int64
			PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses uint32
		}
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation,
				uintptr(unsafe.Pointer(&account)), uint32(unsafe.Sizeof(account)), nil)
			if err != nil || account.ActiveProcesses == 0 {
				return
			}
			time.Sleep(30 * time.Millisecond)
		}
	}, nil
}

func joinAndResume(job windows.Handle, pid uint32) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	if err = windows.AssignProcessToJobObject(job, process); err != nil {
		return err
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		defer windows.CloseHandle(thread)
		count, err := windows.ResumeThread(thread)
		if err == nil && count != 1 {
			return errors.New("unexpected suspend count")
		}
		return err
	}
	return err
}
