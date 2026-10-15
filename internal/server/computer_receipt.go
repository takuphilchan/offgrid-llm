package server

import (
	"encoding/json"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func (s *Server) recordComputerReply(actor, session, run string, action computer.BrowserAction, reply computer.BrowserReply) error {
	effect, known := capabilities.NativeOperationEffect(action.Kind, capabilities.NativeEffectsRevision)
	if !known || effect != capabilities.NativeChange || reply.Error != "" {
		return nil
	}
	if !recoverableComputerResult(reply.Result) {
		return computer.ErrUncertain
	}
	prefix := run + ":"
	if run == "" || !strings.HasPrefix(action.ID, prefix) || reply.ID != action.ID {
		return computer.ErrSession
	}
	return s.agentManager.RecordComputerReceipt(run, actor, session, strings.TrimPrefix(action.ID, prefix), action.Kind, action.Arguments, reply.Result)
}

func recoverableComputerResult(result string) bool {
	var document computer.DocumentActionEvidence
	if computer.DecodeControlMessage(json.RawMessage(result), &document) == nil && document.Validate() == nil {
		return true
	}
	var batch struct {
		Actions []computer.ControlResult `json:"actions"`
	}
	if computer.DecodeControlMessage(json.RawMessage(result), &batch) != nil || len(batch.Actions) != 1 {
		return false
	}
	action := batch.Actions[0]
	return action.ActionID != "" && !action.Uncertain && (action.Outcome == "verified" || action.Outcome == "dispatched") && (action.Document == nil || action.Document.Validate() == nil)
}
