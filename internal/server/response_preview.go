package server

import "github.com/takuphilchan/offgrid-llm/internal/agents"

// This presentation hint does not declare a draft verified or complete. Source
// workflows remain buffered until their existing identity/coverage checks pass.
// taskResponse is only delivered through the owner/source-checked job handlers.
func responsePreviewAllowed(task *agents.Task) bool {
	c := task.Config
	return task.Status == agents.TaskRunning && task.Progress != nil && task.Progress.Phase == "generating" &&
		c.ComputerSession == "" && c.RequestedComputerScope == "" && task.Workflow == nil && task.ParentID == "" &&
		len(c.InputAttachments) == 0 && c.InputEvidence == "" && len(c.InputImages) == 0 &&
		c.KnowledgeContext == "" && len(c.KnowledgeDocuments) == 0 && len(c.KnowledgeSources) == 0 && len(c.KnowledgeDependencies) == 0
}
