package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type SessionCompleter func(context.Context, string, []api.ChatMessage, bool) (string, error)

// SessionHandlers provides HTTP handlers for session management
type SessionHandlers struct {
	manager      *sessions.SessionManager
	completer    SessionCompleter
	streamer     SessionStreamer
	requireAuth  bool
	locksMu      sync.Mutex
	sessionLocks map[string]*sessionLock
}

type sessionLock struct {
	gate chan struct{}
	refs int
}

// Serialize all mutations, including deletion, with generation for this name.
// Reference counting also prevents arbitrary names accumulating locks forever.
func (h *SessionHandlers) lockSession(name string) func() {
	unlock, _ := h.lockSessionContext(context.Background(), name)
	return unlock
}

func (h *SessionHandlers) lockSessionContext(ctx context.Context, name string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.locksMu.Lock()
	if h.sessionLocks == nil {
		h.sessionLocks = make(map[string]*sessionLock)
	}
	lock := h.sessionLocks[name]
	if lock == nil {
		lock = &sessionLock{gate: make(chan struct{}, 1)}
		h.sessionLocks[name] = lock
	}
	lock.refs++
	h.locksMu.Unlock()
	drop := func() {
		h.locksMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(h.sessionLocks, name)
		}
		h.locksMu.Unlock()
	}
	select {
	case lock.gate <- struct{}{}:
		return func() { <-lock.gate; drop() }, nil
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	}
}

func (h *SessionHandlers) scoped(r *http.Request) *sessions.ScopedManager {
	if !h.requireAuth {
		return h.manager.WithAccess(sessions.Access{All: true})
	}
	user := users.GetUser(r)
	if user == nil || !user.HasPermission(users.PermissionSessions) {
		return h.manager.WithAccess(sessions.Access{})
	}
	return h.manager.WithAccess(sessions.Access{UserID: user.ID, All: user.HasPermission(users.PermissionSessionsAll)})
}

func writeSessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sessions.ErrInvalidSessionName):
		writeError(w, "Invalid session name", http.StatusBadRequest)
	case errors.Is(err, sessions.ErrSessionNotFound):
		writeError(w, "Session not found", http.StatusNotFound)
	case errors.Is(err, sessions.ErrSessionExists):
		writeError(w, "Session name is already in use", http.StatusConflict)
	case errors.Is(err, sessions.ErrAccessDenied):
		writeError(w, "Forbidden", http.StatusForbidden)
	default:
		writeServiceError(w, newServiceError(http.StatusInternalServerError, "Session storage operation failed", err))
	}
}

func (h *SessionHandlers) SetCompleter(completer SessionCompleter) {
	h.completer = completer
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

	sessionsList, err := h.scoped(r).List()
	if err != nil {
		writeSessionError(w, err)
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

	unlock := h.lockSession(req.Name)
	defer unlock()
	session, err := h.scoped(r).Create(req.Name, req.ModelID)
	if err != nil {
		writeSessionError(w, err)
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

	session, err := h.scoped(r).Load(name)
	if err != nil {
		writeSessionError(w, err)
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

	unlock := h.lockSession(name)
	defer unlock()
	if err := h.scoped(r).Delete(name); err != nil {
		writeSessionError(w, err)
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

	unlock := h.lockSession(name)
	defer unlock()
	if err := h.scoped(r).AddMessage(name, req.Role, req.Content); err != nil {
		writeSessionError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Message added",
	})
}

// HandleSessionGenerate performs a model turn and persists the complete
// exchange under the same session. Calls for one session are serialized so
// concurrent browser tabs cannot lose messages.
func (h *SessionHandlers) HandleSessionGenerate(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Content          string `json:"content"`
		ModelID          string `json:"model_id,omitempty"`
		UseKnowledgeBase bool   `json:"use_knowledge_base,omitempty"`
		Stream           bool   `json:"stream,omitempty"`
		Profile          string `json:"profile,omitempty"`
		MaxTokens        int    `json:"max_tokens,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		writeError(w, "Content is required", http.StatusBadRequest)
		return
	}

	if err := authorizeChatAccess(r.Context(), h.requireAuth, req.UseKnowledgeBase); err != nil {
		writeServiceError(w, err)
		return
	}
	if req.Stream {
		if req.Profile != "" && req.Profile != "interactive" && req.Profile != "extended" {
			writeError(w, "Unknown chat profile", http.StatusBadRequest)
			return
		}
		if req.MaxTokens < 0 || req.MaxTokens > 4096 {
			writeError(w, "max_tokens must be between 1 and 4096", http.StatusBadRequest)
			return
		}
		if req.MaxTokens == 0 {
			req.MaxTokens = 1024
		}
		h.generateStream(w, r, name, req.Content, req.ModelID, req.UseKnowledgeBase, req.Profile, req.MaxTokens)
		return
	}
	if h.completer == nil {
		writeError(w, "Session chat is unavailable", http.StatusServiceUnavailable)
		return
	}
	unlock := h.lockSession(name)
	defer unlock()

	session, err := h.scoped(r).Load(name)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	modelID := req.ModelID
	if modelID == "" {
		modelID = session.ModelID
	}
	if modelID == "" {
		writeError(w, "Model is required", http.StatusBadRequest)
		return
	}
	messages := make([]api.ChatMessage, 0, len(session.Messages)+1)
	for _, message := range session.Messages {
		messages = append(messages, api.ChatMessage{Role: message.Role, Content: message.Content})
	}
	messages = append(messages, api.ChatMessage{Role: "user", Content: req.Content})
	answer, err := h.completer(r.Context(), modelID, messages, req.UseKnowledgeBase)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	updated, err := h.scoped(r).AppendExchange(name, modelID, req.Content, answer)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session": updated,
		"message": sessions.Message{Role: "assistant", Content: answer, Timestamp: updated.UpdatedAt},
	})
}

// HandleSessions is the main router for session endpoints
func (h *SessionHandlers) HandleSessions(w http.ResponseWriter, r *http.Request) {
	if h.requireAuth {
		user := users.GetUser(r)
		if user == nil {
			writeError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !user.HasPermission(users.PermissionSessions) {
			writeError(w, "Forbidden", http.StatusForbidden)
			return
		}
	}
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
	if parts[1] == "generate" && r.Method == http.MethodPost {
		h.HandleSessionGenerate(w, r, sessionName)
		return
	}

	writeError(w, "Not found", http.StatusNotFound)
}

// GetActiveSessionCount returns the number of active sessions
func (h *SessionHandlers) GetActiveSessionCount() int {
	if h.manager == nil {
		return 0
	}
	sessions, err := h.manager.List()
	if err != nil {
		return 0
	}
	return len(sessions)
}
