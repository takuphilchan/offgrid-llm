package server

import (
	"context"
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) countAgentContext(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (int, error) {
	counter, ok := s.engine.(inference.PromptTokenCounter)
	// Browser captures attach transient image data later. Do not misrepresent a
	// text-only count as their complete multimodal budget.
	if !ok || (task.Config.ComputerSession != "" && !nativeComputerDriver(task.Config.ComputerDriver)) {
		return 0, agents.ErrContextCountUnavailable
	}
	release, err := s.acquireInferenceContext(ctx, task.Model, s.effectiveContextWindow(), nil)
	if err != nil {
		return 0, err
	}
	defer release()
	temperature, maxTokens := float32(task.Config.Temperature), task.Config.MaxTokens
	request := &api.ChatCompletionRequest{Model: task.Model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &temperature, MaxTokens: &maxTokens}
	configureComputerRequest(request, task)
	count, err := counter.CountPromptTokens(ctx, request)
	if errors.Is(err, inference.ErrPromptCountUnavailable) {
		return 0, agents.ErrContextCountUnavailable
	}
	return count, err
}
