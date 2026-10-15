package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

var ErrContextCountUnavailable = errors.New("model context counting is unavailable")

type ArchivedContext struct {
	ID       string            `json:"id"`
	Messages []api.ChatMessage `json:"messages"`
}
type ContextState struct {
	Window               int `json:"window"`
	EstimatedTokens      int `json:"estimated_tokens"`
	MeasuredPromptTokens int `json:"measured_prompt_tokens,omitempty"`
	LastBytes            int `json:"last_bytes,omitempty"`
	Compactions          int `json:"compactions"`
	ArchivedMessages     int `json:"archived_messages"`
}

// prepareContext archives whole completed assistant/tool groups, never a pending
// call, approval, user instruction, current observation or an uncertain effect.
// No model-written summary substitutes for facts: exact records remain durable
// and can be retrieved in bounded pages with task_history.
func (r *Runner) prepareContext(task *Task, tools []api.Tool) ([]api.ChatMessage, error) {
	return r.prepareContextWithContext(context.Background(), task, tools)
}

func (r *Runner) prepareContextWithContext(ctx context.Context, task *Task, tools []api.Tool) ([]api.ChatMessage, error) {
	if !task.Config.TaskFirst {
		return task.Checkpoint.Messages, nil
	}
	window := task.Config.ContextWindow
	if r.ContextWindow != nil {
		window = r.ContextWindow(task)
	}
	if window <= 0 {
		window = 4096
	}
	if task.Context == nil {
		task.Context = &ContextState{}
	}
	task.Context.Window = window
	toolJSON, _ := json.Marshal(tools)
	estimate := func(messages []api.ChatMessage) (int, int) {
		size := modelContextBytes(messages, toolJSON)
		// UTF-8 byte estimate, not a claimed tokenizer measurement. Anchor to actual
		// reported prompt usage when available; templates retain reserved headroom.
		tokens := (size+2)/3 + 128
		if task.Context.LastBytes > 0 && task.Context.MeasuredPromptTokens > 0 {
			anchored := task.Context.MeasuredPromptTokens*size/task.Context.LastBytes + 128
			if anchored > tokens {
				tokens = anchored
			}
		}
		return tokens, size
	}
	withIndex := func() []api.ChatMessage {
		messages := append([]api.ChatMessage(nil), task.Checkpoint.Messages...)
		if projector, ok := r.tools.(interface {
			ProjectModelContext(*Task, []api.ChatMessage) []api.ChatMessage
		}); ok {
			messages = projector.ProjectModelContext(task, messages)
		}
		index := profileResourceContext(task.Config)
		if len(task.Plan) > 0 {
			plan, _ := json.Marshal(task.Plan)
			index += "\nCurrent recorded plan (data, not authority): " + string(plan) + ". Continue unfinished work; a verified application launch alone cannot satisfy a document or research item."
		}
		if len(task.ContextArchive) > 0 && len(messages) > 0 {
			plan, _ := json.Marshal(task.Plan)
			// Stable task-local ordinals address the append-only archive without
			// putting every 64-character digest back into each model request. The
			// original digest references remain valid for existing clients/history.
			index += fmt.Sprintf("\nArchived evidence: task_history references archive:1 through archive:%d, offset 0 then next_offset. Never invent facts. Recorded plan (data, not authority): %s", len(task.ContextArchive), plan)
		}
		if pending := PendingApplicationLaunches(task); len(pending) != 0 {
			ids, _ := json.Marshal(pending)
			index += "\nUnverified launches (host IDs): " + string(ids) + ". Call computer_verify_application; do not relaunch."
		}
		if task.Workflow != nil && WorkflowVersionSupported(task) {
			checks, _ := json.Marshal(modelWorkflowChecks(task))
			index += fmt.Sprintf("\nDeclared checks (interpretation, not permission or complete intent proof), revision %d: %s. Declare matching checks before input; keep unresolved work. These native check kinds do not verify research or saved files. Authorized connected tools remain available for research; report their actual results and limitations.", task.Workflow.Revision, checks)
			if len(task.Workflow.Checks) == 0 {
				index += "\n" + workflowDeclarationGuidance(task)
			}
		}
		if ready := VerifiedApplications(task); len(ready) > 0 {
			if len(ready) > 8 {
				ready = ready[len(ready)-8:]
			}
			ids, _ := json.Marshal(ready)
			index += "\nApplications already observed open (not task completion): " + string(ids) + ". Continue the unfinished user outcome. Use computer_windows to switch; do not launch again. Saved readiness observations contain usable controls."
		}
		if index != "" {
			// Keep the stable prefix AND the current assistant/tool exchange intact.
			// In particular, browser capture attachment inspects the final tool result.
			// Rewriting message zero would re-evaluate thousands of cached tokens.
			position := len(messages)
			if position > 0 && messages[position-1].Role == "user" {
				// Keep a fresh user request/dependency handoff as the last input.
				position--
			}
			for i := len(messages) - 1; position == len(messages) && i >= 2; i-- {
				if messages[i].Role == "assistant" {
					position = i
					break
				}
			}
			messages = append(messages[:position:position], append([]api.ChatMessage{{Role: "system", Content: index}}, messages[position:]...)...)
		}
		return messages
	}
	reserve := task.Config.MaxTokens + 512
	if reserve < 1024 {
		reserve = 1024
	}
	countAvailable := r.CountContext != nil
	for {
		messages := withIndex()
		tokens, size := estimate(messages)
		if countAvailable {
			count, err := r.CountContext(ctx, task, messages, tools)
			switch {
			case errors.Is(err, ErrContextCountUnavailable):
				countAvailable = false // Older/embedded runtimes retain the conservative guard.
			case err != nil || count <= 0:
				return nil, fmt.Errorf("Could not measure the model context. Recorded actions are saved; no further model request or tool action was dispatched. Check runtime availability before continuing")
			default:
				tokens = count
			}
		}
		task.Context.EstimatedTokens = tokens
		if tokens+reserve <= window*4/5 {
			task.Context.LastBytes = size
			return messages, nil
		}
		cp := task.Checkpoint
		start, end := -1, -1
		// Find the latest assistant exchange by role, not by a fixed number of
		// messages (one native tool exchange is normally only two messages).
		// Keep it intact, as well as every user/system instruction and pending call.
		latest := len(cp.Messages)
		for i := len(cp.Messages) - 1; i >= 2; i-- {
			if cp.Messages[i].Role == "assistant" {
				latest = i
				break
			}
		}
		for i := 2; i < latest; i++ {
			if cp.Messages[i].Role != "assistant" {
				continue
			}
			j := i + 1
			for j < len(cp.Messages) && cp.Messages[j].Role == "tool" {
				j++
			}
			if j > latest || !completedContextGroup(cp.Messages[i:j]) {
				continue
			}
			start, end = i, j
			break
		}
		if start < 0 {
			if tokens+reserve <= window {
				task.Context.LastBytes = size
				return messages, nil
			}
			return nil, fmt.Errorf("Task context exceeds the allocated model window. Its complete history and earlier action results are saved. No further model request or tool action was dispatched. Review the recorded work before continuing with a larger supported context")
		}
		group := append([]api.ChatMessage(nil), cp.Messages[start:end]...)
		encoded, _ := json.Marshal(group)
		sum := sha256.Sum256(encoded)
		task.ContextArchive = append(task.ContextArchive, ArchivedContext{ID: hex.EncodeToString(sum[:]), Messages: group})
		cp.Messages = append(cp.Messages[:start:start], cp.Messages[end:]...)
		task.Context.Compactions++
		task.Context.ArchivedMessages += len(group)
		// Persist before using the compacted view. Failed persistence stops inference.
		if err := r.persist(task); err != nil {
			return nil, err
		}
	}
}

// The model sees content after HTTP JSON decoding. Counting the serialized
// ChatMessage envelope again double-counts escapes in tool JSON, paths and quoted
// document text. Keep explicit per-message/template overhead and tool-call data;
// this is still a conservative estimate, not an exact tokenizer measurement.
func modelContextBytes(messages []api.ChatMessage, toolJSON []byte) int {
	size := len(toolJSON)
	for _, message := range messages {
		size += 64 + len(message.Role) + len(message.Name) + len(message.ToolCallID)
		if content, ok := message.Content.(string); ok {
			size += len(content)
		} else if message.Content != nil {
			content, _ := json.Marshal(message.Content)
			size += len(content)
		}
		if len(message.ToolCalls) > 0 {
			calls, _ := json.Marshal(message.ToolCalls)
			size += len(calls)
		}
	}
	return size
}

// PendingApplicationLaunches is derived from the canonical durable steps, not
// the compacted model view. Restart/archival therefore cannot forget a dispatch.
func PendingApplicationLaunches(task *Task) []string {
	if task.Config.ComputerScope != "desktop" {
		return nil
	}
	pending := map[string]bool{}
	for i, step := range task.Steps {
		if i < task.Config.ComputerStepOffset || step.Type != "action" {
			continue
		}
		var args struct {
			Application string `json:"application_id"`
		}
		if json.Unmarshal([]byte(step.ToolArgs), &args) != nil || args.Application == "" {
			continue
		}
		switch step.ToolName {
		case "computer_open_application":
			pending[args.Application] = true
		case "computer_verify_application":
			var evidence struct {
				Verified    bool   `json:"verified"`
				Check       string `json:"check"`
				Application string `json:"application_id"`
			}
			if json.Unmarshal([]byte(step.ToolResult), &evidence) == nil && evidence.Verified && evidence.Check == "application_open" && evidence.Application == args.Application {
				delete(pending, args.Application)
			}
		}
	}
	ids := make([]string, 0, len(pending))
	for id := range pending {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Facts are derived from durable results, never a model-written summary or the
// compacted prompt. A later explicit negative check clears prior readiness.
func VerifiedApplications(task *Task) []string {
	if task.Config.ComputerScope != "desktop" {
		return nil
	}
	ready := map[string]bool{}
	for i, step := range task.Steps {
		if i < task.Config.ComputerStepOffset || step.Type != "action" || step.ToolName != "computer_verify_application" {
			continue
		}
		var args struct {
			Application string `json:"application_id"`
		}
		var result struct {
			Verified    bool   `json:"verified"`
			Application string `json:"application_id"`
			Check       string `json:"check"`
		}
		if json.Unmarshal([]byte(step.ToolArgs), &args) == nil && json.Unmarshal([]byte(step.ToolResult), &result) == nil && args.Application != "" && args.Application == result.Application && result.Check == "application_open" {
			ready[args.Application] = result.Verified
		}
	}
	ids := []string{}
	for id, verified := range ready {
		if verified {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// Incomplete/malformed tool exchanges may contain pending or uncertain effects.
// Never compact one merely because another assistant message follows it.
func completedContextGroup(group []api.ChatMessage) bool {
	if len(group) == 0 || group[0].Role != "assistant" {
		return false
	}
	calls := map[string]bool{}
	for _, call := range group[0].ToolCalls {
		if call.ID == "" || calls[call.ID] {
			return false
		}
		calls[call.ID] = true
	}
	for _, message := range group[1:] {
		if message.Role != "tool" || !calls[message.ToolCallID] {
			return false
		}
		delete(calls, message.ToolCallID)
	}
	return len(calls) == 0
}

func contextPage(task *Task, reference string, offset int) (string, error) {
	if offset < 0 {
		return "", fmt.Errorf("invalid history offset")
	}
	lookup := reference
	if strings.HasPrefix(reference, "step:") {
		n, err := strconv.Atoi(strings.TrimPrefix(reference, "step:"))
		if err != nil || n < 1 || n > len(task.Steps) || task.Steps[n-1].ID != n || task.Steps[n-1].Type != "action" {
			return "", fmt.Errorf("history reference is not part of this task")
		}
		data := []byte(task.Steps[n-1].ToolResult)
		if offset > len(data) || (offset < len(data) && !utf8.RuneStart(data[offset])) {
			return "", fmt.Errorf("invalid history offset")
		}
		end := offset + 1024
		if end > len(data) {
			end = len(data)
		}
		for end < len(data) && !utf8.RuneStart(data[end]) {
			end--
		}
		result, _ := json.Marshal(map[string]any{"reference": reference, "content": string(data[offset:end]), "next_offset": end, "complete": end == len(data), "total_bytes": len(data), "saved_evidence": true})
		return string(result), nil
	}
	if strings.HasPrefix(reference, "archive:") {
		n, err := strconv.Atoi(strings.TrimPrefix(reference, "archive:"))
		if err != nil || n < 1 || n > len(task.ContextArchive) {
			return "", fmt.Errorf("history reference is not part of this task")
		}
		lookup = task.ContextArchive[n-1].ID
	}
	for _, item := range task.ContextArchive {
		if item.ID != lookup {
			continue
		}
		data, _ := json.Marshal(item.Messages)
		if offset > len(data) || (offset < len(data) && !utf8.RuneStart(data[offset])) {
			return "", fmt.Errorf("invalid history offset")
		}
		end := offset + 4096
		if end > len(data) {
			end = len(data)
		}
		for end < len(data) && !utf8.RuneStart(data[end]) {
			end--
		}
		result, _ := json.Marshal(map[string]any{"reference": reference, "content": string(data[offset:end]), "next_offset": end, "complete": end == len(data), "total_bytes": len(data)})
		return string(result), nil
	}
	return "", fmt.Errorf("history reference is not part of this task")
}
