package inference

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// NativeTextEvidenceTarget is measured by trusted service/runtime code. It is
// not populated from a package manifest, user request, or worker response.
// A missing target cannot authorize native execution.
type NativeTextEvidenceTarget struct {
	HardwareID  string
	Backend     string
	Platform    string
	SuiteSHA256 string
}

// NativeTextTargetKey prevents one provider/backend's measured profile from
// authorizing another. A service may install several independently qualified
// local runtimes without treating a single machine-wide target as universal.
type NativeTextTargetKey struct {
	Provider ProviderID
	Backend  string
}

func (t NativeTextEvidenceTarget) Validate() error {
	if t.HardwareID == "" || t.Backend == "" || t.Platform == "" || !completeSHA256Digest("sha256:"+t.SuiteSHA256) {
		return errors.New("native text qualification target is incomplete")
	}
	return nil
}

// QualifiedNativeTextTargets freezes the reviewed installation's target set.
// An absent provider/backend pair is unavailable, even if another pair has
// valid evidence for the same model files or machine.
func QualifiedNativeTextTargets(store *models.CapabilityEvidenceStore, targets map[NativeTextTargetKey]NativeTextEvidenceTarget) (NativeTextQualification, error) {
	if store == nil || len(targets) == 0 {
		return nil, errors.New("native text qualification targets are required")
	}
	selected := maps.Clone(targets)
	for key, target := range selected {
		if key.Provider != ProviderMLX && key.Provider != ProviderTransformers || key.Backend == "" || key.Backend != target.Backend {
			return nil, errors.New("native text qualification target key is invalid")
		}
		if err := target.Validate(); err != nil {
			return nil, err
		}
	}
	return func(ctx context.Context, lease *models.NativeTextLease, worker TrustedNativeWorker) error {
		if lease == nil || worker.Provider != ProviderID(lease.Engine) {
			return errors.New("native text qualification provider identity is invalid")
		}
		target, ok := selected[NativeTextTargetKey{Provider: worker.Provider, Backend: lease.Backend}]
		if !ok {
			return errors.New("native text provider/backend has no qualified target")
		}
		return QualifiedNativeTextPolicy(store, target)(ctx, lease, worker)
	}, nil
}

// QualifiedNativeTextPolicy checks the latest retained evaluation for the
// exact package, reviewed worker, adapter, template, hardware, and suite.
// The evidence store is not a user permission check; callers must separately
// authorize the request and verify its selected model/profile.
func QualifiedNativeTextPolicy(store *models.CapabilityEvidenceStore, target NativeTextEvidenceTarget) NativeTextQualification {
	return func(ctx context.Context, lease *models.NativeTextLease, worker TrustedNativeWorker) error {
		if store == nil || lease == nil || target.Validate() != nil {
			return errors.New("native text qualification target is incomplete")
		}
		if lease.Backend != target.Backend || lease.ID == "" || lease.Revision == "" || lease.TemplateDigest == "" || worker.RunnerRevision != lease.RunnerRevision {
			return errors.New("native text qualification identity does not match the selected package")
		}
		if !completeSHA256Digest(lease.ModelDigest) || !completeSHA256Digest("sha256:"+lease.TemplateDigest) {
			return errors.New("native text qualification requires complete model and template digests")
		}
		dependencyDigest, err := worker.dependencyIdentity()
		if err != nil {
			return err
		}
		fingerprint := models.CapabilityFingerprint{
			ModelSHA256:    strings.TrimPrefix(lease.ModelDigest, "sha256:"),
			Quantization:   lease.Format,
			RuntimeBuild:   fmt.Sprintf("%s:%s:%s:%s", worker.RunnerRevision, worker.PythonSHA256, worker.ScriptSHA256, dependencyDigest),
			TemplateSHA256: lease.TemplateDigest,
			Adapter:        lease.ModelFamily + "@" + lease.AdapterRevision,
			ContextWindow:  lease.ContextLength,
			Backend:        lease.Backend,
			HardwareID:     target.HardwareID,
		}
		if _, err := fingerprint.ID(); err != nil {
			return fmt.Errorf("native text fingerprint is incomplete: %w", err)
		}
		view, err := store.Latest(ctx, lease.ID+"@"+lease.Revision, "native-text-v1", models.CapabilityEvidenceTarget{
			Fingerprint: fingerprint,
			SuiteSHA256: target.SuiteSHA256,
			Platform:    target.Platform,
		})
		if err != nil {
			return err
		}
		if view.State != "workflow_qualified" {
			return fmt.Errorf("native text workflow is %s", view.State)
		}
		return nil
	}
}
