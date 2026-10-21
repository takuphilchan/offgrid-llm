package inference

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// NativeTextQualification is a trusted service decision over current runtime,
// adapter, model, hardware, and workflow evidence. It is never model-authored.
// A nil policy denies all native execution, including text conversation.
type NativeTextQualification func(context.Context, *models.NativeTextLease, TrustedNativeWorker) error

type nativePackageSelection struct {
	id       string
	revision string
	worker   TrustedNativeWorker
}

// NativePackageRoutes binds package identities to provider factories. It does
// not authorize a Chat user or select a model on the user's behalf; those are
// service boundaries that run before Select. Every factory open rechecks the
// bytes and qualification rather than trusting a previously offered menu.
type NativePackageRoutes struct {
	store    *models.PackageStore
	adapters *models.AdapterRegistry
	workers  map[ProviderID]TrustedNativeWorker
	qualify  NativeTextQualification
	open     func(context.Context, *models.NativeTextLease, TrustedNativeWorker, []string) (ProviderRuntime, error)
	mu       sync.RWMutex
	routes   map[string]nativePackageSelection
}

func NewNativePackageRoutes(store *models.PackageStore, adapters *models.AdapterRegistry, workers map[ProviderID]TrustedNativeWorker, qualify NativeTextQualification) (*NativePackageRoutes, error) {
	if store == nil || adapters == nil {
		return nil, errors.New("native routing requires package storage and trusted adapters")
	}
	copyWorkers := make(map[ProviderID]TrustedNativeWorker, len(workers))
	for id, worker := range workers {
		if id != ProviderMLX && id != ProviderTransformers || worker.Provider != id {
			return nil, fmt.Errorf("invalid trusted worker provider %q", id)
		}
		worker.DependencyVersions = maps.Clone(worker.DependencyVersions)
		copyWorkers[id] = worker
	}
	routes := &NativePackageRoutes{store: store, adapters: adapters, workers: copyWorkers, qualify: qualify, routes: make(map[string]nativePackageSelection)}
	routes.open = func(ctx context.Context, lease *models.NativeTextLease, worker TrustedNativeWorker, env []string) (ProviderRuntime, error) {
		return startNativePackageProviderLease(ctx, lease, worker, env)
	}
	return routes, nil
}

// Select is metadata admission only. The provider factory repeats this entire
// check at the execution boundary, so a changed package or revoked evidence
// cannot turn a stale selection into a runnable process.
func (r *NativePackageRoutes) Select(ctx context.Context, id, revision string) (ProviderDescriptor, error) {
	if r.qualify == nil {
		return ProviderDescriptor{}, errors.New("native provider has no qualification policy")
	}
	lease, err := r.store.AcquireNativeText(ctx, id, revision, "native-route", r.adapters)
	if err != nil {
		return ProviderDescriptor{}, err
	}
	defer lease.Release()
	worker, ok := r.workers[ProviderID(lease.Engine)]
	if !ok {
		return ProviderDescriptor{}, errors.New("trusted native provider worker is unavailable")
	}
	if err := worker.Validate(ctx, lease); err != nil {
		return ProviderDescriptor{}, err
	}
	if err := r.qualify(ctx, lease, worker); err != nil {
		return ProviderDescriptor{}, fmt.Errorf("native provider is not qualified: %w", err)
	}
	descriptor := nativeTextDescriptor(lease)
	if err := descriptor.Validate(); err != nil {
		return ProviderDescriptor{}, err
	}
	r.mu.Lock()
	key := providerKey(descriptor)
	if _, exists := r.routes[key]; !exists && len(r.routes) >= 512 {
		r.mu.Unlock()
		return ProviderDescriptor{}, errors.New("native route limit reached")
	}
	r.routes[key] = nativePackageSelection{id: id, revision: revision, worker: worker}
	r.mu.Unlock()
	return descriptor, nil
}

func (r *NativePackageRoutes) openSelected(ctx context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
	r.mu.RLock()
	selection, ok := r.routes[providerKey(descriptor)]
	r.mu.RUnlock()
	if !ok {
		return nil, errors.New("native model was not selected from a qualified package")
	}
	lease, err := r.store.AcquireNativeText(ctx, selection.id, selection.revision, "native-provider", r.adapters)
	if err != nil {
		return nil, err
	}
	owned := true
	defer func() {
		if owned {
			lease.Release()
		}
	}()
	if err := selection.worker.Validate(ctx, lease); err != nil {
		return nil, err
	}
	if err := r.qualify(ctx, lease, selection.worker); err != nil {
		return nil, fmt.Errorf("native provider is not qualified: %w", err)
	}
	if !reflect.DeepEqual(nativeTextDescriptor(lease), descriptor) || !reflect.DeepEqual(r.workers[descriptor.Provider], selection.worker) {
		return nil, errors.New("native model or worker identity changed after selection")
	}
	runtime, err := r.open(ctx, lease, selection.worker, nil)
	if err != nil {
		if errors.Is(err, errProviderStopUncertain) {
			// The factory could not confirm that its just-started process is
			// gone. It already retained this lease; the route must not undo it.
			owned = false
		}
		if runtime != nil {
			if closeErr := runtime.Close(); closeErr != nil {
				// A failed close cannot prove the worker stopped reading the
				// package. Keep its lease rather than allowing replacement.
				owned = false
				return nil, errors.Join(err, fmt.Errorf("native worker stop uncertain: %w", closeErr))
			}
		}
		return nil, err
	}
	if runtime == nil {
		return nil, errors.New("native provider factory returned no runtime")
	}
	actual, err := runtime.Descriptor(ctx)
	if err != nil || !reflect.DeepEqual(actual, descriptor) {
		if closeErr := runtime.Close(); closeErr != nil {
			owned = false
			return nil, errors.Join(errors.New("native worker did not retain the selected model identity"), fmt.Errorf("native worker stop uncertain: %w", closeErr))
		}
		return nil, errors.New("native worker did not retain the selected model identity")
	}
	owned = false
	return &qualifiedNativeRuntime{inner: runtime, lease: lease, worker: selection.worker, qualify: r.qualify}, nil
}

type qualifiedNativeRuntime struct {
	inner    ProviderRuntime
	lease    *models.NativeTextLease
	worker   TrustedNativeWorker
	qualify  NativeTextQualification
	once     sync.Once
	closeErr error
}

func (p *qualifiedNativeRuntime) Descriptor(ctx context.Context) (ProviderDescriptor, error) {
	return p.inner.Descriptor(ctx)
}

func (p *qualifiedNativeRuntime) Resources(ctx context.Context) (ProviderResources, error) {
	if err := p.current(ctx); err != nil {
		return ProviderResources{}, err
	}
	return p.inner.Resources(ctx)
}

func (p *qualifiedNativeRuntime) current(ctx context.Context) error {
	if err := p.worker.Validate(ctx, p.lease); err != nil {
		return err
	}
	return p.qualify(ctx, p.lease, p.worker)
}

func (p *qualifiedNativeRuntime) Generate(ctx context.Context, req ProviderRequest, callback func(ProviderEvent) error) error {
	if err := p.current(ctx); err != nil {
		return fmt.Errorf("native qualification no longer current: %w", err)
	}
	return p.inner.Generate(ctx, req, callback)
}

func (p *qualifiedNativeRuntime) Cancel(ctx context.Context, cancellationID string) error {
	return p.inner.Cancel(ctx, cancellationID)
}

func (p *qualifiedNativeRuntime) Health(ctx context.Context) error {
	if err := p.current(ctx); err != nil {
		return err
	}
	return p.inner.Health(ctx)
}

func (p *qualifiedNativeRuntime) Close() error {
	p.once.Do(func() {
		p.closeErr = p.inner.Close()
		if p.closeErr == nil {
			p.lease.Release()
		}
	})
	return p.closeErr
}

// Register adds only the two trusted native engines to a fresh service-owned
// registry. Unknown package engines cannot add executable factories.
func (r *NativePackageRoutes) Register(registry *ProviderRegistry) error {
	if registry == nil {
		return errors.New("provider registry is required")
	}
	for _, id := range []ProviderID{ProviderMLX, ProviderTransformers} {
		provider := id
		if err := registry.Register(provider, func(ctx context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
			if descriptor.Provider != provider {
				return nil, errors.New("native route provider mismatch")
			}
			return r.openSelected(ctx, descriptor)
		}); err != nil {
			return err
		}
	}
	return nil
}
