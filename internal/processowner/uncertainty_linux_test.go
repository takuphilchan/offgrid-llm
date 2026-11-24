package processowner

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestNamespaceLostPidfdDoesNotInventSettlementAfterExit(t *testing.T) {
	launcher, connection := namespaceFixture(t)
	command := namespaceFixtureCommand("parent")
	job, err := Start(context.Background(), command, launcher)
	if err != nil {
		t.Fatal(err)
	}
	children := namespaceFixtureChildren(t, connection)
	retainedFD := job.initFD
	// Lose the owner's reference while the fixture retains its exact identity
	// solely for cleanup. An external observer cannot replace owner evidence.
	job.initFD = -1
	t.Cleanup(func() {
		unix.PidfdSendSignal(retainedFD, unix.SIGKILL, nil, 0)
		unix.Close(retainedFD)
		job.launcher.Close()
		job.shell.Close()
		if job.namespace != nil {
			job.namespace.Close()
		}
		uncertainNamespaces.Lock()
		delete(uncertainNamespaces.jobs, job)
		uncertainNamespaces.Unlock()
	})
	if err := job.Close(); !errors.Is(err, ErrStopUncertain) {
		t.Fatal("lost ownership reported close", err)
	}
	assertNamespaceChildrenExited(t, children)
	select {
	case <-job.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("fallback stop did not reap fixture")
	}
	if err := job.Close(); !errors.Is(err, ErrStopUncertain) {
		t.Fatal("observed exit manufactured lost owner proof", err)
	}
	uncertainNamespaces.Lock()
	_, retained := uncertainNamespaces.jobs[job]
	uncertainNamespaces.Unlock()
	if !retained {
		t.Fatal("uncertain owner resources were discarded")
	}
}

type cancelAfterStart struct {
	context.Context
	command *exec.Cmd
}

func (c cancelAfterStart) Err() error {
	if c.command.Process != nil {
		return context.Canceled
	}
	return nil
}

func TestNamespacePostLaunchFailurePreservesPossibleEffects(t *testing.T) {
	launcher, _ := namespaceFixture(t)
	command := exec.Command("/bin/sh", "-c", "exit 0")
	command.Env = []string{"PATH=/usr/bin:/bin"}
	job, err := Start(cancelAfterStart{context.Background(), command}, command, launcher)
	if job != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, ErrStarted) || errors.Is(err, ErrStopUncertain) {
		t.Fatal("post-launch cancellation lost dispatch/settlement evidence", job, err)
	}
	if command.ProcessState == nil {
		t.Fatal("owned child was not reaped")
	}
}
