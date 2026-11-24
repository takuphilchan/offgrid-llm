package execution

import (
	"errors"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var executionIsProcessInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")

const maxExecutionJobMembers = 4096

type executionJobAccounting struct {
	UserTime, KernelTime, PeriodUserTime, PeriodKernelTime           int64
	PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses uint32
}

func captureExecutionJobMembers(job windows.Handle, members map[uint32]windows.Handle) error {
	var list struct {
		Assigned, Listed uint32
		PIDs             [maxExecutionJobMembers]uintptr
	}
	err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList, uintptr(unsafe.Pointer(&list)), uint32(unsafe.Sizeof(list)), nil)
	if list.Assigned > maxExecutionJobMembers || list.Listed > maxExecutionJobMembers || errors.Is(err, windows.ERROR_MORE_DATA) {
		return ErrLimit
	}
	if err != nil {
		return err
	}
	if list.Assigned != list.Listed {
		return ErrHostUnavailable
	}
	for _, raw := range list.PIDs[:list.Listed] {
		if raw == 0 || uint64(raw) > 1<<32-1 {
			return ErrHostUnavailable
		}
		if err := captureExecutionJobMember(job, members, uint32(raw)); err != nil {
			return err
		}
	}
	return nil
}

func captureExecutionJobMember(job windows.Handle, members map[uint32]windows.Handle, pid uint32) error {
	if members[pid] != 0 {
		return nil
	}
	if len(members) >= maxExecutionJobMembers {
		return ErrLimit
	}
	process, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	var belongs int32
	ok, _, callErr := executionIsProcessInJob.Call(uintptr(process), uintptr(job), uintptr(unsafe.Pointer(&belongs)))
	if ok == 0 || belongs == 0 {
		windows.CloseHandle(process)
		if ok == 0 {
			return callErr
		}
		return ErrHostUnavailable
	}
	members[pid] = process
	return nil
}

func stopExecutionJob(job windows.Handle, members map[uint32]windows.Handle) error {
	// Capture before terminating: membership removal alone is not an exit
	// barrier. Stop still signals the owned Job if capture cannot be proved.
	return errors.Join(captureExecutionJobMembers(job, members), windows.TerminateJobObject(job, 1))
}

func settleExecutionJob(job windows.Handle, members map[uint32]windows.Handle) error {
	if err := stopExecutionJob(job, members); err != nil {
		return err
	}
	deadline := time.Now().Add(2 * time.Second)
	for _, process := range members {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ErrHostUnavailable
		}
		state, err := windows.WaitForSingleObject(process, uint32((remaining+time.Millisecond-1)/time.Millisecond))
		if err != nil {
			return err
		}
		if state != windows.WAIT_OBJECT_0 {
			return ErrHostUnavailable
		}
	}
	var counts executionJobAccounting
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&counts)), uint32(unsafe.Sizeof(counts)), nil); err != nil {
		return err
	}
	// A transient descendant may have exited before it could be captured.
	// Lifetime count detects that missing proof; restart/empty lists cannot
	// manufacture a handle or authorize cleanup of its former working copy.
	if counts.ActiveProcesses != 0 || counts.TotalProcesses != uint32(len(members)) {
		return ErrHostUnavailable
	}
	return nil
}
