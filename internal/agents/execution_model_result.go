package agents

import (
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func executionModelResult(task *Task, tool string, descriptor capabilities.Descriptor, result string, step int) string {
	expected, ok := execution.Descriptor(tool)
	if !task.Config.TaskFirst || task.Config.Execution == nil || !ok || descriptor != expected {
		return result
	}
	operation := execution.Operation(tool)
	result = execution.ModelReceipt(*task.Config.Execution, operation, result, step)
	if pin := task.Config.ToolAdapter; pin != nil && pin.Adapter == inference.NativeAgentToolAdapter {
		if operation == "file_read" {
			return execution.FileModelReceipt(*task.Config.Execution, result, step)
		}
		if tool == execution.InputWriteTool {
			var view struct {
				Source execution.InputProof `json:"source_input"`
			}
			if json.Unmarshal([]byte(result), &view) == nil {
				for _, id := range InputWriteSources(task) {
					if id != view.Source.Input {
						continue
					}
					if id == "request" {
						return execution.InputWriteModelReceipt(*task.Config.Execution, result, task.Prompt)
					}
					for _, entry := range task.Instructions {
						if id == "correction:"+entry.RequestID {
							return execution.InputWriteModelReceipt(*task.Config.Execution, result, entry.Text)
						}
					}
				}
			}
		}
		return execution.ReadableModelReceipt(*task.Config.Execution, operation, result)
	}
	return result
}
