package server

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

const maxEndpointPolicyHistoryBytes = 128 << 10 // One bounded 64 KiB policy plus archival metadata.

type endpointPolicyHistory struct {
	Version       int                    `json:"version"`
	Actor         string                 `json:"actor"`
	At            string                 `json:"at"`
	Configuration string                 `json:"configuration_revision"`
	Snapshot      endpointPolicySnapshot `json:"snapshot"`
}

func validEndpointDecisionKey(key string) bool {
	parts := strings.Split(key, "@")
	return len(parts) == 2 && inferenceEndpointName.MatchString(parts[0]) && inferenceEndpointName.MatchString(parts[1])
}

func validateEndpointPolicySnapshot(state endpointPolicySnapshot) error {
	if (state.Version != 1 && state.Version != 2) || !validEndpointOwnerDigest(state.Epoch) || !validEndpointOwnerDigest(state.Revision) || len(state.Decisions) > 64 || state.Version == 1 && state.PreviousSnapshot != "" || state.Version == 2 && !validEndpointOwnerDigest(state.PreviousSnapshot) {
		return errors.New("endpoint policy metadata is invalid")
	}
	for key, decision := range state.Decisions {
		_, timeErr := time.Parse(time.RFC3339Nano, decision.At)
		if !validEndpointDecisionKey(key) || !validEndpointOwnerDigest(decision.Pin) || decision.Actor == "" || len(decision.Actor) > 256 || len(decision.At) > 64 || timeErr != nil || decision.Enabled && decision.ReviewRequired {
			return errors.New("endpoint policy decision is invalid")
		}
	}
	return nil
}

func endpointPolicyHistoryDigest(state endpointPolicySnapshot) string {
	encoded, _ := json.Marshal(state)
	return endpointOwnerDigest(encoded)
}

func endpointPolicyHistoryPath(directory, digest string) string {
	return filepath.Join(directory, "inference-endpoint-policy-history-"+digest+".json")
}

// Historical snapshots are evidence only. No read path activates their grants.
// Their predecessor references allow bounded, one-page-at-a-time audit traversal.
func readEndpointPolicyHistory(directory, digest string) (*endpointPolicyHistory, error) {
	if !validEndpointOwnerDigest(digest) {
		return nil, errors.New("invalid endpoint policy history identity")
	}
	var history endpointPolicyHistory
	if _, err := readEndpointOwnerJSON(endpointPolicyHistoryPath(directory, digest), maxEndpointPolicyHistoryBytes, &history); err != nil {
		return nil, err
	}
	_, timeErr := time.Parse(time.RFC3339Nano, history.At)
	if history.Version != 1 || history.Actor == "" || len(history.Actor) > 256 || len(history.At) > 64 || timeErr != nil || !validEndpointOwnerDigest(history.Configuration) || validateEndpointPolicySnapshot(history.Snapshot) != nil || endpointPolicyHistoryDigest(history.Snapshot) != digest {
		return nil, errors.New("endpoint policy history is invalid")
	}
	return &history, nil
}

func (s *Server) preserveEndpointPolicy(state endpointPolicySnapshot, actor, configuration string) (string, error) {
	digest := endpointPolicyHistoryDigest(state)
	existing, err := readEndpointPolicyHistory(s.config.DataDir, digest)
	if err == nil {
		if !reflect.DeepEqual(existing.Snapshot, state) {
			return "", errors.New("endpoint policy history conflicts")
		}
		return digest, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	history := endpointPolicyHistory{Version: 1, Actor: actor, At: time.Now().UTC().Format(time.RFC3339Nano), Configuration: configuration, Snapshot: state}
	encoded, err := json.Marshal(history)
	if err != nil || len(encoded) > maxEndpointPolicyHistoryBytes {
		return "", errors.New("endpoint policy archive exceeds its read bound")
	}
	write := s.endpointOwner.write
	if write == nil {
		write = storage.WriteDurableJSON
	}
	if err := write(endpointPolicyHistoryPath(s.config.DataDir, digest), history); err != nil {
		return "", err
	}
	stored, err := readEndpointPolicyHistory(s.config.DataDir, digest)
	if err != nil || !reflect.DeepEqual(stored.Snapshot, state) {
		return "", errors.New("endpoint policy history verification failed")
	}
	return digest, nil
}
