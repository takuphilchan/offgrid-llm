package server

import (
	"context"
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) countAgentContext(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (int, error) {
	if task.Config.TextOnlyInference {
		// A llama tokenizer cannot count a pinned MLX/Transformers prompt.
		// The runner retains its conservative UTF-8 estimate and headroom.
		return 0, agents.ErrContextCountUnavailable
	}
	counter, ok := s.engine.(inference.PromptTokenCounter)
	// Browser captures attach transient image data later. Do not misrepresent a
	// text-only count as their complete multimodal budget.
	if !ok || (task.Config.ComputerSession != "" && !nativeComputerDriver(task.Config.ComputerDriver)) {
		return 0, agents.ErrContextCountUnavailable
	}
	lease, err := s.acquireInferenceLease(ctx, task.Model, s.effectiveContextWindow(), nil)
	if err != nil {
		return 0, err
	}
	defer lease.Release()
	temperature, maxTokens := float32(task.Config.Temperature), task.Config.MaxTokens
	request := &api.ChatCompletionRequest{Model: task.Model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &temperature, MaxTokens: &maxTokens}
	configureComputerRequest(request, task)
	var count int
	err = lease.Run(ctx, func(admitted context.Context) error {
		var countErr error
		count, countErr = counter.CountPromptTokens(admitted, request)
		return countErr
	})
	if errors.Is(err, inference.ErrPromptCountUnavailable) {
		return 0, agents.ErrContextCountUnavailable
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}
