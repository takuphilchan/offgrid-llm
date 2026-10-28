package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

func writeContextError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agents.ErrContextNotFound), errors.Is(err, agents.ErrProfileNotFound):
		writeJobError(w, 404, "context_not_found", "Owned context resource not found.", false)
	case errors.Is(err, agents.ErrContextConflict):
		writeJobError(w, 409, "context_revision_changed", "The entry, consent or operation changed. Inspect current state before trying again.", false)
	case errors.Is(err, agents.ErrMemoryConsent):
		writeJobError(w, 403, "memory_consent_unavailable", "Memory consent is disabled or changed. Existing tasks cannot acquire renewed consent.", false)
	case errors.Is(err, agentcontext.ErrSkill):
		writeJobError(w, 400, "invalid_skill", err.Error(), false)
	default:
		writeJobError(w, 503, "context_storage_unavailable", "Context storage is unavailable. Saved work was not replaced; inspect before retrying.", true)
	}
}

func contextBody(w http.ResponseWriter, r *http.Request, target any) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 768<<10))
	if err != nil || jsonwire.Validate(data) != nil || parseContextArgumentsLarge(data, target) != nil {
		writeJobError(w, 400, "invalid_context_request", "Provide one bounded JSON object with only the documented fields.", false)
		return false
	}
	return true
}

func parseContextArgumentsLarge(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return agentcontext.ErrSkill
	}
	return nil
}

func (s *Server) handleAgentSkills(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.agentManager == nil || s.workspaceID == "" {
		writeContextError(w, agents.ErrRunStorage)
		return
	}
	actor := s.agentActor(r)
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/agent-skills"), "/"), "/")
	id := parts[0]
	review := len(parts) == 2 && parts[1] == "review"
	if len(parts) > 2 || len(parts) == 2 && !review {
		writeContextError(w, agents.ErrContextNotFound)
		return
	}
	if r.Method == http.MethodGet {
		if id == "" {
			items, err := s.agentManager.ListSkills(s.workspaceID, actor)
			if err != nil {
				writeContextError(w, err)
				return
			}
			writeJSON(w, 200, map[string]any{"skills": items})
			return
		}
		if review {
			writeContextError(w, agents.ErrContextNotFound)
			return
		}
		revision, err := strconv.Atoi(r.URL.Query().Get("revision"))
		if err != nil {
			writeContextError(w, agentcontext.ErrSkill)
			return
		}
		item, bundle, err := s.agentManager.GetSkill(s.workspaceID, actor, agentcontext.SkillReference{ID: id, Revision: revision, Digest: r.URL.Query().Get("digest")})
		if err != nil {
			writeContextError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"skill": item, "bundle": bundle, "executable": false})
		return
	}
	if !(r.Method == http.MethodPost && (id == "" || review) || r.Method == http.MethodPut && id != "" && !review) {
		writeJobError(w, 405, "method_not_allowed", "Use GET, POST to import/review or PUT to add a revision.", false)
		return
	}
	var request struct {
		ExpectedRevision int                      `json:"expected_revision"`
		Digest           string                   `json:"digest,omitempty"`
		Files            []agentcontext.SkillFile `json:"files,omitempty"`
	}
	if !contextBody(w, r, &request) {
		return
	}
	if review {
		item, bundle, err := s.agentManager.GetSkill(s.workspaceID, actor, agentcontext.SkillReference{ID: id, Revision: request.ExpectedRevision, Digest: request.Digest})
		if err != nil {
			writeContextError(w, err)
			return
		}
		if !item.Draft || len(request.Files) != 0 {
			writeContextError(w, agentcontext.ErrSkill)
			return
		}
		request.Files = bundle.Files
		id = ""
		request.ExpectedRevision = 0
	}
	item, err := s.agentManager.ImportSkill(s.workspaceID, actor, id, request.ExpectedRevision, request.Files, "")
	if err != nil {
		writeContextError(w, err)
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) handleAgentMemory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.agentManager == nil || s.workspaceID == "" {
		writeContextError(w, agents.ErrRunStorage)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/agent-memory/"), "/"), "/")
	if len(parts) > 2 || parts[0] == "" {
		writeContextError(w, agents.ErrContextNotFound)
		return
	}
	profile, actor := parts[0], s.agentActor(r)
	if _, err := s.agentManager.GetMemoryConsent(s.workspaceID, actor, profile); err != nil {
		writeContextError(w, err)
		return
	}
	if len(parts) == 2 && parts[1] == "consent" && r.Method == http.MethodPut {
		var request struct {
			ExpectedRevision int   `json:"expected_revision"`
			Enabled          *bool `json:"enabled"`
		}
		if !contextBody(w, r, &request) {
			return
		}
		if request.Enabled == nil {
			writeJobError(w, 400, "invalid_context_request", "Explicit enabled true or false is required.", false)
			return
		}
		consent, err := s.agentManager.SetMemoryConsent(s.workspaceID, actor, profile, request.ExpectedRevision, *request.Enabled)
		if err != nil {
			writeContextError(w, err)
			return
		}
		writeJSON(w, 200, consent)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		consent, err := s.agentManager.GetMemoryConsent(s.workspaceID, actor, profile)
		if err != nil {
			writeContextError(w, err)
			return
		}
		entries, err := s.agentManager.ListMemory(s.workspaceID, actor, profile, 0)
		if err != nil {
			writeContextError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"consent": consent, "entries": entries, "deletion_scope": "Future retrieval only; historical task evidence is not erased."})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPost {
		var request struct {
			Operation        string `json:"operation"`
			ID               string `json:"id,omitempty"`
			ExpectedRevision int    `json:"expected_revision"`
			Label            string `json:"label"`
			Content          string `json:"content"`
		}
		if !contextBody(w, r, &request) {
			return
		}
		if strings.TrimSpace(request.Operation) == "" {
			writeJobError(w, 400, "invalid_context_request", "A stable nonempty operation identity is required.", false)
			return
		}
		entry, err := s.agentManager.WriteMemory(s.workspaceID, actor, profile, 0, agents.MemoryWrite{Operation: "user:" + request.Operation, ID: request.ID, ExpectedRevision: request.ExpectedRevision, Label: request.Label, Content: request.Content})
		if err != nil {
			writeContextError(w, err)
			return
		}
		writeJSON(w, 200, entry)
		return
	}
	if len(parts) == 2 && parts[1] != "consent" && r.Method == http.MethodDelete {
		revision, err := strconv.Atoi(r.URL.Query().Get("revision"))
		if err != nil {
			writeContextError(w, agents.ErrContextConflict)
			return
		}
		if err = s.agentManager.DeleteMemory(s.workspaceID, actor, profile, parts[1], revision); err != nil {
			writeContextError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"deleted": true, "historical_evidence_erased": false})
		return
	}
	writeJobError(w, 405, "method_not_allowed", "Use GET to inspect, PUT consent, POST entries or DELETE one revision.", false)
}
