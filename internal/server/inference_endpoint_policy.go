package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

const endpointPolicyFilename = "inference-endpoint-policy.json"

var errEndpointPolicyPublicationUncertain = errors.New("endpoint policy publication is uncertain")

type endpointDecision struct {
	Pin            string `json:"pin"`
	Enabled        bool   `json:"enabled"`
	Actor          string `json:"actor"`
	At             string `json:"at"`
	ReviewRequired bool   `json:"review_required,omitempty"` // Automatic owner revocation is not a human access review.
}

type endpointPolicySnapshot struct {
	Version          int                         `json:"version"`
	Epoch            string                      `json:"restore_epoch"`
	Revision         string                      `json:"revision"`
	Decisions        map[string]endpointDecision `json:"decisions"`
	PreviousSnapshot string                      `json:"previous_snapshot,omitempty"`
}

// This subtractive store cannot enroll endpoints or broaden file-owned actor
// grants. Every read checks the current on-disk policy and restore epoch.
type endpointPolicy struct {
	mu            sync.Mutex
	dataDir       string
	active        map[string]map[*endpointRequest]struct{}
	fault         error
	ownerFault    error
	guard         func() error            // Startup-owned durable configuration-intent barrier.
	writeSnapshot func(string, any) error // Instance-local fault injection; nil uses durable publication.
}

type endpointRequest struct{ cancel context.CancelFunc }

func endpointPin(entry inferenceEndpointEnrollment) string {
	raw, _ := json.Marshal(entry)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func endpointKey(entry inferenceEndpointEnrollment) string { return entry.ID + "@" + entry.Revision }

func newEndpointPolicy(dataDir string) (*endpointPolicy, error) {
	if !filepath.IsAbs(dataDir) {
		return nil, errors.New("endpoint policy requires an absolute workspace data directory")
	}
	return &endpointPolicy{dataDir: dataDir, active: map[string]map[*endpointRequest]struct{}{}}, nil
}

func (p *endpointPolicy) readLocked() (endpointPolicySnapshot, string, error) {
	var state endpointPolicySnapshot
	if p.fault != nil {
		return state, "unavailable", p.fault
	}
	if p.ownerFault != nil {
		return state, "unavailable", p.ownerFault
	}
	if p.guard != nil {
		if err := p.guard(); err != nil {
			return state, "unavailable", err
		}
	}
	epoch, err := storage.ReadRestoreEpoch(p.dataDir)
	if err != nil {
		return state, "unavailable", err
	}
	path := filepath.Join(p.dataDir, endpointPolicyFilename)
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		digest := sha256.Sum256([]byte("missing\x00" + epoch))
		return endpointPolicySnapshot{Version: 1, Epoch: epoch, Revision: hex.EncodeToString(digest[:]), Decisions: map[string]endpointDecision{}}, "review_required", nil
	}
	if err != nil || !before.Mode().IsRegular() || before.Size() > 64<<10 {
		return state, "unavailable", errors.New("endpoint policy must be a bounded regular file")
	}
	if epoch == "" {
		return state, "unavailable", errors.New("workspace activation epoch is missing; existing endpoint policy cannot authorize access")
	}
	file, err := openObservedInferenceEndpointFile(path, before)
	if err != nil {
		return state, "unavailable", err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err != nil || len(raw) > 64<<10 || jsonwire.Validate(raw) != nil || decoder.Decode(&state) != nil || validateEndpointPolicySnapshot(state) != nil {
		return state, "unavailable", errors.New("endpoint policy is corrupt; no grants are active")
	}
	if state.PreviousSnapshot != "" {
		if _, err := readEndpointPolicyHistory(p.dataDir, state.PreviousSnapshot); err != nil {
			return state, "unavailable", err
		}
	}
	if state.Epoch != epoch {
		// Retain a compare-and-swap token derived from the historical snapshot
		// AND the fresh restore epoch, but never retain its enabled decisions.
		digest := sha256.Sum256(append(append([]byte("restored\x00"+epoch+"\x00"), raw...), '\n'))
		return endpointPolicySnapshot{Version: state.Version, Epoch: epoch, Revision: hex.EncodeToString(digest[:]), Decisions: map[string]endpointDecision{}, PreviousSnapshot: state.PreviousSnapshot}, "restore_review_required", nil
	}
	if state.Decisions == nil {
		state.Decisions = map[string]endpointDecision{}
	}
	return state, "ready", nil
}

func (p *endpointPolicy) allowedLocked(entry inferenceEndpointEnrollment) error {
	state, status, err := p.readLocked()
	if err != nil {
		return err
	}
	decision := state.Decisions[endpointKey(entry)]
	if status != "ready" || !decision.Enabled || decision.ReviewRequired || decision.Pin != endpointPin(entry) {
		return errors.New("endpoint requires current administrator review or is disabled")
	}
	return nil
}

func (p *endpointPolicy) allowed(entry inferenceEndpointEnrollment) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.allowedLocked(entry)
}

func (p *endpointPolicy) acquire(ctx context.Context, entry inferenceEndpointEnrollment) (context.Context, func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if err := p.allowedLocked(entry); err != nil {
		return nil, nil, err
	}
	admitted, cancel := context.WithCancel(ctx)
	request := &endpointRequest{cancel: cancel}
	key := endpointKey(entry)
	if p.active[key] == nil {
		p.active[key] = map[*endpointRequest]struct{}{}
	}
	p.active[key][request] = struct{}{}
	return admitted, func() {
		cancel()
		p.mu.Lock()
		delete(p.active[key], request)
		if len(p.active[key]) == 0 {
			delete(p.active, key)
		}
		p.mu.Unlock()
	}, nil
}

func (p *endpointPolicy) update(entry inferenceEndpointEnrollment, expected, actor string, enabled bool, authorize func() error) error {
	return p.updateDecision(entry, expected, actor, enabled, false, authorize)
}

func (p *endpointPolicy) revokeForOwner(entry inferenceEndpointEnrollment, expected, actor string, authorize func() error) error {
	return p.updateDecision(entry, expected, actor, false, true, authorize)
}

func (p *endpointPolicy) updateDecision(entry inferenceEndpointEnrollment, expected, actor string, enabled, reviewRequired bool, authorize func() error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	state, _, err := p.readLocked()
	if err != nil {
		return err
	}
	if state.Revision != expected {
		return errors.New("endpoint policy changed; refresh before reviewing")
	}
	if err := authorize(); err != nil {
		return err
	}
	if state.Epoch == "" {
		state.Epoch, err = storage.EnsureWorkspaceActivationEpoch(p.dataDir)
		if err != nil {
			return err
		}
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	state.Revision = hex.EncodeToString(nonce[:])
	if len(state.Decisions) >= 64 {
		if _, exists := state.Decisions[endpointKey(entry)]; !exists {
			// Missing decisions deny access. At capacity, automatic revocation
			// need not manufacture a new tombstone for an already denied identity.
			// Live request cancellation is still required if state was removed.
			if !enabled && reviewRequired {
				for request := range p.active[endpointKey(entry)] {
					request.cancel()
				}
				return nil
			}
			return errors.New("endpoint policy capacity reached; owner maintenance is required")
		}
	}
	state.Decisions[endpointKey(entry)] = endpointDecision{Pin: endpointPin(entry), Enabled: enabled, Actor: actor, At: time.Now().UTC().Format(time.RFC3339Nano), ReviewRequired: reviewRequired}
	if err := p.publishLocked(state); err != nil {
		return err
	}
	if !enabled {
		for request := range p.active[endpointKey(entry)] {
			request.cancel()
		}
	}
	return nil
}

// Caller holds p.mu. Every writer shares the same uncertain-publication fence.
func (p *endpointPolicy) publishLocked(state endpointPolicySnapshot) error {
	encoded, err := json.Marshal(state)
	if err != nil || len(encoded) > 64<<10 || validateEndpointPolicySnapshot(state) != nil {
		return errors.New("endpoint policy exceeds its valid storage bound")
	}
	write := p.writeSnapshot
	if write == nil {
		write = storage.WriteDurableJSON
	}
	err = write(filepath.Join(p.dataDir, endpointPolicyFilename), state)
	if err != nil {
		// Publication may have happened before durability confirmation failed.
		// Revoke in-memory authority conservatively until a clean service reload
		// can inspect the actual snapshot; never assume the old value survived.
		p.fault = errors.New("endpoint policy publication is uncertain; restart and review actual state")
		for _, requests := range p.active {
			for request := range requests {
				request.cancel()
			}
		}
		return errors.Join(errEndpointPolicyPublicationUncertain, err)
	}
	return nil
}
