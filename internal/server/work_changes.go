package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type workChangeReference struct {
	view       api.WorkChange
	source     execution.Binding
	digest     string
	validation execution.ValidationSummary
}

func workChangeReferences(workID string, root *agents.Task) []workChangeReference {
	refs := []workChangeReference{}
	for _, record := range agents.TaskChangeRecords(root) {
		view := record.WorkChange
		view.Current = view.InstructionVersion == root.InstructionVersion && view.ControlEpoch == root.ControlEpoch
		refs = append(refs, workChangeReference{view: view, source: record.Source, digest: record.OperationsDigest, validation: record.Validation})
	}
	return refs
}
func (s *Server) authorizeWorkChange(ctx context.Context, actor string, ref workChangeReference) error {
	if ref.source.Actor != actor || ref.source.Workspace != s.workspaceID || s.executionHosts == nil {
		return execution.ErrAuthority
	}
	d, _ := execution.Descriptor("workspace_read")
	if !s.profileActorCapabilityAllowed(actor, d) {
		return execution.ErrAuthority
	}
	_, err := s.executionHosts.Resolve(ctx, actor, s.workspaceID, execution.Reference{Host: ref.source.Host, Session: ref.source.Session, Grant: ref.source.Grant, Revision: ref.source.Revision})
	return err
}

func boundedChangeInteger(r *http.Request, key string, fallback, max int64) (int64, bool) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, true
	}
	n, err := strconv.ParseInt(value, 10, 64)
	return n, err == nil && n >= 0 && n <= max && (key != "limit" || n > 0)
}

func (s *Server) handleWorkChanges(w http.ResponseWriter, r *http.Request, workID string, root *agents.Task, parts []string) {
	w.Header().Set("Cache-Control", "no-store")
	refs := workChangeReferences(workID, root)
	for i := range refs {
		stored, err := s.agentManager.ChangeSetRecord(s.controlScope(r), refs[i].view.ID)
		if err != nil {
			writeControlError(w, err)
			return
		}
		if stored.WorkID != workID || stored.RunID != refs[i].view.RunID || stored.OperationsDigest != refs[i].digest {
			writeControlError(w, agents.ErrRunStorage)
			return
		}
		stored.Current = stored.InstructionVersion == root.InstructionVersion && stored.ControlEpoch == root.ControlEpoch
		refs[i].view, refs[i].validation = stored.WorkChange, stored.Validation
	}
	if len(parts) == 0 {
		limit, valid := boundedChangeInteger(r, "limit", 32, 64)
		if !valid {
			writeJobError(w, 400, "invalid_change_page", "Use a limit between 1 and 64.", false)
			return
		}
		page := api.WorkChangeList{Items: []api.WorkChange{}}
		after := r.URL.Query().Get("after")
		for _, ref := range refs {
			if ref.view.ID <= after {
				continue
			}
			if len(page.Items) == int(limit) {
				page.HasMore = true
				break
			}
			ref.view.Available = s.authorizeWorkChange(r.Context(), root.Actor, ref) == nil
			if !ref.view.Available {
				ref.view.Limitation = "source_unavailable"
			}
			page.Items = append(page.Items, ref.view)
			page.NextCursor = ref.view.ID
		}
		writeJSON(w, 200, page)
		return
	}
	if len(parts) != 1 {
		writeControlError(w, agents.ErrControlNotFound)
		return
	}
	for _, ref := range refs {
		if ref.view.ID != parts[0] {
			continue
		}
		if err := s.authorizeWorkChange(r.Context(), root.Actor, ref); err != nil {
			writeJobError(w, 403, "change_source_unavailable", "The original host access is unavailable. Saved change status remains visible; this read cannot restore access.", false)
			return
		}
		ref.view.Available = true
		s.readWorkChange(w, r, ref)
		return
	}
	writeControlError(w, agents.ErrControlNotFound)
}
