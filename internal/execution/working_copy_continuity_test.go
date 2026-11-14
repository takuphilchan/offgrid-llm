package execution

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkingCopyContinuationAcrossRootAttempts(t *testing.T) {
	e, store, binding, source := workingCopyFixture(t)
	store.policy.MaxTotalBytes = 8 * 1024
	ctx := context.Background()
	writeCandidateFile(t, source, "input.txt", "original")
	firstReq := workingCopyRequest(binding, "first")
	first, err := e.PrepareWorker(ctx, firstReq)
	if err != nil {
		t.Fatal(err)
	}
	writeCandidateFile(t, first.Context.Root, "input.txt", "first private edit")
	set := sealCandidate(t, first, firstReq)
	if err = e.Cancel(ctx, binding); err != nil {
		t.Fatal(err)
	}
	nextBinding := binding
	nextBinding.Job = "next-work-attempt"
	if err = e.Admit(ctx, nextBinding); err != nil {
		t.Fatal(err)
	}
	nextReq := workingCopyRequest(nextBinding, "followup")
	nextReq.ControlEpoch++
	ref := candidateReference(set)
	ref.SourceJob = binding.Job
	nextReq.Prerequisites = []ChangeSetReference{ref}
	next, err := e.PrepareWorker(ctx, nextReq)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(next.Context.Root, "input.txt"))
	if err != nil || string(data) != "first private edit" {
		t.Fatal("lost previous attempt", string(data), err)
	}
	writeCandidateFile(t, next.Context.Root, "input.txt", "next private edit")
	nextSet := sealCandidate(t, next, nextReq)
	lastReq := workingCopyRequest(nextBinding, "dependent")
	lastReq.ControlEpoch = nextReq.ControlEpoch
	lastReq.Prerequisites = []ChangeSetReference{candidateReference(nextSet)}
	last, err := e.PrepareWorker(ctx, lastReq)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(last.Context.Root, "input.txt"))
	if err != nil || string(data) != "next private edit" {
		t.Fatal("transitive previous attempt lost", string(data), err)
	}
	data, err = os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(data) != "original" {
		t.Fatal("original changed", string(data), err)
	}
}
