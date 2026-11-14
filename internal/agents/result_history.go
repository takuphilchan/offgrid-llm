package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Result references bind a handoff to exact saved bytes. An actor-owned run ID
// alone is not a grant to read its contents from another task.
func resultReference(task *Task) string {
	data := []byte(task.Result)
	if task.Config.ResultIdentityVersion == 1 {
		// Equal prose prepared against different instructions or source scope is
		// a different result. Existing runs keep their original content-only IDs.
		data, _ = json.Marshal(struct {
			Content            string
			InstructionVersion uint64
			ControlEpoch       uint64
			Status             TaskStatus
			Sources            []string
		}{task.Result, task.InstructionVersion, task.ControlEpoch, task.Status, resultDependencies(task)})
		data = append([]byte("canonical-result-v1\x00"), data...)
	}
	digest := sha256.Sum256(data)
	return "result:" + task.ID + ":" + hex.EncodeToString(digest[:])
}

func (r *Runner) historyPage(ctx context.Context, task *Task, reference string, offset int) (string, error) {
	if !strings.HasPrefix(reference, "result:") {
		return contextPage(task, reference, offset)
	}
	parts := strings.Split(reference, ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid result reference")
	}
	target, exists := r.manager.GetTask(parts[1])
	if !exists || target.Actor != task.Actor || target.DeletedAt != nil {
		return "", fmt.Errorf("result is unavailable or changed")
	}
	if scope, canonical := resultControlScope(target); canonical {
		result, err := r.manager.ResultRecord(scope, reference)
		if err != nil {
			return "", err
		}
		applyResultRecord(target, result)
	} else if resultReference(target) != reference {
		return "", fmt.Errorf("result is unavailable or changed")
	}
	allowed := false
	stale := target.InstructionVersion != task.InstructionVersion || len(task.PendingInstructions) > 0
	if task.ParentID == "" && target.ParentID == task.ID {
		for _, link := range task.ChildHistory {
			allowed = allowed || link.ID == target.ID
			stale = stale || link.ID == target.ID && link.Invalidated
		}
	} else if task.ParentID != "" && target.ParentID == task.ParentID {
		parent, exists := r.manager.GetTask(task.ParentID)
		if exists && parent.Actor == task.Actor && parent.DeletedAt == nil {
			var dependencies []string
			for _, link := range parent.ChildHistory {
				if link.ID == task.ID {
					dependencies = link.Spec.DependsOn
				}
			}
			for _, link := range parent.ChildHistory {
				if link.ID == target.ID && containsTool(dependencies, link.Spec.Key) {
					allowed = true
				}
				stale = stale || link.ID == target.ID && link.Invalidated
			}
			if task.Worker != nil {
				allowed = false
				for _, pinned := range task.DependencyResults {
					allowed = allowed || pinned == reference
				}
			}
		}
	}
	if !allowed {
		var historical bool
		allowed, historical = r.retainedWorkerResultAccess(task, target, reference)
		stale = stale || historical
	}
	if !allowed || (target.Status != TaskCompleted && target.Status != TaskFailed && target.Status != TaskCancelled) {
		return "", fmt.Errorf("result is outside this task's settled dependencies")
	}
	if err := r.checkSourceAccess(target); err != nil {
		return "", err
	}
	if authorizer, ok := r.tools.(interface {
		AuthorizeTaskResult(context.Context, *Task, *Task) error
	}); ok {
		if err := authorizer.AuthorizeTaskResult(ctx, task, target); err != nil {
			return "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	data := target.Result
	if offset < 0 || offset > len(data) || (offset < len(data) && !utf8.RuneStart(data[offset])) {
		return "", fmt.Errorf("invalid result offset")
	}
	end := min(offset+4096, len(data))
	for end < len(data) && !utf8.RuneStart(data[end]) {
		end--
	}
	page, _ := json.Marshal(map[string]any{"reference": reference, "content": data[offset:end], "next_offset": end, "complete": end == len(data), "total_bytes": len(data), "status": target.Status, "instruction_version": target.InstructionVersion, "stale": stale, "independently_verified": false})
	return string(page), nil
}

type childOutcome struct {
	Changes               *WorkerCapturedOutput `json:"changes,omitempty"`
	WorkerID              string                `json:"worker_id,omitempty"`
	RunID                 string                `json:"run_id"`
	Key                   string                `json:"key"`
	Status                TaskStatus            `json:"status"`
	Output                string                `json:"output,omitempty"`
	ResultReference       string                `json:"result_reference"`
	TotalBytes            int                   `json:"total_bytes"`
	InstructionVersion    uint64                `json:"instruction_version"`
	Stale                 bool                  `json:"stale"`
	Error                 string                `json:"error,omitempty"`
	IndependentlyVerified bool                  `json:"independently_verified"`
	EvidenceSteps         []int                 `json:"evidence_steps,omitempty"`
}

func childOutcomes(parent *Task, children map[string]*Task) []childOutcome {
	results := make([]childOutcome, 0, len(parent.Delegation.Children))
	for _, link := range parent.Delegation.Children {
		child := children[link.Spec.Key]
		outcome := outcomeForChild(link.Spec.Key, child)
		outcome.Changes = workerCapturedOutput(parent, child.ID)
		outcome.Stale = outcome.Stale || link.Invalidated || child.InstructionVersion != parent.InstructionVersion || len(parent.PendingInstructions) > 0
		results = append(results, outcome)
	}
	return results
}

func outcomeForChild(key string, child *Task) childOutcome {
	result := childOutcome{RunID: child.ID, Key: key, Status: child.Status, Output: boundedResult(child.Result), Error: boundedResult(child.Error), ResultReference: resultReference(child), TotalBytes: len(child.Result)}
	result.InstructionVersion = child.InstructionVersion
	if child.Worker != nil {
		result.WorkerID = child.Worker.SessionID
	}
	result.Stale = child.ErrorCode == "dependency_superseded" || child.ErrorCode == "instruction_superseded"
	for _, step := range child.Steps {
		if step.ToolName != "" && step.ID > 0 {
			result.EvidenceSteps = append(result.EvidenceSteps, step.ID)
		}
	}
	return result
}

func outcomeWithChanges(parent *Task, key string, child *Task) childOutcome {
	outcome := outcomeForChild(key, child)
	outcome.Changes = workerCapturedOutput(parent, child.ID)
	return outcome
}

func partialDelegationSummary(results []childOutcome) string {
	var summary strings.Builder
	summary.WriteString("Delegated work partially completed. These recorded outputs have not been independently verified.\n")
	for _, result := range results {
		fmt.Fprintf(&summary, "\n%s (%s):\n", result.Key, result.Status)
		if result.Stale {
			summary.WriteString("Earlier assignment evidence; revalidation is required before use as completion.\n")
		}
		if result.Output != "" {
			summary.WriteString(result.Output + "\n")
		}
		if result.Error != "" {
			summary.WriteString("Limitation: " + result.Error + "\n")
		}
		fmt.Fprintf(&summary, "Saved result: %s\n", result.ResultReference)
	}
	return summary.String()
}

func finishPartialDelegation(parent *Task, children map[string]*Task) {
	results := childOutcomes(parent, children)
	encoded, _ := json.Marshal(results)
	appendDelegationOutcome(parent, string(encoded))
	parent.Checkpoint.Calls = nil
	finishTask(parent, TaskFailed, "Some delegated work could not complete. Independent completed results are retained; inspect the child jobs before retrying.")
	parent.ErrorCode = "delegation_partial"
	parent.Result = partialDelegationSummary(results)
	parent.Steps = append(parent.Steps, Step{ID: len(parent.Steps) + 1, Type: "delegation", ToolName: "delegate_tasks", ToolResult: string(encoded), Timestamp: time.Now().UTC()})
}

func prepareDelegationSynthesis(parent *Task, children map[string]*Task) {
	encoded, _ := json.Marshal(childOutcomes(parent, children))
	parent.Checkpoint.Messages = append(parent.Checkpoint.Messages, api.ChatMessage{Role: "system", Content: "If a dependent specialist produced the requested final artifact, return that artifact verbatim with its headings, citations, uncertainty statements and required length. Do not compress or rewrite it into a status summary."})
	appendDelegationOutcome(parent, string(encoded))
	parent.Checkpoint.Calls, parent.Delegation = nil, nil
	parent.Status, parent.SchedulerReady = TaskPending, true
	parent.Steps = append(parent.Steps, Step{ID: len(parent.Steps) + 1, Type: "delegation", ToolName: "delegate_tasks", ToolResult: string(encoded), Timestamp: time.Now().UTC()})
}

func retainChildResultSources(parent *Task, children map[string]*Task) {
	seen := map[string]bool{}
	for _, id := range parent.Config.KnowledgeDependencies {
		seen[id] = true
	}
	add := func(id string) {
		if id != "" && !seen[id] {
			parent.Config.KnowledgeDependencies = append(parent.Config.KnowledgeDependencies, id)
			seen[id] = true
		}
	}
	for _, link := range parent.Delegation.Children {
		child := children[link.Spec.Key]
		for _, id := range child.Config.KnowledgeDependencies {
			add(id)
		}
		for _, document := range child.Config.KnowledgeDocuments {
			add(document.ID)
		}
		for _, passage := range child.Config.KnowledgeSources {
			add(passage.DocumentID)
		}
	}
}

func boundedResult(value string) string {
	if len(value) <= 8000 {
		return value
	}
	end := 8000
	for !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + "\n[Excerpt. Use task_history with result_reference and offset 0, then next_offset, for the complete result.]"
}
