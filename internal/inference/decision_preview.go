package inference

import "context"

type decisionPreviewKey struct{}
type toolDecisionActivityKey struct{}

// WithToolDecisionActivity reports that the owned envelope has begun a tool
// proposal. It carries no name, arguments, authority or executable decision.
func WithToolDecisionActivity(ctx context.Context, activity func() error) context.Context {
	return context.WithValue(ctx, toolDecisionActivityKey{}, activity)
}

func toolDecisionActivity(ctx context.Context) func() error {
	activity, _ := ctx.Value(toolDecisionActivityKey{}).(func() error)
	return activity
}

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
