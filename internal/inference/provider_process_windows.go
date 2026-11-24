package inference

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A provider job owns exactly one runtime and the descendants it creates.
// Unlike the legacy service-wide job, it can retire one provider without
// terminating unrelated work. No breakaway flags or inheritable handles exist.
type providerProcessJob struct {
	handle windows.Handle
	cmd    *exec.Cmd
	once   sync.Once
	err    error
	mu     sync.RWMutex
	closed bool
}

func startProviderProcess(cmd *exec.Cmd) (func(*exec.Cmd) error, error) {
	owned, err := startProviderJob(cmd)
	if err != nil {
		return nil, err
	}
	return func(requested *exec.Cmd) error {
		if requested != cmd {
			return errors.New("provider termination identity mismatch")
		}
		return owned.close()
	}, nil
}

func startProviderJob(cmd *exec.Cmd) (*providerProcessJob, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	// Suspension closes the child-spawns-before-job-assignment race. The sole
	// initial thread is resumed only after this exact process belongs to the job.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_SUSPENDED}
	if err = cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	assigned := false
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		assigned = err == nil
		windows.CloseHandle(process)
	}
	if err == nil {
		err = resumeProviderInitialThread(uint32(cmd.Process.Pid))
	}
	if err != nil {
		// No runtime code can execute before successful assignment and resume.
		// A failed resume may still leave an assigned child; close also kills it.
		var stopErr error
		if assigned {
			stopErr = (&providerProcessJob{handle: job, cmd: cmd}).close()
		} else {
			stopped := make(chan error, 1)
			go func() { stopped <- terminateProviderProcess(cmd) }()
			select {
			case stopErr = <-stopped:
			case <-time.After(10 * time.Second):
				stopErr = errors.New("unassigned suspended provider did not stop within deadline")
			}
			windows.CloseHandle(job)
		}
		if stopErr != nil {
			return nil, errors.Join(err, errProviderStopUncertain, stopErr)
		}
		return nil, err
	}
	return &providerProcessJob{handle: job, cmd: cmd}, nil
}

func resumeProviderInitialThread(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	var threadID uint32
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID == pid {
			if threadID != 0 {
				return errors.New("suspended provider has unexpected additional threads")
			}
			threadID = entry.ThreadID
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return err
	}
	if threadID == 0 {
		return errors.New("suspended provider initial thread is unavailable")
	}
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME|windows.THREAD_QUERY_LIMITED_INFORMATION, false, threadID)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(thread)
	// Toolhelp IDs are snapshots. Bind the opened handle back to the retained
	// process identity before resuming, even if the original thread disappeared.
	owner, _, queryErr := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessIdOfThread").Call(uintptr(thread))
	if owner == 0 {
		return fmt.Errorf("query provider thread owner: %w", queryErr)
	}
	if uint32(owner) != pid {
		return errors.New("provider initial thread identity changed")
	}
	previous, err := windows.ResumeThread(thread)
	if err != nil {
		return err
	}
	if previous != 1 {
		return errors.New("provider initial thread suspension changed")
	}
	return nil
}

// Layout from JOBOBJECT_BASIC_ACCOUNTING_INFORMATION, including all four
// LARGE_INTEGER fields before the four DWORD counts (48 bytes on both ABIs).
type providerJobAccounting struct {
	UserTime, KernelTime, PeriodUserTime, PeriodKernelTime           int64
	PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses uint32
}

func providerJobCounts(job windows.Handle) (providerJobAccounting, error) {
	var counts providerJobAccounting
	err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&counts)), uint32(unsafe.Sizeof(counts)), nil)
	return counts, err
}

func providerJobProcessHandles(job windows.Handle) ([]windows.Handle, error) {
	var ids struct {
		Assigned, Count uint32
		IDs             [512]uintptr
	}
	err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList, uintptr(unsafe.Pointer(&ids)), uint32(unsafe.Sizeof(ids)), nil)
	if err != nil || ids.Assigned > uint32(len(ids.IDs)) || ids.Count > uint32(len(ids.IDs)) {
		return nil, errors.Join(errors.New("provider process snapshot unavailable or oversized"), err)
	}
	var handles []windows.Handle
	for _, id := range ids.IDs[:ids.Count] {
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(id))
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			continue
		}
		if err != nil {
			for _, opened := range handles {
				windows.CloseHandle(opened)
			}
			return nil, err
		}
		handles = append(handles, handle)
	}
	return handles, nil
}

func (job *providerProcessJob) close() error {
	job.once.Do(func() {
		job.mu.Lock()
		defer job.mu.Unlock()
		job.closed = true
		before, countsErr := providerJobCounts(job.handle)
		handles, snapshotErr := providerJobProcessHandles(job.handle)
		defer func() {
			for _, handle := range handles {
				windows.CloseHandle(handle)
			}
		}()
		killErr := windows.TerminateJobObject(job.handle, 1)
		if countsErr != nil || snapshotErr != nil {
			job.err = errors.Join(errProviderStopUncertain, countsErr, snapshotErr, killErr)
			return
		}
		deadline := time.NewTimer(10 * time.Second)
		defer deadline.Stop()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			counts, err := providerJobCounts(job.handle)
			if err != nil {
				job.err = errors.Join(errProviderStopUncertain, err)
				return
			}
			if counts.TotalProcesses != before.TotalProcesses {
				job.err = errors.Join(errProviderStopUncertain, errors.New("provider process tree changed during termination snapshot"))
				return
			}
			allExited := true
			for _, handle := range handles {
				status, err := windows.WaitForSingleObject(handle, 0)
				if err != nil || status != windows.WAIT_OBJECT_0 {
					allExited = false
					if err != nil {
						job.err = errors.Join(errProviderStopUncertain, err)
						return
					}
				}
			}
			if counts.ActiveProcesses == 0 && allExited {
				break
			}
			if killErr != nil {
				job.err = errors.Join(errProviderStopUncertain, killErr)
				return
			}
			select {
			case <-deadline.C:
				job.err = errors.Join(errProviderStopUncertain, errors.New("provider descendants did not stop within deadline"))
				return
			case <-tick.C:
			}
		}
		// Wait may include pipe-copy goroutines. It is bounded independently of
		// confirmed process exit; a stuck reap never reports released capacity.
		reaped := make(chan error, 1)
		go func() { reaped <- job.cmd.Wait() }()
		select {
		case err := <-reaped:
			var exited *exec.ExitError
			if err != nil && !errors.As(err, &exited) {
				job.err = errors.Join(errProviderStopUncertain, fmt.Errorf("reap provider: %w", err))
				return
			}
		case <-deadline.C:
			job.err = errors.Join(errProviderStopUncertain, errors.New("provider reap exceeded deadline"))
			return
		}
		// Keep the handle on uncertainty, so forced service exit still enforces
		// kill-on-close. A clean release is permitted only after both proofs.
		job.err = windows.CloseHandle(job.handle)
	})
	return job.err
}
