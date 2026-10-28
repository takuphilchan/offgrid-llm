package inference

import (
	"context"
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

const NativeAgentToolAdapter = "native-agent-tools-v1"

// AgentToolPin is persisted before inference, never inferred from model output.
// It records compatibility evidence, not authority to execute any tool.
type AgentToolPin struct {
	Adapter          string `json:"adapter"`
	EvidenceID       string `json:"evidence_id"`
	FingerprintID    string `json:"fingerprint_id"`
	Scope            string `json:"scope,omitempty"`
	CapabilitySHA256 string `json:"capability_sha256,omitempty"`
}

// AgentToolTarget reads trusted installation/runtime measurements. A nil target
// means this model/context is not selected for native tools. Implementations must
// not derive qualification from model declarations or user request fields.
type AgentToolTarget func(context.Context, string, int) (*models.CapabilityEvidenceTarget, error)

type AgentToolPolicy struct {
	store  *models.CapabilityEvidenceStore
	target AgentToolTarget
	live   AgentToolTarget
}

// target resolves reviewed installation identity without loading a model. live
// measures the active runtime under its inference lease. Keeping these separate
// avoids treating installation metadata as proof of the process about to run.
func NewAgentToolPolicy(store *models.CapabilityEvidenceStore, target, live AgentToolTarget) (*AgentToolPolicy, error) {
	if store == nil || target == nil || live == nil {
		return nil, errors.New("native agent tools require installation and live measurements and an evidence store")
	}
	return &AgentToolPolicy{store: store, target: target, live: live}, nil
}

func (p *AgentToolPolicy) Select(ctx context.Context, model string, window int) (*AgentToolPin, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	target, err := p.target(ctx, model, window)
	if err != nil || target == nil {
		return nil, err
	}
	return p.pin(ctx, model, window, target)
}

func (p *AgentToolPolicy) pin(ctx context.Context, model string, window int, target *models.CapabilityEvidenceTarget) (*AgentToolPin, error) {
	if target.Fingerprint.Adapter != NativeAgentToolAdapter || target.Fingerprint.ContextWindow != window {
		return nil, errors.New("native agent target does not match the adapter and context")
	}
	// Older build-string-only evidence remains inspectable, but cannot select
	// native execution. A pinned job fails closed through Validate rather than
	// silently changing its adapter when these measurements disappear.
	if target.Fingerprint.RuntimeSHA256 == "" || target.Fingerprint.RuntimeConfigSHA256 == "" {
		return nil, nil
	}
	id, err := target.Fingerprint.ID()
	if err != nil {
		return nil, err
	}
	view, err := p.store.Latest(ctx, model, NativeAgentToolAdapter, *target)
	if err != nil {
		return nil, err
	}
	if view.State != "workflow_qualified" && view.State != "scope_qualified" {
		return nil, nil
	}
	return &AgentToolPin{Adapter: NativeAgentToolAdapter, EvidenceID: view.EvidenceID, FingerprintID: id, Scope: target.Scope, CapabilitySHA256: target.CapabilitySHA256}, nil
}

func (p *AgentToolPolicy) Validate(ctx context.Context, model string, window int, pinned *AgentToolPin) error {
	if pinned == nil {
		return nil
	}
	if pinned.Adapter != NativeAgentToolAdapter {
		return errors.New("unsupported pinned agent tool adapter")
	}
	current, err := p.Select(ctx, model, window)
	if err != nil {
		return err
	}
	if current == nil || *current != *pinned {
		return errors.New("native agent tool qualification changed or is unavailable; no new decision was dispatched")
	}
	actual, err := p.live(ctx, model, window)
	if err != nil {
		return err
	}
	if actual == nil {
		return errors.New("native agent runtime measurement is unavailable; no new decision was dispatched")
	}
	observed, err := p.pin(ctx, model, window, actual)
	if err != nil {
		return err
	}
	if observed == nil || *observed != *pinned {
		return errors.New("active native agent runtime does not match its pinned qualification; no new decision was dispatched")
	}
	return nil
}
