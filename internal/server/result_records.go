package server

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleResultRecord(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		writeJobError(w, 405, "method_not_allowed", "Use GET to retrieve saved output.", false)
		return
	}
	if s.agentManager == nil {
		writeControlError(w, agents.ErrRunStorage)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v2/results/")
	result, err := s.agentManager.ResultRecord(s.controlScope(r), id)
	if err != nil {
		writeControlError(w, err)
		return
	}
	task, found := s.agentManager.GetTask(result.RunID)
	if !found || task.Actor != s.agentActor(r) || task.DeletedAt != nil {
		writeControlError(w, agents.ErrControlNotFound)
		return
	}
	// Keep the captured dependencies even if the run later lost/rebuilt context.
	task.Config.KnowledgeDependencies = append(task.Config.KnowledgeDependencies, result.SourceDependencies...)
	if !s.taskSourceAllowed(task) {
		writeControlError(w, agents.ErrSourceAccess)
		return
	}
	offset, valid := boundedChangeInteger(r, "offset", 0, int64(len(result.Content)))
	limit, bounded := boundedChangeInteger(r, "limit", 4096, 65536)
	if !valid || !bounded || offset < int64(len(result.Content)) && !utf8.RuneStart(result.Content[int(offset)]) {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	end := min(int(offset+limit), len(result.Content))
	for end < len(result.Content) && !utf8.RuneStart(result.Content[end]) {
		end--
	}
	// A tiny byte limit must still advance over one complete UTF-8 code point.
	if end == int(offset) && end < len(result.Content) {
		_, size := utf8.DecodeRuneInString(result.Content[end:])
		end += size
	}
	writeJSON(w, 200, api.ResultPage{ID: result.ID, RunID: result.RunID, WorkID: result.WorkID, WorkerID: result.WorkerID, ConversationID: result.ConversationID, InstructionVersion: result.InstructionVersion, ControlEpoch: result.ControlEpoch, Status: result.Status, IndependentlyVerified: result.IndependentlyVerified, EvidenceSteps: result.EvidenceSteps, Content: result.Content[int(offset):end], Offset: int(offset), NextOffset: end, TotalBytes: len(result.Content), Complete: end == len(result.Content)})
}
