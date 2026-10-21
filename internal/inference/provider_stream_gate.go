package inference

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// providerStreamGate validates the complete shared provider boundary. Text
// deltas may be displayed provisionally, but tool decisions and terminal
// success remain withheld until Generate itself has returned successfully.
// This prevents a provider that emits a plausible call and then crashes from
// dispatching that call before its outcome is known.
type providerStreamGate struct {
	descriptor ProviderDescriptor
	request    ProviderRequest
	callback   func(ProviderEvent) error
	sequence   uint64
	tool       *ProviderEvent
	done       *ProviderEvent
	sawText    bool
	usageSeen  bool
	textBytes  int
	failure    error
}

func (g *providerStreamGate) accept(event ProviderEvent) error {
	if g.failure != nil {
		return g.failure
	}
	err := g.acceptEvent(event)
	if err != nil {
		g.failure = err
	}
	return err
}

func (g *providerStreamGate) acceptEvent(event ProviderEvent) error {
	if g.done != nil || event.ProtocolRevision != g.descriptor.ProtocolRevision || event.RequestID != g.request.RequestID || event.Sequence <= g.sequence {
		return errors.New("provider stream identity, sequence, or terminal boundary changed")
	}
	if err := validateProviderEvent(event); err != nil {
		return err
	}
	if event.Usage != nil && g.request.ContextLength > 0 && event.Usage.TotalTokens > g.request.ContextLength {
		return errors.New("provider usage exceeds the admitted request context")
	}
	if event.Usage != nil && g.request.MaxOutputTokens > 0 && event.Usage.CompletionTokens > g.request.MaxOutputTokens {
		return errors.New("provider usage exceeds the admitted output token budget")
	}
	if event.Usage != nil {
		if g.usageSeen {
			return errors.New("provider reported token usage more than once")
		}
		g.usageSeen = true
	}
	if event.Resources != nil {
		if event.Type != ProviderEventDone {
			return errors.New("provider resources were reported before completion")
		}
		if err := event.Resources.Validate(g.descriptor); err != nil {
			return err
		}
	}
	g.sequence = event.Sequence
	switch event.Type {
	case ProviderEventDelta:
		if g.usageSeen {
			return errors.New("provider emitted answer text after final token usage")
		}
		if g.tool != nil {
			return errors.New("provider mixed a tool decision with answer text")
		}
		limit := g.request.MaxOutputBytes
		if limit == 0 {
			limit = 8 << 20
		}
		if len(event.Text) > limit-g.textBytes {
			return errors.New("provider text exceeds the admitted output byte budget")
		}
		g.textBytes += len(event.Text)
		g.sawText = true
		return g.callback(event)
	case ProviderEventTool:
		if g.usageSeen {
			return errors.New("provider emitted a tool decision after final token usage")
		}
		if g.sawText || g.tool != nil || !slices.Contains(g.descriptor.Capabilities, CapabilityToolCalls) {
			return errors.New("provider emitted an unsupported or mixed tool decision")
		}
		if err := validateScheduledTool(g.request, event.Tool); err != nil {
			return err
		}
		copy := event
		g.tool = &copy
		return nil
	case ProviderEventUsage:
		return g.callback(event)
	case ProviderEventDone:
		if event.FinishReason == "cancelled" {
			return errors.New("provider reported cancellation instead of successful completion")
		}
		if g.tool != nil && event.FinishReason != "tool_calls" || g.tool == nil && event.FinishReason == "tool_calls" {
			return errors.New("provider terminal reason does not match its decision")
		}
		copy := event
		g.done = &copy
		return nil
	case ProviderEventError:
		return fmt.Errorf("provider %s: %s", event.Error.Code, event.Error.Message)
	default:
		return errors.New("provider emitted a non-generation event")
	}
}

func (g *providerStreamGate) finish() error {
	if g.failure != nil {
		return g.failure
	}
	if g.done == nil {
		return errors.New("provider generation ended without a terminal event")
	}
	if g.request.MaxOutputTokens > 0 && !g.usageSeen {
		return errors.New("provider completed without output budget evidence")
	}
	if g.tool != nil {
		if err := g.callback(*g.tool); err != nil {
			return err
		}
	}
	return g.callback(*g.done)
}

func validateScheduledTool(request ProviderRequest, raw json.RawMessage) error {
	if len(request.Tools) == 0 || len(raw) > 1<<20 || jsonwire.Validate(request.Tools) != nil || jsonwire.Validate(request.Prompt) != nil || jsonwire.Validate(raw) != nil {
		return errors.New("provider tool decision lacks an unambiguous offered catalog")
	}
	var offered []api.Tool
	var prompt api.ChatCompletionRequest
	var calls []api.ToolCall
	if json.Unmarshal(request.Tools, &offered) != nil || json.Unmarshal(request.Prompt, &prompt) != nil || json.Unmarshal(raw, &calls) != nil || !sameTools(offered, prompt.Tools) {
		return errors.New("provider tool decision differs from the offered catalog")
	}
	response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Index: 0, FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: calls}}}}
	return ValidateToolDecision(&prompt, response)
}
