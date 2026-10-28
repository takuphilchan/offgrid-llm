package inference

import "context"

type decisionPreviewKey struct{}

// WithDecisionPreview attaches a presentation-only observer. Draft text is not
// a normalized decision, completion, or dispatch signal. Public streaming and
// the caller's final response still require complete validation.
func WithDecisionPreview(ctx context.Context, preview func(string) error) context.Context {
	return context.WithValue(ctx, decisionPreviewKey{}, preview)
}

func decisionPreview(ctx context.Context) func(string) error {
	preview, _ := ctx.Value(decisionPreviewKey{}).(func(string) error)
	return preview
}
