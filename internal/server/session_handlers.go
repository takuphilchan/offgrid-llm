package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

// SessionHandlers provides HTTP handlers for session management
type SessionHandlers struct {
	manager *sessions.SessionManager
}

// NewSessionHandlers creates a new SessionHandlers instance
func NewSessionHandlers(sessionsDir string) *SessionHandlers {
	return &SessionHandlers{
		manager: sessions.NewSessionManager(sessionsDir),
	}
}

// HandleSessionsList handles GET /v1/sessions - list all sessions
func (h *SessionHandlers) HandleSessionsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionsList, err := h.manager.List()
	if err != nil {
		writeError(w, "Failed to list sessions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": sessionsList,
	})
}

// HandleSessionCreate handles POST /v1/sessions - create a new session
func (h *SessionHandlers) HandleSessionCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		ModelID string `json:"model_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		writeError(w, "Session name is required", http.StatusBadRequest)
		return
	}

	session := sessions.NewSession(req.Name, req.ModelID)
	if err := h.manager.Save(session); err != nil {
		writeError(w, "Failed to create session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session)
}

// HandleSessionGet handles GET /v1/sessions/{name} - get a specific session
func (h *SessionHandlers) HandleSessionGet(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := h.manager.Load(name)
	if err != nil {
		writeError(w, "Session not found: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// HandleSessionDelete handles DELETE /v1/sessions/{name} - delete a session
func (h *SessionHandlers) HandleSessionDelete(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodDelete {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.Delete(name); err != nil {
		writeError(w, "Failed to delete session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Session deleted",
	})
}

// HandleSessionAddMessage handles POST /v1/sessions/{name}/messages - add a message
func (h *SessionHandlers) HandleSessionAddMessage(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Role == "" || req.Content == "" {
		writeError(w, "Role and content are required", http.StatusBadRequest)
		return
	}

	session, err := h.manager.Load(name)
	if err != nil {
		writeError(w, "Session not found: "+err.Error(), http.StatusNotFound)
		return
	}

	session.AddMessage(req.Role, req.Content)

	if err := h.manager.Save(session); err != nil {
		writeError(w, "Failed to save session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Message added",
	})
}

// HandleSessions is the main router for session endpoints
func (h *SessionHandlers) HandleSessions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/sessions")
	path = strings.TrimPrefix(path, "/")

	// GET/POST /v1/sessions
	if path == "" {
		if r.Method == http.MethodGet {
			h.HandleSessionsList(w, r)
		} else if r.Method == http.MethodPost {
			h.HandleSessionCreate(w, r)
		} else {
			writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Parse session name and action
	parts := strings.SplitN(path, "/", 2)
	sessionName := parts[0]

	if len(parts) == 1 {
		// GET/DELETE /v1/sessions/{name}
		if r.Method == http.MethodGet {
			h.HandleSessionGet(w, r, sessionName)
		} else if r.Method == http.MethodDelete {
			h.HandleSessionDelete(w, r, sessionName)
		} else {
			writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// POST /v1/sessions/{name}/messages
	if parts[1] == "messages" && r.Method == http.MethodPost {
		h.HandleSessionAddMessage(w, r, sessionName)
		return
	}

	writeError(w, "Not found", http.StatusNotFound)
}
