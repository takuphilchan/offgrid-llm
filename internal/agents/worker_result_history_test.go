package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestRetainedWorkerResultsAreRetrievableAcrossAttempts(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	defer r.Shutdown(context.Background())
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	accepted, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "followup-history", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Recheck the retained evidence"}})
	if err != nil {
		t.Fatal(err)
	}
	prior, _ := m.GetTask(child.ID)
	currentRoot, _ := m.GetTask(accepted.Worker.RootRunID)
	currentWorker, _ := m.GetTask(accepted.Worker.CurrentRunID)
	if currentRoot.ID == root.ID {
		t.Fatal("fixture did not create a distinct attempt")
	}
	for _, caller := range []*Task{currentRoot, currentWorker} {
		page, err := r.historyPage(context.Background(), caller, resultReference(prior), 0)
		if err != nil {
			t.Fatal("retained result unavailable to same work", caller.ID, err)
		}
		var result struct {
			Content string `json:"content"`
			Stale   bool   `json:"stale"`
		}
		if json.Unmarshal([]byte(page), &result) != nil || result.Content != prior.Result || !result.Stale {
			t.Fatal("old result bytes or revision provenance lost", page)
		}
	}
	foreign, _ := copyTask(currentRoot)
	foreign.WorkID = "unrelated-work"
	if _, err = r.historyPage(context.Background(), foreign, resultReference(prior), 0); err == nil {
		t.Fatal("worker identity alone granted another work access")
	}
	foreign, _ = copyTask(currentRoot)
	foreign.Origin.Workspace = "other-workspace"
	if _, err = r.historyPage(context.Background(), foreign, resultReference(prior), 0); err == nil {
		t.Fatal("worker identity crossed workspace scope")
	}
	r.SourceAccess = func(task *Task) bool { return task.ID != prior.ID }
	if _, err = r.historyPage(context.Background(), currentRoot, resultReference(prior), 0); err == nil {
		t.Fatal("retained reference bypassed live source revocation")
	}
}

func TestRetainedWorkerResultPagesKeepFinalCitations(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	defer r.Shutdown(context.Background())
	text := strings.Repeat("Evidence paragraph. ", 900) + "Final citation [source-42]."
	prior, err := m.updateTask(child.ID, func(task *Task) error { task.Result = text; return nil })
	if err != nil {
		t.Fatal(err)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	accepted, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "large-history", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Review final citations"}})
	if err != nil {
		t.Fatal(err)
	}
	root, _ = m.GetTask(accepted.Worker.RootRunID)
	var assembled strings.Builder
	for offset := 0; offset < len(text); {
		page, err := r.historyPage(context.Background(), root, resultReference(prior), offset)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Content string `json:"content"`
			Next    int    `json:"next_offset"`
		}
		if json.Unmarshal([]byte(page), &result) != nil || result.Next <= offset || len(result.Content) > 4096 {
			t.Fatal("invalid bounded page", page)
		}
		assembled.WriteString(result.Content)
		offset = result.Next
	}
	if assembled.String() != text {
		t.Fatal("retained full result lost its final citation")
	}
}
