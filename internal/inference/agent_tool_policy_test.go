package inference

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestAgentToolPolicyRequiresCurrentWorkflowEvidence(t *testing.T) {
	ctx := context.Background()
	store := models.NewCapabilityEvidenceStore(t.TempDir())
	fingerprint := models.CapabilityFingerprint{ModelSHA256: strings.Repeat("a", 64), TemplateSHA256: strings.Repeat("b", 64), Quantization: "fixture", RuntimeBuild: "fixture", Adapter: NativeAgentToolAdapter, ContextWindow: 8192, Backend: "cpu", HardwareID: "fixture"}
	target := models.CapabilityEvidenceTarget{Fingerprint: fingerprint, SuiteSHA256: strings.Repeat("c", 64), Platform: "fixture"}
	fingerprint.RuntimeSHA256, fingerprint.RuntimeConfigSHA256 = strings.Repeat("e", 64), strings.Repeat("f", 64)
	target.Fingerprint = fingerprint
	lookup := func(_ context.Context, model string, _ int) (*models.CapabilityEvidenceTarget, error) {
		if model != "fixture" {
			return nil, nil
		}
		copy := target
		return &copy, nil
	}
	liveTarget := target
	liveCalls := 0
	var liveErr error
	policy, err := NewAgentToolPolicy(store, lookup, func(context.Context, string, int) (*models.CapabilityEvidenceTarget, error) {
		liveCalls++
		copy := liveTarget
		return &copy, liveErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if pin, err := policy.Select(ctx, "fixture", 8192); err != nil || pin != nil {
		t.Fatal("unmeasured enabled", pin, err)
	}
	e := models.CapabilityEvidence{Version: 1, ModelID: "fixture", CheckID: NativeAgentToolAdapter, Fingerprint: fingerprint, Outcome: "passed", Code: "fixture_passed", RecordedAt: time.Now()}
	if _, err = store.Record(ctx, e); err != nil {
		t.Fatal(err)
	}
	if pin, err := policy.Select(ctx, "fixture", 8192); err != nil || pin != nil {
		t.Fatal("smoke pass enabled workflows", pin, err)
	}
	// Synthetic database fixture, never installed as real model qualification.
	e.Qualification = &models.WorkflowQualification{SuiteSHA256: target.SuiteSHA256, Platform: target.Platform, ArtifactID: strings.Repeat("d", 64), Cases: 30, Repetitions: 3, Verified: 90}
	if _, err = store.Record(ctx, e); err != nil {
		t.Fatal(err)
	}
	pin, err := policy.Select(ctx, "fixture", 8192)
	if liveCalls != 0 {
		t.Fatal("admission measured or loaded a runtime outside its lease")
	}
	if err != nil || pin == nil || policy.Validate(ctx, "fixture", 8192, pin) != nil {
		t.Fatal(pin, err)
	}
	for _, change := range []func(*models.CapabilityFingerprint){
		func(f *models.CapabilityFingerprint) { f.RuntimeSHA256 = "" },
		func(f *models.CapabilityFingerprint) { f.RuntimeConfigSHA256 = "" },
		func(f *models.CapabilityFingerprint) { f.RuntimeSHA256 = strings.Repeat("1", 64) },
		func(f *models.CapabilityFingerprint) { f.RuntimeConfigSHA256 = strings.Repeat("2", 64) },
	} {
		change(&target.Fingerprint)
		if selected, err := policy.Select(ctx, "fixture", 8192); err != nil || selected != nil {
			t.Fatal("changed or missing runtime content selected", selected, err)
		}
		if policy.Validate(ctx, "fixture", 8192, pin) == nil {
			t.Fatal("pinned runtime content change accepted")
		}
		target.Fingerprint = fingerprint
	}
	liveTarget.Fingerprint.RuntimeBuild = "different-live-process"
	if policy.Validate(ctx, "fixture", 8192, pin) == nil {
		t.Fatal("installation record substituted for live measurement")
	}
	liveTarget = target
	liveErr = context.DeadlineExceeded
	if !errors.Is(policy.Validate(ctx, "fixture", 8192, pin), context.DeadlineExceeded) {
		t.Fatal("runtime unavailability was not preserved")
	}
	liveErr = nil
	if policy.Validate(ctx, "fixture", 8192, pin) != nil {
		t.Fatal("operational timeout changed qualification evidence")
	}
	if policy.Validate(ctx, "other", 8192, pin) == nil {
		t.Fatal("cross-model pin accepted")
	}
	if policy.Validate(ctx, "fixture", 4096, pin) == nil {
		t.Fatal("changed context accepted")
	}
	target.Fingerprint.RuntimeBuild = "changed"
	if policy.Validate(ctx, "fixture", 8192, pin) == nil {
		t.Fatal("stale runtime accepted")
	}
	target.Fingerprint = fingerprint
	e.Outcome = "failed"
	e.Code = "fixture_failed"
	e.RecordedAt = time.Now()
	if _, err = store.Record(ctx, e); err != nil {
		t.Fatal(err)
	}
	if policy.Validate(ctx, "fixture", 8192, pin) == nil {
		t.Fatal("superseded pass accepted")
	}
	var absent *AgentToolPolicy
	if absent.Validate(ctx, "fixture", 8192, pin) == nil {
		t.Fatal("missing installed policy accepted")
	}
	if absent.Validate(ctx, "fixture", 8192, nil) != nil {
		t.Fatal("old job changed")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := policy.Select(cancelled, "fixture", 8192); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestAgentToolPolicyRequiresIndependentLiveMeasurement(t *testing.T) {
	lookup := func(context.Context, string, int) (*models.CapabilityEvidenceTarget, error) { return nil, nil }
	if _, err := NewAgentToolPolicy(models.NewCapabilityEvidenceStore(t.TempDir()), lookup, nil); err == nil {
		t.Fatal("installation-only policy accepted")
	}
}

func TestAgentToolPolicyPinsScopeAndRejectsCrossScopeReuse(t *testing.T) {
	ctx := context.Background()
	store := models.NewCapabilityEvidenceStore(t.TempDir())
	f := models.CapabilityFingerprint{ModelSHA256: strings.Repeat("a", 64), TemplateSHA256: strings.Repeat("b", 64), RuntimeSHA256: strings.Repeat("c", 64), RuntimeConfigSHA256: strings.Repeat("d", 64), Quantization: "fixture", RuntimeBuild: "fixture", Adapter: NativeAgentToolAdapter, ContextWindow: 8192, Backend: "cpu", HardwareID: "fixture"}
	target := models.CapabilityEvidenceTarget{Fingerprint: f, Scope: models.NativeWorkspaceScope, SuiteSHA256: models.NativeWorkspaceSuiteSHA256(), CapabilitySHA256: strings.Repeat("e", 64), Platform: "fixture"}
	q := &models.WorkflowQualification{Scope: target.Scope, SuiteSHA256: target.SuiteSHA256, CapabilitySHA256: target.CapabilitySHA256, Platform: target.Platform, ArtifactID: strings.Repeat("f", 64), Cases: len(models.NativeWorkspaceCases()), Repetitions: 3, Verified: 3 * len(models.NativeWorkspaceCases())}
	for _, c := range models.NativeWorkspaceCases() {
		for n := 1; n <= 3; n++ {
			q.Results = append(q.Results, models.WorkflowCaseResult{CaseID: c.ID, Repetition: n, ArtifactID: strings.Repeat("f", 64), Verified: true})
		}
	}
	// Synthetic fixture only, never a real runtime qualification record.
	e := models.CapabilityEvidence{Version: 1, ModelID: "fixture", CheckID: NativeAgentToolAdapter, Fingerprint: f, Outcome: "passed", Code: "fixture", RecordedAt: time.Now(), Qualification: q}
	if _, err := store.Record(ctx, e); err != nil {
		t.Fatal(err)
	}
	lookup := func(context.Context, string, int) (*models.CapabilityEvidenceTarget, error) {
		copy := target
		return &copy, nil
	}
	p, err := NewAgentToolPolicy(store, lookup, lookup)
	if err != nil {
		t.Fatal(err)
	}
	pin, err := p.Select(ctx, "fixture", 8192)
	if err != nil || pin == nil || pin.Scope != target.Scope || pin.CapabilitySHA256 != target.CapabilitySHA256 || p.Validate(ctx, "fixture", 8192, pin) != nil {
		t.Fatal(pin, err)
	}
	for _, change := range []func(*AgentToolPin){func(x *AgentToolPin) { x.Scope = "" }, func(x *AgentToolPin) { x.CapabilitySHA256 = strings.Repeat("0", 64) }} {
		forged := *pin
		change(&forged)
		if p.Validate(ctx, "fixture", 8192, &forged) == nil {
			t.Fatal("scope pin expansion accepted")
		}
	}
	target.Scope = ""
	if next, err := p.Select(ctx, "fixture", 8192); err != nil || next != nil {
		t.Fatal("scoped evidence selected general tools", next, err)
	}
}
