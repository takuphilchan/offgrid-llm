package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func (s *Server) submitModelOperation(w http.ResponseWriter, r *http.Request) {
	var req modelOperationRequest
	if !decodeModelRequest(w, r, &req) {
		return
	}
	if len(req.RequestID) < 8 || len(req.RequestID) > 128 || strings.ContainsAny(req.RequestID, "\x00\r\n") || (req.Action != "install" && req.Action != "repair") || (req.Action == "install" && (req.ResolutionID == "" || req.SourceOperationID != "")) || (req.Action == "repair" && (req.SourceOperationID == "" || req.ResolutionID != "")) {
		writeErrorWithCode(w, "Provide a request ID and an install preview or repair source operation.", 400, "invalid_model_request")
		return
	}
	actor, digest := modelActor(r), modelPayloadDigest(req)
	s.downloadMutex.Lock()
	if s.modelOperations == nil {
		s.modelOperations = map[string]*ModelOperation{}
	}
	for _, o := range s.modelOperations {
		if o.ActorID == actor && o.RequestID == req.RequestID {
			copy := cloneModelOperation(o)
			identical := copy.RequestDigest == digest
			// A fresh preview of the same immutable source is the same request.
			// This permits CLI retries without treating an ephemeral preview ID
			// as model identity. Expired previews only support byte-identical retry.
			if !identical && req.Action == "install" && o.Action == "install" {
				if preview, ok := s.modelResolutions[req.ResolutionID]; ok && preview.Actor == actor && time.Now().Before(preview.ExpiresAt) {
					identical = models.ManifestDigest(preview.Resolution.Manifest) == models.ManifestDigest(*o.Target.Package) && preview.Resolution.Provenance == o.Provenance
				}
			}
			s.downloadMutex.Unlock()
			if !identical {
				writeErrorWithCode(w, "This request ID was already used for different model work.", 409, "request_conflict")
				return
			}
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(copy)
			return
		}
	}
	var resolution models.PackageResolution
	if req.Action == "install" {
		preview, ok := s.modelResolutions[req.ResolutionID]
		if !ok || preview.Actor != actor || time.Now().After(preview.ExpiresAt) {
			s.downloadMutex.Unlock()
			writeErrorWithCode(w, "This preview expired or belongs to another user. Preview the model again.", 409, "resolution_expired")
			return
		}
		resolution = preview.Resolution
	} else {
		original := s.modelOperations[req.SourceOperationID]
		if !s.canReadModelOperation(r, original) || original.State != "complete" || original.Discarded {
			s.downloadMutex.Unlock()
			writeErrorWithCode(w, "Repair requires a completed source-bound installation. Re-import local packages without that receipt.", 422, "repair_source_unavailable")
			return
		}
		resolution = models.PackageResolution{Manifest: *original.Target.Package, Provenance: original.Provenance}
	}
	m := resolution.Manifest
	active := 0
	var reserved int64
	for _, o := range s.modelOperations {
		if o.Target.Package != nil && o.Target.Package.ID == m.ID && o.Target.Package.Revision == m.Revision && (o.active() || (!o.Discarded && o.State != "complete")) {
			s.downloadMutex.Unlock()
			writeErrorWithCode(w, "This package has an existing operation. Resume or explicitly discard its retained data first.", 409, "package_operation_conflict")
			return
		}
		if o.active() {
			active++
			reserved += o.BytesTotal*2 + (16 << 20)
		}
	}
	if active >= 4 || len(s.modelOperations) >= 1000 {
		s.downloadMutex.Unlock()
		writeErrorWithCode(w, "The model-operation capacity is full. Wait for active transfers to settle.", 429, "model_operation_limit")
		return
	}
	previous := int64(0)
	if req.Action == "repair" {
		for _, a := range m.Artifacts {
			previous += a.Size
		}
	}
	preflight, err := s.registry.Packages().Preflight(m, previous)
	if err == nil && preflight.AvailableBytes-reserved < preflight.RequiredFreeBytes {
		err = &models.ModelAcquisitionError{Code: "insufficient_space", Message: "Other active downloads reserve the remaining disk space. Wait for them to finish."}
	}
	if err != nil {
		s.downloadMutex.Unlock()
		modelHTTPError(w, err)
		return
	}
	now := time.Now().UTC()
	o := &ModelOperation{ID: "model-" + strings.ReplaceAll(uuid.NewString(), "-", ""), ActorID: actor, RequestID: req.RequestID, RequestDigest: digest, Action: req.Action, Target: models.InstallationTarget{Kind: "package", Package: &m}, Provenance: resolution.Provenance, State: "queued", CreatedAt: now, UpdatedAt: now, Artifacts: []ModelArtifactProgress{}}
	for _, a := range m.Artifacts {
		o.Artifacts = append(o.Artifacts, ModelArtifactProgress{Path: a.Path, BytesTotal: a.Size})
		o.BytesTotal += a.Size
	}
	if err = o.validate(); err != nil {
		s.downloadMutex.Unlock()
		modelHTTPError(w, err)
		return
	}
	s.modelOperations[o.ID] = o
	if err = s.saveDownloadsLocked(); err != nil {
		delete(s.modelOperations, o.ID)
		s.downloadMutex.Unlock()
		modelHTTPError(w, err)
		return
	}
	copy := cloneModelOperation(o)
	s.launchModelOperationLocked(o.ID)
	s.downloadMutex.Unlock()
	w.Header().Set("Location", "/api/v2/models/operations/"+copy.ID)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(copy)
}

func (s *Server) launchModelOperationLocked(id string) {
	ctx := s.runtimeCtx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	if s.downloadCancelFuncs == nil {
		s.downloadCancelFuncs = map[string]context.CancelFunc{}
	}
	s.downloadCancelFuncs["package:"+id] = cancel
	s.downloadWorkers.Add(1)
	go func() { defer s.downloadWorkers.Done(); defer cancel(); s.runModelOperation(ctx, id) }()
}

func (s *Server) runModelOperation(ctx context.Context, id string) {
	s.downloadMutex.RLock()
	initial := cloneModelOperation(s.modelOperations[id])
	s.downloadMutex.RUnlock()
	if initial == nil {
		return
	}
	checkpoint := func(p models.PackageCheckpoint) error {
		s.downloadMutex.Lock()
		defer s.downloadMutex.Unlock()
		o := s.modelOperations[id]
		if o == nil {
			return fmt.Errorf("model operation disappeared")
		}
		if ctx.Err() != nil || o.State == "cancelling" {
			return context.Canceled
		}
		o.State = p.Phase
		o.UpdatedAt = time.Now().UTC()
		o.ErrorCode = ""
		o.Message = ""
		if p.Artifact >= 0 {
			if p.Artifact >= len(o.Artifacts) || p.BytesDone < 0 || p.BytesDone > o.Artifacts[p.Artifact].BytesTotal {
				return fmt.Errorf("invalid artifact checkpoint")
			}
			o.Artifacts[p.Artifact].BytesDone = p.BytesDone
			o.Artifacts[p.Artifact].Verified = p.Verified
			o.BytesDone = 0
			for _, a := range o.Artifacts {
				o.BytesDone += a.BytesDone
			}
			o.RetainedBytes = o.BytesDone
		}
		if p.Phase == "activating" {
			o.ActivationIntent = true
		}
		return s.saveDownloadsLocked()
	}
	_, err := s.registry.Packages().AcquireFromHub(ctx, *initial.Target.Package, id, initial.Action == "repair", initial.Provenance, s.hubForModels(), checkpoint)
	retained, retainedErr := s.registry.Packages().RetainedAcquisitionBytes(id)
	s.downloadMutex.Lock()
	o := s.modelOperations[id]
	o.UpdatedAt = time.Now().UTC()
	if retainedErr == nil {
		o.RetainedBytes = min(retained, o.BytesTotal*2)
	}
	if err == nil {
		o.State = "complete"
		o.ErrorCode = ""
		o.Message = "Package installed and integrity checked. See the installed model's current runtime status before use."
		o.ActivationIntent = true
		o.BytesDone = o.BytesTotal
		for i := range o.Artifacts {
			o.Artifacts[i].BytesDone = o.Artifacts[i].BytesTotal
			o.Artifacts[i].Verified = true
		}
	} else {
		o.State = "failed"
		o.ErrorCode = "model_operation_failed"
		o.Message = "Model operation stopped. Existing models and partial data are preserved. Check storage or connectivity, then Resume."
		var safe *models.ModelAcquisitionError
		if errors.As(err, &safe) {
			o.ErrorCode = safe.Code
			o.Message = safe.Message
		}
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			o.State = "cancelled"
			o.ErrorCode = "cancelled"
			o.Message = "Transfer stopped; partial data is retained for Resume or Discard."
		}
		if errors.Is(err, models.ErrPackageInUse) {
			o.ErrorCode = "package_in_use"
			o.Message = "This package is in use. Wait for its active work to settle before resuming."
		}
	}
	if saveErr := s.saveDownloadsLocked(); saveErr != nil {
		o.State = "interrupted"
		o.ErrorCode = "persistence_failed"
		o.Message = "Could not persist the result. Inspect installed models before retrying; this operation will reconcile its receipt."
	}
	completed := o.State == "complete"
	// Keep cancellation ownership through cleanup. Removal/resume cannot race
	// a worker which still has filesystem handles or a publication journal.
	s.downloadMutex.Unlock()
	if completed {
		if cleanupErr := s.registry.Packages().SettleAcquisition(*initial.Target.Package, id, false); cleanupErr == nil {
			s.downloadMutex.Lock()
			s.modelOperations[id].RetainedBytes = 0
			_ = s.saveDownloadsLocked()
			s.downloadMutex.Unlock()
		}
	}
	s.downloadMutex.Lock()
	delete(s.downloadCancelFuncs, "package:"+id)
	s.downloadMutex.Unlock()
}

// Restart reconciles local publication receipts only. It never follows a model
// URL or replays a transfer; interrupted work needs an explicit Resume.
func (s *Server) recoverModelOperations() error {
	if s.registry == nil {
		return nil
	}
	for _, o := range s.modelOperations {
		if o.Discarded {
			continue
		}
		if o.State == "complete" {
			continue
		}
		done, err := s.registry.Packages().Reconcile(context.Background(), *o.Target.Package, o.ID)
		if err != nil {
			o.State = "interrupted"
			o.ErrorCode = "recovery_required"
			o.Message = "Package recovery needs attention. Preserve installed and staged data before repairing."
			continue
		}
		if done {
			o.State = "complete"
			o.ErrorCode = ""
			o.Message = "Installed package reconciled from its durable receipt."
			o.ActivationIntent = true
			o.BytesDone = o.BytesTotal
			for i := range o.Artifacts {
				o.Artifacts[i].BytesDone = o.Artifacts[i].BytesTotal
				o.Artifacts[i].Verified = true
			}
		}
		if bytes, err := s.registry.Packages().RetainedAcquisitionBytes(o.ID); err == nil {
			o.RetainedBytes = min(bytes, o.BytesTotal*2)
		}
	}
	return nil
}

func (s *Server) manageModelOperation(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 1 || len(parts) > 2 || !modelOperationID.MatchString(parts[0]) {
		writeErrorWithCode(w, "Model operation not found.", 404, "model_operation_not_found")
		return
	}
	id := parts[0]
	s.downloadMutex.Lock()
	o := s.modelOperations[id]
	if !s.canReadModelOperation(r, o) {
		s.downloadMutex.Unlock()
		writeErrorWithCode(w, "Model operation not found.", 404, "model_operation_not_found")
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		copy := cloneModelOperation(o)
		s.downloadMutex.Unlock()
		json.NewEncoder(w).Encode(copy)
		return
	}
	if len(parts) != 2 || r.Method != http.MethodPost {
		s.downloadMutex.Unlock()
		writeErrorWithCode(w, "Use POST for model operation controls.", 405, "method_not_allowed")
		return
	}
	action := parts[1]
	previous := cloneModelOperation(o)
	switch action {
	case "cancel":
		if o.active() {
			o.State = "cancelling"
			o.UpdatedAt = time.Now().UTC()
			if err := s.saveDownloadsLocked(); err != nil {
				s.modelOperations[id] = previous
				s.downloadMutex.Unlock()
				modelHTTPError(w, err)
				return
			}
			if cancel := s.downloadCancelFuncs["package:"+id]; cancel != nil {
				cancel()
			}
		}
	case "resume":
		if o.active() || o.State == "complete" || o.Discarded || s.downloadCancelFuncs["package:"+id] != nil {
			s.downloadMutex.Unlock()
			writeErrorWithCode(w, "This operation cannot be resumed in its current state.", 409, "operation_state_conflict")
			return
		}
		for _, other := range s.modelOperations {
			if other.ID != id && !other.Discarded && other.Target.Package.ID == o.Target.Package.ID && other.Target.Package.Revision == o.Target.Package.Revision && other.active() {
				s.downloadMutex.Unlock()
				writeErrorWithCode(w, "Another operation owns this package.", 409, "package_in_use")
				return
			}
		}
		active := 0
		var reserved int64
		for _, other := range s.modelOperations {
			if other.active() {
				active++
				reserved += other.BytesTotal*2 + (16 << 20)
			}
		}
		if active >= 4 || (s.runtimeCtx != nil && s.runtimeCtx.Err() != nil) {
			s.downloadMutex.Unlock()
			writeErrorWithCode(w, "Model downloads are busy or the service is stopping. Try again later.", 503, "model_operation_limit")
			return
		}
		priorBytes := int64(0)
		if o.Action == "repair" {
			priorBytes = o.BytesTotal
		}
		preflight, err := s.registry.Packages().Preflight(*o.Target.Package, priorBytes)
		if err == nil && preflight.AvailableBytes-reserved < preflight.RequiredFreeBytes {
			err = &models.ModelAcquisitionError{Code: "insufficient_space", Message: "Other downloads reserve the remaining disk space. Wait for them to finish."}
		}
		if err != nil {
			s.downloadMutex.Unlock()
			modelHTTPError(w, err)
			return
		}
		o.State = "queued"
		o.ErrorCode = ""
		o.Message = ""
		o.UpdatedAt = time.Now().UTC()
		if err := s.saveDownloadsLocked(); err != nil {
			s.modelOperations[id] = previous
			s.downloadMutex.Unlock()
			modelHTTPError(w, err)
			return
		}
		s.launchModelOperationLocked(id)
	case "discard":
		if o.active() || s.downloadCancelFuncs["package:"+id] != nil {
			s.downloadMutex.Unlock()
			writeErrorWithCode(w, "Cancel and wait for the transfer to stop before discarding its data.", 409, "package_in_use")
			return
		}
		o.Discarded = true
		o.UpdatedAt = time.Now().UTC()
		if err := s.saveDownloadsLocked(); err != nil {
			s.modelOperations[id] = previous
			s.downloadMutex.Unlock()
			modelHTTPError(w, err)
			return
		}
		manifest := *o.Target.Package
		s.downloadMutex.Unlock()
		if err := s.registry.Packages().SettleAcquisition(manifest, id, true); err != nil {
			modelHTTPError(w, err)
			return
		}
		s.downloadMutex.Lock()
		o = s.modelOperations[id]
		o.RetainedBytes = 0
		if err := s.saveDownloadsLocked(); err != nil {
			s.downloadMutex.Unlock()
			modelHTTPError(w, err)
			return
		}
	default:
		s.downloadMutex.Unlock()
		writeErrorWithCode(w, "Use cancel, resume or discard.", 404, "model_operation_not_found")
		return
	}
	copy := cloneModelOperation(o)
	s.downloadMutex.Unlock()
	json.NewEncoder(w).Encode(copy)
}
