package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func (s *Server) handleBrowserPairing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", 405)
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, "JSON same-origin request required", 403)
		return
	}
	if s.browserHub == nil {
		writeError(w, "Companion service unavailable", 503)
		return
	}
	var request struct {
		Run      string                `json:"run_id"`
		Input    string                `json:"input_id"`
		Protocol int                   `json:"protocol_version"`
		Previous string                `json:"previous_session"`
		Mode     computer.ApprovalMode `json:"approval_mode"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil && err != io.EOF {
			writeError(w, "Invalid pairing request", 400)
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			writeError(w, "Invalid pairing request", 400)
			return
		}
	}
	var code string
	var err error
	if request.Protocol == computer.DesktopProtocolVersion {
		if request.Previous != "" || request.Mode != "" {
			writeError(w, "Invalid desktop pairing request", 400)
			return
		}
		if s.agentManager == nil {
			writeError(w, "Task unavailable", 409)
			return
		}
		task, ok := s.agentManager.GetTask(request.Run)
		if !ok || task.Actor != s.agentActor(r) || task.Status != agents.TaskInput || task.PendingInput == nil || task.PendingInput.ID != request.Input || task.PendingInput.Kind != "computer" || (task.PendingInput.Mode != "app" && task.PendingInput.Mode != "desktop") {
			writeError(w, "Task access request changed", 409)
			return
		}
		mode := computer.ApprovalMode(task.Config.RequestedComputerApprovalMode)
		if s.rejectUnavailableTask(w, r, task) {
			return
		}
		if !mode.Valid() || task.Config.RequestedComputerScope == "selected_window" {
			writeError(w, "Choose a policy in a new task before requesting desktop access", 409)
			return
		}
		code, err = s.browserHub.PairTaskCode(s.agentActor(r), request.Run, request.Input, s.workspaceID, mode)
	} else if request.Protocol == computer.ProtocolVersion {
		if s.agentManager == nil {
			writeError(w, "Task unavailable", 409)
			return
		}
		task, ok := s.agentManager.GetTask(request.Run)
		if !ok || task.Actor != s.agentActor(r) || task.Status != agents.TaskInput || task.PendingInput == nil || task.PendingInput.Kind != "computer" || task.PendingInput.Mode != "browser" || task.PendingInput.ID != request.Input || task.PendingInput.PreviousSession != request.Previous || task.AdmissionHeld || task.Config.Automation != nil {
			writeError(w, "Task access request changed", 409)
			return
		}
		if !request.Mode.Valid() || (task.Config.RequestedComputerApprovalMode != "" && task.Config.RequestedComputerApprovalMode != string(request.Mode)) || (task.Config.ComputerApprovalMode != "" && task.Config.ComputerApprovalMode != string(request.Mode)) {
			writeError(w, "Browser approval policy changed", 409)
			return
		}
		if s.rejectUnavailableTask(w, r, task) {
			return
		}
		var requested []string
		if request.Previous == "" {
			requested = []string{task.PendingInput.URL}
		}
		code, err = s.browserHub.PairBrowserTaskCode(task.Actor, task.ID, request.Input, s.workspaceID, request.Previous, request.Mode, requested...)
	} else if request.Run != "" || request.Input != "" || request.Protocol != 0 || request.Previous != "" || request.Mode != "" {
		writeError(w, "Invalid pairing protocol", 400)
		return
	} else {
		code, err = s.browserHub.PairCode(s.agentActor(r))
	}
	if err != nil {
		writeError(w, "Pairing unavailable", 409)
		return
	}
	protocol := computer.ProtocolVersion
	if request.Protocol == computer.DesktopProtocolVersion {
		protocol = computer.DesktopProtocolVersion
	}
	writeJSON(w, 200, map[string]any{"code": code, "expires_seconds": 120, "protocol_version": protocol})
}
func (s *Server) handleBrowserSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", 405)
		return
	}
	sessions := []computer.BrowserSession{}
	if s.browserHub != nil {
		sessions = s.browserHub.List(s.agentActor(r))
		for i := range sessions {
			if sessions[i].RunID != "" && s.agentManager != nil {
				if task, ok := s.agentManager.GetTask(sessions[i].RunID); ok {
					switch task.Status {
					case "completed", "failed", "cancelled":
						sessions[i].State = "finished"
					}
				}
			}
		}
	}
	writeJSON(w, 200, map[string]any{"sessions": sessions})
}

// Only these exact routes bypass user authentication. They implement their own
// single-use enrollment / companion token authentication and expose no user API.
func (s *Server) browserTransport(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path != "/api/v2/computer/companion/pair" && path != "/api/v2/computer/companion/poll" && path != "/api/v2/computer/companion/reply" && path != "/api/v2/computer/companion/heartbeat" && path != "/api/v2/computer/companion/capture" && path != "/api/v2/computer/companion/activate" && path != "/api/v2/computer/companion/revoke" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if s.browserHub == nil {
			writeError(w, "Companion unavailable", 503)
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, "Method not allowed", 405)
			return
		}
		if r.Header.Get("Origin") != "" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeError(w, "Companion transport only", 403)
			return
		}
		limit := int64(64 << 10)
		if path == "/api/v2/computer/companion/capture" {
			limit = maxJSONRequestBytes
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		if path == "/api/v2/computer/companion/pair" {
			var req struct {
				Code            string                   `json:"code"`
				Origin          string                   `json:"origin"`
				Protocol        int                      `json:"protocol_version"`
				Title           string                   `json:"title"`
				Target          *computer.TargetIdentity `json:"target"`
				ApprovalMode    computer.ApprovalMode    `json:"approval_mode"`
				NativeFeatures  computer.NativeFeatures  `json:"native_features,omitempty"`
				BrowserFeatures computer.BrowserFeatures `json:"browser_features,omitempty"`
				BrowserOrigins  []string                 `json:"browser_origins,omitempty"`
			}
			var encoded json.RawMessage
			if decodeJSON(r, &encoded) != nil || json.Unmarshal(encoded, &req) != nil || req.NativeFeatures.Validate() != nil || req.BrowserFeatures.Validate() != nil || ((req.BrowserFeatures.Discovery != 0 || req.BrowserFeatures.Workspace != 0 || req.BrowserFeatures.Structured != 0 || req.BrowserFeatures.Transfers != 0 || len(req.BrowserOrigins) > 0) && req.Protocol != computer.ProtocolVersion) {
				writeError(w, "Invalid pairing request", 400)
				return
			}
			// The additive transfer contract accepts no supplied binding or
			// conflicting metadata. Preserve legacy decoding for older peers.
			if req.BrowserFeatures.Transfers == 1 && computer.DecodeControlMessage(encoded, &req) != nil {
				writeError(w, "Invalid pairing request", 400)
				return
			}
			if req.ApprovalMode == "" {
				req.ApprovalMode = computer.ApprovalAskEveryTime
			}
			var session *computer.BrowserSession
			var token string
			var err error
			if req.Protocol == computer.DesktopProtocolVersion && req.Target != nil && req.Origin == "" && req.Title == "" {
				session, token, err = s.browserHub.PairDesktopFeatures(req.Code, *req.Target, req.ApprovalMode, req.NativeFeatures)
			} else if req.Protocol == computer.ControlProtocolVersion && req.Target != nil && req.Origin == "" {
				session, token, err = s.browserHub.PairNativeFeatures(req.Code, req.Title, *req.Target, req.Protocol, req.ApprovalMode, req.NativeFeatures)
			} else if req.Target == nil && req.Title == "" && req.NativeFeatures.Documents == 0 {
				session, token, err = s.browserHub.PairBrowserFeatures(req.Code, req.Origin, req.Protocol, req.ApprovalMode, req.BrowserFeatures, req.BrowserOrigins)
			} else {
				err = computer.ErrControlScope
			}
			if err != nil {
				writeError(w, "Pairing expired, invalid or another session is active", 403)
				return
			}
			response := map[string]any{"session": session, "token": token}
			binding, bindingErr := session.FileBinding()
			if bindingErr != nil {
				s.browserHub.StopSession(session.Actor, session.ID)
				writeError(w, "Browser transfer binding unavailable", 403)
				return
			}
			if binding != nil {
				response["file_binding"] = binding
			}
			writeJSON(w, 200, response)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if path == "/api/v2/computer/companion/revoke" {
			if s.browserHub.RevokeCompanion(token) != nil {
				writeError(w, "Session revoked or unavailable", 403)
				return
			}
			writeJSON(w, 200, map[string]bool{"revoked": true})
			return
		}
		if path == "/api/v2/computer/companion/activate" {
			if s.browserHub.ActivateDesktop(token) != nil {
				writeError(w, "Desktop consent unavailable", 403)
				return
			}
			writeJSON(w, 200, map[string]bool{"active": true})
			return
		}
		if path == "/api/v2/computer/companion/heartbeat" {
			if s.browserHub.Heartbeat(token) != nil {
				writeError(w, "Session revoked", 403)
				return
			}
			writeJSON(w, 200, map[string]bool{"active": true})
			return
		}
		if path == "/api/v2/computer/companion/poll" {
			action, err := s.browserHub.Poll(token)
			if err != nil {
				writeError(w, "Companion session expired or revoked", 403)
				return
			}
			writeJSON(w, 200, map[string]any{"action": action})
			return
		}
		if path == "/api/v2/computer/companion/capture" {
			var capture struct {
				ID            string `json:"id"`
				ObservationID string `json:"observation_id"`
				MediaType     string `json:"media_type"`
				Width         int    `json:"width"`
				Height        int    `json:"height"`
				Data          string `json:"data"`
			}
			if decodeStrictJSON(r, &capture) != nil {
				writeError(w, "Invalid capture", 400)
				return
			}
			reference, err := s.browserHub.StoreCapture(token, capture.ID, capture.ObservationID, capture.MediaType, capture.Width, capture.Height, capture.Data)
			if err != nil {
				writeError(w, "Capture rejected", 409)
				return
			}
			writeJSON(w, 200, map[string]string{"image_ref": reference})
			return
		}
		var reply computer.BrowserReply
		if decodeJSON(r, &reply) != nil {
			writeError(w, "Invalid result", 400)
			return
		}
		if err := s.browserHub.Reply(token, reply); err != nil {
			writeError(w, "Result rejected", 409)
			return
		}
		writeJSON(w, 200, map[string]bool{"accepted": true})
	})
}
