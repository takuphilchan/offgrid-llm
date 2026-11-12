package agents

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// A rejected delegation has not created child jobs or dispatched a tool. Keep
// these errors distinct from persistence and scheduling failures, which must
// still interrupt the run at its durable checkpoint.
type delegationAdmissionError struct{ reason string }

func (e *delegationAdmissionError) Error() string { return e.reason }

func rejectDelegation(reason string) error { return &delegationAdmissionError{reason: reason} }

func sameChildSpec(a, b ChildSpec) bool {
	return a.ProfileID == b.ProfileID && a.Key == b.Key && a.Goal == b.Goal &&
		slices.Equal(a.DependsOn, b.DependsOn) && slices.Equal(a.Tools, b.Tools)
}

// A model may repeat a graph after receiving its completed result. A repeated
// call is an observation of saved work, never a request to create the children
// again. The prior result is the same bounded, actor-scoped result the parent
// already received; no child state is inferred from model prose.
func (r *Runner) reuseCompletedDelegation(task *Task, call api.ToolCall, specs []ChildSpec) (bool, error) {
	if len(specs) == 0 || len(specs) > len(task.ChildHistory) {
		return false, nil
	}
	prior := task.ChildHistory[len(task.ChildHistory)-len(specs):]
	for i, spec := range specs {
		if prior[i].Status != TaskCompleted || !sameChildSpec(prior[i].Spec, spec) {
			return false, nil
		}
	}
	for i := len(task.Steps) - 1; i >= 0; i-- {
		step := task.Steps[i]
		if step.Type != "delegation" || step.ToolName != "delegate_tasks" || step.ToolResult == "" {
			continue
		}
		var saved []childOutcome
		if json.Unmarshal([]byte(step.ToolResult), &saved) != nil || len(saved) != len(prior) {
			continue
		}
		matches := true
		for j, outcome := range saved {
			if outcome.RunID != prior[j].ID || outcome.Key != prior[j].Spec.Key || outcome.Status != TaskCompleted {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		cp := task.Checkpoint
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: step.ToolResult})
		cp.Calls = nil
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "delegation_reused", ToolName: call.Function.Name, ToolResult: step.ToolResult, Timestamp: time.Now().UTC()})
		return true, r.persist(task)
	}
	return false, nil
}

// Admission failures are known to precede child creation. Close the saved
// call with a typed tool result so Resume cannot retry it indefinitely and
// the parent can answer from completed child evidence. Bound repeated bad
// proposals instead of presenting a false completed answer.
func (r *Runner) recordRejectedDelegation(task *Task, call api.ToolCall, reason string) error {
	result, _ := json.Marshal(map[string]any{
		"error": "delegation_not_admitted", "message": reason,
		"effects": "none", "verified": false,
		"recovery": "Use already completed specialist results to answer, or propose a smaller authorized graph only if the request still needs one.",
	})
	cp := task.Checkpoint
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: string(result)})
	cp.Calls = nil
	task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "rejected", ToolName: call.Function.Name, ToolArgs: call.Function.Arguments, ToolResult: string(result), Timestamp: time.Now().UTC()})
	rejections := 0
	for _, step := range task.Steps {
		if step.Type == "rejected" && step.ToolName == "delegate_tasks" {
			rejections++
		}
	}
	if rejections >= 3 {
		task.ErrorCode = "delegation_not_admitted"
		_, err := r.fail(task, "Delegation could not be admitted after three proposals: "+reason+". Completed specialist results remain available for inspection.")
		return err
	}
	return r.persist(task)
}

func containsTool(allowed []string, name string) bool {
	for _, item := range allowed {
		if item == name {
			return true
		}
	}
	return false
}

// Names alone are not authority: user tools can replace built-ins. Keep this
// boundary at both grant creation and dispatch (including restored children).
func delegatedCapability(name string, d capabilities.Descriptor) bool {
	if d.Name != name || d.Source != "builtin" || d.Namespace != "tools" {
		return false
	}
	switch name {
	case "calculator", "current_time":
		return d.Kind == capabilities.Read && d.Risk == capabilities.RiskLow
	case "read_file", "list_files":
		return d.Kind == capabilities.Read && d.Risk == capabilities.RiskMedium
	case "http_get":
		// Governed network access is not proof of a side-effect-free outcome.
		return d.Kind == capabilities.Network && d.Risk == capabilities.RiskMedium
	}
	return false
}

func delegationTool() api.Tool {
	item := map[string]interface{}{
		"type": "object", "additionalProperties": false,
		"required": []string{"key", "goal", "depends_on", "tools"},
		"properties": map[string]interface{}{
			"key": map[string]string{"type": "string"}, "goal": map[string]string{"type": "string"},
			"depends_on": map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
			"tools":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string", "enum": []string{"calculator", "current_time", "read_file", "list_files", "http_get"}}},
		},
	}
	parameters := map[string]interface{}{"type": "object", "additionalProperties": false, "required": []string{"tasks"}, "properties": map[string]interface{}{"tasks": map[string]interface{}{"type": "array", "minItems": 1, "maxItems": 4, "items": item}}}
	return api.Tool{Type: "function", Function: api.FunctionDef{Name: "delegate_tasks", Description: "Delegate 1–4 bounded analysis/read-only subtasks with isolated contexts to durable child jobs. Each goal must contain its required context. Dependencies run in order; independent tasks share bounded inference admission. Children cannot control the computer, write files, run code or delegate. Use only for useful decomposition. Tool permissions never widen; blocked children need user attention.", Parameters: parameters}}
}

func validateChildSpecs(specs []ChildSpec) error {
	if len(specs) < 1 || len(specs) > 4 {
		return fmt.Errorf("Delegate between one and four tasks")
	}
	byKey := map[string]ChildSpec{}
	for _, s := range specs {
		if !validTaskID(s.Key) || len(s.Key) > 64 || strings.TrimSpace(s.Goal) == "" || len(s.Goal) > 8000 || len(s.Tools) > 5 || len(s.DependsOn) > 3 {
			return fmt.Errorf("Invalid delegated task")
		}
		if _, ok := byKey[s.Key]; ok {
			return fmt.Errorf("Duplicate task key")
		}
		byKey[s.Key] = s
		for _, name := range s.Tools {
			if !containsTool([]string{"calculator", "current_time", "read_file", "list_files", "http_get"}, name) {
				return fmt.Errorf("Delegated tasks are limited to governed read-only tools")
			}
		}
	}
	marks := map[string]int{}
	var visit func(string) error
	visit = func(key string) error {
		spec, ok := byKey[key]
		if !ok {
			return fmt.Errorf("Unknown task dependency")
		}
		if marks[key] == 1 {
			return fmt.Errorf("Task dependencies contain a cycle")
		}
		if marks[key] == 2 {
			return nil
		}
		marks[key] = 1
		for _, next := range spec.DependsOn {
			if err := visit(next); err != nil {
				return err
			}
		}
		marks[key] = 2
		return nil
	}
	for key := range byKey {
		if err := visit(key); err != nil {
			return err
		}
	}
	return nil
}
