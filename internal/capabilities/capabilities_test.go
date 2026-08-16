package capabilities

import (
	"context"
	"errors"
	"testing"
)

func TestDefaultPolicyRequiresApprovalForHighRisk(t *testing.T) {
	broker := NewBroker(nil)
	descriptor := Descriptor{Name: "shell", Kind: Execute, Risk: RiskHigh}
	if err := broker.Register(descriptor); err != nil {
		t.Fatal(err)
	}
	_, err := broker.Authorize(context.Background(), Request{Capability: descriptor})
	if !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expected approval requirement, got %v", err)
	}
	if _, err := broker.Authorize(context.Background(), Request{Capability: descriptor, Approved: true}); err != nil {
		t.Fatalf("approved request rejected: %v", err)
	}
}
