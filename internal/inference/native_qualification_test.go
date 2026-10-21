package inference

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestQualifiedNativeTextPolicyRequiresExactCurrentWorkflowEvidence(t *testing.T) {
	ctx := context.Background()
	store := models.NewCapabilityEvidenceStore(t.TempDir())
	digest := func(ch string) string { return strings.Repeat(ch, 64) }
	lease := &models.NativeTextLease{
		ID: "fixture", Revision: "r1", ModelDigest: "sha256:" + digest("a"),
		ModelFamily: "fixture", Engine: string(ProviderTransformers), Format: string(FormatSafetensors),
		Backend: "cpu", RunnerRevision: "runner1", AdapterRevision: "adapter1", ContextLength: 2048,
		TemplateDigest: digest("b"),
	}
	worker := TrustedNativeWorker{
		Provider: ProviderTransformers, RunnerRevision: "runner1",
		PythonSHA256: "sha256:" + digest("c"), ScriptSHA256: "sha256:" + digest("d"),
		DependencyVersions: map[string]string{"torch": "1.0", "transformers": "1.0", "safetensors": "1.0"},
	}
	dependencyDigest, err := worker.dependencyIdentity()
	if err != nil {
		t.Fatal(err)
	}
	target := NativeTextEvidenceTarget{HardwareID: "test-cpu", Backend: "cpu", Platform: "linux-test", SuiteSHA256: digest("e")}
	qualify := QualifiedNativeTextPolicy(store, target)
	if err := qualify(ctx, lease, worker); err == nil {
		t.Fatal("unmeasured native provider was admitted")
	}
	fingerprint := models.CapabilityFingerprint{
		ModelSHA256: digest("a"), Quantization: string(FormatSafetensors),
		RuntimeBuild:   worker.RunnerRevision + ":" + worker.PythonSHA256 + ":" + worker.ScriptSHA256 + ":" + dependencyDigest,
		TemplateSHA256: digest("b"), Adapter: "fixture@adapter1", ContextWindow: 2048,
		Backend: "cpu", HardwareID: "test-cpu",
	}
	evidence := models.CapabilityEvidence{
		Version: models.CapabilityEvidenceVersion, ModelID: "fixture@r1", CheckID: "native-text-v1",
		Fingerprint: fingerprint, Outcome: "passed", Code: "workflow_passed", RecordedAt: time.Now().UTC(),
		Qualification: &models.WorkflowQualification{
			SuiteSHA256: digest("e"), Platform: "linux-test", ArtifactID: digest("f"),
			Cases: 30, Repetitions: 3, Verified: 81,
		},
	}
	if _, err := store.Record(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	if err := qualify(ctx, lease, worker); err != nil {
		t.Fatalf("exact qualification was rejected: %v", err)
	}
	targets := map[NativeTextTargetKey]NativeTextEvidenceTarget{{Provider: ProviderTransformers, Backend: "cpu"}: target}
	multi, err := QualifiedNativeTextTargets(store, targets)
	if err != nil || multi(ctx, lease, worker) != nil {
		t.Fatalf("provider/backend-specific qualification rejected exact evidence: %v", err)
	}
	delete(targets, NativeTextTargetKey{Provider: ProviderTransformers, Backend: "cpu"})
	if err := multi(ctx, lease, worker); err != nil {
		t.Fatalf("caller mutated installed qualification targets: %v", err)
	}
	wrongProvider, err := QualifiedNativeTextTargets(store, map[NativeTextTargetKey]NativeTextEvidenceTarget{{Provider: ProviderMLX, Backend: "cpu"}: target})
	if err != nil || wrongProvider(ctx, lease, worker) == nil {
		t.Fatalf("other provider's qualification target authorized Transformers: %v", err)
	}
	if _, err := QualifiedNativeTextTargets(store, map[NativeTextTargetKey]NativeTextEvidenceTarget{{Provider: ProviderTransformers, Backend: "cuda"}: target}); err == nil {
		t.Fatal("mismatched backend target installed")
	}
	changed := *lease
	changed.TemplateDigest = digest("0")
	if err := qualify(ctx, &changed, worker); err == nil {
		t.Fatal("changed template retained qualification")
	}
	changed = *lease
	changed.Revision = "r2"
	if err := qualify(ctx, &changed, worker); err == nil {
		t.Fatal("different package revision retained qualification")
	}
	changedWorker := worker
	changedWorker.ScriptSHA256 = "sha256:" + digest("0")
	if err := qualify(ctx, lease, changedWorker); err == nil {
		t.Fatal("changed worker retained qualification")
	}
	changedWorker = worker
	changedWorker.DependencyVersions = map[string]string{"torch": "2.0", "transformers": "1.0", "safetensors": "1.0"}
	if err := qualify(ctx, lease, changedWorker); err == nil {
		t.Fatal("changed native library version retained qualification")
	}
	wrongHardware := target
	wrongHardware.HardwareID = "different-cpu"
	if err := QualifiedNativeTextPolicy(store, wrongHardware)(ctx, lease, worker); err == nil {
		t.Fatal("different hardware retained qualification")
	}
	wrongSuite := target
	wrongSuite.SuiteSHA256 = digest("0")
	if err := QualifiedNativeTextPolicy(store, wrongSuite)(ctx, lease, worker); err == nil {
		t.Fatal("different evaluation suite retained qualification")
	}
	wrongBackend := target
	wrongBackend.Backend = "cuda"
	if err := QualifiedNativeTextPolicy(store, wrongBackend)(ctx, lease, worker); err == nil {
		t.Fatal("different backend retained qualification")
	}
	if err := QualifiedNativeTextPolicy(nil, target)(ctx, lease, worker); err == nil {
		t.Fatal("missing evidence store admitted native provider")
	}
}
