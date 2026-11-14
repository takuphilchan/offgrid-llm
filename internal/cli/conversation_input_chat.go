package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type conversationOutput struct {
	mu  sync.Mutex
	out io.Writer
}

func (w *conversationOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.out.Write(p)
}

// Reading the next line is independent of inference and observation. Exiting
// cancels only this observer, never service-owned work or its authority.
func RunCanonicalConversationChat(ctx context.Context, client *serviceclient.Client, saved *sessions.Session, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := &conversationOutput{out: out}
	fmt.Fprintf(w, "Conversation: %s\n/exit detaches. /help lists controls.\n", TerminalSafe(saved.Name))
	questions := &conversationQuestions{}
	if err := questions.refresh(ctx, client, saved.CanonicalID, w); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() { defer close(done); observeCanonicalConversation(ctx, client, saved, w, questions) }()
	defer func() { cancel(); <-done }()
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	var pending *api.ConversationInboxInput
	requestID := ""
	workerControls := &conversationWorkerControls{}
	folderControl := &conversationFolderControl{}
	for {
		question := questions.snapshot()
		fmt.Fprint(w, "\n> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		command, arg, _ := strings.Cut(text, " ")
		arg = strings.TrimSpace(arg)
		if command == "/exit" || command == "/quit" {
			return nil
		}
		if command == "/help" {
			fmt.Fprintln(w, "/result RESULT_REFERENCE retrieves complete saved output")
			fmt.Fprintln(w, "/folder SESSION|none selects an already approved folder and remembers its project")
			fmt.Fprintln(w, "/retry-folder retries a folder choice whose response was lost")
			fmt.Fprintln(w, "/changes [CHANGE_ID [files|validation|integration|recovery|content|preserved [SHA256 [ARTIFACT_ID]]]]")
			fmt.Fprintln(w, "/status · /configuration · /new MESSAGE · /pause · /stop · /continue · /stop-input ID · /continue-input ID · /model MODEL · /profile ID [REVISION]|none · /project ID · /retry · /exit")
			fmt.Fprintln(w, "/workers | /worker ID | /message-worker ID MESSAGE | /continue-worker ID MESSAGE | /stop-worker ID | /close-worker ID | /retry-worker")
			fmt.Fprintln(w, "/answer-worker ID QUESTION_ID ANSWER answers an inspected worker question")
			fmt.Fprintln(w, "/message MESSAGE discusses focused work without answering its pending question")
			continue
		}
		if command == "/changes" {
			if err := inspectConversationChanges(ctx, client, saved.CanonicalID, strings.Fields(arg), w, false); err != nil {
				fmt.Fprintln(w, TerminalSafe(err.Error()))
			}
			continue
		}
		if command == "/result" {
			if err := inspectSavedResult(ctx, client, arg, w, false); err != nil {
				fmt.Fprintln(w, TerminalSafe(err.Error()))
			}
			continue
		}
		if handled, err := folderControl.handle(ctx, client, saved.CanonicalID, command, arg, w); handled {
			if err != nil {
				fmt.Fprintln(w, TerminalSafe(err.Error()))
			}
			continue
		}
		if handled, err := workerControls.handle(ctx, client, saved.CanonicalID, command, arg, w); handled {
			if err != nil {
				fmt.Fprintln(w, TerminalSafe(err.Error()))
			}
			continue
		}
		if command == "/status" || command == "/configuration" {
			var value any
			var err error
			if command == "/configuration" {
				value, err = client.ConversationConfiguration(ctx, saved.CanonicalID)
			} else {
				value, err = client.CanonicalConversation(ctx, saved.CanonicalID)
			}
			if err != nil {
				fmt.Fprintln(w, TerminalSafe(err.Error()))
			} else {
				data, _ := json.MarshalIndent(value, "", "  ")
				fmt.Fprintln(w, TerminalSafe(string(data)))
			}
			continue
		}
		if strings.HasPrefix(command, "/") && command != "/new" && command != "/message" && command != "/retry" {
			if err := canonicalChatControl(ctx, client, saved.CanonicalID, command, arg); err != nil {
				fmt.Fprintln(w, TerminalSafe(err.Error()))
			} else {
				fmt.Fprintln(w, "Control accepted.")
			}
			continue
		}
		if command == "/retry" {
			if pending == nil {
				fmt.Fprintln(w, "No pending input to retry.")
				continue
			}
		} else {
			if pending != nil && pending.Intent == "answer" {
				fmt.Fprintln(w, "The previous answer has an unknown outcome. Use /retry to recover it before sending another answer.")
				continue
			}
			input := questionConversationInput(text, question)
			if command == "/new" || command == "/message" {
				if arg == "" {
					fmt.Fprintln(w, command+" MESSAGE")
					continue
				}
				input.Content, input.Intent = arg, "new_work"
				input.QuestionID, input.FocusedWorkID = "", nil
				if command == "/message" {
					input.Intent = "message"
					if question != nil {
						input.FocusedWorkID = &question.WorkID
					}
				}
			}
			pending = &input
			requestID = rand.Text()
		}
		entry, err := client.SubmitSessionInput(ctx, saved.CanonicalID, requestID, *pending)
		if err != nil {
			fmt.Fprintf(w, "%s\nInput retained. /retry uses request %s.\n", TerminalSafe(err.Error()), requestID)
			continue
		}
		fmt.Fprintf(w, "Accepted input %s\n", TerminalSafe(entry.ID))
		pending = nil
	}
}

func observeCanonicalConversation(ctx context.Context, client *serviceclient.Client, saved *sessions.Session, out io.Writer, questions *conversationQuestions) {
	seen := map[string]bool{}
	key := func(message sessions.Message) string { data, _ := json.Marshal(message); return string(data) }
	for _, message := range saved.Messages {
		seen[key(message)] = true
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastError, lastState := "", ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		current, err := LoadServiceSession(ctx, client, saved.Name)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if err.Error() != lastError {
				fmt.Fprintln(out, TerminalSafe(err.Error()))
				lastError = err.Error()
			}
			continue
		}
		lastError = ""
		if err := questions.refresh(ctx, client, saved.CanonicalID, out); err != nil {
			fmt.Fprintln(out, TerminalSafe(err.Error()))
		}
		for _, message := range current.Messages {
			identity := key(message)
			if !seen[identity] && message.Role == "assistant" {
				fmt.Fprintf(out, "\n%s\n", TerminalSafe(message.Content))
			}
			seen[identity] = true
		}
		if current.Turn != nil {
			state := current.Turn.JobID + ":" + current.Turn.Status
			if state != lastState {
				fmt.Fprintf(out, "[%s]\n", TerminalSafe(current.Turn.Status))
				lastState = state
			}
		}
	}
}
