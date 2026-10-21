package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// ProviderScheduler coordinates local inference admission and lifecycle.
// ModelCache implements the attached legacy llama process behind this boundary.
type ProviderScheduler struct {
	registry         *ProviderRegistry
	mu               sync.Mutex
	resident         map[string]ProviderRuntime
	residentMemory   map[string]ProviderResources
	active           map[string]int
	loading          map[string]*providerLoad
	retiring         int
	requests         map[string]*scheduledRequest
	pendingCancel    map[string]int
	cancelCh         chan struct{}
	requestIDs       map[string]struct{}
	idleCh           chan struct{}
	familyCh         chan struct{}
	legacyRuns       int
	legacyTimings    ProviderTiming
	legacySamples    uint64
	embeddingTimings ProviderTiming
	embeddingSamples uint64
	nativeRun        bool
	nativeWait       int
	loadLimit        chan struct{}
	limit            chan struct{}
	retireLegacy     func(context.Context) (bool, error)
	legacyShutdown   func() error
	legacyResident   func() int
	legacyMemory     func() LegacyProcessMemory
	legacyCache      *ModelCache
	legacyEngine     Engine
	embeddingOwner   *EmbeddingEngine
	closeCtx         context.Context
	closeRun         context.CancelFunc
	closed           bool
	unloadErr        error
	closeDone        chan struct{}
	closeErr         error
}

// legacyProcessLeaseKey is deliberately private. An attached llama process
// accepts lifecycle calls only from a scheduler-admitted operation, not from a
// service component merely holding its compatibility handle.
type legacyProcessLeaseKey struct{}

type legacyProcessLease struct {
	owner  *ProviderScheduler
	active atomic.Bool
}

func (s *ProviderScheduler) withLegacyProcessLease(ctx context.Context, action func(context.Context) error) error {
	lease := &legacyProcessLease{owner: s}
	lease.active.Store(true)
	defer lease.active.Store(false)
	return action(context.WithValue(ctx, legacyProcessLeaseKey{}, lease))
}

// AcquireGeneration admits an existing service-owned inference path to the
// same generation queue used by provider-backed requests. It does not open a
// runner or grant model/tool authority; callers still hold their own model
// lifecycle lease and must release this admission when generation ends.
func (s *ProviderScheduler) acquireSlot(ctx context.Context) (func(), time.Duration, error) {
	queued := time.Now()
	select {
	case s.limit <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-s.limit
			return nil, time.Since(queued), err
		}
		select {
		case <-s.closeCtx.Done():
			<-s.limit
			return nil, time.Since(queued), errors.New("provider scheduler is closed")
		default:
		}
		var once sync.Once
		return func() { once.Do(func() { <-s.limit }) }, time.Since(queued), nil
	case <-ctx.Done():
		return nil, time.Since(queued), ctx.Err()
	case <-s.closeCtx.Done():
		return nil, time.Since(queued), errors.New("provider scheduler is closed")
	}
}

// Native inference is exclusive until mixed-engine memory and throughput are
// qualified. Existing configured llama slots remain shared with each other.
// A waiting native request prevents new llama work from starving the switch.
func (s *ProviderScheduler) acquireFamily(ctx context.Context, native bool) (func(), time.Duration, error) {
	queued := time.Now()
	waitingNative := native
	if native {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, 0, errors.New("provider scheduler is closed")
		}
		s.nativeWait++
		s.signalFamilyLocked()
		s.mu.Unlock()
		defer func() {
			if waitingNative {
				s.mu.Lock()
				s.nativeWait--
				s.signalFamilyLocked()
				s.mu.Unlock()
			}
		}()
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, time.Since(queued), err
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, time.Since(queued), errors.New("provider scheduler is closed")
		}
		if native && !s.nativeRun && s.legacyRuns == 0 {
			s.nativeRun = true
			s.nativeWait--
			waitingNative = false
			s.signalFamilyLocked()
			s.mu.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					s.mu.Lock()
					s.nativeRun = false
					s.signalFamilyLocked()
					s.mu.Unlock()
				})
			}, time.Since(queued), nil
		}
		if !native && !s.nativeRun && s.nativeWait == 0 {
			s.legacyRuns++
			s.mu.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					s.mu.Lock()
					s.legacyRuns--
					s.signalFamilyLocked()
					s.mu.Unlock()
				})
			}, time.Since(queued), nil
		}
		changed := s.familyCh
		s.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, time.Since(queued), ctx.Err()
		case <-s.closeCtx.Done():
			return nil, time.Since(queued), errors.New("provider scheduler is closed")
		}
	}
}

func (s *ProviderScheduler) signalFamilyLocked() {
	close(s.familyCh)
	s.familyCh = make(chan struct{})
}

// AcquireGeneration admits the established llama path. Its lease holds family
// safety and one numeric slot until the request ends.
func (s *ProviderScheduler) AcquireGeneration(ctx context.Context) (func(), time.Duration, error) {
	release, timing, err := s.AcquireGenerationTimed(ctx)
	// The legacy caller has only an elapsed-wait field. Keep its historical
	// elapsed admission semantics while the typed path retains unload timing.
	return release, timing.Queue + timing.Unload, err
}

func (s *ProviderScheduler) AcquireGenerationTimed(ctx context.Context) (func(), ProviderTiming, error) {
	var timing ProviderTiming
	releaseFamily, familyQueue, err := s.acquireFamily(ctx, false)
	timing.Queue += familyQueue
	if err != nil {
		return nil, timing, err
	}
	releaseSlot, slotQueue, err := s.acquireSlot(ctx)
	timing.Queue += slotQueue
	if err != nil {
		releaseFamily()
		return nil, timing, err
	}
	// The legacy llama loader lives outside this scheduler. Before handing it
	// the generation lease, retire idle native workers so an unmeasured MLX or
	// PyTorch allocation cannot remain resident while llama loads its model.
	// Family admission prevents a native request from becoming active here.
	retireQueue, unload, err := s.retireNativeResidents(ctx)
	timing.Queue += retireQueue
	timing.Unload += unload
	if err != nil {
		releaseSlot()
		releaseFamily()
		return nil, timing, err
	}
	if err := ctx.Err(); err != nil {
		releaseSlot()
		releaseFamily()
		return nil, timing, err
	}
	select {
	case <-s.closeCtx.Done():
		releaseSlot()
		releaseFamily()
		return nil, timing, errors.New("provider scheduler is closed")
	default:
	}
	var once sync.Once
	return func() { once.Do(func() { releaseSlot(); releaseFamily() }) }, timing, nil
}

// AcquireLegacyMaintenance exclusively admits a llama process mutation such
// as a manual hot-swap. A normal generation lease may share configured llama
// slots; a process mutation cannot, because eviction could stop an in-flight
// generation even while another numeric slot is free.
func (s *ProviderScheduler) AcquireLegacyMaintenance(ctx context.Context) (func(), ProviderTiming, error) {
	var timing ProviderTiming
	releaseFamily, familyQueue, err := s.acquireFamily(ctx, true)
	timing.Queue += familyQueue
	if err != nil {
		return nil, timing, err
	}
	releaseSlot, slotQueue, err := s.acquireSlot(ctx)
	timing.Queue += slotQueue
	if err != nil {
		releaseFamily()
		return nil, timing, err
	}
	retireQueue, unload, err := s.retireNativeResidents(ctx)
	timing.Queue += retireQueue
	timing.Unload += unload
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		releaseSlot()
		releaseFamily()
		return nil, timing, err
	}
	var once sync.Once
	return func() { once.Do(func() { releaseSlot(); releaseFamily() }) }, timing, nil
}

// AcquireLegacyModel binds the established llama lifecycle gate to provider
// admission. The loader is invoked only by the lifecycle gate when a model or
// context switch is required. The caller retains the returned lease through
// generation, so native workers cannot start during a llama request. This
// preserves the current model cache and Engine API while moving ordering and
// load/queue accounting to the shared scheduler.
func (s *ProviderScheduler) AcquireLegacyModel(ctx context.Context, gate *LifecycleGate, model string, contextWindow int, alive func() bool, load func(context.Context, string) error) (func(), ProviderTiming, error) {
	var timing ProviderTiming
	if gate == nil || model == "" || load == nil {
		return nil, timing, errors.New("legacy model admission requires lifecycle gate, model identity, and loader")
	}
	releaseProvider, admission, err := s.AcquireGenerationTimed(ctx)
	timing = admission
	if err != nil {
		return nil, timing, err
	}
	if alive != nil && !alive() {
		gate.Invalidate(model)
	}
	var loading time.Duration
	waiting := time.Now()
	releaseLegacy, err := gate.AcquireWithContext(ctx, model, contextWindow, func(ctx context.Context, selected string) error {
		started := time.Now()
		loadErr := s.withLegacyProcessLease(ctx, func(admitted context.Context) error {
			return load(admitted, selected)
		})
		loading += time.Since(started)
		return loadErr
	})
	elapsed := time.Since(waiting)
	timing.Load += loading
	timing.Queue += elapsed - loading
	if err != nil {
		releaseProvider()
		return nil, timing, err
	}
	return func() { releaseLegacy(); releaseProvider() }, timing, nil
}

func (s *ProviderScheduler) retireNativeResidents(ctx context.Context) (time.Duration, time.Duration, error) {
	queued := time.Now()
	select {
	case s.loadLimit <- struct{}{}:
		defer func() { <-s.loadLimit }()
		_, unloaded, err := s.releaseIdleResidentsMatching(ctx, func(key string) bool {
			var descriptor ProviderDescriptor
			// Keys are encoded by providerKey. A malformed identity must not be
			// retained across the legacy load boundary.
			return json.Unmarshal([]byte(key), &descriptor) != nil || descriptor.Provider != ProviderLlamaCPP
		})
		return time.Since(queued) - unloaded, unloaded, err
	case <-ctx.Done():
		return time.Since(queued), 0, ctx.Err()
	case <-s.closeCtx.Done():
		return time.Since(queued), 0, errors.New("provider scheduler is closed")
	}
}

type providerLoad struct {
	done     chan struct{}
	err      error
	retiring bool
}

type scheduledRequest struct {
	cancel   context.CancelFunc
	runtime  ProviderRuntime
	cancelAt time.Time
	done     chan struct{}
}

type ProviderTiming struct {
	Load       time.Duration
	Queue      time.Duration
	Generation time.Duration
	Cancel     time.Duration
	Unload     time.Duration
	Resources  ProviderResources
}

func NewProviderScheduler(registry *ProviderRegistry, concurrentGenerations int) (*ProviderScheduler, error) {
	if registry == nil {
		return nil, errors.New("provider scheduler requires a registry")
	}
	if concurrentGenerations < 1 {
		concurrentGenerations = 1
	}
	closeCtx, closeRun := context.WithCancel(context.Background())
	return &ProviderScheduler{registry: registry, resident: make(map[string]ProviderRuntime), residentMemory: make(map[string]ProviderResources), active: make(map[string]int), loading: make(map[string]*providerLoad), requests: make(map[string]*scheduledRequest), pendingCancel: make(map[string]int), cancelCh: make(chan struct{}), requestIDs: make(map[string]struct{}), idleCh: make(chan struct{}), familyCh: make(chan struct{}), loadLimit: make(chan struct{}, 1), limit: make(chan struct{}, concurrentGenerations), closeCtx: closeCtx, closeRun: closeRun, closeDone: make(chan struct{})}, nil
}

// ConfigureLegacyRetirement connects the existing llama process owner to the
// provider admission boundary. It is a startup-only registration: changing it
// after requests or workers exist could allow mixed-engine residency.
func (s *ProviderScheduler) ConfigureLegacyRetirement(retire func(context.Context) (bool, error)) error {
	if retire == nil {
		return errors.New("legacy retirement callback is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.retireLegacy != nil || len(s.requests) != 0 || len(s.resident) != 0 || len(s.loading) != 0 || s.legacyRuns != 0 || s.nativeRun {
		return errors.New("legacy retirement can only be configured before provider activity")
	}
	s.retireLegacy = retire
	return nil
}

func providerKey(descriptor ProviderDescriptor) string {
	identity, _ := json.Marshal(descriptor)
	return string(identity)
}

func scheduledKey(descriptor ProviderDescriptor, id string) string {
	return providerKey(descriptor) + "\x00" + id
}

func (s *ProviderScheduler) Runtime(ctx context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	managed := s.retireLegacy != nil
	s.mu.Unlock()
	// Standalone schedulers retain the established direct-runtime probe
	// behavior. A service-owned scheduler has a legacy process owner and must
	// use the same family/retirement gate as an actual generation.
	if !managed {
		return s.runtimeWithTiming(ctx, descriptor, nil, false)
	}
	if descriptor.Provider == ProviderLlamaCPP {
		release, _, err := s.AcquireGenerationTimed(ctx)
		if err != nil {
			return nil, err
		}
		defer release()
	} else {
		releaseFamily, _, err := s.acquireFamily(ctx, true)
		if err != nil {
			return nil, err
		}
		defer releaseFamily()
		releaseSlot, _, err := s.acquireSlot(ctx)
		if err != nil {
			return nil, err
		}
		defer releaseSlot()
		if _, err := s.retireLegacyBeforeNative(ctx); err != nil {
			return nil, err
		}
	}
	return s.runtimeWithTiming(ctx, descriptor, nil, false)
}

func (s *ProviderScheduler) retireLegacyBeforeNative(ctx context.Context) (time.Duration, error) {
	s.mu.Lock()
	retire := s.retireLegacy
	embedding := s.embeddingOwner
	unresolved := s.unloadErr
	s.mu.Unlock()
	if unresolved != nil {
		return 0, fmt.Errorf("provider retirement is unresolved: %w", unresolved)
	}
	if retire == nil && embedding == nil {
		return 0, ctx.Err()
	}
	started := time.Now()
	var retired bool
	var err error
	if retire != nil {
		err = s.withLegacyProcessLease(ctx, func(admitted context.Context) (retireErr error) {
			retired, retireErr = retire(admitted)
			return retireErr
		})
	}
	if err == nil && embedding != nil {
		err = ctx.Err()
		if err == nil {
			var embeddingRetired bool
			embeddingRetired, err = embedding.RetireForProvider()
			retired = retired || embeddingRetired
		}
	}
	var duration time.Duration
	if retired {
		duration = time.Since(started)
	}
	if err != nil {
		if retired {
			// Partial retirement is unresolved. Pre-effect cancellation is safe
			// for a later, explicitly new request.
			s.recordUnloadFailure(err)
		}
		return duration, fmt.Errorf("retire llama runtime before native load: %w", err)
	}
	return duration, ctx.Err()
}

// reserve pins the runtime before returning to the generation caller. Without
// it, a different model can retire a just-opened worker between load and the
// caller's active counter update when more than one inference slot is enabled.
func (s *ProviderScheduler) runtimeWithTiming(ctx context.Context, descriptor ProviderDescriptor, timing *ProviderTiming, reserve bool) (ProviderRuntime, error) {
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	key := providerKey(descriptor)
	// A failed health check is pre-dispatch. One bounded replacement is safe;
	// a failed Generate is never replayed here because its effect is unknown.
	replacements := 0
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, errors.New("provider scheduler is closed")
		}
		if s.unloadErr != nil {
			err := s.unloadErr
			s.mu.Unlock()
			return nil, fmt.Errorf("provider retirement is unresolved: %w", err)
		}
		if runtime := s.resident[key]; runtime != nil {
			checked, retry, checkErr := s.checkResidentLocked(ctx, descriptor, key, runtime, reserve, &replacements)
			if retry {
				continue
			}
			return checked, checkErr
		}
		if pending := s.loading[key]; pending != nil {
			s.mu.Unlock()
			waiting := time.Now()
			select {
			case <-pending.done:
				if timing != nil {
					timing.Queue += time.Since(waiting)
				}
				if pending.err != nil {
					// A cancelled leader has not established that the model or
					// runner is unavailable. A still-live waiter may own the
					// next load attempt with its own context.
					if (errors.Is(pending.err, context.Canceled) || errors.Is(pending.err, context.DeadlineExceeded)) && ctx.Err() == nil {
						continue
					}
					return nil, pending.err
				}
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-s.closeCtx.Done():
				return nil, errors.New("provider scheduler is closed")
			}
		}
		pending := &providerLoad{done: make(chan struct{})}
		s.loading[key] = pending
		s.mu.Unlock()

		// On unqualified hardware even different model identities cannot load
		// simultaneously. Keep the pending entry so concurrent requests for the
		// same model share one load while requests for other models wait.
		var runtime ProviderRuntime
		var err error
		loadHeld := false
		loadCtx := ctx
		var cancelLoad context.CancelFunc
		var stopLoad func() bool
		var loadingStarted time.Time
		var resources ProviderResources
		waiting := time.Now()
		select {
		case s.loadLimit <- struct{}{}:
			loadHeld = true
			if timing != nil {
				timing.Queue += time.Since(waiting)
			}
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				err = errors.New("provider scheduler is closed")
			} else if err = ctx.Err(); err == nil {
				// Until concurrent model residency has measured memory evidence,
				// release idle workers before loading another model. An active
				// worker is never killed to make room for a different request.
				var queued, unloaded time.Duration
				queued, unloaded, err = s.releaseIdleResidents(ctx, key)
				if timing != nil {
					timing.Queue += queued
					timing.Unload += unloaded
				}
				if err == nil {
					loadCtx, cancelLoad = context.WithCancel(ctx)
					stopLoad = context.AfterFunc(s.closeCtx, cancelLoad)
					loadingStarted = time.Now()
					runtime, err = s.registry.Open(loadCtx, descriptor)
				}
			}
		case <-ctx.Done():
			err = ctx.Err()
		case <-s.closeCtx.Done():
			err = errors.New("provider scheduler is closed")
		}
		if err == nil && runtime == nil {
			err = errors.New("provider factory returned no runtime")
		}
		if err == nil {
			var actual ProviderDescriptor
			actual, err = runtime.Descriptor(loadCtx)
			if err == nil && !reflect.DeepEqual(actual, descriptor) {
				err = errors.New("provider runtime identity mismatch")
			}
			if err == nil {
				err = runtime.Health(loadCtx)
			}
			if err == nil {
				resources, err = runtime.Resources(loadCtx)
				if err == nil {
					err = resources.Validate(descriptor)
				}
			}
		}
		if err == nil {
			err = loadCtx.Err()
		}
		if timing != nil && !loadingStarted.IsZero() {
			timing.Load += time.Since(loadingStarted)
		}
		if stopLoad != nil {
			stopLoad()
			cancelLoad()
		}
		s.mu.Lock()
		if s.closed && err == nil {
			err = errors.New("provider scheduler closed during load")
		}
		if err == nil {
			err = s.accountResidentMemoryLocked(key, resources)
		}
		if err == nil {
			s.resident[key] = runtime
			if reserve {
				s.active[key]++
			}
		}
		s.mu.Unlock()
		if err != nil {
			if runtime != nil {
				if closeErr := runtime.Close(); closeErr != nil {
					s.recordUnloadFailure(closeErr)
					err = errors.Join(err, fmt.Errorf("failed to retire rejected provider: %w", closeErr))
				}
			}
		}
		// Keep the serialized load lease through handshake, residency
		// publication, and failed-worker cleanup. Otherwise a second model can
		// start while the first process exists but is not yet in resident.
		s.mu.Lock()
		pending.err = err
		delete(s.loading, key)
		close(pending.done)
		s.mu.Unlock()
		if loadHeld {
			<-s.loadLimit
		}
		if err != nil {
			return nil, fmt.Errorf("open %s runtime: %w", descriptor.Provider, err)
		}
		return runtime, nil
	}
}

func (s *ProviderScheduler) Generate(ctx context.Context, descriptor ProviderDescriptor, request ProviderRequest, callback func(ProviderEvent) error) error {
	_, err := s.GenerateTimed(ctx, descriptor, request, callback)
	return err
}

func (s *ProviderScheduler) GenerateTimed(ctx context.Context, descriptor ProviderDescriptor, request ProviderRequest, callback func(ProviderEvent) error) (timing ProviderTiming, err error) {
	if err := descriptor.Validate(); err != nil {
		return timing, err
	}
	if err := request.Validate(); err != nil {
		return timing, err
	}
	if request.ModelDigest != descriptor.ModelDigest {
		return timing, errors.New("provider request model does not match selected runtime")
	}
	if descriptor.ContextLength > 0 && request.ContextLength > descriptor.ContextLength {
		return timing, errors.New("provider request exceeds the selected runtime context")
	}
	if callback == nil {
		return timing, errors.New("provider callback is required")
	}
	runCtx, cancel := context.WithCancel(ctx)
	cancelKey := scheduledKey(descriptor, request.CancellationID)
	requestKey := scheduledKey(descriptor, request.RequestID)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		cancel()
		return timing, errors.New("provider scheduler is closed")
	}
	if s.requests[cancelKey] != nil || s.pendingCancel[cancelKey] != 0 {
		s.mu.Unlock()
		cancel()
		return timing, errors.New("provider cancellation identity is already active")
	}
	if _, duplicate := s.requestIDs[requestKey]; duplicate {
		s.mu.Unlock()
		cancel()
		return timing, errors.New("provider request identity is already active")
	}
	tracked := &scheduledRequest{cancel: cancel, done: make(chan struct{})}
	s.requests[cancelKey] = tracked
	s.requestIDs[requestKey] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if !tracked.cancelAt.IsZero() {
			timing.Cancel = time.Since(tracked.cancelAt)
		}
		delete(s.requests, cancelKey)
		delete(s.requestIDs, requestKey)
		close(tracked.done)
		s.mu.Unlock()
		cancel()
	}()
	// Family admission precedes the numeric slot. An unqualified native worker
	// cannot overlap legacy inference even if llama has parallel slots enabled.
	if descriptor.Provider != ProviderLlamaCPP {
		releaseFamily, familyQueue, familyErr := s.acquireFamily(runCtx, true)
		timing.Queue += familyQueue
		if familyErr != nil {
			return timing, familyErr
		}
		defer releaseFamily()
		releaseSlot, slotQueue, slotErr := s.acquireSlot(runCtx)
		timing.Queue += slotQueue
		if slotErr != nil {
			return timing, slotErr
		}
		defer releaseSlot()
		// The established llama binary manager is a separate process owner. A
		// native worker cannot load while one of its model processes remains
		// resident on hardware without mixed-engine memory qualification.
		unloaded, retireErr := s.retireLegacyBeforeNative(runCtx)
		timing.Unload += unloaded
		if retireErr != nil {
			return timing, retireErr
		}
	} else {
		releaseLegacy, admissionTiming, admissionErr := s.AcquireGenerationTimed(runCtx)
		timing.Queue += admissionTiming.Queue
		timing.Unload += admissionTiming.Unload
		if admissionErr != nil {
			return timing, admissionErr
		}
		defer releaseLegacy()
	}
	runtime, err := s.runtimeWithTiming(runCtx, descriptor, &timing, true)
	if err != nil {
		return timing, err
	}
	key := providerKey(descriptor)
	defer func() {
		s.mu.Lock()
		s.releaseActiveLocked(key)
		s.mu.Unlock()
	}()
	defer func() {
		if err == nil {
			return
		}
		if unknown, ok := s.invalidateResidentMemory(key, runtime); ok {
			timing.Resources = unknown
		}
	}()
	timing.Resources, err = runtime.Resources(runCtx)
	if err != nil {
		return timing, err
	}
	if err := timing.Resources.Validate(descriptor); err != nil {
		return timing, err
	}
	s.mu.Lock()
	if s.closed || s.resident[key] != runtime {
		s.mu.Unlock()
		return timing, errors.New("provider changed before generation dispatch; retry explicitly")
	}
	if err := runCtx.Err(); err != nil {
		s.mu.Unlock()
		return timing, err
	}
	if err := s.accountResidentMemoryLocked(key, timing.Resources); err != nil {
		s.mu.Unlock()
		return timing, err
	}
	s.requests[cancelKey].runtime = runtime
	s.mu.Unlock()
	generated := time.Now()
	stop := context.AfterFunc(s.closeCtx, cancel)
	stream := &providerStreamGate{descriptor: descriptor, request: request, callback: callback}
	err = runtime.Generate(runCtx, request, stream.accept)
	stop()
	timing.Generation = time.Since(generated)
	if err == nil {
		err = runCtx.Err()
	}
	if err == nil {
		var finalResources ProviderResources
		finalResources, err = runtime.Resources(runCtx)
		if err == nil {
			err = finalResources.Validate(descriptor)
		}
		if err == nil {
			timing.Resources = finalResources
			s.mu.Lock()
			if s.resident[key] == runtime {
				err = s.accountResidentMemoryLocked(key, finalResources)
			}
			s.mu.Unlock()
		}
	}
	if err == nil {
		err = stream.finish()
	}
	return timing, err
}

// Cancel never opens a provider. During load the caller must cancel the load
// context; a Stop request must not create a fresh inference process.
func (s *ProviderScheduler) Cancel(ctx context.Context, descriptor ProviderDescriptor, cancellationID string) error {
	if cancellationID == "" {
		return errors.New("cancellation identity is required")
	}
	s.mu.Lock()
	key := scheduledKey(descriptor, cancellationID)
	request := s.requests[key]
	var runtime ProviderRuntime
	if request != nil {
		if request.cancelAt.IsZero() {
			request.cancelAt = time.Now()
		}
		runtime = request.runtime
		if runtime != nil {
			s.pendingCancel[key]++
		}
	}
	s.mu.Unlock()
	if request != nil {
		request.cancel()
	}
	if runtime != nil {
		defer func() {
			s.mu.Lock()
			s.pendingCancel[key]--
			if s.pendingCancel[key] == 0 {
				delete(s.pendingCancel, key)
			}
			close(s.cancelCh)
			s.cancelCh = make(chan struct{})
			s.mu.Unlock()
		}()
		// Context cancellation is the immediate Stop path. Also send the
		// provider's request-scoped cancellation signal; keep a bound even if
		// the caller did not set one so Stop cannot wait on a stuck worker.
		// The Stop request's transport context may already be cancelled (for
		// example, a disconnected UI). Preserve its trace values but not its
		// cancellation so the worker still receives one bounded cancel signal.
		cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		return runtime.Cancel(cancelCtx, cancellationID)
	}
	return nil
}
