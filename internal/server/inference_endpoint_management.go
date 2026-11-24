package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/audit"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

const inferenceEndpointsPath = "/api/v2/inference-endpoints"

type endpointReview struct {
	ID             string `json:"id"`
	Revision       string `json:"revision"`
	ConfigPin      string `json:"config_pin"`
	PolicyRevision string `json:"policy_revision"`
	Enabled        *bool  `json:"enabled,omitempty"`
}

type endpointManagementItem struct {
	ID              string   `json:"id"`
	Revision        string   `json:"revision"`
	ModelID         string   `json:"model_id"`
	Provider        string   `json:"provider"`
	URL             string   `json:"url"`
	Model           string   `json:"model"`
	Actors          []string `json:"actors"`
	ConfigPin       string   `json:"config_pin"`
	Installed       bool     `json:"installed"`
	Current         bool     `json:"current"`
	Enabled         bool     `json:"enabled"`
	ReviewRequired  bool     `json:"review_required"`
	CredentialReady bool     `json:"credential_ready"`
	Managed         bool     `json:"managed"`
	ProbeAllowed    bool     `json:"probe_allowed"`
	ReviewedBy      string   `json:"reviewed_by,omitempty"`
	ReviewedAt      string   `json:"reviewed_at,omitempty"`
}

type endpointManagementSnapshot struct {
	Configured     bool                     `json:"configured"`
	PolicyState    string                   `json:"policy_state"`
	PolicyRevision string                   `json:"policy_revision"`
	Items          []endpointManagementItem `json:"items"`
}

func (s *Server) endpointAdministrator(r *http.Request) (string, error) {
	actor := s.agentActor(r)
	if !s.scheduleActorPermission(actor, users.PermissionAdmin) {
		return "", errors.New("current administrator permission is required")
	}
	return actor, nil
}

func (s *Server) endpointManagementState(actor string) (endpointManagementSnapshot, error) {
	result := endpointManagementSnapshot{Items: []endpointManagementItem{}, PolicyState: "not_configured"}
	if s.endpointConfigurationPath() == "" {
		return result, nil
	}
	result.Configured = true
	configuration, _, err := s.readEndpointConfiguration()
	if err != nil {
		return result, errors.New("trusted endpoint configuration is unavailable")
	}
	s.nativeRuntimeMu.RLock()
	policy, installed := s.endpointPolicy, slices.Clone(s.installedEndpoints)
	s.nativeRuntimeMu.RUnlock()
	state := endpointPolicySnapshot{Decisions: map[string]endpointDecision{}}
	if policy != nil {
		policy.mu.Lock()
		state, result.PolicyState, err = policy.readLocked()
		policy.mu.Unlock()
		if err != nil {
			return result, errors.New("endpoint policy is unavailable; all endpoint requests are blocked")
		}
		result.PolicyRevision = state.Revision
	}
	entries := slices.Clone(configuration.Endpoints)
	for _, entry := range installed {
		if !slices.ContainsFunc(entries, func(current inferenceEndpointEnrollment) bool { return endpointKey(current) == endpointKey(entry) }) {
			entries = append(entries, entry)
		}
	}
	for _, entry := range entries {
		current := slices.ContainsFunc(configuration.Endpoints, func(value inferenceEndpointEnrollment) bool { return reflect.DeepEqual(value, entry) })
		loaded := slices.ContainsFunc(installed, func(value inferenceEndpointEnrollment) bool { return reflect.DeepEqual(value, entry) })
		decision, reviewed := state.Decisions[endpointKey(entry)]
		reviewed = reviewed && !decision.ReviewRequired && decision.Pin == endpointPin(entry) && result.PolicyState == "ready"
		credential := entry.CredentialEnv == "" || os.Getenv(entry.CredentialEnv) != ""
		enabled := reviewed && decision.Enabled && loaded && current
		item := endpointManagementItem{ID: entry.ID, Revision: entry.Revision, ModelID: "endpoint:" + endpointKey(entry), Provider: string(entry.Provider), URL: entry.URL, Model: entry.Model, Actors: slices.Clone(entry.Actors), ConfigPin: endpointPin(entry), Installed: loaded, Current: current, Enabled: enabled, ReviewRequired: !reviewed, CredentialReady: credential, Managed: entry.ManagedOllama != nil, ProbeAllowed: enabled && credential && slices.Contains(entry.Actors, actor) && s.scheduleActorPermission(actor, users.PermissionChat)}
		if reviewed {
			item.ReviewedBy, item.ReviewedAt = decision.Actor, decision.At
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *Server) currentEndpointForReview(input endpointReview, enabled bool) (inferenceEndpointEnrollment, error) {
	configuration, _, err := s.readEndpointConfiguration()
	if err != nil {
		return inferenceEndpointEnrollment{}, err
	}
	s.nativeRuntimeMu.RLock()
	installed := slices.Clone(s.installedEndpoints)
	s.nativeRuntimeMu.RUnlock()
	entries := configuration.Endpoints
	if !enabled {
		entries = append(entries, installed...)
	}
	for _, entry := range entries {
		if entry.ID == input.ID && entry.Revision == input.Revision && endpointPin(entry) == input.ConfigPin {
			if enabled && !slices.ContainsFunc(installed, func(pinned inferenceEndpointEnrollment) bool { return reflect.DeepEqual(pinned, entry) }) {
				return entry, errors.New("endpoint installation changed; service restart and review are required")
			}
			return entry, nil
		}
	}
	return inferenceEndpointEnrollment{}, errors.New("endpoint configuration changed; refresh before reviewing")
}

func (s *Server) handleInferenceEndpoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, err := s.endpointAdministrator(r)
	if err != nil {
		writeError(w, "Current administrator permission is required", http.StatusForbidden)
		return
	}
	if r.URL.Path == inferenceEndpointsPath+"/configuration" || strings.HasPrefix(r.URL.Path, inferenceEndpointsPath+"/configuration/") {
		s.handleEndpointOwner(w, r, actor)
		return
	}
	if r.URL.Path == inferenceEndpointsPath && r.Method == http.MethodGet {
		state, err := s.endpointManagementState(actor)
		if err != nil {
			writeError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != inferenceEndpointsPath+"/review" && r.URL.Path != inferenceEndpointsPath+"/probe" {
		writeError(w, "Method or endpoint is unavailable", http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 4097))
	var input endpointReview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err != nil || len(raw) > 4096 || jsonwire.Validate(raw) != nil || decoder.Decode(&input) != nil || !inferenceEndpointName.MatchString(input.ID) || !inferenceEndpointName.MatchString(input.Revision) || len(input.ConfigPin) != 64 || len(input.PolicyRevision) != 64 {
		writeError(w, "A bounded endpoint identity and current review revisions are required", http.StatusBadRequest)
		return
	}
	s.nativeRuntimeMu.RLock()
	policy := s.endpointPolicy
	s.nativeRuntimeMu.RUnlock()
	if policy == nil {
		writeError(w, "Endpoint installation requires service restart", http.StatusConflict)
		return
	}
	probe := r.URL.Path == inferenceEndpointsPath+"/probe"
	if probe && input.Enabled != nil || !probe && input.Enabled == nil {
		writeError(w, "Endpoint action is malformed", http.StatusBadRequest)
		return
	}
	enabled := probe || *input.Enabled
	entry, err := s.currentEndpointForReview(input, enabled)
	if err != nil {
		writeError(w, "Endpoint installation changed; refresh and review its current configuration", http.StatusConflict)
		return
	}
	if !probe {
		err = policy.update(entry, input.PolicyRevision, actor, enabled, func() error {
			if _, err := s.endpointAdministrator(r); err != nil {
				return err
			}
			_, err := s.currentEndpointForReview(input, enabled)
			return err
		})
		if err != nil {
			if errors.Is(err, errEndpointPolicyPublicationUncertain) {
				writeErrorWithCode(w, "Endpoint policy publication is uncertain. Access is blocked in this service; restart and review the actual stored state", http.StatusServiceUnavailable, "endpoint_policy_publication_uncertain")
				return
			}
			writeError(w, "Endpoint review could not be saved; refresh its configuration and policy", http.StatusConflict)
			return
		}
		s.auditLog(audit.EventTypeModel, "endpoint_review", actor, r.RemoteAddr, map[string]string{"id": entry.ID, "revision": entry.Revision}, true)
		state, err := s.endpointManagementState(actor)
		if err != nil {
			writeError(w, "Endpoint review was saved but current status is unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
		return
	}
	policy.mu.Lock()
	state, _, err := policy.readLocked()
	policy.mu.Unlock()
	if err != nil || state.Revision != input.PolicyRevision {
		writeError(w, "Endpoint policy changed; refresh before checking", http.StatusConflict)
		return
	}
	descriptor, _, err := s.selectEndpointChat(r.Context(), endpointKey(entry))
	if err != nil || s.providerScheduler == nil {
		writeError(w, "Endpoint access or its scheduler is unavailable", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	// Runtime performs health inside the shared admission/resource gate. The
	// returned pointer is deliberately not used after that gate is released.
	if _, err = s.providerScheduler.Runtime(ctx, descriptor); err != nil {
		writeError(w, "Endpoint connection check failed; no model qualification was established", http.StatusServiceUnavailable)
		return
	}
	if _, _, err = s.selectEndpointChat(ctx, endpointKey(entry)); err != nil {
		writeError(w, "Endpoint access changed during the connection check", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"model_id": "endpoint:" + endpointKey(entry), "connection_verified": true, "qualified": false})
}
