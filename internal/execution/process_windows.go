package execution

import (
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Start suspended, assign to an owned kill-on-close job, then resume. This
// avoids letting a running child spawn outside the job before assignment.
func startOwned(command *exec.Cmd) (func() error, func() error, func() error, error) {
	return startOwnedJob(command, true)
}

// The unobserved path exists only for a deterministic missed-notification test;
// production callers always enable discovery before assigning the first member.
func startOwnedJob(command *exec.Cmd, observe bool) (func() error, func() error, func() error, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	var lifecycle sync.Mutex
	closed := false
	jobClosed := false
	members := map[uint32]windows.Handle{}
	var watch *executionJobWatch
	memberMu := &sync.Mutex{}
	closeHandles := func() {
		for pid, handle := range members {
			windows.CloseHandle(handle)
			delete(members, pid)
		}
	}
	release := func() error {
		lifecycle.Lock()
		defer lifecycle.Unlock()
		if closed {
			return nil
		}
		if !jobClosed {
			memberMu.Lock()
			stopErr := stopExecutionJob(job, members)
			memberMu.Unlock()
			if watch != nil {
				if err := watch.drain(); err != nil {
					return errors.Join(stopErr, err)
				}
			}
			memberMu.Lock()
			settleErr := settleExecutionJob(job, members)
			if watch != nil {
				settleErr = errors.Join(settleErr, watch.fault)
			}
			memberMu.Unlock()
			if err := errors.Join(stopErr, settleErr); err != nil {
				return err
			}
			if err := windows.CloseHandle(job); err != nil {
				return err
			}
			jobClosed = true
		}
		if watch != nil {
			if err := watch.close(); err != nil {
				return err
			}
		}
		closed = true
		closeHandles()
		return nil
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		release()
		return nil, nil, nil, err
	}
	if observe {
		watch, err = newExecutionJobWatch(job, members)
		if err != nil {
			windows.CloseHandle(job)
			return nil, nil, nil, err
		}
		memberMu = &watch.mu
	}
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
	if err = command.Start(); err != nil {
		release()
		return nil, nil, nil, err
	}
	fail := func(e error) (func() error, func() error, func() error, error) {
		command.Process.Kill()
		command.Wait()
		if cleanupErr := release(); cleanupErr != nil {
			// The caller retains launch uncertainty; these capabilities must
			// not leak after a failed construction with no returned owner.
			if !jobClosed {
				windows.CloseHandle(job)
			}
			if watch == nil || watch.close() == nil {
				closeHandles()
			}
			e = errors.Join(e, errProcessCleanupUncertain, cleanupErr)
		}
		return nil, nil, nil, e
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(command.Process.Pid))
	if err != nil {
		return fail(err)
	}
	memberMu.Lock()
	err = windows.AssignProcessToJobObject(job, process)
	if err == nil {
		members[uint32(command.Process.Pid)] = process
	}
	memberMu.Unlock()
	if err != nil {
		windows.CloseHandle(process)
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
		if jobClosed {
			return ErrHostUnavailable
		}
		memberMu.Lock()
		defer memberMu.Unlock()
		return stopExecutionJob(job, members)
	}, command.Wait, release, nil
}

func nativeCleanupVerified(err error) bool { return err == nil }
