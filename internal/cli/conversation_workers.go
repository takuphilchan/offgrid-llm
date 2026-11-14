package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type conversationWorkerControls struct {
	pending *api.WorkerCommand
}

func (controls *conversationWorkerControls) handle(ctx context.Context, client *serviceclient.Client, conversation, command, arg string, out io.Writer) (bool, error) {
	switch command {
	case "/workers", "/worker", "/message-worker", "/answer-worker", "/continue-worker", "/stop-worker", "/close-worker", "/retry-worker":
	default:
		return false, nil
	}
	if command == "/workers" {
		workers, err := focusedConversationWorkers(ctx, client, conversation)
		if err != nil {
			return true, err
		}
		return true, printWorkerJSON(out, workers)
	}
	if command == "/retry-worker" {
		if controls.pending == nil {
			return true, Usage("no pending worker command")
		}
		return true, controls.dispatch(ctx, client, out)
	}
	id, content, _ := strings.Cut(arg, " ")
	content = strings.TrimSpace(content)
	if id == "" {
		return true, Usage("%s WORKER_ID", command)
	}
	kind := strings.TrimSuffix(strings.TrimPrefix(command, "/"), "-worker")
	questionID := ""
	if kind == "answer" {
		questionID, content, _ = strings.Cut(content, " ")
		content = strings.TrimSpace(content)
		if questionID == "" || content == "" {
			return true, Usage("/answer-worker WORKER_ID QUESTION_ID ANSWER")
		}
	}
	if (kind == "message" || kind == "continue" || kind == "answer") && content == "" {
		return true, Usage("%s WORKER_ID MESSAGE", command)
	}
	if kind != "message" && kind != "continue" && kind != "answer" && content != "" {
		return true, Usage("%s WORKER_ID", command)
	}
	if command != "/worker" && controls.pending != nil && controls.pending.TargetID == id && controls.pending.Kind == kind && controls.pending.Payload.Content == content && controls.pending.Payload.QuestionID == questionID {
		return true, controls.dispatch(ctx, client, out)
	}
	worker, err := client.Worker(ctx, id)
	if err != nil {
		return true, err
	}
	current, err := client.CanonicalConversation(ctx, conversation)
	if err != nil {
		return true, err
	}
	if current.FocusedWorkID == "" || worker.WorkID != current.FocusedWorkID {
		return true, Usage("this worker does not belong to the focused work; use /workers")
	}
	if command == "/worker" {
		if err = printWorkerJSON(out, worker); err != nil {
			return true, err
		}
		for after := int64(0); ; {
			page, err := client.WorkerMessages(ctx, id, after, 100)
			if err != nil {
				return true, err
			}
			if err = printWorkerJSON(out, page); err != nil {
				return true, err
			}
			if !page.HasMore {
				return true, nil
			}
			if page.NextCursor <= after {
				return true, fmt.Errorf("invalid worker message cursor")
			}
			after = page.NextCursor
		}
	}
	controls.pending = &api.WorkerCommand{RequestID: rand.Text(), TargetID: id, ExpectedRevision: worker.Revision, Kind: kind, Payload: api.WorkerCommandPayload{Content: content, QuestionID: questionID}}
	return true, controls.dispatch(ctx, client, out)
}

func (controls *conversationWorkerControls) dispatch(ctx context.Context, client *serviceclient.Client, out io.Writer) error {
	receipt, err := client.WorkerCommand(ctx, *controls.pending)
	if err != nil {
		var remote *serviceclient.Error
		if errors.As(err, &remote) && remote.Status == http.StatusConflict {
			controls.pending = nil
		} else {
			fmt.Fprintf(out, "Worker command retained. /retry-worker uses request %s.\n", TerminalSafe(controls.pending.RequestID))
		}
		return err
	}
	controls.pending = nil
	return printWorkerJSON(out, receipt)
}

func printWorkerJSON(out io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, TerminalSafe(string(data)))
	return err
}

func focusedConversationWorkers(ctx context.Context, client *serviceclient.Client, conversation string) ([]api.WorkerSession, error) {
	c, err := client.CanonicalConversation(ctx, conversation)
	if err != nil {
		return nil, err
	}
	workers := []api.WorkerSession{}
	if c.FocusedWorkID == "" {
		return workers, nil
	}
	work, err := client.WorkItem(ctx, c.FocusedWorkID)
	if err != nil {
		return nil, err
	}
	runs := work.RunIDs
	if len(runs) == 0 {
		runs = []string{work.RunID}
	}
	seen := map[string]bool{}
	for _, id := range runs {
		var run struct {
			Children []struct {
				WorkerID string `json:"worker_id"`
			} `json:"children"`
		}
		if err = client.JSON(ctx, http.MethodGet, "/api/v2/jobs/"+url.PathEscape(id), nil, &run); err != nil {
			return nil, err
		}
		for _, child := range run.Children {
			if child.WorkerID == "" || seen[child.WorkerID] {
				continue
			}
			seen[child.WorkerID] = true
			worker, err := client.Worker(ctx, child.WorkerID)
			if err != nil {
				return nil, err
			}
			if worker.WorkID != work.ID {
				return nil, fmt.Errorf("worker does not belong to the focused work")
			}
			workers = append(workers, *worker)
		}
	}
	return workers, nil
}
