package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// This ceiling contains metadata identities, never executable grants or schemas.
// It is independently retained when foreground admission clears AllowedCapabilities.
type ConversationDiscoveryScope struct {
	Version      int                                `json:"version"`
	Capabilities map[string]capabilities.Descriptor `json:"capabilities"`
}

// Description is provider prose, not an identity or permission. Do not retain
// an unbounded description in the admission ceiling or truncate identity fields.
func ConversationMetadataIdentity(descriptor capabilities.Descriptor) (capabilities.Descriptor, bool) {
	descriptor.Description = ""
	for _, value := range []string{descriptor.Name, descriptor.Namespace, descriptor.Source, descriptor.Revision, descriptor.Location, string(descriptor.Kind), string(descriptor.Risk)} {
		if !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") {
			return descriptor, false
		}
	}
	encoded, err := json.Marshal(descriptor)
	return descriptor, err == nil && len(encoded) <= 768 && descriptor.Name != "" && descriptor.Source != ""
}

func conversationMetadataDiscovery(task *Task) bool {
	return task.ConversationTurn != nil && task.ParentID == "" && !task.Config.TextOnlyInference &&
		task.Config.ConversationDiscovery != nil && task.Config.ConversationDiscovery.Version == 1 &&
		len(task.Config.ConversationDiscovery.Capabilities) > 0
}

func (r *Runner) discoverConversationTools(ctx context.Context, task *Task, query string, offset int) (string, error) {
	provider, ok := r.tools.(interface {
		ConversationToolMetadata(context.Context, *Task) []capabilities.Descriptor
	})
	var descriptors []capabilities.Descriptor
	if ok && qualifiedTool(task, "task_find_tools", capabilities.Descriptor{}, false) {
		descriptors = provider.ConversationToolMetadata(ctx, task)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Check the durable ceiling again even if an adapter returns an overbroad
	// inventory. Never copy metadata into the executable discovery checkpoint.
	tools := []api.Tool{}
	identities := map[string]capabilities.Descriptor{}
	for _, descriptor := range descriptors {
		pinned, exists := task.Config.ConversationDiscovery.Capabilities[descriptor.Name]
		identity, valid := ConversationMetadataIdentity(descriptor)
		if !exists || pinned != identity || !valid || !utf8.ValidString(descriptor.Description) {
			continue
		}
		visible := descriptor
		visible.Description = boundedToolText(visible.Description, 256)
		encoded, err := json.Marshal(visible)
		if err != nil || len(encoded) > 1024 {
			continue
		}
		if _, duplicate := identities[descriptor.Name]; duplicate {
			continue
		}
		identities[descriptor.Name] = visible
		tools = append(tools, api.Tool{Function: api.FunctionDef{Name: descriptor.Name, Description: visible.Description}})
	}
	rankTools(tools, query)
	terms, exact := toolSearchTerms(query), false
	for _, tool := range tools {
		exact = exact || strings.EqualFold(tool.Function.Name, strings.TrimSpace(query))
	}
	matches := tools[:0]
	for _, tool := range tools {
		if exact && strings.EqualFold(tool.Function.Name, strings.TrimSpace(query)) || !exact && (len(terms) == 0 || toolRelevance(tool, terms) > 0) {
			matches = append(matches, tool)
		}
	}
	if offset > len(matches) {
		return "", fmt.Errorf("catalog changed or offset is invalid; search again from offset 0")
	}
	end := offset + min(discoveryPageSize, len(matches)-offset)
	type row struct {
		capabilities.Descriptor
		SchemaState string `json:"model_schema_state"`
	}
	rows := make([]row, 0, end-offset)
	for _, tool := range matches[offset:end] {
		rows = append(rows, row{identities[tool.Function.Name], "work_admission_required"})
	}
	var next *int
	if end < len(matches) {
		next = &end
	}
	encoded, err := json.Marshal(map[string]any{"tools": rows, "total_matches": len(matches), "next_offset": next,
		"metadata_is_untrusted": true, "execution_authorized": false, "metadata_only": true,
		"details": "These are capability descriptions, not callable schemas or proof that a host is connected. Foreground discovery grants nothing. Ongoing work requires a separate canonical work admission and current model, provider and host authorization."})
	return string(encoded), err
}
