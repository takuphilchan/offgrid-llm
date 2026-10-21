package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func progressDescriptor(name string) capabilities.Descriptor {
	return capabilities.Descriptor{Name: name, Source: "local-companion", Namespace: "computer", Kind: capabilities.Computer, Risk: capabilities.RiskLow, Revision: capabilities.NativeEffectsRevision}
}

func progressTask() *Task {
	cfg := observationConfig()
	cfg.NativeWorkflowVersion = DocumentWorkflowEvidenceVersion
	cfg.NativeActionContractVersion = 1
	return &Task{Actor: "alice", Status: TaskRunning, Config: cfg, Checkpoint: &Checkpoint{}}
}

func progressJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func progressView(sequence int, text string) map[string]any {
	return map[string]any{
		"observation": map[string]any{"id": fmt.Sprintf("observation-%d", sequence), "sequence": sequence, "captured_at": fmt.Sprintf("time-%d", sequence), "state_digest": "stable", "target": map[string]any{"id": "target", "process_generation": "process", "surface": "window", "driver": "windows-uia"}},
		"elements":    []any{map[string]any{"id": fmt.Sprintf("element-%d", sequence), "identity": "stable-control", "name": "Editor", "role": "edit", "writable": true, "text": text}},
		"limited":     false,
	}
}

func progressDocuments(sequence int, revision string) map[string]any {
	view := progressView(sequence, "")
	view["observation"].(map[string]any)["state_digest"] = fmt.Sprintf("binding-dependent-digest-%d", sequence)
	return map[string]any{
		"observation": view["observation"], "complete": true,
		"documents": []any{map[string]any{"id": "document", "generation": "generation", "container": "tab", "selected": true, "capabilities": map[string]any{"read": true, "insert_text": true}}},
		"active":    map[string]any{"target": view["observation"].(map[string]any)["target"], "document": "document", "generation": "generation", "container": "tab", "editor": "editor", "revision": revision, "observation": fmt.Sprintf("observation-%d", sequence)},
		"commands":  []any{map[string]any{"id": fmt.Sprintf("command-%d", sequence), "element": fmt.Sprintf("element-%d", sequence), "kind": "new_document", "profile": "editor", "revision": "reviewed-1"}},
	}
}

func progressRead(sequence, pageStart int) map[string]any {
	inventory := progressDocuments(sequence, "content-1")
	return map[string]any{"binding": inventory["active"], "snapshot": fmt.Sprintf("snapshot-%d", sequence), "complete": true, "text_sha256": "content-digest", "bytes": 24, "composition": "stable", "selection": map[string]any{"start": 0, "end": 0}, "page": map[string]any{"snapshot": fmt.Sprintf("snapshot-%d", sequence), "binding": inventory["active"], "start": pageStart, "end": pageStart + 8, "total_scalars": 24, "next": pageStart + 8, "text": "private!", "source_complete": true}}
}

func TestNativeProgressIgnoresFreshReferencesAndPersistsBoundedWarning(t *testing.T) {
	task := progressTask()
	for i := 0; i <= nativeProgressLimit; i++ {
		task.Checkpoint.Iteration++ // The actual runner charges the model decision.
		paused := recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", progressJSON(progressView(i, "private-fixture-text")))
		if paused != (i == nativeProgressLimit) {
			t.Fatalf("iteration %d pause=%v state=%+v", i, paused, task.Checkpoint.NativeProgress)
		}
		if task.Checkpoint.Iteration != i+1 {
			t.Fatal("recovery changed root budget")
		}
		// Simulate a durable checkpoint round-trip between all reads.
		encoded, err := json.Marshal(task.Checkpoint)
		if err != nil || json.Unmarshal(encoded, &task.Checkpoint) != nil {
			t.Fatal("checkpoint round-trip failed", err)
		}
	}
	if task.Status != TaskInterrupted || task.ErrorCode != "computer_no_progress" || len(task.Steps) != 1 || len(task.Checkpoint.Messages) != 1 {
		t.Fatal("missing truthful interruption or duplicate feedback", task)
	}
	encoded, _ := json.Marshal(task.Checkpoint.NativeProgress)
	if strings.Contains(string(encoded), "private") || len(encoded) > 1600 {
		t.Fatal("progress state retained private/raw observation", string(encoded))
	}
	if recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", progressJSON(progressView(99, "changed"))) || task.Status != TaskInterrupted {
		t.Fatal("progress check resumed an interrupted task")
	}
}

func TestNativeProgressDetectsAlternatingReadsWithoutMetadataReset(t *testing.T) {
	task := progressTask()
	for i := 0; i < 9; i++ {
		name, result := "computer_documents", progressJSON(progressDocuments(i, "unchanged"))
		if i%3 == 1 {
			name, result = "computer_read_text", progressJSON(progressRead(i, 0))
		} else if i%3 == 2 {
			name, result = "computer_observe", progressJSON(progressView(i, "unchanged"))
		}
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "task_metadata", ToolName: "task_plan", Content: fmt.Sprintf("cosmetic plan %d", i)})
		task.Workflow = &WorkflowEvidence{Checks: []OutcomeCheck{{Expectation: OutcomeExpectation{ID: fmt.Sprintf("new-%d", i), Kind: "document_blank"}, State: "pending"}}}
		paused := recordNativeReadProgress(task, progressDescriptor(name), name, result)
		if i == 7 && !paused {
			t.Fatal("alternating reads/metadata escaped bounded pause", task.Checkpoint.NativeProgress)
		}
		if paused {
			return
		}
	}
	t.Fatal("no-progress cycle was not detected")
}

func TestNativeProgressMeaningfulEvidenceResetsNotVolatileIDs(t *testing.T) {
	for _, change := range []string{"target", "control-text", "capability", "document-revision", "page", "verified-check"} {
		t.Run(change, func(t *testing.T) {
			task := progressTask()
			name, result := "computer_observe", progressView(1, "old")
			if change == "capability" || change == "document-revision" {
				name, result = "computer_documents", progressDocuments(1, "old")
			} else if change == "page" {
				name, result = "computer_read_text", progressRead(1, 0)
			}
			for i := 0; i <= nativeProgressWarning; i++ {
				recordNativeReadProgress(task, progressDescriptor(name), name, progressJSON(result))
			}
			if !task.Checkpoint.NativeProgress.Warned {
				t.Fatal("fixture did not reach warning")
			}
			switch change {
			case "target":
				result["observation"].(map[string]any)["target"].(map[string]any)["id"] = "other-target"
			case "control-text":
				result["elements"].([]any)[0].(map[string]any)["text"] = "new"
			case "capability":
				result["documents"].([]any)[0].(map[string]any)["capabilities"].(map[string]any)["insert_text"] = false
			case "document-revision":
				result["active"].(map[string]any)["revision"] = "new"
			case "page":
				result = progressRead(2, 8)
			case "verified-check":
				task.Workflow = &WorkflowEvidence{Checks: []OutcomeCheck{{Expectation: OutcomeExpectation{Kind: "document_text_equals", Document: "document"}, State: "verified", EvidenceSteps: []int{1}}}}
			}
			if recordNativeReadProgress(task, progressDescriptor(name), name, progressJSON(result)) || task.Checkpoint.NativeProgress.Unchanged != 0 || task.Checkpoint.NativeProgress.Warned {
				t.Fatal("meaningful new evidence did not reset stalled state", task.Checkpoint.NativeProgress)
			}
		})
	}
}

func TestNativeProgressDoesNotClassifyErrorsMutationsOrUncertainty(t *testing.T) {
	task := progressTask()
	result := progressJSON(progressView(1, "unchanged"))
	for i := 0; i < 3; i++ {
		recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", result)
	}
	before, _ := json.Marshal(task.Checkpoint.NativeProgress)
	for _, invalid := range []string{`{"error":"unavailable"}`, `{"status":"failed"}`, `{`, `{"observation":{},"elements":[]}`, `{"error":null,"error":"duplicate"}`} {
		if recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", invalid) {
			t.Fatal("error became a no-progress interruption")
		}
	}
	for _, descriptor := range []capabilities.Descriptor{progressDescriptor("computer_insert_text"), {Name: "computer_observe", Source: "mcp:fake", Namespace: "computer", Kind: capabilities.Computer, Risk: capabilities.RiskLow, Revision: capabilities.NativeEffectsRevision}, {Name: "computer_observe", Source: "local-companion", Namespace: "computer", Kind: capabilities.Computer, Risk: capabilities.RiskLow, Revision: "unreviewed"}} {
		recordNativeReadProgress(task, descriptor, descriptor.Name, result)
	}
	task.Checkpoint.ExecutingCall = "uncertain-input"
	recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", result)
	task.Checkpoint.ExecutingCall = ""
	task.Checkpoint.Calls = []api.ToolCall{{ID: "pending"}}
	recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", result)
	task.Checkpoint.Calls = nil
	task.Status = TaskUncertain
	recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", result)
	after, _ := json.Marshal(task.Checkpoint.NativeProgress)
	if string(before) != string(after) || task.Status != TaskUncertain || len(task.Checkpoint.Messages) != 0 {
		t.Fatal("error/mutation/untrusted provider changed progress or uncertainty")
	}
}

func TestNativeProgressApplicationCatalogIgnoresStepAndTracksResults(t *testing.T) {
	task := progressTask()
	for i := 0; i < 4; i++ {
		data := map[string]any{"applications": []any{map[string]any{"id": "editor", "name": "Editor"}}, "query": "Editor", "resolution": "unique", "total_matches": 1, "catalog_step": i, "offgrid_step_id": i}
		recordNativeReadProgress(task, progressDescriptor("computer_find_applications"), "computer_find_applications", progressJSON(data))
	}
	if !task.Checkpoint.NativeProgress.Warned {
		t.Fatal("changing catalog step escaped progress accounting")
	}
}

func TestNativeProgressLeavesLegacyAndUnknownContractsUnchanged(t *testing.T) {
	for _, version := range []int{0, 2} {
		task := progressTask()
		task.Config.NativeActionContractVersion = version
		for i := 0; i < 12; i++ {
			if recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", progressJSON(progressView(i, "unchanged"))) {
				t.Fatalf("contract %d acquired new recovery behavior", version)
			}
		}
		if task.Status != TaskRunning || task.Checkpoint.NativeProgress != nil || len(task.Checkpoint.Messages) != 0 {
			t.Fatalf("contract %d changed", version)
		}
	}
}

func TestNativeProgressDuplicateCheckMetadataIsNotNewEvidence(t *testing.T) {
	task := progressTask()
	check := OutcomeCheck{Expectation: OutcomeExpectation{ID: "one", Kind: "application_open", ApplicationID: "editor"}, State: "verified", EvidenceSteps: []int{1}}
	task.Workflow = &WorkflowEvidence{Checks: []OutcomeCheck{check}}
	want := nativeVerifiedProgress(task)
	check.Expectation.ID, check.Expectation.Title = "two", "New title for the same evidence"
	check.EvidenceSteps = []int{1, 2}
	task.Workflow.Checks = append(task.Workflow.Checks, check)
	if nativeVerifiedProgress(task) != want {
		t.Fatal("duplicate check metadata masqueraded as progress")
	}
}

func TestNativeProgressAppliedCorrectionStartsFreshEpoch(t *testing.T) {
	task := progressTask()
	result := progressJSON(progressView(1, "unchanged"))
	for i := 0; i <= nativeProgressWarning; i++ {
		recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", result)
	}
	if !task.Checkpoint.NativeProgress.Warned {
		t.Fatal("fixture did not reach warning")
	}
	before := task.Checkpoint.NativeProgress.Scope
	task.PendingInstructions = []Instruction{{RequestID: "correction", Digest: "digest", Text: "Inspect a different part of this application."}}
	task.Workflow = &WorkflowEvidence{Revision: 99}
	recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", result)
	if task.Checkpoint.NativeProgress.Scope != before || task.Checkpoint.NativeProgress.Unchanged != nativeProgressWarning+1 {
		t.Fatal("pending correction or model metadata reset progress")
	}
	task.Instructions, task.PendingInstructions = task.PendingInstructions, nil
	if recordNativeReadProgress(task, progressDescriptor("computer_observe"), "computer_observe", result) || task.Checkpoint.NativeProgress.Scope == before || task.Checkpoint.NativeProgress.Unchanged != 0 || task.Checkpoint.NativeProgress.Warned {
		t.Fatal("applied correction did not establish a new progress epoch")
	}
}

type progressRunTools struct{ observationTestTools }

func (*progressRunTools) Capability(name string) (capabilities.Descriptor, bool) {
	return progressDescriptor(name), true
}

func (*progressRunTools) GetTools() []api.Tool {
	return []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "computer_observe", Parameters: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}}}}
}

func TestNativeProgressRunnerRetainsRootBudgetAndPausesReadLoop(t *testing.T) {
	for _, limit := range []int{3, 12} {
		t.Run(fmt.Sprintf("budget-%d", limit), func(t *testing.T) {
			tools := &progressRunTools{}
			tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
			sequence := 0
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
				sequence++
				return progressJSON(progressView(sequence, "unchanged")), nil
			}
			runner := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: "computer_observe", Arguments: `{}`}}}}), nil
			})
			defer runner.Shutdown(context.Background())
			cfg := observationConfig()
			cfg.NativeWorkflowVersion = DocumentWorkflowEvidenceVersion
			cfg.NativeActionContractVersion = 1
			cfg.MaxIterations = limit
			task, err := runner.Create("Inspect the selected application", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil {
				t.Fatal(err)
			}
			if limit == 3 {
				if got.Status != TaskFailed || got.Checkpoint.Iteration != 3 || sequence != 3 {
					t.Fatalf("existing root budget extended: status=%s iterations=%d calls=%d", got.Status, got.Checkpoint.Iteration, sequence)
				}
			} else if got.Status != TaskInterrupted || got.ErrorCode != "computer_no_progress" || got.Checkpoint.Iteration != nativeProgressLimit+1 || sequence != nativeProgressLimit+1 {
				t.Fatalf("no-progress integration: status=%s code=%s iterations=%d calls=%d", got.Status, got.ErrorCode, got.Checkpoint.Iteration, sequence)
			}
		})
	}
}
