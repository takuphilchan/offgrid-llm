package inference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// TrustedNativeWorker is service-owned installation metadata, never copied
// from a model package or a model response. A revision is meaningful only when
// both the interpreter and the worker script match their reviewed digests.
type TrustedNativeWorker struct {
	Provider       ProviderID
	RunnerRevision string
	PythonPath     string
	PythonSHA256   string
	ScriptPath     string
	ScriptSHA256   string
	// DependencyVersions is installation-owned, not model-package metadata.
	// The worker verifies these exact installed distribution versions before
	// importing MLX or PyTorch; the canonical digest binds qualification.
	DependencyVersions map[string]string
}

var nativeDependencyVersion = regexp.MustCompile(`^[0-9][0-9A-Za-z.!+_-]*$`)
var nativeDependencyName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)

func (w TrustedNativeWorker) dependencyIdentity() (string, error) {
	var required []string
	switch w.Provider {
	case ProviderMLX:
		required = []string{"mlx", "mlx-lm", "transformers"}
	case ProviderTransformers:
		required = []string{"torch", "transformers", "safetensors"}
	default:
		return "", errors.New("unsupported native worker dependencies")
	}
	if len(w.DependencyVersions) < len(required) {
		return "", errors.New("native worker dependency versions are incomplete")
	}
	for _, name := range required {
		if !nativeDependencyVersion.MatchString(w.DependencyVersions[name]) {
			return "", fmt.Errorf("native worker requires pinned %s version", name)
		}
	}
	names := make([]string, 0, len(w.DependencyVersions))
	for name, version := range w.DependencyVersions {
		if !nativeDependencyName.MatchString(name) || !nativeDependencyVersion.MatchString(version) {
			return "", errors.New("invalid native worker dependency pin")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	hash := sha256.New()
	for _, name := range names {
		_, _ = io.WriteString(hash, name+"="+w.DependencyVersions[name]+"\n")
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func verifyTrustedWorkerFile(ctx context.Context, filename, expected string) error {
	if !filepath.IsAbs(filename) || !completeSHA256Digest(expected) {
		return errors.New("trusted worker requires an absolute path and pinned SHA-256")
	}
	info, err := os.Lstat(filename)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("trusted worker path is not a regular file")
	}
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	buffer := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := f.Read(buffer)
		if n > 0 {
			if _, err := hash.Write(buffer[:n]); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if "sha256:"+hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("trusted worker digest changed")
	}
	return nil
}

func (w TrustedNativeWorker) Validate(ctx context.Context, lease *models.NativeTextLease) error {
	if lease == nil || w.Provider != ProviderID(lease.Engine) || w.RunnerRevision != lease.RunnerRevision || w.RunnerRevision == "" {
		return errors.New("trusted worker does not match the pinned package runtime")
	}
	return w.ValidateIdentity(ctx)
}

// ValidateIdentity verifies installation-owned worker bytes independently of
// any imported model package. Selection repeats the check with a model lease.
func (w TrustedNativeWorker) ValidateIdentity(ctx context.Context) error {
	if (w.Provider != ProviderMLX && w.Provider != ProviderTransformers) || w.RunnerRevision == "" {
		return errors.New("trusted worker provider and runner revision are required")
	}
	if _, err := w.dependencyIdentity(); err != nil {
		return err
	}
	for _, item := range []struct{ path, digest string }{{w.PythonPath, w.PythonSHA256}, {w.ScriptPath, w.ScriptSHA256}} {
		if err := verifyTrustedWorkerFile(ctx, item.path, item.digest); err != nil {
			return fmt.Errorf("verify trusted native worker: %w", err)
		}
	}
	return nil
}

func nativeTextDescriptor(lease *models.NativeTextLease) ProviderDescriptor {
	return ProviderDescriptor{
		ProtocolRevision: ProviderProtocolRevision,
		Provider:         ProviderID(lease.Engine),
		RunnerRevision:   lease.RunnerRevision,
		AdapterRevision:  lease.AdapterRevision,
		ModelDigest:      lease.ModelDigest,
		Format:           ModelFormat(lease.Format),
		Backend:          lease.Backend,
		Capabilities:     []ProviderCapability{CapabilityText, CapabilityStreaming},
		ContextLength:    lease.ContextLength,
		TemplateDigest:   lease.TemplateDigest,
	}
}

// StartNativePackageProvider is the trusted model-package entry point for MLX
// and Transformers workers. It acquires an immutable package lease and a pinned
// model-family adapter before a process can start. A matching backend and
// hardware qualification are still required for production selection.
func StartNativePackageProvider(ctx context.Context, store *models.PackageStore, adapters *models.AdapterRegistry, id, revision, consumer string, worker TrustedNativeWorker, env []string) (*ProcessProvider, error) {
	if store == nil || adapters == nil {
		return nil, errors.New("native provider requires package storage and trusted adapters")
	}
	lease, err := store.AcquireNativeText(ctx, id, revision, consumer, adapters)
	if err != nil {
		return nil, err
	}
	return startNativePackageProviderLease(ctx, lease, worker, env)
}

// startNativePackageProviderLease takes ownership of the supplied verified
// lease. It releases it on startup failure or when the process closes.
func startNativePackageProviderLease(ctx context.Context, lease *models.NativeTextLease, worker TrustedNativeWorker, env []string) (*ProcessProvider, error) {
	if lease == nil {
		return nil, errors.New("native provider requires a verified package lease")
	}
	if err := worker.Validate(ctx, lease); err != nil {
		lease.Release()
		return nil, err
	}
	providerID := ProviderID(lease.Engine)
	descriptor := nativeTextDescriptor(lease)
	if err := descriptor.Validate(); err != nil {
		lease.Release()
		return nil, err
	}
	var provider *ProcessProvider
	var err error
	switch providerID {
	case ProviderMLX:
		provider, err = NewMLXProcessProvider(ctx, worker.PythonPath, worker.ScriptPath, lease.ModelDir, descriptor, worker.DependencyVersions, env)
	case ProviderTransformers:
		provider, err = NewTransformersProcessProvider(ctx, worker.PythonPath, worker.ScriptPath, lease.ModelDir, descriptor, worker.DependencyVersions, env)
	default:
		err = errors.New("native provider requires MLX or Transformers")
	}
	if err != nil {
		if !errors.Is(err, errProviderStopUncertain) {
			lease.Release()
		}
		return nil, err
	}
	provider.release = lease.Release
	return provider, nil
}
