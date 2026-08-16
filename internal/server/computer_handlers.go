package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func computerActor(r *http.Request) string {
	if actor := users.GetUserID(r); actor != "" {
		return actor
	}
	return "local-admin"
}

func (s *Server) handleComputerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.computerController == nil {
		writeError(w, "Computer use is unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, s.computerController.Status())
}

func (s *Server) handleComputerSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Scope    computer.Scope `json:"scope"`
		Approved bool           `json:"approved"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	session, err := s.computerController.Begin(r.Context(), computerActor(r), request.Scope, request.Approved)
	if err != nil {
		writeComputerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) handleComputerAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var action computer.Action
	if err := decodeJSON(r, &action); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := s.computerController.Execute(r.Context(), computerActor(r), action)
	if err != nil {
		writeComputerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleComputerStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.computerController.EmergencyStop(computerActor(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleComputerReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Approved bool `json:"approved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.computerController.Reset(r.Context(), computerActor(r), request.Approved); err != nil {
		writeComputerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeComputerError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, capabilities.ErrApprovalRequired) {
		status = http.StatusConflict
	}
	writeError(w, err.Error(), status)
}
