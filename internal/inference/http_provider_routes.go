package inference

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// HTTPEndpointRoutes is populated only by trusted service startup. Factories
// recheck current authority at execution; selection alone confers no access.
type HTTPEndpointRoutes struct{ endpoints map[string]HTTPEndpoint }

func NewHTTPEndpointRoutes(endpoints map[string]HTTPEndpoint) (*HTTPEndpointRoutes, error) {
	if len(endpoints) == 0 || len(endpoints) > 64 {
		return nil, errors.New("HTTP endpoint enrollment must contain 1–64 models")
	}
	r := &HTTPEndpointRoutes{endpoints: make(map[string]HTTPEndpoint, len(endpoints))}
	keys := map[string]bool{}
	for id, endpoint := range endpoints {
		if id == "" || endpoint.Authorize == nil {
			return nil, errors.New("HTTP endpoint enrollment is incomplete")
		}
		if err := endpoint.Descriptor.Validate(); err != nil {
			return nil, err
		}
		client, err := endpointHTTPClient(endpoint)
		if err != nil {
			return nil, err
		}
		client.CloseIdleConnections()
		if endpoint.ManagedOllama != nil {
			if err := validateManagedOllama(endpoint); err != nil {
				return nil, err
			}
		}
		if !endpoint.Remote && endpoint.AcquireLocal == nil && endpoint.ManagedOllama == nil {
			return nil, errors.New("local HTTP endpoint requires a supervisor lease")
		}
		key := providerKey(endpoint.Descriptor)
		if keys[key] {
			return nil, errors.New("HTTP endpoint descriptor identity is not unique")
		}
		keys[key] = true
		endpoint.AllowedIPs = append([]string(nil), endpoint.AllowedIPs...)
		endpoint.ManagedOllama = cloneManagedOllama(endpoint.ManagedOllama)
		endpoint.Descriptor.Capabilities = append([]ProviderCapability(nil), endpoint.Descriptor.Capabilities...)
		r.endpoints[id] = endpoint
	}
	return r, nil
}

func (r *HTTPEndpointRoutes) Select(ctx context.Context, id, revision string) (ProviderDescriptor, error) {
	endpoint, ok := r.endpoints[id+"@"+revision]
	if !ok {
		return ProviderDescriptor{}, errors.New("HTTP model enrollment is unavailable")
	}
	if _, err := endpoint.Authorize(ctx); err != nil {
		return ProviderDescriptor{}, errors.New("HTTP model enrollment is unavailable or revoked")
	}
	d := endpoint.Descriptor
	d.Capabilities = append([]ProviderCapability(nil), d.Capabilities...)
	return d, nil
}

func (r *HTTPEndpointRoutes) Register(registry *ProviderRegistry) error {
	if registry == nil {
		return errors.New("HTTP provider registry is required")
	}
	providers := map[ProviderID]bool{}
	for _, endpoint := range r.endpoints {
		providers[endpoint.Descriptor.Provider] = true
	}
	for _, id := range []ProviderID{ProviderOllama, ProviderVLLM} {
		if !providers[id] {
			continue
		}
		provider := id
		if err := registry.Register(provider, func(ctx context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
			for _, endpoint := range r.endpoints {
				if endpoint.Descriptor.Provider == provider && reflect.DeepEqual(endpoint.Descriptor, descriptor) {
					runtime, err := NewHTTPProvider(ctx, endpoint)
					if err != nil {
						return nil, err
					}
					return runtime, nil
				}
			}
			return nil, errors.New("HTTP provider descriptor was not explicitly enrolled")
		}); err != nil {
			return fmt.Errorf("register HTTP provider: %w", err)
		}
	}
	return nil
}
