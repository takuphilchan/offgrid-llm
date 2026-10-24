package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Protocol selection belongs to governed work, not public tool cardinality.
func configureComputerRequest(request *api.ChatCompletionRequest, task *agents.Task) {
	if task.Config.ComputerSession != "" || task.Config.TaskFirst {
		parallel := false
		request.ParallelToolCalls = &parallel
		request.SingleStepTools = true
	}
}
