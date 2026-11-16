package inference

import "context"

type boundedDocumentGenerationKey struct{}

// Document sections are explicitly retried by their durable job owner. Limit
// transport response allocation and avoid silently repeating a failed request.
func WithBoundedDocumentGeneration(ctx context.Context) context.Context {
	return context.WithValue(ctx, boundedDocumentGenerationKey{}, true)
}
func boundedDocumentGeneration(ctx context.Context) bool {
	value, _ := ctx.Value(boundedDocumentGenerationKey{}).(bool)
	return value
}
