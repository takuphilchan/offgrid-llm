package inference

import "context"

type generationGuardKey struct{}

// WithGenerationGuard carries a service-owned, request-local access check.
// It is never populated from model output or serialized provider arguments.
// Guards compose so adding a source check cannot replace an existing boundary.
func WithGenerationGuard(ctx context.Context, check func() error) context.Context {
	previous, _ := ctx.Value(generationGuardKey{}).(func() error)
	return context.WithValue(ctx, generationGuardKey{}, func() error {
		if previous != nil {
			if err := previous(); err != nil {
				return err
			}
		}
		if check != nil {
			return check()
		}
		return nil
	})
}

// CheckGenerationGuard checks cancellation and current authority at a dispatch
// or delivery boundary. Already delivered output cannot be recalled.
func CheckGenerationGuard(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if check, ok := ctx.Value(generationGuardKey{}).(func() error); ok {
		return check()
	}
	return nil
}

func generateWithGuard(ctx context.Context, runtime ProviderRuntime, request ProviderRequest, emit func(ProviderEvent) error) error {
	if err := CheckGenerationGuard(ctx); err != nil {
		return err
	}
	err := runtime.Generate(ctx, request, func(event ProviderEvent) error {
		if err := CheckGenerationGuard(ctx); err != nil {
			return err
		}
		return emit(event)
	})
	if err != nil {
		return err
	}
	return CheckGenerationGuard(ctx)
}
