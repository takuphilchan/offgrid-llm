package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"testing"
)

func TestResponsePreviewEligibility(t *testing.T) {
	for name, change := range map[string]func(*agents.Task){
		"completed":  func(task *agents.Task) { task.Status = agents.TaskCompleted },
		"tool":       func(task *agents.Task) { task.Progress.Phase = "preparing_tool" },
		"attachment": func(task *agents.Task) { task.Config.InputAttachments = []api.ConversationAttachment{{}} },
		"evidence":   func(task *agents.Task) { task.Config.InputEvidence = "retained evidence" },
		"knowledge":  func(task *agents.Task) { task.Config.KnowledgeContext = "source" },
		"computer":   func(task *agents.Task) { task.Config.ComputerSession = "session" },
		"workflow":   func(task *agents.Task) { task.Workflow = &agents.WorkflowEvidence{} },
	} {
		t.Run(name, func(t *testing.T) {
			task := &agents.Task{Status: agents.TaskRunning, Progress: &agents.RunProgress{Phase: "generating", Preview: "draft"}}
			if !responsePreviewAllowed(task) {
				t.Fatal("ordinary live response was buffered")
			}
			change(task)
			if taskResponse(task)["response_preview_allowed"] != false {
				t.Fatal("unsafe or settled preview advertised as live response")
			}
		})
	}
}
