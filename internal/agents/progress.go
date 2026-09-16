package agents

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// RunProgress is a bounded, provisional projection, never completed model
// context. It survives reconnection/restart without replaying any execution.
type RunProgress struct {
	Phase     string    `json:"phase"`
	Iteration int       `json:"iteration"`
	Tool      string    `json:"tool,omitempty"`
	Preview   string    `json:"preview"`
	Truncated bool      `json:"truncated"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RunStreamCaller func(context.Context, *Task, []api.ChatMessage, []api.Tool, func(phase, text string) error) (*api.ChatCompletionResponse, error)

func setRunPhase(task *Task, phase, tool string) {
	now := time.Now().UTC()
	if task.Progress == nil {
		task.Progress = &RunProgress{}
	}
	if task.Progress.Phase != phase {
		task.Progress.StartedAt = now
	}
	task.Progress.Phase, task.Progress.Tool, task.Progress.UpdatedAt = phase, tool, now
	task.Progress.Iteration = task.Checkpoint.Iteration + 1
	if phase == "tool" || phase == "approval" {
		task.Progress.Iteration = task.Checkpoint.Iteration
	}
}

func (r *Runner) saveProgress(task *Task) error {
	progress := *task.Progress
	_, err := r.manager.updateTask(task.ID, func(current *Task) error {
		if current.Status != TaskRunning {
			return context.Canceled
		}
		current.Progress = &progress
		return nil
	})
	// Do not append full previews to the event log or notify a network consumer
	// synchronously. Durable snapshots are the v1 progress/reconnection contract.
	return err
}

func (r *Runner) callWithProgress(ctx context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
	task.Progress = nil // each model turn gets its own explicitly provisional preview
	setRunPhase(task, "queued", "")
	if err := r.saveProgress(task); err != nil {
		return nil, err
	}
	lastSaved := time.Now()
	dirty := false
	emit := func(phase, text string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		changed := phase != task.Progress.Phase
		setRunPhase(task, phase, "")
		const limit = 64 * 1024
		if text != "" && !task.Progress.Truncated {
			room := limit - len(task.Progress.Preview)
			if len(text) > room {
				text = text[:room]
				for !utf8.ValidString(text) {
					text = text[:len(text)-1]
				}
				task.Progress.Truncated = true
			}
			task.Progress.Preview += text
		}
		dirty = true
		// First text and phase changes are immediate. Subsequent deltas coalesce
		// to four durable writes/second, not one fsync per token.
		if changed || time.Since(lastSaved) >= 250*time.Millisecond {
			if err := r.saveProgress(task); err != nil {
				return err
			}
			lastSaved, dirty = time.Now(), false
		}
		return nil
	}
	var response *api.ChatCompletionResponse
	var err error
	if r.StreamCaller != nil {
		response, err = r.StreamCaller(ctx, task, messages, tools, emit)
	} else {
		if err = emit("processing", ""); err == nil {
			response, err = r.caller(ctx, task, messages, tools)
		}
	}
	if dirty {
		if saveErr := r.saveProgress(task); saveErr != nil {
			err = errors.Join(err, saveErr)
		}
	}
	return response, err
}
