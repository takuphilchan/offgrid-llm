//go:build !llama

package inference

// constrainSingleStepStrings operates on the private generation-schema copy.
// The pinned runtime translates regex patterns over decoded strings directly
// into grammar characters, without intersecting them with JSON string syntax.
// A filename pattern such as ^[^/\\:]+\.txt$ can therefore consume the closing
// quote, the rest of the response and subsequent prose as filename characters.
// Use its JSON-aware string grammar instead. The original catalog remains in
// the prompt and ValidateToolDecision still enforces every original constraint
// after clean termination, before a decision can reach execution.
func constrainSingleStepStrings(schema map[string]any) {
	delete(schema, "pattern")
	// Traverse schema-bearing keywords only: const/enum/default/examples are
	// data and can legitimately contain a property named "pattern".
	for _, key := range []string{"properties", "patternProperties", "$defs", "definitions", "dependentSchemas"} {
		if children, ok := schema[key].(map[string]any); ok {
			for _, child := range children {
				if nested, ok := child.(map[string]any); ok {
					constrainSingleStepStrings(nested)
				}
			}
		}
	}
	for _, key := range []string{"items", "additionalProperties", "additionalItems", "contains", "propertyNames", "not", "if", "then", "else", "unevaluatedProperties", "unevaluatedItems"} {
		if nested, ok := schema[key].(map[string]any); ok {
			constrainSingleStepStrings(nested)
		}
	}
	for _, key := range []string{"oneOf", "anyOf", "allOf", "prefixItems", "items"} {
		if children, ok := schema[key].([]any); ok {
			for _, child := range children {
				if nested, ok := child.(map[string]any); ok {
					constrainSingleStepStrings(nested)
				}
			}
		}
	}
}
