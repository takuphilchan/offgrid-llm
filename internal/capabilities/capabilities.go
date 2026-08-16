// Package capabilities defines the authorization boundary shared by agent,
// MCP, computer-use, network, and model-management actions.
package capabilities

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Kind string

const (
	Read     Kind = "read"
	Write    Kind = "write"
	Execute  Kind = "execute"
	Network  Kind = "network"
	Computer Kind = "computer_use"
	Model    Kind = "model_management"
)

type Risk string

const (
	RiskLow      Risk = "low"
	RiskMedium   Risk = "medium"
	RiskHigh     Risk = "high"
	RiskCritical Risk = "critical"
)

type Decision string

const (
	Allow           Decision = "allow"
	Deny            Decision = "deny"
	RequireApproval Decision = "require_approval"
)

type Descriptor struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Source      string `json:"source"`
	Kind        Kind   `json:"kind"`
	Risk        Risk   `json:"risk"`
	Description string `json:"description,omitempty"`
}

type Request struct {
	RunID      string         `json:"run_id,omitempty"`
	Actor      string         `json:"actor,omitempty"`
	Capability Descriptor     `json:"capability"`
	Arguments  map[string]any `json:"arguments,omitempty"`
	Approved   bool           `json:"approved"`
}

type Result struct {
	Decision Decision `json:"decision"`
	Reason   string   `json:"reason,omitempty"`
}

type Policy interface {
	Evaluate(context.Context, Request) Result
}

// DefaultPolicy allows read-only and medium-risk local operations, requires an
// explicit approval for high-risk actions, and denies critical actions.
type DefaultPolicy struct{}

func (DefaultPolicy) Evaluate(_ context.Context, request Request) Result {
	if request.Approved && request.Capability.Risk != RiskCritical {
		return Result{Decision: Allow, Reason: "explicitly approved"}
	}
	switch request.Capability.Risk {
	case RiskHigh:
		return Result{Decision: RequireApproval, Reason: "high-risk capability requires approval"}
	case RiskCritical:
		return Result{Decision: Deny, Reason: "critical capability is denied by default"}
	default:
		return Result{Decision: Allow}
	}
}

var (
	ErrDenied           = errors.New("capability denied")
	ErrApprovalRequired = errors.New("capability approval required")
)

type Broker struct {
	mu          sync.RWMutex
	descriptors map[string]Descriptor
	policy      Policy
}

func NewBroker(policy Policy) *Broker {
	if policy == nil {
		policy = DefaultPolicy{}
	}
	return &Broker{descriptors: make(map[string]Descriptor), policy: policy}
}

func (b *Broker) Register(descriptor Descriptor) error {
	if descriptor.Name == "" || descriptor.Kind == "" || descriptor.Risk == "" {
		return fmt.Errorf("capability name, kind, and risk are required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.descriptors[descriptor.Name]; exists {
		return fmt.Errorf("capability already registered: %s", descriptor.Name)
	}
	b.descriptors[descriptor.Name] = descriptor
	return nil
}

func (b *Broker) Resolve(name string) (Descriptor, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	descriptor, ok := b.descriptors[name]
	return descriptor, ok
}

func (b *Broker) Authorize(ctx context.Context, request Request) (Result, error) {
	result := b.policy.Evaluate(ctx, request)
	switch result.Decision {
	case Allow:
		return result, nil
	case RequireApproval:
		return result, fmt.Errorf("%w: %s", ErrApprovalRequired, result.Reason)
	default:
		return result, fmt.Errorf("%w: %s", ErrDenied, result.Reason)
	}
}
