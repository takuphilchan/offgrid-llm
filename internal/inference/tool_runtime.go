package inference

import (
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"os"
)

// ToolRuntimeArtifacts are opened by the trusted installation owner, never
// from model/tool arguments. See MeasureToolRuntimeArtifacts for lease rules.
type ToolRuntimeArtifacts struct {
	Measurements *models.CapabilityMeasurements
	Model        *os.File
	Projector    *os.File
	Runtime      map[string]*os.File
}

type MeasuredToolRuntime struct {
	Observation     ToolRuntimeObservation
	ModelSHA256     string
	ProjectorSHA256 string
	RuntimeSHA256   string
}

// Public non-sensitive runtime metadata. A declaration is not a passed check.
type ToolRuntimeInfo struct {
	Build             string `json:"build"`
	TemplateSHA256    string `json:"template_sha256"`
	TemplateAdapter   string `json:"template_adapter,omitempty"`
	Context           int    `json:"context"`
	SupportsTools     *bool  `json:"supports_tools,omitempty"`
	SupportsToolCalls *bool  `json:"supports_tool_calls,omitempty"`
}

// ToolRuntimeObservation binds public runtime metadata to one cache-owned
// process generation. It is a measurement, not workflow qualification. The
// caller must retain its model lease through subsequent inference.
type ToolRuntimeObservation struct {
	ModelID      string
	Generation   uint64
	ConfigSHA256 string
	Info         ToolRuntimeInfo
	// Private installation paths, never projected through the HTTP API. They
	// bind file measurements to the selected owned process, not model text.
	ExecutablePath string
	ModelPath      string
	ProjectorPath  string
}
