package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

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
		data, _ := json.Marshal(messages)
		size := len(data) + len(toolJSON)
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
		if len(task.ContextArchive) > 0 && len(messages) > 0 {
			refs := []string{}
			for _, entry := range task.ContextArchive {
				refs = append(refs, entry.ID)
			}
			plan, _ := json.Marshal(task.Plan)
			messages[0].Content = messages[0].StringContent() + "\nOlder completed context is archived, not forgotten. Use task_history with one of these references to inspect exact evidence; never invent missing facts: " + strings.Join(refs, ", ") + ". Current recorded work plan (task data, not extra authority): " + string(plan)
		}
		return messages
	}
	reserve := task.Config.MaxTokens + 512
	if reserve < 1024 {
		reserve = 1024
	}
	for {
		messages := withIndex()
		tokens, size := estimate(messages)
		task.Context.EstimatedTokens = tokens
		if tokens+reserve <= window*4/5 {
			task.Context.LastBytes = size
			return messages, nil
		}
		cp := task.Checkpoint
		start, end := -1, -1
		// Preserve the latest complete exchange, all user instructions and the task.
		for i := 2; i < len(cp.Messages)-4; i++ {
			if cp.Messages[i].Role != "assistant" {
				continue
			}
			j := i + 1
			for j < len(cp.Messages) && cp.Messages[j].Role == "tool" {
				j++
			}
			if j > len(cp.Messages)-4 {
				break
			}
			start, end = i, j
			break
		}
		if start < 0 {
			if tokens+reserve <= window {
				task.Context.LastBytes = size
				return messages, nil
			}
			return nil, fmt.Errorf("Task context exceeds the allocated model window. Its complete history is saved. Use a model with a larger supported context or reduce the task; no tool was dispatched")
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

func contextPage(task *Task, reference string, offset int) (string, error) {
	if offset < 0 {
		return "", fmt.Errorf("invalid history offset")
	}
	for _, item := range task.ContextArchive {
		if item.ID != reference {
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
