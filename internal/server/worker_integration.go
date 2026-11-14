package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func hasIntegrationCandidate(task *agents.Task) bool {
	if task.Checkpoint == nil || task.Checkpoint.StorageBudget == nil {
		return false
	}
	for _, entry := range task.Checkpoint.StorageBudget.Reservations {
		if entry.Capture != nil && entry.Capture.Result != nil && entry.Capture.Result.Candidate != nil && entry.Capture.Result.Candidate.State == "ready" && entry.Capture.Result.InstructionVersion == task.InstructionVersion && entry.Capture.Result.ControlEpoch == task.ControlEpoch {
			return true
		}
	}
	return false
}

// File-effect evidence must match the durable service reservation and current
// source, not merely a model-visible JSON result. This does not certify the task.
func verifiedWorkspaceIntegration(task *agents.Task, step agents.Step) bool {
	if task.Config.Execution == nil || task.Origin == nil || task.ParentID != "" || task.Checkpoint == nil || task.Checkpoint.StorageBudget == nil || step.CallID == "" {
		return false
	}
	ref := task.Config.Execution
	source := execution.Binding{Actor: task.Actor, Workspace: task.Origin.Workspace, Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision, Job: task.ID}
	var output struct {
		Integration *agents.WorkerIntegrationResult `json:"integration"`
	}
	if json.Unmarshal([]byte(step.ToolResult), &output) != nil || output.Integration == nil {
		return false
	}
	for _, entry := range task.Checkpoint.StorageBudget.Reservations {
		i := entry.Integration
		if i != nil && i.CallID == step.CallID && i.State == "integrated" && i.Result != nil && *i.Result == *output.Integration && i.Result.Total > 0 && i.Result.Integrated == i.Result.Total && i.Request.Source == source && i.Request.InstructionVersion == task.InstructionVersion && i.Request.ControlEpoch == task.ControlEpoch {
			return true
		}
	}
	return false
}

func (b *browserRunTools) integrateWorkerChanges(ctx context.Context, task *agents.Task, args json.RawMessage, call agents.ToolExecution) (string, error) {
	var input struct {
		WorkerRunID string `json:"worker_run_id"`
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || input.WorkerRunID == "" || len(input.WorkerRunID) > 256 {
		return "", &agents.ToolFailure{Code: "integration_arguments_invalid", Message: "Select the completed test worker whose combined candidate covers the editing results. No integration was dispatched."}
	}
	req, fresh, err := b.server.agentManager.ReserveWorkerIntegration(task.ID, task.Actor, input.WorkerRunID, call.CallID)
	if err != nil {
		return "", &agents.ToolFailure{Code: "integration_not_ready", Message: "The candidate is not current, fully tested with the completed editing branches, or within the remaining storage allowance. Preserve the proposals and resolve the limitation; no integration was dispatched.", Cause: err}
	}
	ref := execution.Reference{Host: req.Source.Host, Session: req.Source.Session, Grant: req.Source.Grant, Revision: req.Source.Revision}
	operation := "integration_status"
	if fresh {
		operation = "integrate"
	}
	invoke := func(callCtx context.Context, operation string) (execution.WorkerArtifactReply, error) {
		return b.server.executionHosts.WorkerArtifact(callCtx, task.Actor, b.server.workspaceID, ref, execution.WorkerArtifactRequest{Operation: operation, Source: req.Source, Integrate: &req})
	}
	cancel := func() (execution.WorkerArtifactReply, error) {
		stopCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		return invoke(stopCtx, "integration_cancel")
	}
	for {
		var reply execution.WorkerArtifactReply
		if ctx.Err() != nil || !b.server.agentManager.WorkerIntegrationControlCurrent(task.ID, task.Actor, input.WorkerRunID, call.CallID, req) {
			reply, err = cancel()
		} else {
			reply, err = invoke(ctx, operation)
		}
		if err != nil {
			reply = execution.WorkerArtifactReply{State: "uncertain"}
		}
		if err = b.server.agentManager.RecordWorkerIntegration(task.ID, task.Actor, input.WorkerRunID, req, reply); err != nil {
			return "", err
		}
		if reply.State != "accepted" {
			current, found := b.server.agentManager.GetTask(task.ID)
			if !found {
				return "", agents.ErrTaskNotFound
			}
			result := agents.WorkerIntegrationOutput(current, input.WorkerRunID)
			if result == nil {
				return "", agents.ErrRunConflict
			}
			if result.State == "uncertain" {
				return "", fmt.Errorf("Integration outcome is uncertain. Retained receipt %s must be inspected; do not repeat the mutation", result.ID)
			}
			guidance := "These are native file-operation receipts, not proof of semantic task completion. Conflicts and partial outcomes retain original versions; do not repeat integration or overwrite conflicting files."
			if result.State == "abandoned" {
				guidance = "This backend cannot preserve displaced contents during replacement. No file mutation was dispatched. The sealed change set remains available for inspection and download through the work changes interface. Deliver the reviewable changes and explain this limitation; do not replace files through another tool."
			}
			encoded, _ := json.Marshal(map[string]any{"integration": result, "semantic_completion_verified": false, "guidance": guidance})
			return string(encoded), nil
		}
		operation = "integration_status"
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
