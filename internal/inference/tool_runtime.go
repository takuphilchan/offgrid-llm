package inference

// Public non-sensitive runtime metadata. A declaration is not a passed check.
type ToolRuntimeInfo struct {
	Build             string `json:"build"`
	TemplateSHA256    string `json:"template_sha256"`
	Context           int    `json:"context"`
	SupportsTools     *bool  `json:"supports_tools,omitempty"`
	SupportsToolCalls *bool  `json:"supports_tool_calls,omitempty"`
}
