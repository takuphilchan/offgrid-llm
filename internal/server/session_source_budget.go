package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type chatSourcesKey struct{}
type chatSources struct {
	sources []sessions.AttachmentText
	turn    *sessions.Turn
}

func withChatSources(ctx context.Context, sources []sessions.AttachmentText, turn *sessions.Turn) context.Context {
	return context.WithValue(ctx, chatSourcesKey{}, chatSources{sources, turn})
}

func sourceBudgetError(window, response int) error {
	return newCodedServiceError(422, "attachment_context_limit", fmt.Sprintf("This turn cannot fit the conversation, document excerpts and the %d-token response limit in the %d-token context. Lower Maximum response tokens or select Extended in Response settings if supported. Your history, files and draft are retained.", response, window), nil)
}

// Called inside the generation lease, after Knowledge enrichment. Count the
// complete model template, including history and images, with the same resident
// model used for generation. Byte budgets below select candidates, not tokens.
func prepareChatSources(ctx context.Context, request *api.ChatCompletionRequest, window int, counter inference.PromptTokenCounter) error {
	scope, _ := ctx.Value(chatSourcesKey{}).(chatSources)
	if len(scope.sources) == 0 {
		return nil
	}
	response := 512
	if request.MaxTokens != nil {
		response = max(response, *request.MaxTokens)
	}
	limit := window - response - 64
	if limit <= 0 {
		return sourceBudgetError(window, response)
	}
	base := *request
	base.Messages = append([]api.ChatMessage(nil), request.Messages...)
	last := len(base.Messages) - 1
	if last < 0 || base.Messages[last].Role != "user" {
		return newServiceError(422, "Document context requires a current user question", nil)
	}
	baseTokens := 0
	if counter != nil {
		var err error
		baseTokens, err = counter.CountPromptTokens(ctx, &base)
		if errors.Is(err, inference.ErrPromptCountUnavailable) {
			counter = nil
		} else if err != nil {
			return newCodedServiceError(503, "context_count_unavailable", "The local runtime could not measure this document question. Your draft is retained; retry when the runtime is ready.", err)
		}
	}
	budget := 0
	if counter == nil {
		// Older/non-llama providers cannot claim measured usage. Preserve the
		// conservative fallback; generation still enforces actual admission.
		budget = window - response - 1024
		for _, message := range base.Messages {
			budget -= len(message.StringContent()) + 32
		}
	} else {
		budget = min(sessions.MaxAttachmentContextBytes, max(0, limit-baseTokens)*3)
	}
	minimum := len(scope.sources) * 600
	for attempt := 0; attempt < 6 && budget >= minimum; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		selected, err := sessions.SelectAttachmentText(scope.sources, scope.turn.Prompt, budget)
		if err != nil {
			return sourceBudgetError(window, response)
		}
		candidate := base
		candidate.Messages = append([]api.ChatMessage(nil), base.Messages...)
		// Keep any authorized Knowledge enrichment in the current message.
		candidate.Messages[last].Content = api.SessionContent(sessions.AttachmentPrompt(base.Messages[last].StringContent(), selected), scope.turn.Images)
		if counter != nil {
			count, err := counter.CountPromptTokens(ctx, &candidate)
			if err != nil {
				return newCodedServiceError(503, "context_count_unavailable", "The local runtime could not measure the selected document excerpts. Your draft is retained.", err)
			}
			if count > limit {
				if budget == minimum || baseTokens >= limit {
					break
				}
				// Bounded read-only reselection, never a generation retry. Shrink
				// for actual tokenizer density; multilingual text is not bytes/4.
				next := budget * max(0, limit-baseTokens) / max(1, count-baseTokens) * 9 / 10
				budget = max(minimum, min(budget-1, next))
				continue
			}
		}
		request.Messages = candidate.Messages
		scope.turn.AttachmentSelections = sessions.AttachmentSelections(selected)
		return nil
	}
	return sourceBudgetError(window, response)
}

// Conservative assembly for custom streamers without runtime token counting.
func attachmentMessages(scope *sessions.ScopedManager, name string, session *sessions.Session, turn *sessions.Turn, window int) ([]api.ChatMessage, error) {
	messages, sources, err := collectAttachmentMessages(scope, name, session, turn)
	if err != nil {
		return nil, err
	}
	request := &api.ChatCompletionRequest{Messages: messages, MaxTokens: &turn.MaxTokens}
	err = prepareChatSources(withChatSources(context.Background(), sources, turn), request, window, nil)
	return request.Messages, err
}
