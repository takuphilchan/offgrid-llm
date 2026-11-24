package execution

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestProcessJobWatchRejectsForeignProcessIdentity(t *testing.T) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)
	members := map[uint32]windows.Handle{}
	if err = captureExecutionJobMember(job, members, uint32(os.Getpid())); err == nil || len(members) != 0 {
		t.Fatal("foreign process became Job membership evidence", err, members)
	}
}

func TestProcessJobWatchDrainsAndClosesItsOwnMonitor(t *testing.T) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)
	watch, err := newExecutionJobWatch(job, map[uint32]windows.Handle{})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err = watch.drain(); err != nil {
			watch.close()
			t.Fatal(err)
		}
	}
	if err = watch.close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-watch.done:
	default:
		t.Fatal("completion port closed without monitor settlement")
	}
}
