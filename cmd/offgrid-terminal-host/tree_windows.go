package main

import (
	"errors"
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

type ownedTree struct{ job windows.Handle }

func (t *ownedTree) release() { _ = windows.CloseHandle(t.job) }
func (t *ownedTree) stop() error {
	if err := windows.TerminateJobObject(t.job, 1); err != nil {
		return err
	}
	// A shell exit is not proof that grandchildren stopped. Query the owned job.
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		var info struct {
			UserTime, KernelTime, PeriodUserTime, PeriodKernelTime int64
			PageFaults, Total, Active, Terminated                  uint32
		}
		if err := windows.QueryInformationJobObject(t.job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
			return err
		}
		if info.Active == 0 {
			return nil
		}
	}
	return errors.New("terminal_stop_unconfirmed")
}
func startTree(command *exec.Cmd) (*ownedTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	tree := &ownedTree{job: job}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		tree.release()
		return nil, err
	}
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
	if err = command.Start(); err != nil {
		tree.release()
		return nil, err
	}
	fail := func(e error) (*ownedTree, error) {
		_ = command.Process.Kill()
		_ = command.Wait()
		tree.release()
		return nil, e
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
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != uint32(command.Process.Pid) {
			continue
		}
		thread, e := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if e != nil {
			return fail(e)
		}
		_, e = windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		if e != nil {
			return fail(e)
		}
		return tree, nil
	}
	return fail(errors.New("terminal_worker_thread_missing"))
}
