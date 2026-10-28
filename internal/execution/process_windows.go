package execution

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Start suspended, assign to an owned kill-on-close job, then resume. This
// avoids letting a running child spawn outside the job before assignment.
func startOwned(command *exec.Cmd) (func() error, func() error, func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	var lifecycle sync.Mutex
	closed := false
	release := func() {
		lifecycle.Lock()
		defer lifecycle.Unlock()
		if !closed {
			closed = true
			windows.CloseHandle(job)
		}
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		release()
		return nil, nil, nil, err
	}
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
	if err = command.Start(); err != nil {
		release()
		return nil, nil, nil, err
	}
	fail := func(e error) (func() error, func() error, func(), error) {
		command.Process.Kill()
		command.Wait()
		release()
		return nil, nil, nil, e
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err != nil {
		return fail(err)
	}
	err = windows.AssignProcessToJobObject(job, process)
	windows.CloseHandle(process)
	if err != nil {
		return fail(err)
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fail(err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	err = windows.Thread32First(snapshot, &entry)
	resumed := false
	for err == nil {
		if entry.OwnerProcessID == uint32(command.Process.Pid) {
			thread, e := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if e != nil {
				return fail(e)
			}
			_, e = windows.ResumeThread(thread)
			windows.CloseHandle(thread)
			if e != nil {
				return fail(e)
			}
			resumed = true
			break
		}
		err = windows.Thread32Next(snapshot, &entry)
	}
	if !resumed {
		return fail(windows.ERROR_INVALID_HANDLE)
	}
	return func() error {
		lifecycle.Lock()
		defer lifecycle.Unlock()
		if closed {
			return nil
		}
		return windows.TerminateJobObject(job, 1)
	}, command.Wait, release, nil
}
