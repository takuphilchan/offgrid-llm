package server

import (
	"errors"
	"net/http"
	"slices"
)

type endpointPolicyMaintenanceView struct {
	Revision  string   `json:"policy_revision"`
	Count     int      `json:"decision_count"`
	Retirable []string `json:"retirable"`
}

type endpointPolicyMaintenanceInput struct {
	Expected       string   `json:"expected_revision"`
	PolicyRevision string   `json:"expected_policy_revision"`
	Endpoints      []string `json:"endpoints"`
}

type endpointPolicyMaintenanceResult struct {
	Retired  []string `json:"retired"`
	Revision string   `json:"policy_revision"`
	History  string   `json:"history_revision"`
}

// Caller holds the policy lock; installed is protected by nativeRuntimeMu.
func retirableEndpointDecisions(state endpointPolicySnapshot, current, installed []inferenceEndpointEnrollment, policy *endpointPolicy) []string {
	keep := map[string]bool{}
	for _, entries := range [][]inferenceEndpointEnrollment{current, installed} {
		for _, entry := range entries {
			keep[endpointKey(entry)] = true
		}
	}
	result := []string{}
	for key := range state.Decisions {
		if !keep[key] && len(policy.active[key]) == 0 {
			result = append(result, key)
		}
	}
	slices.Sort(result)
	return result
}

func (s *Server) endpointPolicyMaintenanceState(configuration inferenceEndpointFile) *endpointPolicyMaintenanceView {
	s.nativeRuntimeMu.RLock()
	defer s.nativeRuntimeMu.RUnlock()
	policy := s.endpointPolicy
	if policy == nil {
		return nil
	}
	policy.mu.Lock()
	defer policy.mu.Unlock()
	state, _, err := policy.readLocked()
	if err != nil {
		return nil
	}
	return &endpointPolicyMaintenanceView{Revision: state.Revision, Count: len(state.Decisions), Retirable: retirableEndpointDecisions(state, configuration.Endpoints, s.installedEndpoints, policy)}
}

func (s *Server) retireEndpointDecisions(r *http.Request, input endpointPolicyMaintenanceInput) (endpointPolicyMaintenanceResult, error) {
	var result endpointPolicyMaintenanceResult
	if s.endpointOwner == nil || s.endpointPolicy == nil {
		return result, errors.New("endpoint maintenance is unavailable")
	}
	owner := s.endpointOwner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	actor, err := s.endpointAdministrator(r)
	if err != nil {
		return result, err
	}
	if owner.fault != nil {
		return result, owner.fault
	}
	if len(input.Endpoints) == 0 || len(input.Endpoints) > 64 {
		return result, errors.New("reviewed obsolete endpoint identities are required")
	}
	requested := slices.Clone(input.Endpoints)
	slices.Sort(requested)
	for index, key := range requested {
		if !validEndpointDecisionKey(key) || index > 0 && key == requested[index-1] {
			return result, errors.New("retirement identities are invalid or duplicated")
		}
	}
	host, hostPin, err := readEndpointOwnerHost(s.config.InferenceEndpointOwnerPolicyFile)
	if err != nil {
		return result, err
	}
	release, err := acquireEndpointOwnerWriter(s.endpointConfigurationPath(), s.config.DataDir, host.EditableFile)
	if err != nil {
		return result, err
	}
	defer release()
	configuration, pin, err := s.readEndpointConfiguration()
	if err != nil || pin != input.Expected {
		return result, errors.New("endpoint configuration changed; refresh maintenance review")
	}
	s.nativeRuntimeMu.RLock()
	defer s.nativeRuntimeMu.RUnlock()
	policy := s.endpointPolicy
	policy.mu.Lock()
	defer policy.mu.Unlock()
	state, status, err := policy.readLocked()
	if err != nil {
		return result, err
	}
	if status != "ready" || state.Revision != input.PolicyRevision {
		return result, errors.New("endpoint policy changed; refresh maintenance review")
	}
	eligible := retirableEndpointDecisions(state, configuration.Endpoints, s.installedEndpoints, policy)
	for _, key := range requested {
		if !slices.Contains(eligible, key) {
			return result, errors.New("endpoint decision is current, installed, active or no longer present")
		}
	}
	authorize := func() error {
		if err := r.Context().Err(); err != nil {
			return err
		}
		current, err := s.endpointAdministrator(r)
		if err != nil {
			return err
		}
		if current != actor || len(actor) > 256 {
			return errors.New("endpoint maintenance administrator changed")
		}
		return nil
	}
	if err := authorize(); err != nil {
		return result, err
	}
	history, err := s.preserveEndpointPolicy(state, actor, pin)
	if err != nil {
		return result, err
	}
	if err := authorize(); err != nil {
		return result, err
	}
	_, liveHost, err := readEndpointOwnerHost(s.config.InferenceEndpointOwnerPolicyFile)
	if err != nil || liveHost != hostPin {
		return result, errors.New("installation policy changed during maintenance")
	}
	_, livePin, err := s.readEndpointConfiguration()
	if err != nil || livePin != pin {
		return result, errors.New("endpoint configuration changed during maintenance")
	}
	live, liveStatus, err := policy.readLocked()
	if err != nil || liveStatus != "ready" || endpointPolicyHistoryDigest(live) != history {
		return result, errors.New("endpoint policy or activation epoch changed during maintenance")
	}
	// No authority is recreated from history. Absent decisions always deny.
	for _, key := range requested {
		delete(state.Decisions, key)
	}
	state.Version, state.PreviousSnapshot = 2, history
	state.Revision, err = endpointOwnerNonce()
	if err != nil {
		return result, err
	}
	if err := policy.publishLocked(state); err != nil {
		return result, err
	}
	return endpointPolicyMaintenanceResult{Retired: requested, Revision: state.Revision, History: history}, nil
}
