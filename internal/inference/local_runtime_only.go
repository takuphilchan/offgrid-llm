package inference

import "context"

type localRuntimeOnlyKey struct{}

// WithLocalRuntimeOnly forbids implicit runtime acquisition for local document
// processing. It is internal service policy, not a client-supplied model option.
func WithLocalRuntimeOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, localRuntimeOnlyKey{}, true)
}

func localRuntimeOnly(ctx context.Context) bool {
	value, _ := ctx.Value(localRuntimeOnlyKey{}).(bool)
	return value
}
