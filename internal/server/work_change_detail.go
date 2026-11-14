package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func workFileVersion(entry *execution.WorkingCopyEntry) *api.WorkFileVersion {
	if entry == nil {
		return nil
	}
	return &api.WorkFileVersion{Kind: entry.Kind, Bytes: entry.Bytes, Mode: entry.Mode, SHA256: entry.SHA256}
}

func (s *Server) readWorkChange(w http.ResponseWriter, r *http.Request, ref workChangeReference) {
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "files"
	}
	maximum, defaultLimit := int64(64), int64(32)
	if view == "content" || view == "preserved" {
		maximum, defaultLimit = 65536, 65536
	}
	offset, valid := boundedChangeInteger(r, "offset", 0, 1<<53-1)
	limit, validLimit := boundedChangeInteger(r, "limit", defaultLimit, maximum)
	if !valid || !validLimit {
		writeJobError(w, 400, "invalid_change_page", "Use a nonnegative offset and a bounded page limit.", false)
		return
	}
	request := execution.WorkerArtifactRequest{Source: ref.source, ChangeSetID: ref.view.ID, Offset: offset, Limit: int(limit)}
	switch view {
	case "files":
		request.Operation = "inspect"
	case "validation":
		request.Operation = "validation"
	case "integration", "recovery", "preserved":
		if ref.view.IntegrationID == "" {
			writeJobError(w, 409, "change_not_integrated", "This change has no integration attempt.", false)
			return
		}
		request.Operation, request.IntegrationID, request.ChangeSetID = view, ref.view.IntegrationID, ""
		if view == "preserved" {
			request.Hash, request.ArtifactID = r.URL.Query().Get("hash"), r.URL.Query().Get("artifact")
		}
	case "content":
		request.Operation, request.Hash = "blob", r.URL.Query().Get("hash")
	default:
		writeJobError(w, 400, "invalid_change_view", "Use files, validation, integration, recovery, content or preserved.", false)
		return
	}
	host := execution.Reference{Host: ref.source.Host, Session: ref.source.Session, Grant: ref.source.Grant, Revision: ref.source.Revision}
	reply, err := s.executionHosts.WorkerArtifact(r.Context(), ref.source.Actor, s.workspaceID, host, request)
	if err != nil {
		writeJobError(w, 409, "change_evidence_unavailable", "The requested saved evidence is unavailable. No work was dispatched or repeated.", true)
		return
	}
	detail := api.WorkChangeDetail{Change: ref.view, View: view}
	next := func(value *int) {
		if value != nil {
			n := int64(*value)
			detail.NextOffset = &n
		}
	}
	switch view {
	case "files":
		p := reply.Page
		if p == nil || p.ChangeSet.ID != ref.view.ID || p.ChangeSet.Source != ref.source || p.ChangeSet.WorkerRunID != ref.view.WorkerRunID || p.ChangeSet.OperationsDigest != ref.digest || p.ChangeSet.InstructionVersion != ref.view.InstructionVersion || p.ChangeSet.ControlEpoch != ref.view.ControlEpoch {
			writeJobError(w, 409, "change_evidence_mismatch", "Saved evidence does not match this work's change reference.", false)
			return
		}
		// The immutable native manifest remains sealed after integration. Keep
		// the service's integration lifecycle when inspecting its file list.
		if ref.view.IntegrationID == "" {
			detail.Change.State = p.ChangeSet.State
		}
		for _, file := range p.Operations {
			detail.Files = append(detail.Files, api.WorkFileChange{Path: file.Path, Kind: file.Kind, Base: workFileVersion(file.Base), Result: workFileVersion(file.Result)})
		}
		next(p.NextOffset)
	case "validation":
		p := reply.Validation
		if p == nil || p.Summary != ref.validation {
			writeJobError(w, 409, "change_evidence_mismatch", "Validation evidence does not match the captured result.", false)
			return
		}
		for _, check := range p.Checks {
			detail.Checks = append(detail.Checks, api.WorkValidationCheck{ID: check.OperationID, State: check.State, ExitCode: check.ExitCode, RequestDigest: check.RequestDigest, ReceiptDigest: check.ReceiptDigest})
		}
		next(p.NextOffset)
	case "integration":
		p := reply.Integration
		if p == nil || p.ID != ref.view.IntegrationID || p.ChangeSetID != ref.view.ID {
			writeJobError(w, 409, "change_evidence_mismatch", "Integration evidence does not match the captured result.", false)
			return
		}
		detail.Change.State, detail.Change.IntegratedCount = p.State, p.Integrated
		detail.ConflictPath, detail.Error = p.ConflictPath, p.Error
		for _, receipt := range p.Files {
			retained := receipt.DisplacedDigest != "" || receipt.State == "integrated" && receipt.Operation == "delete" && receipt.Displaced != ""
			detail.Receipts = append(detail.Receipts, api.WorkFileReceipt{Ordinal: receipt.Ordinal, Path: receipt.Path, Operation: receipt.Operation, State: receipt.State, Dispatched: receipt.Dispatched, OriginalRetained: retained, DisplacedDigest: receipt.DisplacedDigest, ResultDigest: receipt.ResultDigest, Error: receipt.Error})
		}
		next(p.NextOffset)
	case "recovery":
		p := reply.Recovery
		if p == nil || p.IntegrationID != ref.view.IntegrationID || p.ChangeSetID != ref.view.ID {
			writeJobError(w, 409, "change_evidence_mismatch", "Recovery files do not match this integration attempt.", false)
			return
		}
		for _, file := range p.Files {
			detail.Recovery = append(detail.Recovery, api.WorkRecoveryFile{ID: file.ID, Path: file.Path, Role: file.Role, SHA256: file.SHA256, Bytes: file.Bytes, Limitation: file.Limitation})
		}
		next(p.NextOffset)
	case "content", "preserved":
		p := reply.Blob
		if p == nil || p.SHA256 != request.Hash || p.Offset != offset {
			writeJobError(w, 409, "change_evidence_mismatch", "Content does not match the requested sealed digest.", false)
			return
		}
		detail.Content = &api.WorkChangeContent{SHA256: p.SHA256, Bytes: p.Bytes, Offset: p.Offset, Data: p.Data}
		detail.NextOffset = p.NextOffset
	}
	writeJSON(w, 200, detail)
}
