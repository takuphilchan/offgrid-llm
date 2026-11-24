package inference

import (
	"encoding/json"
	"errors"
	"fmt"
)

func validateProviderEvent(event ProviderEvent) error {
	if event.Descriptor != nil ||
		(event.Type != ProviderEventDelta && event.Text != "") ||
		(event.Type != ProviderEventEmbeddings && event.Embeddings != nil) ||
		(event.Type != ProviderEventTool && len(event.Tool) != 0) ||
		(event.Type != ProviderEventDone && event.FinishReason != "") ||
		(event.Type != ProviderEventError && event.Error != nil) ||
		(event.Type != ProviderEventUsage && event.Type != ProviderEventDone && event.Usage != nil) ||
		(event.Type != ProviderEventDone && event.Resources != nil) {
		return errors.New("provider generation event contains fields from another event type")
	}
	if event.Usage != nil {
		usage := event.Usage
		if usage.PromptTokens < 0 || usage.CompletionTokens < 0 || usage.TotalTokens < 0 || usage.TotalTokens != usage.PromptTokens+usage.CompletionTokens {
			return errors.New("provider token usage is inconsistent")
		}
	}
	switch event.Type {
	case ProviderEventEmbeddings:
		if err := validateONNXEmbeddingVectors(event.Embeddings); err != nil {
			return err
		}
	case ProviderEventDelta:
		if event.Text == "" {
			return errors.New("provider delta cannot be empty")
		}
	case ProviderEventTool:
		if len(event.Tool) == 0 || !json.Valid(event.Tool) {
			return errors.New("provider tool decision must be valid JSON")
		}
	case ProviderEventUsage:
		if event.Usage == nil {
			return errors.New("provider usage event has no token counts")
		}
	case ProviderEventDone:
		if event.FinishReason != "stop" && event.FinishReason != "length" && event.FinishReason != "cancelled" && event.FinishReason != "tool_calls" {
			return errors.New("provider terminal event has no supported finish reason")
		}
	case ProviderEventError:
		if event.Error == nil || event.Error.Code == "" || event.Error.Message == "" {
			return errors.New("provider error event is incomplete")
		}
	default:
		return fmt.Errorf("provider emitted unsupported event type %q", event.Type)
	}
	return nil
}
