package inference

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func configureBackgroundProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// One non-inheritable job handle lives for the service's lifetime. Windows
// closes it even after a forced service exit, terminating only our children.
var runtimeJob struct {
	sync.Once
	handle windows.Handle
	err    error
}

func startOwnedProcess(cmd *exec.Cmd) error {
	runtimeJob.Do(func() {
		handle, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			runtimeJob.err = err
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		_, err = windows.SetInformationJobObject(handle, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
		if err != nil {
			windows.CloseHandle(handle)
			runtimeJob.err = err
			return
		}
		runtimeJob.handle = handle
	})
	if runtimeJob.err != nil {
		return fmt.Errorf("create runtime ownership job: %w", runtimeJob.err)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(runtimeJob.handle, handle)
		windows.CloseHandle(handle)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("attach runtime ownership job: %w", err)
	}
	return nil
}

func ownedProcessAlive(process *os.Process) bool {
	if process == nil {
		return false
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(process.Pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, 0)
	return err == nil && status == uint32(windows.WAIT_TIMEOUT)
}
