package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

func (s *Server) newExecutionEnrollment() *execution.Enrollment {
	enrollment := execution.NewEnrollment(s.executionHosts, s.version)
	enrollment.MayUse = func(actor, workspace string) bool {
		d, _ := execution.Descriptor("workspace_read")
		return workspace == s.workspaceID && s.profileActorCapabilityAllowed(actor, d)
	}
	enrollment.RecordReceipt = func(_ context.Context, request execution.Request, response execution.Response) error {
		if s.agentManager == nil || request.Binding.Workspace != s.workspaceID {
			return execution.ErrAuthority
		}
		return s.agentManager.RecordExecutionReceipt(request.Binding.Job, request.Binding.Actor, request, response)
	}
	return enrollment
}

func (s *Server) executionPreview(w http.ResponseWriter) bool {
	w.Header().Set("Cache-Control", "no-store")
	if s.config == nil || !s.config.EnableExecutionPreview || s.executionEnrollment == nil {
		writeJobError(w, 503, "execution_preview_disabled", "Native execution is not enabled on this service. No folder authority was created.", false)
		return false
	}
	return true
}

func decodeExecutionRequest(w http.ResponseWriter, r *http.Request, target any, limit int64) bool {
	if r.Method != http.MethodPost {
		writeJobError(w, 405, "method_not_allowed", "Use POST for this execution control request.", false)
		return false
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" || strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		writeJobError(w, 403, "execution_request_denied", "Same-origin JSON execution control is required.", false)
		return false
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	trimmed := bytes.TrimSpace(data)
	if err != nil || len(trimmed) == 0 || trimmed[0] != '{' || jsonwire.Validate(data) != nil {
		writeJobError(w, 400, "invalid_execution_request", "Provide one bounded execution request.", false)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		writeJobError(w, 400, "invalid_execution_request", "Provide one supported execution request.", false)
		return false
	}
	return true
}

func writeExecutionControlError(w http.ResponseWriter, err error) {
	status := 403
	code := "execution_authority_denied"
	switch {
	case errors.Is(err, execution.ErrHostUnavailable):
		status, code = 410, "execution_host_unavailable"
	case errors.Is(err, execution.ErrProtocol), errors.Is(err, execution.ErrUnsupported):
		status, code = 400, "execution_protocol_mismatch"
	case errors.Is(err, execution.ErrLimit):
		status, code = 429, "execution_capacity"
	case errors.Is(err, execution.ErrConflict):
		status, code = 409, "execution_control_conflict"
	}
	writeJobError(w, status, code, "The execution connection is unavailable or its scope changed. Reconnect locally; no operation was replayed.", false)
}

func (s *Server) handleExecutionPairing(w http.ResponseWriter, r *http.Request) {
	if !s.executionPreview(w) || !decodeExecutionRequest(w, r, &struct{}{}, 4096) {
		return
	}
	pair, err := s.executionEnrollment.Pair(s.agentActor(r), s.workspaceID)
	if err != nil {
		writeExecutionControlError(w, err)
		return
	}
	writeJSON(w, 200, pair)
}

func (s *Server) handleExecutionRevoke(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reference execution.Reference `json:"reference"`
	}
	if !s.executionPreview(w) || !decodeExecutionRequest(w, r, &body, 4096) {
		return
	}
	if err := s.executionEnrollment.RevokeOwner(s.agentActor(r), s.workspaceID, body.Reference); err != nil {
		writeExecutionControlError(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"revoked": true})
}

// Exact host transport routes use only a one-use scoped pairing ticket or a
// host-only credential. They do not accept session cookies as local approval
// and expose no general user API. The worker never receives the user's API key.
func (s *Server) executionTransport(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/execution/host/enroll", "/api/v2/execution/host/poll", "/api/v2/execution/host/reply", "/api/v2/execution/host/heartbeat", "/api/v2/execution/host/revoke":
			s.handleExecutionHost(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func (s *Server) handleExecutionHost(w http.ResponseWriter, r *http.Request) {
	if !s.executionPreview(w) {
		return
	}
	if r.URL.Path == "/api/v2/execution/host/enroll" {
		var body struct {
			Code    string            `json:"code"`
			Token   string            `json:"token"`
			Context execution.Context `json:"context"`
			Pack    execution.Pack    `json:"pack"`
		}
		if !decodeExecutionRequest(w, r, &body, 128<<10) {
			return
		}
		ref, err := s.executionEnrollment.Enroll(body.Code, body.Token, body.Context, body.Pack)
		if err != nil {
			writeExecutionControlError(w, err)
			return
		}
		writeJSON(w, 201, map[string]any{"reference": ref, "state": "awaiting_host_check"})
		return
	}
	var body struct {
		Session string                 `json:"session"`
		Token   string                 `json:"token"`
		Reply   *execution.BridgeReply `json:"reply,omitempty"`
	}
	if !decodeExecutionRequest(w, r, &body, execution.MaxFrameBytes+4096) {
		return
	}
	if r.URL.Path != "/api/v2/execution/host/reply" && body.Reply != nil {
		writeExecutionControlError(w, execution.ErrProtocol)
		return
	}
	var err error
	switch r.URL.Path {
	case "/api/v2/execution/host/poll":
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		var message execution.BridgeMessage
		message, err = s.executionEnrollment.Poll(ctx, body.Session, body.Token)
		if errors.Is(err, context.DeadlineExceeded) && r.Context().Err() == nil {
			err = nil
		}
		if err == nil {
			writeJSON(w, 200, message)
			return
		}
	case "/api/v2/execution/host/reply":
		if body.Reply == nil {
			writeExecutionControlError(w, execution.ErrProtocol)
			return
		}
		var ready bool
		ready, err = s.executionEnrollment.Reply(r.Context(), body.Session, body.Token, *body.Reply)
		if err == nil {
			writeJSON(w, 200, map[string]bool{"ready": ready})
			return
		}
	case "/api/v2/execution/host/heartbeat":
		err = s.executionEnrollment.Heartbeat(body.Session, body.Token)
	case "/api/v2/execution/host/revoke":
		err = s.executionEnrollment.RevokeHost(body.Session, body.Token)
	default:
		err = execution.ErrProtocol
	}
	if err != nil {
		writeExecutionControlError(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}
