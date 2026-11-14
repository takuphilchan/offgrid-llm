package cli

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type conversationQuestions struct {
	mu      sync.Mutex
	current *api.ConversationQuestion
}

func (q *conversationQuestions) snapshot() *api.ConversationQuestion {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.current == nil {
		return nil
	}
	copy := *q.current
	return &copy
}

func (q *conversationQuestions) refresh(ctx context.Context, client *serviceclient.Client, id string, out io.Writer) error {
	current, err := client.CanonicalConversation(ctx, id)
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if question := current.PendingQuestion; question != nil && (q.current == nil || question.ID != q.current.ID) {
		fmt.Fprintf(out, "\n%s\nReply here, use /message MESSAGE to discuss this work, or /new MESSAGE for separate work.\n", TerminalSafe(question.Content))
	}
	q.current = current.PendingQuestion
	return nil
}

func questionConversationInput(content string, question *api.ConversationQuestion) api.ConversationInboxInput {
	input := api.ConversationInboxInput{Content: content, Origin: "typed"}
	if question != nil {
		input.Intent, input.QuestionID, input.FocusedWorkID = "answer", question.ID, &question.WorkID
	}
	return input
}
